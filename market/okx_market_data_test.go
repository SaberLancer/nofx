package market

import "testing"

func TestParseOKXOpenInterestJSON(t *testing.T) {
	body := []byte(`{"code":"0","data":[{"oi":"12345","oiCcy":"678.9","instId":"BTC-USDT-SWAP"}]}`)
	data, err := parseOKXOpenInterestJSON(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if data.Latest != 678.9 {
		t.Fatalf("expected oiCcy 678.9, got %v", data.Latest)
	}
}

func TestParseOKXFundingRateJSON(t *testing.T) {
	body := []byte(`{"code":"0","data":[{"fundingRate":"0.0001","instId":"BTC-USDT-SWAP"}]}`)
	rate, err := parseOKXFundingRateJSON(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if rate != 0.0001 {
		t.Fatalf("expected 0.0001, got %v", rate)
	}
}

func TestFundingRateCacheKey(t *testing.T) {
	key := fundingRateCacheKey("okx", "BTCUSDT")
	if key != "okx:BTCUSDT" {
		t.Fatalf("unexpected cache key: %s", key)
	}
}
