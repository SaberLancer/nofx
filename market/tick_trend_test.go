package market

import (
	"testing"
	"time"
)

func TestEvaluateTrendLongSignal(t *testing.T) {
	now := time.Now()
	settings := PreDecisionSettings{
		WindowSec:       60,
		MinTicks:        5,
		MinBuyPressure:  0.55,
		MinSellPressure: 0.55,
		MinMomentumPct:  0.03,
	}.normalized()

	ticks := []RawTick{
		{Symbol: "BTCUSDT", Price: 100, Quantity: 1, Side: TickSideBuy, Timestamp: now},
		{Symbol: "BTCUSDT", Price: 100.2, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(1 * time.Second)},
		{Symbol: "BTCUSDT", Price: 100.4, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(2 * time.Second)},
		{Symbol: "BTCUSDT", Price: 100.5, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(3 * time.Second)},
		{Symbol: "BTCUSDT", Price: 100.6, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(4 * time.Second)},
	}

	signal := evaluateTrend("BTCUSDT", ticks, settings, now.Add(5*time.Second))
	if signal.Direction != TrendLong {
		t.Fatalf("expected long signal, got %+v", signal)
	}
}

func TestEvaluateTrendInsufficientTicks(t *testing.T) {
	now := time.Now()
	settings := PreDecisionSettings{WindowSec: 60, MinTicks: 20}.normalized()
	ticks := []RawTick{
		{Symbol: "BTCUSDT", Price: 100, Quantity: 1, Side: TickSideBuy, Timestamp: now},
	}
	signal := evaluateTrend("BTCUSDT", ticks, settings, now)
	if signal.Direction != TrendNone {
		t.Fatalf("expected no signal, got %+v", signal)
	}
}

func TestTickTrendTrackerAnyDirectionalSignal(t *testing.T) {
	tracker := NewTickTrendTracker(PreDecisionSettings{
		WindowSec: 60, MinTicks: 3, MinBuyPressure: 0.55, MinMomentumPct: 0.01,
	})
	now := time.Now()
	tracker.IngestBatch([]RawTick{
		{Symbol: "ETHUSDT", Price: 2000, Quantity: 1, Side: TickSideBuy, Timestamp: now},
		{Symbol: "ETHUSDT", Price: 2005, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(time.Second)},
		{Symbol: "ETHUSDT", Price: 2010, Quantity: 1, Side: TickSideBuy, Timestamp: now.Add(2 * time.Second)},
	})
	signal, ok := tracker.AnyDirectionalSignal([]string{"ETHUSDT"})
	if !ok || signal.Direction != TrendLong {
		t.Fatalf("expected directional signal, got ok=%v signal=%+v", ok, signal)
	}
}
