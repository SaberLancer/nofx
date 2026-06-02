package kernel

import (
	"strings"
	"testing"
	"time"
)

func TestFormatBeijingDateTime(t *testing.T) {
	tm := time.Date(2026, 5, 31, 22, 11, 50, 0, time.UTC)
	got := FormatBeijingDateTime(tm)
	if got != "2026-06-01 06:11:50" {
		t.Fatalf("expected Beijing datetime, got %s", got)
	}
}

func TestFormatDecisionContextTime_BeijingOnly(t *testing.T) {
	tm := time.Date(2026, 5, 31, 22, 11, 50, 0, time.UTC)
	got := FormatDecisionContextTime(tm, LangChinese)
	if !strings.Contains(got, "北京时间 2026-06-01 06:11:50") {
		t.Fatalf("unexpected: %s", got)
	}
	if strings.Contains(got, "UTC") {
		t.Fatalf("should not contain UTC: %s", got)
	}
}

func TestFormatBeijingKlineTimeMs(t *testing.T) {
	ms := time.Date(2026, 5, 31, 22, 11, 0, 0, time.UTC).UnixMilli()
	got := FormatBeijingKlineTimeMs(ms)
	if got != "06-01 06:11" {
		t.Fatalf("expected 06-01 06:11, got %s", got)
	}
}
