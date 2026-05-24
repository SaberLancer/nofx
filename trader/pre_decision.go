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

func (at *AutoTrader) shouldGateAIByPreDecision(ctx *kernel.Context) (bool, string) {
	if !at.preDecisionEnabled() {
		return false, ""
	}
	cfg := at.preDecisionConfig()
	if cfg.AlwaysWhenPositions && ctx.Account.PositionCount > 0 {
		return false, ""
	}
	if at.preDecisionTracker == nil {
		return false, ""
	}

	symbols := make([]string, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbols = append(symbols, coin.Symbol)
	}
	signal, ok := at.preDecisionTracker.AnyDirectionalSignal(symbols)
	if ok {
		return false, fmt.Sprintf("pre-decision signal: %s %s buy=%.1f%% sell=%.1f%% momentum=%+.3f%% ticks=%d",
			signal.Symbol, signal.Direction, signal.BuyPressure*100, signal.SellPressure*100, signal.MomentumPct, signal.TickCount)
	}
	if len(symbols) == 0 {
		return true, "pre-decision: no candidate symbols"
	}
	return true, fmt.Sprintf("pre-decision: waiting tick trend (%s)", strings.Join(symbols, ", "))
}
