package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"strings"
	"time"
)

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.GetWithExchangeOptions(decision.Symbol, at.exchange, at.klineOptions())
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, marketData.CurrentPrice, decision.Leverage, 0)

	// Record position opening time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	at.applyOpenPositionProtection(decision, decision.Symbol, "LONG", quantity, actionRecord)
	at.ResetPeakPnL(decision.Symbol, "long")

	return nil
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.GetWithExchangeOptions(decision.Symbol, at.exchange, at.klineOptions())
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, marketData.CurrentPrice, decision.Leverage, 0)

	// Record position opening time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	at.applyOpenPositionProtection(decision, decision.Symbol, "SHORT", quantity, actionRecord)
	at.ResetPeakPnL(decision.Symbol, "short")

	return nil
}

func isCodeEnforcedClose(decision *kernel.Decision) bool {
	return decision != nil && strings.Contains(decision.Reasoning, "[CODE ENFORCED PnL]")
}

func codeEnforcedClosePrice(decision *kernel.Decision) (float64, bool) {
	if decision == nil || decision.Price <= 0 {
		return 0, false
	}
	if isCodeEnforcedClose(decision) {
		return decision.Price, true
	}
	return 0, false
}

// interpretCloseOrderResult treats exchange NO_POSITION as failure for code-enforced closes
// (UI would show success while OKX still holds the position or state diverged).
func interpretCloseOrderResult(order map[string]interface{}, err error, codeEnforced bool) error {
	if err != nil {
		return err
	}
	if order == nil {
		if codeEnforced {
			return fmt.Errorf("close returned empty result")
		}
		return nil
	}
	status, _ := order["status"].(string)
	if status != "NO_POSITION" {
		return nil
	}
	msg, _ := order["message"].(string)
	if msg == "" {
		msg = "exchange reports no matching position (NO_POSITION)"
	}
	if codeEnforced {
		return fmt.Errorf("forced close mismatch: %s", msg)
	}
	logger.Infof("  ℹ️ Close skipped: %s", msg)
	return nil
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	if px, ok := codeEnforcedClosePrice(decision); ok {
		actionRecord.Price = px
	} else {
		marketData, err := market.GetWithExchangeOptions(decision.Symbol, at.exchange, at.klineOptions())
		if err != nil {
			return err
		}
		actionRecord.Price = marketData.CurrentPrice
	}

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity.
	// For partial close, always prefer exchange live quantity as source of truth.
	var entryPrice float64
	var quantity float64

	// First read local DB for entry price fallback.
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Always try exchange API and prefer live quantity to avoid stale local-db mismatch.
	exchangeQty := 0.0
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				if ep, ok := pos["entryPrice"].(float64); ok && ep > 0 {
					entryPrice = ep
				}
				if amt, ok := pos["positionAmt"].(float64); ok && amt > 0 {
					exchangeQty = amt
				}
				break
			}
		}
	}
	if exchangeQty > 0 {
		quantity = exchangeQty
		logger.Infof("  📊 Using exchange position data (preferred): qty=%.8f, entry=%.2f", quantity, entryPrice)
	} else {
		logger.Infof("  📊 Exchange position data unavailable, fallback qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	closeQty := 0.0
	if decision.CloseRatio > 0 && decision.CloseRatio < 1 && quantity > 0 {
		closeQty = quantity * decision.CloseRatio
	}
	order, err := at.trader.CloseLong(decision.Symbol, closeQty) // 0 = close all
	if closeErr := interpretCloseOrderResult(order, err, isCodeEnforcedClose(decision)); closeErr != nil {
		return closeErr
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	closedQty := quantityOrDefault(closeQty, quantity)
	at.recordAndConfirmOrder(order, decision.Symbol, "close_long", closedQty, actionRecord.Price, 0, entryPrice)

	if closedQty >= quantity*0.999 || quantity <= 0 {
		at.clearUnprotected(decision.Symbol, "long")
		at.clearPnLEnforceTier("", decision.Symbol, "long")
		at.ClearPeakPnLCache("", decision.Symbol, "long")
	}
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	if px, ok := codeEnforcedClosePrice(decision); ok {
		actionRecord.Price = px
	} else {
		marketData, err := market.GetWithExchangeOptions(decision.Symbol, at.exchange, at.klineOptions())
		if err != nil {
			return err
		}
		actionRecord.Price = marketData.CurrentPrice
	}

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity.
	// For partial close, always prefer exchange live quantity as source of truth.
	var entryPrice float64
	var quantity float64

	// First read local DB for entry price fallback.
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Always try exchange API and prefer live quantity to avoid stale local-db mismatch.
	exchangeQty := 0.0
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				if ep, ok := pos["entryPrice"].(float64); ok && ep > 0 {
					entryPrice = ep
				}
				if amt, ok := pos["positionAmt"].(float64); ok {
					if amt < 0 {
						amt = -amt
					}
					if amt > 0 {
						exchangeQty = amt
					}
				}
				break
			}
		}
	}
	if exchangeQty > 0 {
		quantity = exchangeQty
		logger.Infof("  📊 Using exchange position data (preferred): qty=%.8f, entry=%.2f", quantity, entryPrice)
	} else {
		logger.Infof("  📊 Exchange position data unavailable, fallback qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	closeQty := 0.0
	if decision.CloseRatio > 0 && decision.CloseRatio < 1 && quantity > 0 {
		closeQty = quantity * decision.CloseRatio
	}
	order, err := at.trader.CloseShort(decision.Symbol, closeQty) // 0 = close all
	if closeErr := interpretCloseOrderResult(order, err, isCodeEnforcedClose(decision)); closeErr != nil {
		return closeErr
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	closedQty := quantityOrDefault(closeQty, quantity)
	at.recordAndConfirmOrder(order, decision.Symbol, "close_short", closedQty, actionRecord.Price, 0, entryPrice)

	if closedQty >= quantity*0.999 || quantity <= 0 {
		at.clearUnprotected(decision.Symbol, "short")
		at.clearPnLEnforceTier("", decision.Symbol, "short")
		at.ClearPeakPnLCache("", decision.Symbol, "short")
	}
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

func quantityOrDefault(qty float64, fallback float64) float64 {
	if qty > 0 {
		return qty
	}
	return fallback
}
