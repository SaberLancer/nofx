package okx

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/market"
	"nofx/trader/types"
	"sort"
	"strconv"
	"strings"
	"time"
)

type closeOrderAgg struct {
	exchangeOrderID string
	orderAction     string
	positionSide    string
	execQty         float64
	weightedPrice   float64
	fee             float64
	realizedPnL     float64
	filledAt        time.Time
}

func positionOrderActions(side string) map[string]struct{} {
	side = strings.ToUpper(strings.TrimSpace(side))
	if side == "SHORT" {
		return map[string]struct{}{
			"open_short":  {},
			"close_short": {},
		}
	}
	return map[string]struct{}{
		"open_long":  {},
		"close_long": {},
	}
}

// GetPositionCloseOrders returns open/add and close/reduce orders for a historical position from OKX fills-history.
func (t *OKXTrader) GetPositionCloseOrders(symbol, side string, entryTime, exitTime time.Time, limit int) ([]types.PositionCloseOrderRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	symbol = market.Normalize(symbol)
	side = strings.ToUpper(strings.TrimSpace(side))
	if symbol == "" || (side != "LONG" && side != "SHORT") {
		return nil, fmt.Errorf("invalid symbol or side")
	}

	allowedActions := positionOrderActions(side)

	beginMs := int64(0)
	if !entryTime.IsZero() {
		beginMs = entryTime.Add(-30 * time.Second).UnixMilli()
	} else if !exitTime.IsZero() {
		beginMs = exitTime.Add(-7 * 24 * time.Hour).UnixMilli()
	}
	endMs := int64(0)
	if !exitTime.IsZero() {
		endMs = exitTime.Add(30 * time.Second).UnixMilli()
	}

	instID := t.convertSymbol(symbol)
	fills, err := t.fetchFillsHistoryWindow(instID, "", beginMs, endMs, limit*20)
	if err != nil {
		return nil, err
	}

	byOrder := make(map[string]*closeOrderAgg)
	for _, fill := range fills {
		if fill.Symbol != symbol {
			continue
		}
		if _, ok := allowedActions[fill.OrderAction]; !ok {
			continue
		}
		if !entryTime.IsZero() && fill.ExecTime.Before(entryTime.Add(-time.Second)) {
			continue
		}
		if !exitTime.IsZero() && fill.ExecTime.After(exitTime.Add(time.Second)) {
			continue
		}
		if fill.FillQtyBase <= 0 {
			continue
		}

		orderID := strings.TrimSpace(fill.OrderID)
		if orderID == "" {
			orderID = fill.TradeID
		}
		groupKey := orderID + "|" + fill.OrderAction
		agg, ok := byOrder[groupKey]
		if !ok {
			agg = &closeOrderAgg{
				exchangeOrderID: orderID,
				orderAction:     fill.OrderAction,
				positionSide:    side,
				filledAt:        fill.ExecTime,
			}
			byOrder[groupKey] = agg
		}

		agg.execQty += fill.FillQtyBase
		agg.weightedPrice += fill.FillPrice * fill.FillQtyBase
		agg.fee += fill.Fee
		agg.realizedPnL += fill.FillPnl
		if fill.ExecTime.After(agg.filledAt) {
			agg.filledAt = fill.ExecTime
		}
	}

	out := make([]types.PositionCloseOrderRecord, 0, len(byOrder))
	for _, agg := range byOrder {
		if agg.execQty <= 0 {
			continue
		}
		execPrice := agg.weightedPrice / agg.execQty
		out = append(out, types.PositionCloseOrderRecord{
			ExchangeOrderID: agg.exchangeOrderID,
			OrderAction:     agg.orderAction,
			PositionSide:    agg.positionSide,
			ExecQuantity:    math.Round(agg.execQty*1e8) / 1e8,
			ExecPrice:       execPrice,
			Fee:             agg.fee,
			RealizedPnL:     agg.realizedPnL,
			FilledAt:        agg.filledAt,
			Status:          "FILLED",
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].FilledAt.After(out[j].FilledAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (t *OKXTrader) fetchFillsHistoryWindow(instID, subType string, beginMs, endMs int64, maxFills int) ([]OKXTrade, error) {
	if maxFills <= 0 {
		maxFills = 200
	}

	var all []OKXTrade
	afterBillID := ""
	seenBill := make(map[string]struct{})

	for page := 0; page < maxFillsPages && len(all) < maxFills; page++ {
		batch, err := t.fetchFillsHistoryWindowPage(instID, subType, beginMs, endMs, 100, afterBillID)
		if err != nil {
			return all, err
		}
		if len(batch) == 0 {
			break
		}

		added := 0
		for _, fill := range batch {
			billKey := fill.BillID
			if billKey == "" {
				billKey = fill.TradeID
			}
			if _, ok := seenBill[billKey]; ok {
				continue
			}
			seenBill[billKey] = struct{}{}
			all = append(all, fill)
			added++
			if len(all) >= maxFills {
				break
			}
		}
		if added == 0 {
			break
		}

		last := batch[len(batch)-1]
		nextAfter := last.BillID
		if nextAfter == "" || nextAfter == afterBillID {
			break
		}
		afterBillID = nextAfter
	}

	return all, nil
}

func (t *OKXTrader) fetchFillsHistoryWindowPage(instID, subType string, beginMs, endMs int64, limit int, afterBillID string) ([]OKXTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	path := fmt.Sprintf("/api/v5/trade/fills-history?instType=SWAP&limit=%d", limit)
	if instID != "" {
		path += "&instId=" + instID
	}
	if subType != "" {
		path += "&subType=" + subType
	}
	if beginMs > 0 {
		path += fmt.Sprintf("&begin=%d", beginMs)
	}
	if endMs > 0 {
		path += fmt.Sprintf("&end=%d", endMs)
	}
	if afterBillID != "" {
		path += "&after=" + afterBillID
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get fills history: %w", err)
	}

	var fills []struct {
		InstID   string `json:"instId"`
		TradeID  string `json:"tradeId"`
		OrdID    string `json:"ordId"`
		BillID   string `json:"billId"`
		Side     string `json:"side"`
		PosSide  string `json:"posSide"`
		FillPx   string `json:"fillPx"`
		FillSz   string `json:"fillSz"`
		Fee      string `json:"fee"`
		FillPnl  string `json:"fillPnl"`
		FeeCcy   string `json:"feeCcy"`
		Ts       string `json:"ts"`
		ExecType string `json:"execType"`
		SubType  string `json:"subType"`
	}

	if err := json.Unmarshal(data, &fills); err != nil {
		return nil, fmt.Errorf("failed to parse fills: %w", err)
	}

	trades := make([]OKXTrade, 0, len(fills))
	for _, fill := range fills {
		fillPrice, _ := strconv.ParseFloat(fill.FillPx, 64)
		fillSz, _ := strconv.ParseFloat(fill.FillSz, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		fillPnl, _ := strconv.ParseFloat(fill.FillPnl, 64)
		ts, _ := strconv.ParseInt(fill.Ts, 10, 64)
		if fillPrice <= 0 || fillSz <= 0 {
			continue
		}

		symbol := t.convertSymbolBack(fill.InstID)
		fillQtyBase := fillSz
		if inst, err := t.getInstrument(symbol); err == nil && inst.CtVal > 0 {
			fillQtyBase = fillSz * inst.CtVal
		}
		fillQtyBase = math.Round(fillQtyBase*1e8) / 1e8

		trades = append(trades, OKXTrade{
			InstID:      fill.InstID,
			Symbol:      symbol,
			TradeID:     fill.TradeID,
			OrderID:     fill.OrdID,
			BillID:      fill.BillID,
			Side:        fill.Side,
			PosSide:     fill.PosSide,
			FillPrice:   fillPrice,
			FillQty:     fillSz,
			FillQtyBase: fillQtyBase,
			Fee:         -fee,
			FillPnl:     fillPnl,
			FeeAsset:    fill.FeeCcy,
			ExecTime:    time.UnixMilli(ts).UTC(),
			IsMaker:     fill.ExecType == "M",
			OrderAction: okxOrderAction(fill.PosSide, fill.Side),
		})
	}

	return trades, nil
}
