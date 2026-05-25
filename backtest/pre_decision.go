package backtest

import (
	"fmt"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
)

func (r *Runner) initPreDecision() {
	if r.strategyEngine == nil {
		return
	}
	cfg := r.strategyEngine.GetConfig().PreDecision
	if !cfg.Enabled {
		return
	}
	cfg.Normalize()
	r.preDecisionTracker = market.NewTickTrendTracker(market.PreDecisionSettings{
		WindowSec:       cfg.WindowSec,
		MinTicks:        cfg.MinTicks,
		MinBuyPressure:  cfg.MinBuyPressure,
		MinSellPressure: cfg.MinSellPressure,
		MinMomentumPct:  cfg.MinMomentumPct,
	})
}

func (r *Runner) preDecisionConfig() store.PreDecisionConfig {
	if r.strategyEngine == nil {
		return store.PreDecisionConfig{}
	}
	cfg := r.strategyEngine.GetConfig().PreDecision
	cfg.Normalize()
	return cfg
}

func (r *Runner) preDecisionEnabled() bool {
	return r.preDecisionTracker != nil && r.preDecisionConfig().Enabled
}

func (r *Runner) ingestPreDecisionTicks(ctx *kernel.Context, ts int64) {
	if !r.preDecisionEnabled() {
		return
	}
	cfg := r.preDecisionConfig()
	symbols := r.preDecisionWatchSymbols(ctx)
	if len(symbols) == 0 {
		return
	}
	r.preDecisionTracker.TrackSymbols(symbols)

	limit := cfg.MinTicks
	if limit < 100 {
		limit = 100
	}
	refTime := time.UnixMilli(ts).UTC()

	for _, symbol := range symbols {
		ticks, err := market.FetchTicksInWindow("binance", symbol, refTime, cfg.WindowSec, limit)
		if err != nil {
			logger.Infof("📊 Backtest pre-decision: tick fetch failed for %s: %v", symbol, err)
			continue
		}
		r.preDecisionTracker.IngestBatchAt(ticks, refTime)
	}
}

func (r *Runner) preDecisionWatchSymbols(ctx *kernel.Context) []string {
	symbols := make([]string, 0, len(ctx.CandidateCoins)+len(ctx.Positions))
	seen := make(map[string]struct{})
	add := func(symbol string) {
		symbol = market.Normalize(symbol)
		if symbol == "" {
			return
		}
		if _, ok := seen[symbol]; ok {
			return
		}
		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}
	for _, coin := range ctx.CandidateCoins {
		add(coin.Symbol)
	}
	for _, pos := range ctx.Positions {
		add(pos.Symbol)
	}
	if len(symbols) == 0 && r.strategyEngine != nil {
		for _, symbol := range r.strategyEngine.GetConfig().CoinSource.StaticCoins {
			add(symbol)
		}
	}
	return symbols
}

// shouldGateAIByPreDecision mirrors live trader pre-decision gating.
// Returns (gate=true) to skip AI, and a log line for ExecutionLog.
func (r *Runner) shouldGateAIByPreDecision(ctx *kernel.Context) (bool, string) {
	if !r.preDecisionEnabled() {
		return false, ""
	}
	cfg := r.preDecisionConfig()
	if cfg.AlwaysWhenPositions && ctx.Account.PositionCount > 0 {
		return false, ""
	}

	symbols := make([]string, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbols = append(symbols, coin.Symbol)
	}
	signal, ok := r.preDecisionTracker.AnyDirectionalSignal(symbols)
	if ok {
		return false, fmt.Sprintf("pre-decision signal: %s %s buy=%.1f%% sell=%.1f%% momentum=%+.3f%% ticks=%d",
			signal.Symbol, signal.Direction, signal.BuyPressure*100, signal.SellPressure*100, signal.MomentumPct, signal.TickCount)
	}
	if len(symbols) == 0 {
		return true, "pre-decision: no candidate symbols"
	}
	return true, fmt.Sprintf("pre-decision: waiting tick trend (%s)", strings.Join(symbols, ", "))
}
