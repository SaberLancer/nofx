package market

import (
	"testing"
	"time"
)

func TestGetKlinesRange_UnknownExchange(t *testing.T) {
	_, err := GetKlinesRange("BTCUSDT", "15m", mustParseTime("2024-01-01"), mustParseTime("2024-01-02"), KlineRangeOptions{
		Exchange: "hyperliquid",
	})
	if err == nil {
		t.Fatal("expected error for unsupported exchange")
	}
}

func TestResolveKlineRangeOptions(t *testing.T) {
	sim := true
	got := ResolveKlineRangeOptions("okx", &sim, false)
	if got.Exchange != "okx" || !got.Simulated {
		t.Fatalf("explicit okx simulated: %+v", got)
	}
	got = ResolveKlineRangeOptions("", nil, true)
	if got.Exchange != "okx" || !got.Simulated {
		t.Fatalf("okx testnet: %+v", got)
	}
}

func mustParseTime(s string) (t time.Time) {
	t, _ = time.Parse("2006-01-02", s)
	return t
}
