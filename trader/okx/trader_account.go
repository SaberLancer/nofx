package okx

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

// GetBalance gets account balance
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached OKX account balance")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("🔄 Calling OKX API to get account balance...")
	data, err := t.doRequest("GET", okxAccountPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var balances []struct {
		TotalEq string `json:"totalEq"`
		AdjEq   string `json:"adjEq"`
		IsoEq   string `json:"isoEq"`
		OrdFroz string `json:"ordFroz"`
		Details []struct {
			Ccy      string `json:"ccy"`
			Eq       string `json:"eq"`
			CashBal  string `json:"cashBal"`
			AvailBal string `json:"availBal"`
			UPL      string `json:"upl"`
		} `json:"details"`
	}

	if err := json.Unmarshal(data, &balances); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	if len(balances) == 0 {
		return nil, fmt.Errorf("no balance data received")
	}

	balance := balances[0]

	// Find USDT balance
	var usdtAvail, usdtUPL float64
	for _, detail := range balance.Details {
		if detail.Ccy == "USDT" {
			usdtAvail, _ = strconv.ParseFloat(detail.AvailBal, 64)
			usdtUPL, _ = strconv.ParseFloat(detail.UPL, 64)
			break
		}
	}

	totalEq, _ := strconv.ParseFloat(balance.TotalEq, 64)

	result := map[string]interface{}{
		"totalWalletBalance":    totalEq,
		"availableBalance":      usdtAvail,
		"totalUnrealizedProfit": usdtUPL,
	}

	logger.Infof("✓ OKX balance: Total equity=%.2f, Available=%.2f, Unrealized PnL=%.2f", totalEq, usdtAvail, usdtUPL)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode configures the margin mode (cross/isolated) that will be applied
// to all subsequent leverage and order requests for this trader instance.
//
// OKX V5 unified accounts do not expose a per-symbol mode-switch endpoint that
// works reliably — the legacy /api/v5/account/set-isolated-mode endpoint returns
// error 51000 ("Parameter isoMode error") when called on a unified account.
// Instead, OKX applies the mode per-request via the mgnMode field on
// /api/v5/account/set-leverage and via the tdMode field on order placement.
//
// This implementation therefore stores the configured mode locally and injects it
// into each subsequent API request, rather than making an API call here.
// NOTE: unlike Binance/Bybit implementations of this interface, no network call
// is made — the method only updates local state.
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	t.isCrossMargin = isCrossMargin
	mgnMode := t.marginMode()

	// OKX V5 unified account applies cross/isolated per order via tdMode,
	// while leverage uses mgnMode on /account/set-leverage.
	// Persist the configured mode locally so subsequent leverage/order calls use it,
	// instead of calling the legacy isolated-mode endpoint that returns 51000 errors.
	logger.Infof("  ✓ %s margin mode configured as %s (applied via tdMode/mgnMode on subsequent requests)", symbol, mgnMode)
	return nil
}

// SetLeverage sets leverage
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	instId := t.convertSymbol(symbol)
	marginMode := t.marginMode()

	// Set leverage for both long and short
	for _, posSide := range []string{"long", "short"} {
		body := map[string]interface{}{
			"instId":  instId,
			"lever":   strconv.Itoa(leverage),
			"mgnMode": marginMode,
			"posSide": posSide,
		}

		_, err := t.doRequest("POST", okxLeveragePath, body)
		if err != nil {
			// Ignore if already at target leverage
			if strings.Contains(err.Error(), "same") {
				continue
			}
			logger.Infof("  ⚠️ Failed to set %s %s leverage: %v", symbol, posSide, err)
		}
	}

	logger.Infof("  ✓ %s leverage set to %dx (%s)", symbol, leverage, marginMode)
	return nil
}

// GetMarketPrice gets market price
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s?instId=%s", okxTickerPath, instId)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no price data received")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// GetClosedPnL retrieves closed position PnL records from OKX positions-history API.
// OKX API: /api/v5/account/positions-history
func (t *OKXTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var (
		records    []types.ClosedPnLRecord
		afterUTime string
	)

	for len(records) < limit {
		pageLimit := 100
		if remaining := limit - len(records); remaining < pageLimit {
			pageLimit = remaining
		}

		rows, err := t.fetchPositionsHistoryPage(startTime, pageLimit, afterUTime)
		if err != nil {
			return records, err
		}
		if len(rows) == 0 {
			break
		}

		reachedWindowStart := false
		for _, row := range rows {
			record, ok := t.okxPositionHistoryRowToRecord(row)
			if !ok {
				continue
			}
			if !startTime.IsZero() && record.ExitTime.Before(startTime) {
				reachedWindowStart = true
				continue
			}
			records = append(records, record)
			if len(records) >= limit {
				break
			}
		}

		if len(records) >= limit || reachedWindowStart || len(rows) < pageLimit {
			break
		}

		afterUTime = rows[len(rows)-1].UTime
		if afterUTime == "" {
			break
		}
	}

	return records, nil
}

func (t *OKXTrader) fetchPositionsHistoryPage(startTime time.Time, limit int, afterUTime string) ([]okxPositionHistoryRow, error) {
	path := buildOKXPositionsHistoryPath(startTime, limit, afterUTime)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions history: %w", err)
	}
	rows, err := parseOKXPositionsHistoryData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse positions history: %w", err)
	}
	return rows, nil
}

