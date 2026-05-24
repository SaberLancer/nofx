package market

import (
	"sort"
	"sync"
	"time"
)

// PreDecisionSettings controls tick trend tracking and AI gating thresholds.
type PreDecisionSettings struct {
	WindowSec       int
	MinTicks        int
	MinBuyPressure  float64
	MinSellPressure float64
	MinMomentumPct  float64
}

func (s PreDecisionSettings) normalized() PreDecisionSettings {
	out := s
	if out.WindowSec <= 0 {
		out.WindowSec = 60
	}
	if out.MinTicks <= 0 {
		out.MinTicks = 20
	}
	if out.MinBuyPressure <= 0 {
		out.MinBuyPressure = 0.55
	}
	if out.MinSellPressure <= 0 {
		out.MinSellPressure = 0.55
	}
	if out.MinMomentumPct <= 0 {
		out.MinMomentumPct = 0.03
	}
	return out
}

// TickTrendTracker maintains rolling TICK windows and emits directional signals.
type TickTrendTracker struct {
	mu       sync.RWMutex
	settings PreDecisionSettings
	ticks    map[string][]RawTick
	signals  map[string]TrendSignal
}

func NewTickTrendTracker(settings PreDecisionSettings) *TickTrendTracker {
	return &TickTrendTracker{
		settings: settings.normalized(),
		ticks:    make(map[string][]RawTick),
		signals:  make(map[string]TrendSignal),
	}
}

func (t *TickTrendTracker) Settings() PreDecisionSettings {
	return t.settings
}

func (t *TickTrendTracker) TrackSymbols(symbols []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, symbol := range symbols {
		symbol = Normalize(symbol)
		if _, ok := t.ticks[symbol]; !ok {
			t.ticks[symbol] = nil
		}
	}
}

func (t *TickTrendTracker) IngestBatch(batch []RawTick) {
	if len(batch) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(t.settings.WindowSec) * time.Second)

	for _, tick := range batch {
		symbol := Normalize(tick.Symbol)
		buf := append(t.ticks[symbol], tick)
		buf = pruneTicks(buf, cutoff)
		t.ticks[symbol] = buf
		t.signals[symbol] = evaluateTrend(symbol, buf, t.settings, now)
	}
}

func pruneTicks(buf []RawTick, cutoff time.Time) []RawTick {
	out := buf[:0]
	for _, tick := range buf {
		if tick.Timestamp.After(cutoff) {
			out = append(out, tick)
		}
	}
	return out
}

func evaluateTrend(symbol string, ticks []RawTick, settings PreDecisionSettings, now time.Time) TrendSignal {
	signal := TrendSignal{
		Symbol:        symbol,
		Direction:     TrendNone,
		WindowSeconds: settings.WindowSec,
		TickCount:     len(ticks),
		UpdatedAt:     now,
	}
	if len(ticks) < settings.MinTicks {
		return signal
	}

	sort.Slice(ticks, func(i, j int) bool {
		return ticks[i].Timestamp.Before(ticks[j].Timestamp)
	})

	first := ticks[0].Price
	last := ticks[len(ticks)-1].Price
	if first <= 0 {
		return signal
	}
	signal.MomentumPct = (last - first) / first * 100

	var buyVol, sellVol float64
	for _, tick := range ticks {
		switch tick.Side {
		case TickSideBuy:
			buyVol += tick.Quantity * tick.Price
		case TickSideSell:
			sellVol += tick.Quantity * tick.Price
		}
	}
	total := buyVol + sellVol
	if total <= 0 {
		return signal
	}
	signal.BuyPressure = buyVol / total
	signal.SellPressure = sellVol / total

	longOK := signal.BuyPressure >= settings.MinBuyPressure && signal.MomentumPct >= settings.MinMomentumPct
	shortOK := signal.SellPressure >= settings.MinSellPressure && signal.MomentumPct <= -settings.MinMomentumPct
	switch {
	case longOK && shortOK:
		if signal.MomentumPct >= 0 {
			signal.Direction = TrendLong
		} else {
			signal.Direction = TrendShort
		}
	case longOK:
		signal.Direction = TrendLong
	case shortOK:
		signal.Direction = TrendShort
	}
	return signal
}

func (t *TickTrendTracker) Signal(symbol string) (TrendSignal, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	signal, ok := t.signals[Normalize(symbol)]
	return signal, ok
}

func (t *TickTrendTracker) AnyDirectionalSignal(symbols []string) (TrendSignal, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var best TrendSignal
	found := false
	for _, symbol := range symbols {
		signal, ok := t.signals[Normalize(symbol)]
		if !ok || signal.Direction == TrendNone {
			continue
		}
		if !found || abs(signal.MomentumPct) > abs(best.MomentumPct) {
			best = signal
			found = true
		}
	}
	return best, found
}

func (t *TickTrendTracker) Snapshot() map[string]TrendSignal {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]TrendSignal, len(t.signals))
	for k, v := range t.signals {
		out[k] = v
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
