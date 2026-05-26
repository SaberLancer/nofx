package kernel

import (
	"strings"
	"testing"
)

func TestFixReasoningPositionPnLPct_replacesPriceChangeWithMarginPnL(t *testing.T) {
	positions := []PositionInfo{{
		Symbol:           "BTCUSDT",
		Side:             "short",
		EntryPrice:       76833.53,
		MarkPrice:        76755.10,
		UnrealizedPnLPct: 0.51,
		Leverage:         5,
	}}

	cot := "当前持仓: BTCUSDT空头, 持仓8小时, 未实现盈亏+0.12% (微利), 峰值PnL为0.00%"
	fixed := FixReasoningPositionPnLPct(cot, positions)
	if fixed == cot {
		t.Fatalf("expected correction, got unchanged: %q", fixed)
	}
	if !strings.Contains(fixed, "+0.51%") {
		t.Fatalf("expected +0.51%% in output, got: %q", fixed)
	}
	if strings.Contains(fixed, "+0.12%") {
		t.Fatalf("should not keep wrong +0.12%%, got: %q", fixed)
	}
}

func TestFixReasoningPositionPnLPct_fixesMislabeledMarginPnL(t *testing.T) {
	positions := []PositionInfo{{
		Symbol:           "BTCUSDT",
		Side:             "long",
		EntryPrice:       77621.32,
		MarkPrice:        77733.30,
		UnrealizedPnLPct: 0.72,
		Leverage:         5,
	}}
	cot := "BTCUSDT多头，Margin PnL%仅+0.14%，远未达到锁盈线+8%"
	fixed := FixReasoningPositionPnLPct(cot, positions)
	if !strings.Contains(fixed, "+0.72%") {
		t.Fatalf("expected +0.72%%, got %q", fixed)
	}
}

func TestFixReasoningPositionPnLPct_skipsWhenAlreadyCorrect(t *testing.T) {
	positions := []PositionInfo{{
		Symbol:           "BTCUSDT",
		Side:             "short",
		EntryPrice:       76833.53,
		MarkPrice:        76755.10,
		UnrealizedPnLPct: 0.51,
	}}
	cot := "BTCUSDT SHORT 未实现盈亏+0.51%"
	fixed := FixReasoningPositionPnLPct(cot, positions)
	if fixed != cot {
		t.Fatalf("expected no change, got %q", fixed)
	}
}
