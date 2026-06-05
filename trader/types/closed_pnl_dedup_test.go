package types

import (
	"testing"
	"time"
)

func TestDedupeClosedPnLRecords_SamePosId(t *testing.T) {
	exit := time.UnixMilli(1717500000000)
	records := []ClosedPnLRecord{
		{
			Symbol: "BTCUSDT", Side: "short", ExchangeID: "pos-1",
			EntryPrice: 62210.86, ExitPrice: 61599.15, Quantity: 1,
			RealizedPnL: 2560, ExitTime: exit, CloseType: "partial",
		},
		{
			Symbol: "BTCUSDT", Side: "short", ExchangeID: "pos-1",
			EntryPrice: 62210.86, ExitPrice: 61599.15, Quantity: 1,
			RealizedPnL: 2560, ExitTime: exit, CloseType: "full",
		},
	}
	out := DedupeClosedPnLRecords(records)
	if len(out) != 1 {
		t.Fatalf("expected 1 record, got %d", len(out))
	}
	if out[0].CloseType != "full" {
		t.Fatalf("expected full close row, got %q", out[0].CloseType)
	}
}

func TestDedupeClosedPnLRecords_SamePosIdDifferentExitTime(t *testing.T) {
	records := []ClosedPnLRecord{
		{
			Symbol: "BTCUSDT", Side: "short", ExchangeID: "pos-1",
			EntryPrice: 62000, ExitPrice: 61800, Quantity: 0.5,
			RealizedPnL: 100, ExitTime: time.UnixMilli(1717500000000), CloseType: "partial",
		},
		{
			Symbol: "BTCUSDT", Side: "short", ExchangeID: "pos-1",
			EntryPrice: 62000, ExitPrice: 61500, Quantity: 0.5,
			RealizedPnL: 250, ExitTime: time.UnixMilli(1717503600000), CloseType: "full",
		},
	}
	out := DedupeClosedPnLRecords(records)
	if len(out) != 2 {
		t.Fatalf("expected 2 close events for same posId, got %d", len(out))
	}
}

func TestDedupeClosedPnLRecords_PaginationOverlap(t *testing.T) {
	exit := time.UnixMilli(1717500000000)
	dup := ClosedPnLRecord{
		Symbol: "SOLUSDT", Side: "short", ExchangeID: "pos-sol",
		EntryPrice: 69.72, ExitPrice: 68.5862, Quantity: 100,
		RealizedPnL: 1530, ExitTime: exit, CloseType: "full",
	}
	out := DedupeClosedPnLRecords([]ClosedPnLRecord{dup, dup})
	if len(out) != 1 {
		t.Fatalf("expected 1 record after duplicate pages, got %d", len(out))
	}
}
