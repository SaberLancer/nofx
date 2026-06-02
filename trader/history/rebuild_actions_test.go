package history

import (
	"fmt"
	"testing"
	"time"

	"nofx/trader/types"
)

func TestRebuildMergesMultipleCloseFills(t *testing.T) {
	t0 := time.Unix(1717000000, 0).UTC()
	tClose := t0.Add(5 * time.Minute)
	trades := []types.TradeRecord{
		{
			TradeID:     "open1",
			Symbol:      "BTCUSDT",
			OrderAction: "open_long",
			Price:       73950.0,
			Quantity:    1.0,
			Fee:         10.0,
			Time:        t0,
		},
	}
	closeQtys := []float64{0.3, 0.3, 0.4}
	closeFees := []float64{-1.0, -1.0, -1.2}
	for i, qty := range closeQtys {
		trades = append(trades, types.TradeRecord{
			TradeID:     fmt.Sprintf("close-fill-%d", i),
			OrderID:     "same-close-order",
			Symbol:      "BTCUSDT",
			OrderAction: "close_long",
			Price:       73855.80,
			Quantity:    qty,
			Fee:         closeFees[i],
			Time:        tClose,
		})
	}

	records := RebuildPositionsFromOrderActions(trades)
	if len(records) != 1 {
		t.Fatalf("expected 1 merged closed position, got %d", len(records))
	}
	rec := records[0]
	if rec.Quantity < 0.99 || rec.Quantity > 1.01 {
		t.Fatalf("expected total qty ~1.0, got %v", rec.Quantity)
	}
	if rec.EntryPrice != 73950.0 {
		t.Fatalf("expected entry 73950, got %v", rec.EntryPrice)
	}
	if rec.ExitPrice != 73855.80 {
		t.Fatalf("expected exit 73855.80, got %v", rec.ExitPrice)
	}
}

func TestRebuildPositionsFromOrderActionsShortRoundTrip(t *testing.T) {
	t0 := time.Unix(1717000000, 0).UTC()
	trades := []types.TradeRecord{
		{
			TradeID:     "t1",
			Symbol:      "SOLUSDT",
			OrderAction: "open_short",
			Price:       85.0,
			Quantity:    100,
			Fee:         1.2,
			Time:        t0,
		},
		{
			TradeID:     "t2",
			Symbol:      "SOLUSDT",
			OrderAction: "close_short",
			Price:       82.0,
			Quantity:    100,
			Fee:         1.1,
			Time:        t0.Add(4 * time.Hour),
		},
	}

	records := RebuildPositionsFromOrderActions(trades)
	if len(records) != 1 {
		t.Fatalf("expected 1 closed position, got %d", len(records))
	}
	rec := records[0]
	if rec.Side != "short" {
		t.Fatalf("expected short, got %s", rec.Side)
	}
	if rec.EntryPrice != 85.0 || rec.ExitPrice != 82.0 {
		t.Fatalf("unexpected prices: entry=%v exit=%v", rec.EntryPrice, rec.ExitPrice)
	}
	wantPnL := (85.0 - 82.0) * 100
	if rec.RealizedPnL != wantPnL {
		t.Fatalf("expected pnl %v, got %v", wantPnL, rec.RealizedPnL)
	}
}
