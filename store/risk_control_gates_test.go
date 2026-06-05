package store

import (
	"testing"
	"time"
)

func TestSanitizePeakPnLPct_RejectsAbsurdStoredPeak(t *testing.T) {
	got := SanitizePeakPnLPct(511, 8, 200, 30)
	if got != 8 {
		t.Fatalf("expected peak clamped to current 8, got %v", got)
	}
}

func TestSanitizePeakPnLPct_RejectsSingleTickJump(t *testing.T) {
	got := SanitizePeakPnLPct(45, 8, 200, 30)
	if got != 8 {
		t.Fatalf("expected jump-sanitized peak 8, got %v", got)
	}
}

func TestSanitizePeakPnLPct_KeepsValidPeak(t *testing.T) {
	got := SanitizePeakPnLPct(15, 12, 200, 30)
	if got != 15 {
		t.Fatalf("expected valid peak 15, got %v", got)
	}
}

func TestSyncOpenPositionPeakPnLPct_SanitizesDirtyStoredPeak(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	exchangeID := "ex1"
	now := time.Now().UTC().UnixMilli()

	pos := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		Symbol:             "SOLUSDT",
		Side:               "LONG",
		Quantity:           10,
		EntryPrice:         72,
		EntryTime:          now,
		PeakPnLPct:         511,
		ExchangePositionID: "okx_sol_pos",
		Status:             "OPEN",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := ps.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create: %v", err)
	}

	peak, err := ps.SyncOpenPositionPeakPnLPct(traderID, exchangeID, "okx_sol_pos", "SOLUSDT", "LONG", 8, 200, 30)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if peak != 8 {
		t.Fatalf("expected sanitized peak 8, got %v", peak)
	}

	row, err := ps.GetOpenPositionByExchangePositionID(exchangeID, "okx_sol_pos")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row == nil || row.PeakPnLPct != 8 {
		t.Fatalf("expected DB peak 8 after sanitize, got %v", row)
	}
}
