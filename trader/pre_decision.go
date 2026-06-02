package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"strings"
)

func (at *AutoTrader) preDecisionConfig() store.PreDecisionConfig {
	if at == nil || at.config.StrategyConfig == nil {
		return store.PreDecisionConfig{}
	}
	cfg := at.config.StrategyConfig.PreDecision
	cfg.Normalize()
	return cfg
}

func (at *AutoTrader) preDecisionEnabled() bool {
	return at.preDecisionConfig().Enabled
}

func (at *AutoTrader) preDecisionSettings() market.PreDecisionSettings {
	cfg := at.preDecisionConfig()
	return market.PreDecisionSettings{
		WindowSec:       cfg.WindowSec,
		MinTicks:        cfg.MinTicks,
		MinBuyPressure:  cfg.MinBuyPressure,
		MinSellPressure: cfg.MinSellPressure,
		MinMomentumPct:  cfg.MinMomentumPct,
	}
}

func (at *AutoTrader) ensurePreDecisionTracker() {
	if !at.preDecisionEnabled() {
		return
	}
	if at.preDecisionTracker != nil {
		return
	}
	at.preDecisionTracker = market.NewTickTrendTracker(at.preDecisionSettings())
}

func (at *AutoTrader) preDecisionWatchSymbols(ctx *kernel.Context) []string {
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
	if len(symbols) == 0 && at.config.StrategyConfig != nil {
		for _, symbol := range at.config.StrategyConfig.CoinSource.StaticCoins {
			add(symbol)
		}
	}
	return symbols
}

// PreDecisionOutcome controls AI invocation after the tick pre-decision gate.
type PreDecisionOutcome int

const (
	PreDecisionFullAI          PreDecisionOutcome = iota // tick signal → full candidate analysis
	PreDecisionPositionsOnlyAI                           // gate failed, open positions → AI on positions only
	PreDecisionSkipAI                                    // gate failed, flat → skip AI
)

// evaluatePreDecision runs the tick gate. With open positions, pre-decision is always evaluated
// (no bypass). When the gate fails and AlwaysWhenPositions is enabled, AI still runs on position symbols only.
func (at *AutoTrader) evaluatePreDecision(ctx *kernel.Context) (PreDecisionOutcome, string) {
	if !at.preDecisionEnabled() {
		return PreDecisionFullAI, ""
	}
	if at.preDecisionTracker == nil {
		return PreDecisionFullAI, ""
	}

	cfg := at.preDecisionConfig()
	symbols := make([]string, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbols = append(symbols, coin.Symbol)
	}
	signal, ok := at.preDecisionTracker.AnyDirectionalSignal(symbols)
	if ok {
		return PreDecisionFullAI, fmt.Sprintf("pre-decision signal: %s %s buy=%.1f%% sell=%.1f%% momentum=%+.3f%% ticks=%d",
			signal.Symbol, signal.Direction, signal.BuyPressure*100, signal.SellPressure*100, signal.MomentumPct, signal.TickCount)
	}

	waitReason := ""
	if len(symbols) == 0 {
		waitReason = "pre-decision: no candidate symbols"
	} else {
		waitReason = fmt.Sprintf("pre-decision: waiting tick trend (%s)", strings.Join(symbols, ", "))
	}

	hasPositions := len(ctx.Positions) > 0 || ctx.Account.PositionCount > 0
	if hasPositions && cfg.AlwaysWhenPositions {
		return PreDecisionPositionsOnlyAI, waitReason
	}
	return PreDecisionSkipAI, waitReason
}
