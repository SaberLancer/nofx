package market

import "testing"

func TestCalculateADX_InsufficientData(t *testing.T) {
	res := CalculateADX(nil, 14)
	if res.ADX != 0 {
		t.Fatalf("expected zero ADX, got %v", res.ADX)
	}
}

func TestCalculateADX_FlatSeriesLowADX(t *testing.T) {
	klines := makeChoppyKlines(60, 100)
	res := CalculateADX(klines, 14)
	if res.ADX <= 0 || res.ADX >= 25 {
		t.Fatalf("expected low ADX on choppy range series, got %.2f", res.ADX)
	}
}

func makeChoppyKlines(n int, base float64) []Kline {
	out := make([]Kline, n)
	for i := range out {
		wave := float64(i%4) * 0.15
		price := base + wave
		out[i] = Kline{
			OpenTime: int64(i),
			Open:     price,
			High:     price + 0.2,
			Low:      price - 0.2,
			Close:    price + 0.05,
		}
	}
	return out
}

func makeFlatKlines(n int, price float64) []Kline {
	out := make([]Kline, n)
	for i := range out {
		out[i] = Kline{
			OpenTime: int64(i),
			Open:     price,
			High:     price + 0.1,
			Low:      price - 0.1,
			Close:    price,
		}
	}
	return out
}
