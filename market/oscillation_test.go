package market

import "testing"

func TestDetectOscillation_LowADX(t *testing.T) {
	klines := makeChoppyKlines(60, 100)
	snap := DetectOscillation(klines, DefaultOscillationParams())
	if !snap.IsOscillating {
		t.Fatalf("expected oscillating on choppy low-ADX series, reason=%q adx=%.1f", snap.Reason, snap.ADX)
	}
}

func TestDetectOscillation_TrendingWideRange(t *testing.T) {
	klines := makeTrendKlines(60, 100, 2)
	snap := DetectOscillation(klines, DefaultOscillationParams())
	if snap.IsOscillating {
		t.Fatalf("expected trending series not oscillating, reason=%q adx=%.1f range=%.1f", snap.Reason, snap.ADX, snap.RangePct)
	}
}

func makeTrendKlines(n int, start, step float64) []Kline {
	out := make([]Kline, n)
	price := start
	for i := range out {
		hi := price + step*0.4
		lo := price - step*0.1
		out[i] = Kline{
			OpenTime: int64(i),
			Open:     price,
			High:     hi,
			Low:      lo,
			Close:    price + step*0.8,
		}
		price += step
	}
	return out
}
