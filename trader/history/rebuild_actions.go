package history

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"nofx/trader/types"
)

const closeFillMergeGap = 2 * time.Minute

type openTradeEntry struct {
	Price    float64
	Quantity float64
	Fee      float64
	Time     time.Time
	TradeID  string
}

type positionState struct {
	OpenTrades   []openTradeEntry
	TotalQty     float64
	PendingClose *closeAccumulator
}

type closeMatchChunk struct {
	Symbol      string
	Side        string
	EntryPrice  float64
	EntryTime   time.Time
	ExitPrice   float64
	ExitTime    time.Time
	Quantity    float64
	RealizedPnL float64
	Fee         float64
	OrderID     string
	TradeID     string
}

type closeAccumulator struct {
	Symbol          string
	Side            string
	OrderID         string
	entryWeighted   float64
	exitWeighted    float64
	Quantity        float64
	RealizedPnL     float64
	Fee             float64
	EntryTime       time.Time
	ExitTime        time.Time
	ExchangeID      string
	lastFillTime    time.Time
}

func isOpenOrderAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "open_short":
		return true
	default:
		return false
	}
}

func isCloseOrderAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "close_long", "close_short":
		return true
	default:
		return false
	}
}

func sideFromOrderAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "close_long":
		return "long"
	case "open_short", "close_short":
		return "short"
	default:
		return ""
	}
}

func determinePositionSide(trade types.TradeRecord) string {
	posSide := strings.ToUpper(strings.TrimSpace(trade.PositionSide))
	if posSide == "LONG" || posSide == "SHORT" {
		return strings.ToLower(posSide)
	}
	side := strings.ToUpper(strings.TrimSpace(trade.Side))
	switch side {
	case "BUY":
		return "long"
	case "SELL":
		return "short"
	default:
		return ""
	}
}

// RebuildPositionsFromOrderActions reconstructs closed positions from exchange fills
// that carry open_/close_ order actions (OKX fills-history).
// Multiple partial fills for the same close order are merged into one record.
func RebuildPositionsFromOrderActions(trades []types.TradeRecord) []types.ClosedPnLRecord {
	if len(trades) == 0 {
		return nil
	}

	sorted := append([]types.TradeRecord(nil), trades...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Time.Before(sorted[j].Time)
	})

	states := make(map[string]*positionState)
	var records []types.ClosedPnLRecord

	flushPending := func(state *positionState) {
		if state == nil || state.PendingClose == nil {
			return
		}
		if rec := state.PendingClose.toRecord(); rec != nil {
			records = append(records, *rec)
		}
		state.PendingClose = nil
	}

	for _, trade := range sorted {
		side := sideFromOrderAction(trade.OrderAction)
		if side == "" {
			side = determinePositionSide(trade)
		}
		if side == "" {
			continue
		}

		key := fmt.Sprintf("%s_%s", trade.Symbol, side)
		if states[key] == nil {
			states[key] = &positionState{}
		}
		state := states[key]

		if isOpenOrderAction(trade.OrderAction) {
			flushPending(state)
			state.OpenTrades = append(state.OpenTrades, openTradeEntry{
				Price:    trade.Price,
				Quantity: trade.Quantity,
				Fee:      trade.Fee,
				Time:     trade.Time,
				TradeID:  trade.TradeID,
			})
			state.TotalQty += trade.Quantity
			continue
		}

		if !isCloseOrderAction(trade.OrderAction) {
			continue
		}

		if shouldFlushPendingClose(state.PendingClose, trade) {
			flushPending(state)
		}

		chunk := matchCloseFill(trade, side)
		if chunk == nil {
			continue
		}
		applyCloseMatchToState(state, chunk)

		if state.PendingClose == nil {
			state.PendingClose = newCloseAccumulator(chunk)
		} else {
			state.PendingClose.absorb(chunk)
		}

		if len(state.OpenTrades) == 0 {
			flushPending(state)
		}
	}

	for _, state := range states {
		flushPending(state)
	}

	return records
}

func shouldFlushPendingClose(pending *closeAccumulator, trade types.TradeRecord) bool {
	if pending == nil {
		return false
	}
	if pending.OrderID != "" && trade.OrderID != "" && pending.OrderID != trade.OrderID {
		return true
	}
	if !pending.lastFillTime.IsZero() && trade.Time.Sub(pending.lastFillTime) > closeFillMergeGap {
		return true
	}
	return false
}

func matchCloseFill(trade types.TradeRecord, side string) *closeMatchChunk {
	if trade.Price <= 0 || trade.Quantity <= 0 {
		return nil
	}

	orderID := trade.OrderID
	if orderID == "" {
		orderID = trade.TradeID
	}

	return &closeMatchChunk{
		Symbol:      trade.Symbol,
		Side:        side,
		EntryTime:   trade.Time,
		ExitPrice:   trade.Price,
		ExitTime:    trade.Time,
		Quantity:    trade.Quantity,
		RealizedPnL: trade.RealizedPnL,
		Fee:         trade.Fee,
		OrderID:     orderID,
		TradeID:     trade.TradeID,
	}
}

