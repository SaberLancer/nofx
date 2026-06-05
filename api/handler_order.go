package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/logger"
	"nofx/market"
	"nofx/store"
	tradpkg "nofx/trader"

	"github.com/gin-gonic/gin"
)

// handleTraderList Trader list
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get trader list", err)
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// Get real-time running status
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// Get strategy name if strategy_id is set
		var strategyName string
		if trader.StrategyID != "" {
			if strategy, err := s.store.Strategy().Get(userID, trader.StrategyID); err == nil {
				strategyName = strategy.Name
			}
		}

		// Return complete AIModelID (e.g. "admin_deepseek"), don't truncate
		// Frontend needs complete ID to verify model exists (consistent with handleGetTraderConfig)
		result = append(result, map[string]interface{}{
			"trader_id":           trader.ID,
			"trader_name":         trader.Name,
			"ai_model":            trader.AIModelID, // Use complete ID
			"exchange_id":         trader.ExchangeID,
			"is_running":          isRunning,
			"show_in_competition": trader.ShowInCompetition,
			"initial_balance":     trader.InitialBalance,
			"strategy_id":         trader.StrategyID,
			"strategy_name":       strategyName,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig Get trader detailed configuration
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	fullCfg, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader config")
		return
	}
	traderConfig := fullCfg.Trader

	// Get real-time running status
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// Return complete model ID without conversion, consistent with frontend model list
	aiModelID := traderConfig.AIModelID

	result := map[string]interface{}{
		"trader_id":             traderConfig.ID,
		"trader_name":           traderConfig.Name,
		"ai_model":              aiModelID,
		"exchange_id":           traderConfig.ExchangeID,
		"strategy_id":           traderConfig.StrategyID,
		"initial_balance":       traderConfig.InitialBalance,
		"scan_interval_minutes": traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":      traderConfig.BTCETHLeverage,
		"altcoin_leverage":      traderConfig.AltcoinLeverage,
		"trading_symbols":       traderConfig.TradingSymbols,
		"custom_prompt":         traderConfig.CustomPrompt,
		"override_base_prompt":  traderConfig.OverrideBasePrompt,
		"is_cross_margin":       traderConfig.IsCrossMargin,
		"use_ai500":             traderConfig.UseAI500,
		"use_oi_top":            traderConfig.UseOITop,
		"is_running":            isRunning,
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus System status
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount Account information
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	logger.Infof("📊 Received account snapshot request [%s]", trader.GetName())
	snapshot, err := trader.GetTraderSnapshot()
	if err != nil {
		SafeInternalError(c, "Get account snapshot", err)
		return
	}

	positionCount := 0
	if positions, ok := snapshot["positions"].([]map[string]interface{}); ok {
		positionCount = len(positions)
	}
	logger.Infof("✓ Returning account snapshot [%s]: equity=%.2f, available=%.2f, pnl=%.2f (%.2f%%), positions=%d",
		trader.GetName(),
		snapshot["total_equity"],
		snapshot["available_balance"],
		snapshot["total_pnl"],
		snapshot["total_pnl_pct"],
		positionCount)
	c.JSON(http.StatusOK, snapshot)
}

// handlePositions Position list
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		SafeInternalError(c, "Get positions", err)
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handlePositionHistory Historical closed positions with statistics
func (s *Server) handlePositionHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	at, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}

	// OKX: always use exchange positions-history (authoritative closed positions).
	if strings.EqualFold(at.GetExchange(), "okx") {
		underlying := at.GetUnderlyingTrader()
		if underlying == nil {
			SafeInternalError(c, "Fetch OKX position history", fmt.Errorf("OKX trader not initialized"))
			return
		}

		days := 90
		if d, err := strconv.Atoi(c.DefaultQuery("days", "90")); err == nil && d > 0 && d <= 365 {
			days = d
		}
		start := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

		// limit=0: pull full window from OKX (partial + full close rows stay separate).
		records, err := underlying.GetClosedPnL(start, 0)
		if err != nil {
			SafeInternalError(c, "Fetch OKX position history", err)
			return
		}
		positions := store.ClosedPnLRecordsToTraderPositions(
			at.GetID(), at.GetExchangeID(), at.GetExchange(), toStoreClosedRecords(records),
		)
		logger.Infof("📜 OKX position history API [%s]: %d records (days=%d)", traderID, len(positions), days)
		sortPositionsByExitTimeDesc(positions)
		stats := store.ComputeFullStatsFromPositions(positions)
		symbolStats := store.ComputeSymbolStatsFromPositions(positions, 10)
		directionStats := store.ComputeDirectionStatsFromPositions(positions)
		c.JSON(http.StatusOK, gin.H{
			"positions":       positionsToAPIJSON(positions),
			"stats":           stats,
			"symbol_stats":    symbolStats,
			"direction_stats": directionStats,
			"source":          "okx_positions_history",
		})
		return
	}

	// Get store
	st := at.GetStore()
	if st == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Best-effort sync from exchange closed-position API (fills UI when DB missed OPEN records)
	if underlying := at.GetUnderlyingTrader(); underlying != nil {
		if err := tradpkg.SyncClosedPositionsFromExchange(
			underlying, at.GetID(), at.GetExchangeID(), at.GetExchange(),
			st, 30*24*time.Hour, 100,
		); err != nil {
			logger.Infof("⚠️ Closed position sync before history API: %v", err)
		}
	}

	// Get closed positions
	positions, err := st.Position().GetClosedPositions(at.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get position history", err)
		return
	}

	// Get statistics
	stats, _ := st.Position().GetFullStats(at.GetID())

	// Get symbol stats
	symbolStats, _ := st.Position().GetSymbolStats(at.GetID(), 10)

	// Get direction stats
	directionStats, _ := st.Position().GetDirectionStats(at.GetID())

	c.JSON(http.StatusOK, gin.H{
		"positions":       positionsToAPIJSON(positions),
		"stats":           stats,
		"symbol_stats":    symbolStats,
		"direction_stats": directionStats,
		"source":          "local_db",
	})
}

