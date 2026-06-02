package trader

import (
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func TestEvaluatePreDecision_PositionsOnlyWhenGateFails(t *testing.T) {
	tracker := market.NewTickTrendTracker(market.PreDecisionSettings{
		WindowSec: 60, MinTicks: 5, MinBuyPressure: 0.55, MinSellPressure: 0.55, MinMomentumPct: 0.02,
	})
	at := &AutoTrader{
		preDecisionTracker: tracker,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				PreDecision: store.PreDecisionConfig{
					Enabled:             true,
					AlwaysWhenPositions: true,
				},
			},
		},
	}
	ctx := &kernel.Context{
		Account: kernel.AccountInfo{PositionCount: 1},
		Positions: []kernel.PositionInfo{{Symbol: "ETHUSDT"}},
		CandidateCoins: []kernel.CandidateCoin{
			{Symbol: "BTCUSDT"},
			{Symbol: "ETHUSDT"},
		},
	}
	out, reason := at.evaluatePreDecision(ctx)
	if out != PreDecisionPositionsOnlyAI {
		t.Fatalf("expected positions-only AI, got %v reason=%q", out, reason)
	}
	if !strings.Contains(reason, "waiting tick trend") {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestEvaluatePreDecision_SkipWhenFlatAndGateFails(t *testing.T) {
	tracker := market.NewTickTrendTracker(market.PreDecisionSettings{WindowSec: 60, MinTicks: 5})
	at := &AutoTrader{
		preDecisionTracker: tracker,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				PreDecision: store.PreDecisionConfig{Enabled: true, AlwaysWhenPositions: true},
			},
		},
	}
	ctx := &kernel.Context{
		CandidateCoins: []kernel.CandidateCoin{{Symbol: "BTCUSDT"}},
	}
	out, _ := at.evaluatePreDecision(ctx)
	if out != PreDecisionSkipAI {
		t.Fatalf("expected skip, got %v", out)
	}
}