func applyCloseMatchToState(state *positionState, chunk *closeMatchChunk) {
	if state == nil || chunk == nil || chunk.Quantity <= 0 {
		return
	}

	remainingQty := chunk.Quantity
	var weightedSum float64
	var matchedQty float64
	var totalEntryFee float64
	entryTime := chunk.EntryTime

	for i := 0; i < len(state.OpenTrades) && remainingQty > 0.00000001; i++ {
		ot := &state.OpenTrades[i]
		matchQty := ot.Quantity
		if matchQty > remainingQty {
			matchQty = remainingQty
		}

		weightedSum += ot.Price * matchQty
		matchedQty += matchQty
		totalEntryFee += ot.Fee * (matchQty / ot.Quantity)

		if !ot.Time.IsZero() && (entryTime.IsZero() || ot.Time.Before(entryTime)) {
			entryTime = ot.Time
		}

		remainingQty -= matchQty
		ot.Quantity -= matchQty
		if ot.Quantity <= 0.00000001 {
			state.OpenTrades = append(state.OpenTrades[:i], state.OpenTrades[i+1:]...)
			i--
		}
	}

	if matchedQty <= 0.00000001 {
		return
	}

	chunk.EntryPrice = weightedSum / matchedQty
	chunk.Quantity = matchedQty
	chunk.EntryTime = entryTime
	chunk.Fee += totalEntryFee

	if chunk.RealizedPnL == 0 && chunk.EntryPrice > 0 {
		if chunk.Side == "long" {
			chunk.RealizedPnL = (chunk.ExitPrice - chunk.EntryPrice) * chunk.Quantity
		} else {
			chunk.RealizedPnL = (chunk.EntryPrice - chunk.ExitPrice) * chunk.Quantity
		}
		chunk.RealizedPnL = math.Round(chunk.RealizedPnL*100) / 100
	}

	state.TotalQty -= chunk.Quantity
}

func newCloseAccumulator(chunk *closeMatchChunk) *closeAccumulator {
	acc := &closeAccumulator{
		Symbol:       chunk.Symbol,
		Side:         chunk.Side,
		OrderID:      chunk.OrderID,
		Quantity:     chunk.Quantity,
		RealizedPnL:  chunk.RealizedPnL,
		Fee:          chunk.Fee,
		EntryTime:    chunk.EntryTime,
		ExitTime:     chunk.ExitTime,
		ExchangeID:   chunk.TradeID,
		lastFillTime: chunk.ExitTime,
	}
	acc.entryWeighted = chunk.EntryPrice * chunk.Quantity
	acc.exitWeighted = chunk.ExitPrice * chunk.Quantity
	return acc
}

func (acc *closeAccumulator) absorb(chunk *closeMatchChunk) {
	if acc == nil || chunk == nil || chunk.Quantity <= 0 {
		return
	}
	acc.entryWeighted += chunk.EntryPrice * chunk.Quantity
	acc.exitWeighted += chunk.ExitPrice * chunk.Quantity
	acc.Quantity += chunk.Quantity
	acc.RealizedPnL += chunk.RealizedPnL
	acc.Fee += chunk.Fee
	if chunk.EntryTime.Before(acc.EntryTime) || acc.EntryTime.IsZero() {
		acc.EntryTime = chunk.EntryTime
	}
	if chunk.ExitTime.After(acc.ExitTime) {
		acc.ExitTime = chunk.ExitTime
	}
	if chunk.ExitTime.After(acc.lastFillTime) {
		acc.lastFillTime = chunk.ExitTime
	}
	if acc.OrderID == "" {
		acc.OrderID = chunk.OrderID
	}
}

func (acc *closeAccumulator) toRecord() *types.ClosedPnLRecord {
	if acc == nil || acc.Quantity <= 0 {
		return nil
	}
	entryPrice := acc.entryWeighted / acc.Quantity
	exitPrice := acc.exitWeighted / acc.Quantity
	return &types.ClosedPnLRecord{
		Symbol:      acc.Symbol,
		Side:        acc.Side,
		EntryPrice:  entryPrice,
		ExitPrice:   exitPrice,
		Quantity:    acc.Quantity,
		RealizedPnL: math.Round(acc.RealizedPnL*100) / 100,
		Fee:         acc.Fee,
		EntryTime:   acc.EntryTime,
		ExitTime:    acc.ExitTime,
		OrderID:     acc.OrderID,
		ExchangeID:  acc.ExchangeID,
		CloseType:   "fills_history",
	}
}
