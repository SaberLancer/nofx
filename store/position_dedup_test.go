package store

import "testing"

func TestDedupeClosedPositionsKeepsExchangePnL(t *testing.T) {
	exitMs := int64(1717172040000)
	orderSync := &TraderPosition{
		ID:                 1,
		Symbol:             "SOLUSDT",
		Side:               "SHORT",
		EntryPrice:         82.72,
		ExitPrice:          82.72,
		Quantity:           362.58,
		EntryQuantity:      362.58,
		RealizedPnL:        0,
		Fee:                -29.99,
		ExitTime:           exitMs,
		ExchangePositionID: "sync_SOL_SHORT_123",
		Status:             "CLOSED",
	}
	exchange := &TraderPosition{
		ID:                 2,
		Symbol:             "SOLUSDT",
		Side:               "SHORT",
		EntryPrice:         82.72,
		ExitPrice:          82.72,
		Quantity:           362.58,
		EntryQuantity:      362.58,
		RealizedPnL:        -28.86,
		Fee:                -31.12,
		ExitTime:           exitMs + 1000,
		ExchangePositionID: "okx_pos_999",
		Status:             "CLOSED",
	}

	out := DedupeClosedPositions([]*TraderPosition{orderSync, exchange})
	if len(out) != 1 {
		t.Fatalf("expected 1 row after dedupe, got %d", len(out))
	}
	if out[0].RealizedPnL != -28.86 {
		t.Fatalf("expected exchange PnL -28.86, got %v", out[0].RealizedPnL)
	}
	if out[0].ExchangePositionID != "okx_pos_999" {
		t.Fatalf("expected OKX position id, got %q", out[0].ExchangePositionID)
	}
}

func TestQuantitiesCloseTolerance(t *testing.T) {
	if !quantitiesClose(362.58, 362.58001) {
		t.Fatal("expected quantities to match within tolerance")
	}
}
