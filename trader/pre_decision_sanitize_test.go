package trader

import (
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func TestTickConflictLevel_LongVsHeavySell_Block(t *testing.T) {
	cfg := store.PreDecisionConfig{Enabled: true, MinSellPressure: 0.55, MinBuyPressure: 0.55}
	cfg.Normalize()
	signal := market.TrendSignal{
		Direction:    market.TrendShort,
		BuyPressure:  0.051,
		SellPressure: 0.949,
		MomentumPct:  -0.04,
	}
	level, _ := evaluateTickConflict("open_long", signal, cfg)
	if level != tickConflictBlock {
		t.Fatalf("expected block, got %v", level)
	}
}

func TestTickConflictLevel_LongVsMildSell_Reduce(t *testing.T) {
	cfg := store.PreDecisionConfig{Enabled: true, MinSellPressure: 0.55, MinBuyPressure: 0.55}
	cfg.Normalize()
	signal := market.TrendSignal{
		Direction:    market.TrendNone,
		BuyPressure:  0.42,
		SellPressure: 0.60,
		MomentumPct:  -0.01,
	}
	level, _ := evaluateTickConflict("open_long", signal, cfg)
	if level != tickConflictReduce {
		t.Fatalf("expected reduce, got %v", level)
	}
}

func TestSanitizeOpenDecisionsGate_TickBlock(t *testing.T) {
	tracker := market.NewTickTrendTracker(market.PreDecisionSettings{
		WindowSec: 60, MinTicks: 3, MinBuyPressure: 0.55, MinSellPressure: 0.55, MinMomentumPct: 0.02,
	})
	now := time.Now()
	tracker.IngestBatchAt([]market.RawTick{
		{Symbol: "SOLUSDT", Price: 72.0, Quantity: 1, Side: market.TickSideSell, Timestamp: now.Add(-2 * time.Second)},
		{Symbol: "SOLUSDT", Price: 71.9, Quantity: 2, Side: market.TickSideSell, Timestamp: now.Add(-1 * time.Second)},
		{Symbol: "SOLUSDT", Price: 71.8, Quantity: 3, Side: market.TickSideSell, Timestamp: now},
	}, now)

	disabled := false
	at := &AutoTrader{
		preDecisionTracker: tracker,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				PreDecision: store.PreDecisionConfig{Enabled: true},
				RiskControl: store.RiskControlConfig{OscillationGateEnabled: &disabled},
			},
		},
	}
	decisions := []kernel.Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_long",
		Leverage:        5,
		PositionSizeUSD: 1000,
	}}
	notes := at.sanitizeOpenDecisionsGate(&kernel.Context{}, decisions)
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

func TestSanitizeOpenDecisionsGate_OscillationBlock(t *testing.T) {
	klines := makeOscTestKlines(60, 100)
	bars := make([]market.KlineBar, len(klines))
	for i, k := range klines {
		bars[i] = market.KlineBar{Time: k.OpenTime, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close}
	}
	ctx := &kernel.Context{
		MarketDataMap: map[string]*market.Data{
			"SOLUSDT": {
				TimeframeData: map[string]*market.TimeframeSeriesData{
					"15m": {Timeframe: "15m", Klines: bars},
				},
			},
		},
	}
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Indicators: store.IndicatorConfig{
					Klines: store.KlineConfig{PrimaryTimeframe: "15m"},
				},
				PreDecision: store.PreDecisionConfig{Enabled: false},
			},
		},
	}
	decisions := []kernel.Decision{{Symbol: "SOLUSDT", Action: "open_short", PositionSizeUSD: 500}}
	notes := at.sanitizeOpenDecisionsGate(ctx, decisions)
	if len(notes) == 0 {
		t.Fatal("expected oscillation gate note")
	}
	if decisions[0].Action != "wait" {
		t.Fatalf("expected wait, got %s", decisions[0].Action)
	}
}

func makeOscTestKlines(n int, base float64) []market.Kline {
	out := make([]market.Kline, n)
	for i := range out {
		wave := float64(i%4) * 0.15
		price := base + wave
		out[i] = market.Kline{
			OpenTime: int64(i),
			Open:     price,
			High:     price + 0.2,
			Low:      price - 0.2,
			Close:    price + 0.05,
		}
	}
	return out
}
