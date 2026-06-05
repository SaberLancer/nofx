package market

import (
	"fmt"
	"math"
)

// OscillationParams configures range / ADX oscillation detection on the primary timeframe.
type OscillationParams struct {
	MaxADX            float64 // ADX below this → ranging (default 22)
	MaxRangePct       float64 // narrow range over lookback bars (default 4.5)
	RangeLookback     int     // bars for range / swing scan (default 14)
	SwingLookback     int     // bars for repeated high/low test (default 12)
	SwingTolerancePct float64 // highs/lows within this % count as same level (default 0.6)
	ADXLagMax         float64 // ADX below this AND narrow range → ranging despite ADX lag (default 30)
}

// DefaultOscillationParams returns product defaults tuned for 15m primary TF.
func DefaultOscillationParams() OscillationParams {
	return OscillationParams{
		MaxADX:            22,
		MaxRangePct:       4.5,
		RangeLookback:     14,
		SwingLookback:     12,
		SwingTolerancePct: 0.6,
		ADXLagMax:         30,
	}
}

// OscillationSnapshot describes whether opens should be paused for a symbol.
type OscillationSnapshot struct {
	IsOscillating  bool
	ADX            float64
	PlusDI         float64
	MinusDI        float64
	RangePct       float64
	RepeatedSwings bool
	Reason         string
}

// DetectOscillation analyzes primary-TF klines for low ADX and/or repeated range swings.
func DetectOscillation(klines []Kline, p OscillationParams) OscillationSnapshot {
	if p.MaxADX <= 0 {
		p.MaxADX = DefaultOscillationParams().MaxADX
	}
	if p.MaxRangePct <= 0 {
		p.MaxRangePct = DefaultOscillationParams().MaxRangePct
	}
	if p.RangeLookback <= 0 {
		p.RangeLookback = DefaultOscillationParams().RangeLookback
	}
	if p.SwingLookback <= 0 {
		p.SwingLookback = DefaultOscillationParams().SwingLookback
	}
	if p.SwingTolerancePct <= 0 {
		p.SwingTolerancePct = DefaultOscillationParams().SwingTolerancePct
	}
	if p.ADXLagMax <= 0 {
		p.ADXLagMax = DefaultOscillationParams().ADXLagMax
	}

	adxRes := CalculateADX(klines, 14)
	rangePct := priceRangePct(klines, p.RangeLookback)
	repeated := hasRepeatedSwingLevels(klines, p.SwingLookback, p.SwingTolerancePct)

	snap := OscillationSnapshot{
		ADX:            adxRes.ADX,
		PlusDI:         adxRes.PlusDI,
		MinusDI:        adxRes.MinusDI,
		RangePct:       rangePct,
		RepeatedSwings: repeated,
	}

	switch {
	case adxRes.ADX > 0 && adxRes.ADX < p.MaxADX:
		snap.IsOscillating = true
		snap.Reason = formatOscReason("ADX low", adxRes.ADX, rangePct, repeated)
	case rangePct > 0 && rangePct <= p.MaxRangePct && repeated:
		snap.IsOscillating = true
		snap.Reason = formatOscReason("range swings", adxRes.ADX, rangePct, repeated)
	case rangePct > 0 && rangePct <= p.MaxRangePct && adxRes.ADX > 0 && adxRes.ADX < p.ADXLagMax:
		snap.IsOscillating = true
		snap.Reason = formatOscReason("narrow range + ADX lag", adxRes.ADX, rangePct, repeated)
	}
	return snap
}

func formatOscReason(kind string, adx, rangePct float64, repeated bool) string {
	msg := fmt.Sprintf("%s: ADX=%.1f range=%.1f%%", kind, adx, rangePct)
	if repeated {
		msg += " repeated_swings"
	}
	return msg
}

func priceRangePct(klines []Kline, lookback int) float64 {
	if len(klines) == 0 || lookback <= 0 {
		return 0
	}
	start := len(klines) - lookback
	if start < 0 {
		start = 0
	}
	window := klines[start:]
	hi, lo := window[0].High, window[0].Low
	for _, k := range window {
		if k.High > hi {
			hi = k.High
		}
		if k.Low < lo {
			lo = k.Low
		}
	}
	mid := window[len(window)-1].Close
	if mid <= 0 {
		return 0
	}
	return (hi - lo) / mid * 100
}

func hasRepeatedSwingLevels(klines []Kline, lookback int, tolerancePct float64) bool {
	if len(klines) < lookback+2 || tolerancePct <= 0 {
		return false
	}
	window := klines[len(klines)-lookback:]
	var highs, lows []float64
	for i := 1; i < len(window)-1; i++ {
		if window[i].High >= window[i-1].High && window[i].High >= window[i+1].High {
			highs = append(highs, window[i].High)
		}
		if window[i].Low <= window[i-1].Low && window[i].Low <= window[i+1].Low {
			lows = append(lows, window[i].Low)
		}
	}
	return countSimilarLevels(highs, tolerancePct) >= 2 && countSimilarLevels(lows, tolerancePct) >= 2
}

func countSimilarLevels(levels []float64, tolerancePct float64) int {
	if len(levels) < 2 {
		return len(levels)
	}
	maxCluster := 1
	for i := 0; i < len(levels); i++ {
		cluster := 1
		for j := i + 1; j < len(levels); j++ {
			if levelsClose(levels[i], levels[j], tolerancePct) {
				cluster++
			}
		}
		if cluster > maxCluster {
			maxCluster = cluster
		}
	}
	return maxCluster
}

func levelsClose(a, b, tolerancePct float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	mid := (a + b) / 2
	return math.Abs(a-b)/mid*100 <= tolerancePct
}

// KlinesFromBars converts stored timeframe bars to full kline slice for indicators.
func KlinesFromBars(bars []KlineBar) []Kline {
	out := make([]Kline, len(bars))
	for i, b := range bars {
		out[i] = Kline{
			OpenTime:  b.Time,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			Volume:    b.Volume,
			CloseTime: b.Time,
		}
	}
	return out
}
