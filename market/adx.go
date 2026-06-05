package market

import "math"

// ADXResult holds Wilder ADX(period) and directional indicators at the last bar.
type ADXResult struct {
	ADX    float64
	PlusDI float64
	MinusDI float64
}

// CalculateADX computes ADX(period) on ascending OHLC klines. Returns zero values if insufficient data.
func CalculateADX(klines []Kline, period int) ADXResult {
	if period <= 0 {
		period = 14
	}
	n := len(klines)
	if n < period*2 {
		return ADXResult{}
	}

	tr := make([]float64, n)
	pdm := make([]float64, n)
	mdm := make([]float64, n)
	for i := 1; i < n; i++ {
		up := klines[i].High - klines[i-1].High
		down := klines[i-1].Low - klines[i].Low
		if up > down && up > 0 {
			pdm[i] = up
		}
		if down > up && down > 0 {
			mdm[i] = down
		}
		h, l, pc := klines[i].High, klines[i].Low, klines[i-1].Close
		tr[i] = math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
	}

	sumTR := 0.0
	sumPDM := 0.0
	sumMDM := 0.0
	for i := 1; i <= period; i++ {
		sumTR += tr[i]
		sumPDM += pdm[i]
		sumMDM += mdm[i]
	}

	atr := sumTR
	sp := sumPDM
	sm := sumMDM

	dxBuf := make([]float64, 0, n-period)
	var lastPlusDI, lastMinusDI float64

	for i := period; i < n; i++ {
		if i > period {
			atr = atr - atr/float64(period) + tr[i]
			sp = sp - sp/float64(period) + pdm[i]
			sm = sm - sm/float64(period) + mdm[i]
		}
		var dx float64
		if atr == 0 {
			lastPlusDI, lastMinusDI = 0, 0
		} else {
			lastPlusDI = 100 * sp / atr
			lastMinusDI = 100 * sm / atr
			denom := lastPlusDI + lastMinusDI
			if denom > 0 {
				dx = 100 * math.Abs(lastPlusDI-lastMinusDI) / denom
			}
		}
		dxBuf = append(dxBuf, dx)
	}

	if len(dxBuf) < period {
		return ADXResult{}
	}

	adx := 0.0
	for i := 0; i < period; i++ {
		adx += dxBuf[i]
	}
	adx /= float64(period)
	for i := period; i < len(dxBuf); i++ {
		adx = (adx*float64(period-1) + dxBuf[i]) / float64(period)
	}

	return ADXResult{ADX: adx, PlusDI: lastPlusDI, MinusDI: lastMinusDI}
}
