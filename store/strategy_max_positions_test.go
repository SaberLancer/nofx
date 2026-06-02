package store

import "testing"

func TestEffectiveMaxPositions(t *testing.T) {
	t.Run("defaults to candidate count when unset", func(t *testing.T) {
		cfg := StrategyConfig{
			CoinSource: CoinSourceConfig{SourceType: "ai500", AI500Limit: 5},
			RiskControl: RiskControlConfig{MaxPositions: 0},
		}
		if got := cfg.EffectiveMaxPositions(); got != 5 {
			t.Fatalf("expected 5, got %d", got)
		}
	})

	t.Run("uses configured value when lower than candidate count", func(t *testing.T) {
		cfg := StrategyConfig{
			CoinSource: CoinSourceConfig{SourceType: "static", StaticCoins: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}},
			RiskControl: RiskControlConfig{MaxPositions: 2},
		}
		if got := cfg.EffectiveMaxPositions(); got != 2 {
			t.Fatalf("expected 2, got %d", got)
		}
	})

	t.Run("caps at candidate count when configured higher", func(t *testing.T) {
		cfg := StrategyConfig{
			CoinSource: CoinSourceConfig{SourceType: "ai500", AI500Limit: 3},
			RiskControl: RiskControlConfig{MaxPositions: 8},
		}
		if got := cfg.EffectiveMaxPositions(); got != 3 {
			t.Fatalf("expected 3, got %d", got)
		}
	})
}

func TestClampLimitsMaxPositionsAllowsZeroAndUpToCandidateCap(t *testing.T) {
	cfg := StrategyConfig{
		CoinSource: CoinSourceConfig{SourceType: "ai500", AI500Limit: 3},
		RiskControl: RiskControlConfig{MaxPositions: 0},
	}
	cfg.ClampLimits()
	if cfg.RiskControl.MaxPositions != 0 {
		t.Fatalf("expected 0 (auto), got %d", cfg.RiskControl.MaxPositions)
	}

	cfg.RiskControl.MaxPositions = 12
	cfg.ClampLimits()
	if cfg.RiskControl.MaxPositions != MaxCandidateCoins {
		t.Fatalf("expected clamp to %d, got %d", MaxCandidateCoins, cfg.RiskControl.MaxPositions)
	}
}
