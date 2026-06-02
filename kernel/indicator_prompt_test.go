package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestConfiguredIndicatorLines_OnlyEnabled(t *testing.T) {
	ind := store.IndicatorConfig{
		Klines: store.KlineConfig{
			PrimaryTimeframe: "3m",
			PrimaryCount:     30,
			SelectedTimeframes: []string{"1m", "3m", "5m"},
		},
		EnableOI: true,
		EnableRSI: false,
		EnableMACD: false,
	}
	lines := configuredIndicatorLines(ind, store.CoinSourceConfig{}, LangChinese)
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "OI") {
		t.Fatalf("expected OI in lines: %s", text)
	}
	if strings.Contains(text, "RSI") || strings.Contains(text, "MACD") {
		t.Fatalf("disabled indicators should not appear: %s", text)
	}
	if !strings.Contains(text, "1m") {
		t.Fatalf("expected selected timeframes: %s", text)
	}
}

func TestGetSchemaPromptForIndicators_OmitsOIWhenDisabled(t *testing.T) {
	ind := store.IndicatorConfig{EnableOI: false, EnableQuantOI: false, EnableVolume: true}
	prompt := GetSchemaPromptForIndicators(LangChinese, &ind)
	if strings.Contains(prompt, "持仓量(OI)变化解读") {
		t.Fatalf("OI interpretation should be omitted when OI disabled")
	}
	if !strings.Contains(prompt, "成交量") {
		t.Fatalf("volume should remain when enabled")
	}
}

func TestBuildSystemPrompt_DynamicEntrySection(t *testing.T) {
	engine := &StrategyEngine{
		config: &store.StrategyConfig{
			RiskControl: store.RiskControlConfig{
				MaxPositions:    3,
				MinPositionSize: 10,
				MinConfidence:   70,
				EnableStopLoss:  boolPtr(false),
				EnableTakeProfit: boolPtr(false),
			},
			Indicators: store.IndicatorConfig{
				Klines: store.KlineConfig{
					PrimaryTimeframe:   "3m",
					SelectedTimeframes: []string{"3m", "5m"},
				},
				EnableOI: true,
			},
		},
	}
	prompt := engine.BuildSystemPrompt("")
	if strings.Contains(prompt, "multiple signals resonate") {
		t.Fatalf("default generic entry standards should be removed")
	}
	if !strings.Contains(prompt, "本周期可用数据") && !strings.Contains(prompt, "Available Data This Strategy") {
		t.Fatalf("expected config-driven data section")
	}
	if strings.Contains(prompt, "RSI") && strings.Contains(prompt, "禁止") {
		// RSI mentioned only in prohibition text is ok
	}
}