// handlePositionCloseOperations returns detailed reduce/close execution records for a historical position row.
func (s *Server) handlePositionCloseOperations(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	at, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	symbol := market.Normalize(c.Query("symbol"))
	side := c.Query("side")
	if symbol == "" || side == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol and side are required"})
		return
	}

	entryTimeMs, err := parseHistoryTimeToMs(c.Query("entry_time"))
	if err != nil {
		SafeBadRequest(c, "Invalid entry_time")
		return
	}
	exitTimeMs, err := parseHistoryTimeToMs(c.Query("exit_time"))
	if err != nil {
		SafeBadRequest(c, "Invalid exit_time")
		return
	}

	limit := 100
	if limitStr := c.DefaultQuery("limit", "100"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	if strings.EqualFold(at.GetExchange(), "okx") {
		underlying := at.GetUnderlyingTrader()
		if underlying == nil {
			SafeInternalError(c, "Get position close operations", fmt.Errorf("OKX trader not initialized"))
			return
		}
		provider, ok := underlying.(tradpkg.PositionCloseOrdersProvider)
		if !ok {
			SafeInternalError(c, "Get position close operations", fmt.Errorf("OKX close orders provider unavailable"))
			return
		}

		entryTime := time.UnixMilli(entryTimeMs).UTC()
		exitTime := time.UnixMilli(exitTimeMs).UTC()
		records, err := provider.GetPositionCloseOrders(symbol, side, entryTime, exitTime, limit)
		if err != nil {
			SafeInternalError(c, "Fetch OKX position close orders", err)
			return
		}

		out := make([]map[string]interface{}, 0, len(records))
		for i, r := range records {
			out = append(out, map[string]interface{}{
				"id":                i + 1,
				"exchange_order_id": r.ExchangeOrderID,
				"order_action":      r.OrderAction,
				"position_side":     r.PositionSide,
				"status":            r.Status,
				"exec_quantity":     r.ExecQuantity,
				"exec_price":        r.ExecPrice,
				"avg_fill_price":    r.ExecPrice,
				"fee":               r.Fee,
				"realized_pnl":      r.RealizedPnL,
				"filled_at":         r.FilledAt.UTC().Format(time.RFC3339),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"operations": out,
			"count":      len(out),
			"symbol":     symbol,
			"side":       side,
			"source":     "okx_fills_history",
		})
		return
	}

	st := at.GetStore()
	if st == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	orders, err := st.Order().GetCloseOperationsByWindow(at.GetID(), symbol, side, entryTimeMs, exitTimeMs, limit)
	if err != nil {
		SafeInternalError(c, "Get position close operations", err)
		return
	}

	out := make([]map[string]interface{}, 0, len(orders))
	for _, o := range orders {
		if o == nil {
			continue
		}
		execQty := o.FilledQuantity
		if execQty <= 0 {
			execQty = o.Quantity
		}
		execPrice := o.AvgFillPrice
		if execPrice <= 0 {
			execPrice = o.Price
		}
		out = append(out, map[string]interface{}{
			"id":                o.ID,
			"exchange_order_id": o.ExchangeOrderID,
			"order_action":      o.OrderAction,
			"position_side":     o.PositionSide,
			"side":              o.Side,
			"status":            o.Status,
			"quantity":          o.Quantity,
			"filled_quantity":   o.FilledQuantity,
			"exec_quantity":     execQty,
			"price":             o.Price,
			"avg_fill_price":    o.AvgFillPrice,
			"exec_price":        execPrice,
			"created_at":        msToRFC3339(o.CreatedAt),
			"filled_at":         msToRFC3339(o.FilledAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"operations": out,
		"count":      len(out),
		"symbol":     symbol,
		"side":       side,
		"source":     "local_db",
	})
}

func parseHistoryTimeToMs(v string) (int64, error) {
	if v == "" {
		return 0, nil
	}
	if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
		return ms, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func toStoreClosedRecords(records []tradpkg.ClosedPnLRecord) []store.ClosedPnLRecord {
	out := make([]store.ClosedPnLRecord, len(records))
	for i, r := range records {
		out[i] = store.ClosedPnLRecord{
			Symbol:          r.Symbol,
			Side:            r.Side,
			EntryPrice:      r.EntryPrice,
			ExitPrice:       r.ExitPrice,
			Quantity:        r.Quantity,
			MaxOpenQuantity: r.MaxOpenQuantity,
			RealizedPnL:     r.RealizedPnL,
			NetRealizedPnL: r.NetRealizedPnL,
			PnlRatio:       r.PnlRatio,
			Fee:            r.Fee,
			FundingFee:     r.FundingFee,
			Leverage:       r.Leverage,
			EntryTime:      r.EntryTime.UnixMilli(),
			ExitTime:       r.ExitTime.UnixMilli(),
			OrderID:        r.OrderID,
			CloseType:      r.CloseType,
			ExchangeID:     r.ExchangeID,
		}
	}
	return out
}

func sortPositionsByExitTimeDesc(positions []*store.TraderPosition) {
	sort.Slice(positions, func(i, j int) bool {
		if positions[i] == nil {
			return false
		}
		if positions[j] == nil {
			return true
		}
		return positions[i].ExitTime > positions[j].ExitTime
	})
}

func positionsToAPIJSON(positions []*store.TraderPosition) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(positions))
	for _, p := range positions {
		if p == nil {
			continue
		}
		entryQty := p.EntryQuantity
		if entryQty == 0 {
			entryQty = p.Quantity
		}
		out = append(out, map[string]interface{}{
			"id":              p.ID,
			"trader_id":       p.TraderID,
			"exchange_id":     p.ExchangeID,
			"exchange_type":   p.ExchangeType,
			"symbol":          p.Symbol,
			"side":            p.Side,
			"quantity":        p.Quantity,
			"entry_quantity":  entryQty,
			"entry_price":     p.EntryPrice,
			"entry_order_id":  p.EntryOrderID,
			"entry_time":      msToRFC3339(p.EntryTime),
			"exit_price":      p.ExitPrice,
			"exit_order_id":   p.ExitOrderID,
			"exit_time":       msToRFC3339(p.ExitTime),
			"realized_pnl":     p.RealizedPnL,
			"net_realized_pnl": p.NetRealizedPnL,
			"pnl_ratio":        p.PnlRatio,
			"fee":              p.Fee,
			"funding_fee":      p.FundingFee,
			"leverage":        p.Leverage,
			"status":          p.Status,
			"close_reason":    p.CloseReason,
			"created_at":      msToRFC3339(p.CreatedAt),
			"updated_at":      msToRFC3339(p.UpdatedAt),
		})
	}
	return out
}

func msToRFC3339(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// handleTrades Historical trades list
func (s *Server) handleTrades(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	symbol := c.Query("symbol")
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	// Normalize symbol (add USDT suffix if not present)
	if symbol != "" {
		symbol = market.Normalize(symbol)
	}

	// Get trades from store
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	allTrades, err := store.Position().GetRecentTrades(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get trades", err)
		return
	}

	// Filter by symbol if specified
	if symbol != "" {
		var result []interface{}
		for _, trade := range allTrades {
			if trade.Symbol == symbol {
				result = append(result, trade)
			}
		}
		c.JSON(http.StatusOK, result)
		return
	}

	c.JSON(http.StatusOK, allTrades)
}

// handleOrders Order list (all orders including open, close, stop loss, take profit, etc.)
func (s *Server) handleOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	symbol := c.Query("symbol")
	statusFilter := c.Query("status") // NEW, FILLED, CANCELED, etc.
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	// Normalize symbol (add USDT suffix if not present)
	if symbol != "" {
		symbol = market.Normalize(symbol)
	}

	// Get orders from store
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Get orders with filters applied at database level
	orders, err := store.Order().GetTraderOrdersFiltered(trader.GetID(), symbol, statusFilter, limit)
	if err != nil {
		SafeInternalError(c, "Get orders", err)
		return
	}

	c.JSON(http.StatusOK, orders)
}

// handleOrderFills Order fill details (all fills for a specific order)
func (s *Server) handleOrderFills(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Get fills for this order
	fills, err := store.Order().GetOrderFills(orderID)
	if err != nil {
		SafeInternalError(c, "Get order fills", err)
		return
	}

	c.JSON(http.StatusOK, fills)
}

// handleOpenOrders Get open orders (pending SL/TP) from exchange
func (s *Server) handleOpenOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get symbol parameter (required for exchange query)
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	// Normalize symbol
	symbol = market.Normalize(symbol)

	// Get open orders from exchange
	openOrders, err := trader.GetOpenOrders(symbol)
	if err != nil {
		SafeInternalError(c, "Get open orders", err)
		return
	}

	c.JSON(http.StatusOK, openOrders)
}
