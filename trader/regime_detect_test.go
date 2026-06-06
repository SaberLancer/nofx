package trader

import (
	"nofx/market"
	"nofx/store"
	"testing"
)

func makeFlatKlines(n int, price float64) []market.Kline {
	out := make([]market.Kline, n)
	for i := range out {
		out[i] = market.Kline{
			Open:  price,
			High:  price + 1,
			Low:   price - 1,
			Close: price,
		}
	}
	return out
}

func makeOscillatingKlines(n int, base float64, swing float64) []market.Kline {
	out := make([]market.Kline, n)
	for i := range out {
		offset := swing
		if i%2 == 0 {
			offset = -swing
		}
		close := base + offset
		out[i] = market.Kline{
			Open:  close - 0.2,
			High:  close + 0.5,
			Low:   close - 0.5,
			Close: close,
		}
	}
	return out
}

func TestDetectMultiTFRegime3mConfirmsRange(t *testing.T) {
	cfg := store.DefaultRegimeDetectionConfig()
	cfg.Layer1H.Enabled = false
	cfg.Layer15m.Enabled = false
	cfg.Layer3m.Enabled = true

	klines3m := makeOscillatingKlines(80, 100, 0.8)
	snap := detectMultiTFRegime(nil, nil, klines3m, cfg)
	if !snap.Decisive {
		t.Fatalf("expected decisive from 3m-only flat market, got reason=%q", snap.Reason)
	}
	if snap.Verdict != market.RegimeVerdictRange {
		t.Fatalf("verdict=%s want range", snap.Verdict)
	}
	if snap.ADX3m <= 0 {
		t.Fatalf("expected ADX3m populated")
	}
	if snap.ATR3m <= 0 {
		t.Fatalf("expected ATR3m populated")
	}
}

func TestDetectMultiTFRegime3mBlocksConflict(t *testing.T) {
	cfg := store.DefaultRegimeDetectionConfig()
	cfg.Layer1H.ADXRangingBelow = 5
	cfg.Layer1H.ADXTrendAbove = 50
	cfg.Layer3m.Enabled = true

	trend1h := make([]market.Kline, 80)
	for i := range trend1h {
		p := 100 + float64(i)*2
		trend1h[i] = market.Kline{Open: p, High: p + 2, Low: p - 1, Close: p + 1}
	}
	flat3m := makeFlatKlines(80, 100)

	snap := detectMultiTFRegime(trend1h, nil, flat3m, cfg)
	if snap.Decisive {
		t.Fatalf("expected 3m range to block 1H trend verdict, reason=%q", snap.Reason)
	}
}
