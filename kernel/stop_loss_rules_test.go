package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func boolPtr(b bool) *bool { return &b }

func TestAppendConfiguredStopLossRules_IncludesReasoningGuardZH(t *testing.T) {
	var sb strings.Builder
	rc := store.RiskControlConfig{}
	AppendConfiguredStopLossRules(&sb, rc, LangChinese, "3m", "15m")
	out := sb.String()
	if !strings.Contains(out, "止损价距越大") {
		t.Fatalf("expected SL distance reasoning guard in Chinese prompt")
	}
	if !strings.Contains(out, "禁止") || !strings.Contains(out, "更容易被扫") {
		t.Fatalf("expected contradiction guard in Chinese prompt")
	}
}

func TestAppendConfiguredStopLossRules_SkipsWhenDisabled(t *testing.T) {
	var sb strings.Builder
	rc := store.RiskControlConfig{
		EnableStopLoss:   boolPtr(false),
		EnableTakeProfit: boolPtr(false),
	}
	AppendConfiguredStopLossRules(&sb, rc, LangChinese, "3m", "15m")
	out := sb.String()
	if strings.Contains(out, "最小止损价距") && strings.Contains(out, "BTCUSDT") {
		t.Fatalf("should not inject min SL distance rules when SL/TP disabled")
	}
	if !strings.Contains(out, "策略已关闭") {
		t.Fatalf("expected disabled notice in Chinese prompt")
	}
}

func TestAppendConfiguredStopLossRules_IncludesReasoningGuardEN(t *testing.T) {
	var sb strings.Builder
	rc := store.RiskControlConfig{}
	AppendConfiguredStopLossRules(&sb, rc, LangEnglish, "3m", "15m")
	out := sb.String()
	if !strings.Contains(out, "Larger SL distance") {
		t.Fatalf("expected SL distance reasoning guard in English prompt")
	}
}
