package store

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestPositionDB(t *testing.T) *PositionStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&TraderPosition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &PositionStore{db: db}
}

func TestPositionLiveKey(t *testing.T) {
	if got := PositionLiveKey("ethusdt", "LONG"); got != "ETHUSDT_long" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestReconcileOpenPositionsClosesGhostRows(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	now := time.Now().UTC().UnixMilli()

	ghost := &TraderPosition{
		TraderID:    traderID,
		Symbol:      "ETHUSDT",
		Side:        "SHORT",
		Quantity:    100,
		EntryPrice:  1800,
		EntryTime:   now - 3600000,
		PeakPnLPct:  11.73,
		Status:      "OPEN",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := ps.CreateOpenPosition(ghost); err != nil {
		t.Fatalf("create ghost: %v", err)
	}

	closed, err := ps.ReconcileOpenPositions(traderID, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if closed != 1 {
		t.Fatalf("expected 1 closed, got %d", closed)
	}

	row, err := ps.GetOpenPositionBySymbol(traderID, "ETHUSDT", "SHORT")
	if err != nil {
		t.Fatalf("get open: %v", err)
	}
	if row != nil {
		t.Fatal("expected no OPEN row after ghost reconcile")
	}
}

func TestReconcileOpenPositionsKeepsLiveRow(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	now := time.Now().UTC().UnixMilli()

	live := &TraderPosition{
		TraderID:   traderID,
		Symbol:     "SOLUSDT",
		Side:       "LONG",
		Quantity:   10,
		EntryPrice: 72,
		EntryTime:  now,
		Status:     "OPEN",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := ps.CreateOpenPosition(live); err != nil {
		t.Fatalf("create live: %v", err)
	}

	closed, err := ps.ReconcileOpenPositions(traderID, map[string]bool{"SOLUSDT_long": true}, nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if closed != 0 {
		t.Fatalf("expected 0 closed, got %d", closed)
	}

	row, err := ps.GetOpenPositionBySymbol(traderID, "SOLUSDT", "LONG")
	if err != nil {
		t.Fatalf("get open: %v", err)
	}
	if row == nil || row.Status != "OPEN" {
		t.Fatal("expected live OPEN row to remain")
	}
}

func TestResetOpenPositionPeakPnLPct(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	now := time.Now().UTC().UnixMilli()

	pos := &TraderPosition{
		TraderID:   traderID,
		Symbol:     "BTCUSDT",
		Side:       "SHORT",
		Quantity:   1,
		EntryPrice: 65000,
		EntryTime:  now,
		PeakPnLPct: 10.34,
		Status:     "OPEN",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := ps.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := ps.ResetOpenPositionPeakPnLPct(traderID, "BTCUSDT", "short"); err != nil {
		t.Fatalf("reset peak: %v", err)
	}

	row, err := ps.GetOpenPositionBySymbol(traderID, "BTCUSDT", "SHORT")
	if err != nil {
		t.Fatalf("get open: %v", err)
	}
	if row == nil || row.PeakPnLPct != 0 {
		t.Fatalf("expected peak 0, got %v", row)
	}
}

func TestPeakCacheKeyUsesExchangePositionID(t *testing.T) {
	got := PeakCacheKey("okx_pos_123", "ETHUSDT", "short")
	if got != "pos:okx_pos_123" {
		t.Fatalf("unexpected key: %q", got)
	}
	got = PeakCacheKey("sync_ETH_SHORT_1", "ETHUSDT", "short")
	if got != "ETHUSDT_short" {
		t.Fatalf("expected symbol_side fallback, got %q", got)
	}
}

func TestSyncOpenPositionPeakPnLPctIgnoresGhostPeak(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	exchangeID := "ex1"
	now := time.Now().UTC().UnixMilli()

	ghost := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		Symbol:             "ETHUSDT",
		Side:               "SHORT",
		Quantity:           100,
		EntryPrice:         1800,
		EntryTime:          now - 3600000,
		PeakPnLPct:         11.73,
		ExchangePositionID: "sync_ETH_SHORT_old",
		Status:             "OPEN",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := ps.CreateOpenPosition(ghost); err != nil {
		t.Fatalf("create ghost: %v", err)
	}

	livePosID := "okx_live_pos_999"
	peak, err := ps.SyncOpenPositionPeakPnLPct(traderID, exchangeID, livePosID, "ETHUSDT", "SHORT", 1.5)
	if err != nil {
		t.Fatalf("sync peak: %v", err)
	}
	if peak != 1.5 {
		t.Fatalf("expected peak 1.5 from current only, got %v (ghost peak leaked)", peak)
	}

	row, err := ps.GetOpenPositionBySymbol(traderID, "ETHUSDT", "SHORT")
	if err != nil {
		t.Fatalf("get open: %v", err)
	}
	if row == nil {
		t.Fatal("expected OPEN row after bind")
	}
	if row.PeakPnLPct != 1.5 {
		t.Fatalf("expected bound row peak 1.5, got %v", row.PeakPnLPct)
	}
	if row.ExchangePositionID != livePosID {
		t.Fatalf("expected bound pos id %q, got %q", livePosID, row.ExchangePositionID)
	}
}

func TestBindOpenPositionExchangePositionID(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	exchangeID := "ex1"
	now := time.Now().UTC().UnixMilli()

	pos := &TraderPosition{
		TraderID:           traderID,
		Symbol:             "SOLUSDT",
		Side:               "LONG",
		Quantity:           10,
		EntryPrice:         72,
		EntryTime:          now,
		ExchangePositionID: "sync_SOL_LONG_1",
		Status:             "OPEN",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := ps.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := ps.BindOpenPositionExchangePositionID(traderID, exchangeID, "SOLUSDT", "LONG", "okx_sol_pos"); err != nil {
		t.Fatalf("bind: %v", err)
	}

	row, err := ps.GetOpenPositionByExchangePositionID(exchangeID, "okx_sol_pos")
	if err != nil {
		t.Fatalf("get by exchange pos id: %v", err)
	}
	if row == nil || row.ExchangePositionID != "okx_sol_pos" {
		t.Fatalf("expected bound row, got %v", row)
	}
	if row.PeakPnLPct != 0 {
		t.Fatalf("expected peak reset on bind, got %v", row.PeakPnLPct)
	}
}

func TestReconcileOpenPositionsPrefersMatchingExchangePositionID(t *testing.T) {
	ps := openTestPositionDB(t)
	traderID := "t1"
	exchangeID := "ex1"
	now := time.Now().UTC().UnixMilli()
	livePosID := "okx_live_pos_999"

	stale := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		Symbol:             "ETHUSDT",
		Side:               "SHORT",
		Quantity:           100,
		EntryPrice:         1800,
		EntryTime:          now,
		PeakPnLPct:         11.73,
		ExchangePositionID: "sync_ETH_SHORT_newer",
		Status:             "OPEN",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	matched := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		Symbol:             "ETHUSDT",
		Side:               "SHORT",
		Quantity:           100,
		EntryPrice:         1800,
		EntryTime:          now - 3600000,
		PeakPnLPct:         0,
		ExchangePositionID: livePosID,
		Status:             "OPEN",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := ps.CreateOpenPosition(matched); err != nil {
		t.Fatalf("create matched: %v", err)
	}
	if err := ps.CreateOpenPosition(stale); err != nil {
		t.Fatalf("create stale duplicate: %v", err)
	}

	closed, err := ps.ReconcileOpenPositions(
		traderID,
		map[string]bool{"ETHUSDT_short": true},
		map[string]string{"ETHUSDT_short": livePosID},
	)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if closed != 1 {
		t.Fatalf("expected 1 duplicate closed, got %d", closed)
	}

	row, err := ps.GetOpenPositionByExchangePositionID(exchangeID, livePosID)
	if err != nil {
		t.Fatalf("get matched open: %v", err)
	}
	if row == nil || row.Status != "OPEN" {
		t.Fatal("expected matched OPEN row to remain")
	}
}
