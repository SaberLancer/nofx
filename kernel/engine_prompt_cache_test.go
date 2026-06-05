package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestBuildSystemPrompt_StableAcrossEquity(t *testing.T) {
	cfg := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MaxPositions:                 3,
			MinPositionSize:              10,
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 5,
			MinConfidence:                70,
		},
		Indicators: store.IndicatorConfig{
			Klines: store.KlineConfig{PrimaryTimeframe: "3m"},
		},
	}
	engine := &StrategyEngine{config: cfg}

	prompt := engine.BuildSystemPrompt("")
	if strings.Contains(prompt, "= equity ") {
		t.Fatalf("system prompt should use ratios only, not inline equity USDT caps")
	}
	if !strings.Contains(prompt, "Current Session Limits") && !strings.Contains(prompt, "本周期仓位上限") {
		t.Fatalf("system prompt should reference user-message session limits section")
	}
}

func TestBuildUserPrompt_IncludesSessionLimits(t *testing.T) {
	engine := &StrategyEngine{
		config: &store.StrategyConfig{
			RiskControl: store.RiskControlConfig{
				BTCETHMaxPositionValueRatio:  5,
				AltcoinMaxPositionValueRatio: 5,
			},
			Indicators: store.IndicatorConfig{
				Klines: store.KlineConfig{PrimaryTimeframe: "3m"},
			},
		},
	}
	ctx := &Context{
		Account: AccountInfo{TotalEquity: 10000},
	}
	user := engine.BuildUserPrompt(ctx)
	if !strings.Contains(user, "10000.00 USDT") {
		t.Fatalf("user prompt should include equity in session limits: %s", truncateStr(user, 800))
	}
	if !strings.Contains(user, "50000 USDT") {
		t.Fatalf("user prompt should include BTC/ETH cap 10000*5: %s", truncateStr(user, 800))
	}
	limitsIdx := strings.Index(user, "Current Session Limits")
	if limitsIdx < 0 {
		limitsIdx = strings.Index(user, "本周期仓位上限")
	}
	if limitsIdx < 0 {
		t.Fatal("missing session limits heading")
	}
	// Session limits should appear before candidate coins / large market blocks
	if strings.Contains(user, "Candidate Coins") {
		candIdx := strings.Index(user, "Candidate Coins")
		if limitsIdx > candIdx {
			t.Fatal("session limits should appear before candidate coins section")
		}
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
