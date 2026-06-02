package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestBuildSystemPrompt_OmitsOpenProtectionWhenSLTPDisabled(t *testing.T) {
	engine := &StrategyEngine{
		config: &store.StrategyConfig{
			RiskControl: store.RiskControlConfig{
				MaxPositions:                 3,
				MinPositionSize:              10,
				EnableStopLoss:               boolPtr(false),
				EnableTakeProfit:             boolPtr(false),
				BtcEthMinStopLossDistPct:     0.5,
				AltcoinMinStopLossDistPct:    0.95,
				MinRiskRewardRatio:           2,
			},
			Indicators: store.IndicatorConfig{
				Klines: store.KlineConfig{PrimaryTimeframe: "3m"},
			},
		},
	}

	prompt := engine.BuildSystemPrompt("")
	if strings.Contains(prompt, "CODE ENFORCED (Open protection") {
		t.Fatalf("should not include open protection CODE ENFORCED when SL/TP disabled")
	}
	if !strings.Contains(prompt, "do **not** include stop_loss") && !strings.Contains(prompt, "勿") {
		t.Fatalf("expected omit stop_loss instruction in prompt")
	}
	if strings.Contains(prompt, "Min risk-reward on open") {
		t.Fatalf("should not mention min risk-reward when SL/TP disabled")
	}
}
