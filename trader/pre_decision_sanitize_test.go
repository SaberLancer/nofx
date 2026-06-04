package trader

import (
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func TestTickConflictsWithOpen_LongVsHeavySell(t *testing.T) {
	cfg := store.PreDecisionConfig{Enabled: true, MinSellPressure: 0.55, MinBuyPressure: 0.55}
	signal := market.TrendSignal{
		Symbol:       "SOLUSDT",
		Direction:    market.TrendShort,
		BuyPressure:  0.051,
		SellPressure: 0.949,
		MomentumPct:  -0.04,
	}
	conflict, reason := tickConflictsWithOpen("open_long", signal, cfg)
	if !conflict {
		t.Fatalf("expected conflict for open_long vs heavy sell, reason=%q", reason)
	}
}

func TestTickConflictsWithOpen_ShortVsHeavyBuy(t *testing.T) {
	cfg := store.PreDecisionConfig{Enabled: true, MinBuyPressure: 0.55, MinSellPressure: 0.55}
	signal := market.TrendSignal{
		Direction:    market.TrendLong,
		BuyPressure:  0.82,
		SellPressure: 0.18,
		MomentumPct:  0.05,
	}
	conflict, _ := tickConflictsWithOpen("open_short", signal, cfg)
	if !conflict {
		t.Fatal("expected conflict for open_short vs heavy buy")
	}
}

func TestSanitizeOpenDecisionsAgainstTick_Downgrades(t *testing.T) {
	tracker := market.NewTickTrendTracker(market.PreDecisionSettings{
		WindowSec: 60, MinTicks: 3, MinBuyPressure: 0.55, MinSellPressure: 0.55, MinMomentumPct: 0.02,
	})
	now := time.Now()
	tracker.IngestBatchAt([]market.RawTick{
		{Symbol: "SOLUSDT", Price: 72.0, Quantity: 1, Side: market.TickSideSell, Timestamp: now.Add(-2 * time.Second)},
		{Symbol: "SOLUSDT", Price: 71.9, Quantity: 2, Side: market.TickSideSell, Timestamp: now.Add(-1 * time.Second)},
		{Symbol: "SOLUSDT", Price: 71.8, Quantity: 3, Side: market.TickSideSell, Timestamp: now},
	}, now)

	at := &AutoTrader{
		preDecisionTracker: tracker,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				PreDecision: store.PreDecisionConfig{Enabled: true},
			},
		},
	}
	decisions := []kernel.Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_long",
		Leverage:        5,
		PositionSizeUSD: 1000,
	}}
	notes := at.sanitizeOpenDecisionsAgainstTick(decisions)
	if len(notes) == 0 {
		t.Fatal("expected downgrade note")
	}
	if decisions[0].Action != "wait" {
		t.Fatalf("expected wait, got %s", decisions[0].Action)
	}
	if !strings.Contains(decisions[0].Reasoning, "tick conflict") {
		t.Fatalf("expected reasoning to mention tick conflict, got %q", decisions[0].Reasoning)
	}
}

func TestInterpretCloseOrderResult_CodeEnforcedNO_POSITION(t *testing.T) {
	order := map[string]interface{}{
		"status":  "NO_POSITION",
		"message": "No short position found for SOLUSDT on OKX",
	}
	err := interpretCloseOrderResult(order, nil, true)
	if err == nil {
		t.Fatal("expected error for code-enforced NO_POSITION")
	}
	if !strings.Contains(err.Error(), "forced close mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInterpretCloseOrderResult_AINoPositionOK(t *testing.T) {
	order := map[string]interface{}{"status": "NO_POSITION"}
	if err := interpretCloseOrderResult(order, nil, false); err != nil {
		t.Fatalf("AI close should tolerate NO_POSITION, got %v", err)
	}
}