func (t *OKXTrader) okxPositionHistoryRowToRecord(pos okxPositionHistoryRow) (types.ClosedPnLRecord, bool) {
	record := types.ClosedPnLRecord{}

	parts := strings.Split(pos.InstID, "-")
	if len(parts) >= 2 {
		record.Symbol = parts[0] + parts[1]
	} else {
		record.Symbol = pos.InstID
	}
	record.Symbol = market.Normalize(record.Symbol)

	record.Side = pos.PosSide
	if record.Side == "" {
		record.Side = pos.Direction
	}

	record.EntryPrice, _ = strconv.ParseFloat(pos.OpenAvgPx, 64)
	record.ExitPrice, _ = strconv.ParseFloat(pos.CloseAvgPx, 64)

	closeQty, _ := strconv.ParseFloat(pos.CloseTotalPos, 64)
	maxQty, _ := strconv.ParseFloat(pos.OpenMaxPos, 64)
	if inst, err := t.getInstrument(record.Symbol); err == nil && inst.CtVal > 0 {
		closeQty = closeQty * inst.CtVal
		maxQty = maxQty * inst.CtVal
	}
	record.Quantity = closeQty
	record.MaxOpenQuantity = maxQty
	if record.MaxOpenQuantity <= 0 {
		record.MaxOpenQuantity = record.Quantity
	}

	if record.Symbol == "" || record.Quantity <= 0 || record.EntryPrice <= 0 || record.ExitPrice <= 0 {
		return record, false
	}

	// OKX: pnl = gross trading PnL (matches 平仓收益); realizedPnl = net after fee/funding.
	record.RealizedPnL, _ = strconv.ParseFloat(pos.Pnl, 64)
	record.NetRealizedPnL, _ = strconv.ParseFloat(pos.RealizedPnl, 64)
	if record.RealizedPnL == 0 && record.NetRealizedPnL != 0 {
		record.RealizedPnL = record.NetRealizedPnL
	}

	fee, _ := strconv.ParseFloat(pos.Fee, 64)
	fundingFee, _ := strconv.ParseFloat(pos.FundingFee, 64)
	record.Fee = math.Abs(fee)
	record.FundingFee = fundingFee

	lev, _ := strconv.ParseFloat(pos.Lever, 64)
	record.Leverage = int(lev)

	// OKX App「平仓收益率」= net realized PnL / margin (matches 已实现收益 + 已实现收益率).
	displayPnL := record.NetRealizedPnL
	if displayPnL == 0 {
		displayPnL = record.RealizedPnL
	}
	if displayPnL != 0 && record.EntryPrice > 0 && record.MaxOpenQuantity > 0 && record.Leverage > 0 {
		margin := record.EntryPrice * record.MaxOpenQuantity / float64(record.Leverage)
		if margin > 0 {
			record.PnlRatio = displayPnL / margin
		}
	} else {
		record.PnlRatio, _ = strconv.ParseFloat(pos.PnlRatio, 64)
	}

	cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)
	record.EntryTime = time.UnixMilli(cTime).UTC()
	record.ExitTime = time.UnixMilli(uTime).UTC()

	switch pos.Type {
	case "3", "4":
		record.CloseType = "liquidation"
	case "2":
		record.CloseType = "full"
	case "1":
		record.CloseType = "partial"
	default:
		record.CloseType = "unknown"
	}

	record.ExchangeID = pos.PosId
	return record, true
}

type okxPositionHistoryRow struct {
	InstID        string `json:"instId"`
	Direction     string `json:"direction"`
	PosSide       string `json:"posSide"`
	OpenAvgPx     string `json:"openAvgPx"`
	CloseAvgPx    string `json:"closeAvgPx"`
	OpenMaxPos    string `json:"openMaxPos"`
	CloseTotalPos string `json:"closeTotalPos"`
	Pnl           string `json:"pnl"`
	RealizedPnl   string `json:"realizedPnl"`
	PnlRatio      string `json:"pnlRatio"`
	Fee           string `json:"fee"`
	FundingFee    string `json:"fundingFee"`
	Lever         string `json:"lever"`
	CTime         string `json:"cTime"`
	UTime         string `json:"uTime"`
	Type          string `json:"type"`
	PosId         string `json:"posId"`
}

func buildOKXPositionsHistoryPath(startTime time.Time, limit int, afterUTime string) string {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	path := fmt.Sprintf("/api/v5/account/positions-history?instType=SWAP&limit=%d", limit)
	if afterUTime != "" {
		path += "&after=" + afterUTime
	} else if !startTime.IsZero() {
		// First page: records with uTime newer than startTime (forward sync / lookback window).
		path += fmt.Sprintf("&before=%d", startTime.UnixMilli())
	}
	return path
}

func parseOKXPositionsHistoryData(data []byte) ([]okxPositionHistoryRow, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var rows []okxPositionHistoryRow
	if err := json.Unmarshal(data, &rows); err == nil {
		return rows, nil
	}
	// Backward compat if caller passes full envelope.
	var envelope struct {
		Code string                  `json:"code"`
		Msg  string                  `json:"msg"`
		Data []okxPositionHistoryRow `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != "" && envelope.Code != "0" && envelope.Code != "1" {
		return nil, fmt.Errorf("OKX API error: %s - %s", envelope.Code, envelope.Msg)
	}
	return envelope.Data, nil
}
