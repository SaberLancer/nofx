package kernel

import (
	"crypto/sha256"
	"testing"

	"nofx/store"
)

func TestBuildSystemPrompt_ByteStableAcrossCalls(t *testing.T) {
	e := &StrategyEngine{
		config: &store.StrategyConfig{
			RiskControl: store.RiskControlConfig{
				MaxPositions:    3,
				MinPositionSize: 10,
				MinConfidence:   70,
			},
			Indicators: store.IndicatorConfig{
				Klines:       store.KlineConfig{PrimaryTimeframe: "3m"},
				EnableVolume: true,
				EnableOI:     true,
			},
		},
	}
	ref := sha256.Sum256([]byte(e.BuildSystemPrompt("")))
	for i := 0; i < 30; i++ {
		h := sha256.Sum256([]byte(e.BuildSystemPrompt("")))
		if h != ref {
			t.Fatalf("system prompt changed between call 0 and %d (schema map order?)", i)
		}
	}
}
