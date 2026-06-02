package okx

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/trader/history"
	"nofx/trader/types"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxFillsPages = 20 // up to 2000 fills

// GetClosedPnLFromFillsHistory rebuilds closed positions from OKX fills-history (历史成交).
func (t *OKXTrader) GetClosedPnLFromFillsHistory(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	fills, err := t.fetchFillsHistory(startTime, maxFillsPages*100)
	if err != nil {
		return nil, err
	}
	if len(fills) == 0 {
		return nil, nil
	}

	tradeRecords := make([]types.TradeRecord, 0, len(fills))
	for _, fill := range fills {
		if fill.OrderAction == "" {
			continue
		}
		tradeRecords = append(tradeRecords, types.TradeRecord{
			TradeID:      fill.TradeID,
			OrderID:      fill.OrderID,
			Symbol:       fill.Symbol,
			Side:         strings.ToUpper(fill.Side),
			PositionSide: strings.ToUpper(fill.PosSide),
			OrderAction:  fill.OrderAction,
			Price:        fill.FillPrice,
			Quantity:     fill.FillQtyBase,
			Fee:          fill.Fee,
			Time:         fill.ExecTime,
		})
	}

	records := history.RebuildPositionsFromOrderActions(tradeRecords)
	sort.Slice(records, func(i, j int) bool {
		return records[i].ExitTime.After(records[j].ExitTime)
	})
	if len(records) > limit {
		records = records[:limit]
	}

	out := make([]types.ClosedPnLRecord, len(records))
	for i := range records {
		out[i] = records[i]
	}
	return out, nil
}

func (t *OKXTrader) fetchFillsHistory(since time.Time, maxFills int) ([]OKXTrade, error) {
	if maxFills <= 0 {
		maxFills = 1000
	}

	var all []OKXTrade
	afterBillID := ""
	seenBill := make(map[string]struct{})

	for page := 0; page < maxFillsPages && len(all) < maxFills; page++ {
		batch, err := t.fetchFillsHistoryPage(100, since, afterBillID)
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

func (t *OKXTrader) fetchFillsHistoryPage(limit int, since time.Time, afterBillID string) ([]OKXTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	path := fmt.Sprintf("/api/v5/trade/fills-history?instType=SWAP&limit=%d", limit)
	if !since.IsZero() {
		path += fmt.Sprintf("&begin=%d", since.UnixMilli())
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
		FeeCcy   string `json:"feeCcy"`
		Ts       string `json:"ts"`
		ExecType string `json:"execType"`
	}

	if err := json.Unmarshal(data, &fills); err != nil {
		return nil, fmt.Errorf("failed to parse fills: %w", err)
	}

	trades := make([]OKXTrade, 0, len(fills))
	for _, fill := range fills {
		fillPrice, _ := strconv.ParseFloat(fill.FillPx, 64)
		fillSz, _ := strconv.ParseFloat(fill.FillSz, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
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

		orderAction := okxOrderAction(fill.PosSide, fill.Side)
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
			FeeAsset:    fill.FeeCcy,
			ExecTime:    time.UnixMilli(ts).UTC(),
			IsMaker:     fill.ExecType == "M",
			OrderAction: orderAction,
		})
	}

	return trades, nil
}

func okxOrderAction(posSide, side string) string {
	posSide = strings.ToLower(strings.TrimSpace(posSide))
	side = strings.ToLower(strings.TrimSpace(side))
	switch posSide {
	case "long":
		if side == "buy" {
			return "open_long"
		}
		return "close_long"
	case "short":
		if side == "sell" {
			return "open_short"
		}
		return "close_short"
	default:
		if side == "buy" {
			return "open_long"
		}
		return "open_short"
	}
}
