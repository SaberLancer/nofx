package trader

import (
	"fmt"
	"nofx/market"
	"nofx/store"
)

// detectMultiTFRegime classifies market using 1H bias + 15m decision desk + optional 3m trigger.
func detectMultiTFRegime(klines1h, klines15m, klines3m []market.Kline, cfg store.RegimeDetectionConfig) market.MultiTFRegimeSnapshot {
	cfg = cfg.Normalize()
	l1h := cfg.Layer1H
	l15 := cfg.Layer15m
	l3m := cfg.Layer3m

	snap := market.MultiTFRegimeSnapshot{Verdict: market.RegimeVerdictInconclusive}

	var bias1h market.MultiTFRegimeVerdict = market.RegimeVerdictInconclusive
	if l1h.Enabled && len(klines1h) >= l1h.ADXPeriod*2 {
		adx1h := market.CalculateADX(klines1h, l1h.ADXPeriod)
		snap.ADX1H = adx1h.ADX
		emaFast := market.ExportCalculateEMA(klines1h, l1h.EMAFast)
		emaSlow := market.ExportCalculateEMA(klines1h, l1h.EMASlow)
		if emaFast > emaSlow*1.001 {
			snap.EMABias1H = "long"
		} else if emaFast < emaSlow*0.999 {
			snap.EMABias1H = "short"
		} else {
			snap.EMABias1H = "flat"
		}
		switch {
		case adx1h.ADX > 0 && adx1h.ADX < l1h.ADXRangingBelow:
			bias1h = market.RegimeVerdictRange
			snap.Reason = fmt.Sprintf("1H ADX=%.1f<%.0f 震荡日", adx1h.ADX, l1h.ADXRangingBelow)
		case adx1h.ADX >= l1h.ADXTrendAbove:
			bias1h = market.RegimeVerdictTrend
			snap.Reason = fmt.Sprintf("1H ADX=%.1f≥%.0f 趋势日 EMA%d/%d=%s", adx1h.ADX, l1h.ADXTrendAbove, l1h.EMAFast, l1h.EMASlow, snap.EMABias1H)
		default:
			snap.Reason = fmt.Sprintf("1H ADX=%.1f 灰区(%.0f–%.0f)", adx1h.ADX, l1h.ADXRangingBelow, l1h.ADXTrendAbove)
		}
	}

	var bias15m market.MultiTFRegimeVerdict = market.RegimeVerdictInconclusive
	if l15.Enabled && len(klines15m) >= l15.ADXPeriod*2 {
		adx15 := market.CalculateADX(klines15m, l15.ADXPeriod)
		snap.ADX15m = adx15.ADX
		snap.BBW15mPct = market.ExportBollingerBandWidthPct(klines15m, l15.BBPeriod, 2)

		osc := market.DetectOscillation(klines15m, cfg.OscillationParams15m())
		emaFast := market.ExportCalculateEMA(klines15m, l15.EMAFast)
		emaSlow := market.ExportCalculateEMA(klines15m, l15.EMASlow)
		emaAligned := (emaFast > emaSlow && snap.EMABias1H != "short") || (emaFast < emaSlow && snap.EMABias1H != "long")

		switch {
		case osc.IsOscillating || snap.BBW15mPct > 0 && snap.BBW15mPct < l15.BBWThinBelowPct:
			bias15m = market.RegimeVerdictRange
			if snap.Reason != "" {
				snap.Reason += " | "
			}
			snap.Reason += fmt.Sprintf("15m ADX=%.1f BBW=%.2f%% 偏震荡", adx15.ADX, snap.BBW15mPct)
		case adx15.ADX >= l15.ADXTrendAbove && emaAligned:
			bias15m = market.RegimeVerdictTrend
			if snap.Reason != "" {
				snap.Reason += " | "
			}
			snap.Reason += fmt.Sprintf("15m ADX=%.1f EMA结构同向 偏趋势", adx15.ADX)
		default:
			if snap.Reason != "" {
				snap.Reason += " | "
			}
			snap.Reason += fmt.Sprintf("15m ADX=%.1f BBW=%.2f%% 未决", adx15.ADX, snap.BBW15mPct)
		}
	}

	bias3m := classifyRegime3mBias(klines3m, l3m, l15, &snap)

	switch {
	case bias1h == market.RegimeVerdictRange:
		snap.Verdict = market.RegimeVerdictRange
		snap.Decisive = true
	case bias1h == market.RegimeVerdictTrend:
		if !l15.Enabled || bias15m != market.RegimeVerdictRange {
			snap.Verdict = market.RegimeVerdictTrend
			snap.Decisive = true
		} else {
			snap.Reason += " | 15m 与 1H 趋势判定冲突，暂不切"
		}
	case bias15m == market.RegimeVerdictRange || bias15m == market.RegimeVerdictTrend:
		snap.Verdict = bias15m
		snap.Decisive = l15.Enabled
	default:
		if l3m.Enabled && bias3m != market.RegimeVerdictInconclusive {
			snap.Verdict = bias3m
			snap.Decisive = true
			if snap.Reason != "" {
				snap.Reason += " | "
			}
			snap.Reason += "1H/15m未决，参考3m扳机"
		} else {
			snap.Decisive = false
			if snap.Reason == "" {
				snap.Reason = "1H/15m/3m 数据不足或未决"
			}
		}
	}

	applyRegime3mTriggerFilter(&snap, l3m, bias3m)

	snap.IsOscillating = snap.Verdict == market.RegimeVerdictRange
	return snap
}

func classifyRegime3mBias(
	klines3m []market.Kline,
	l3m store.RegimeLayer3m,
	l15 store.RegimeLayer15m,
	snap *market.MultiTFRegimeSnapshot,
) market.MultiTFRegimeVerdict {
	if !l3m.Enabled || snap == nil {
		return market.RegimeVerdictInconclusive
	}

	adxPeriod := l15.ADXPeriod
	if adxPeriod <= 0 {
		adxPeriod = 14
	}
	if len(klines3m) < adxPeriod*2 || len(klines3m) < l3m.ATRPeriod+1 {
		return market.RegimeVerdictInconclusive
	}

	adx3m := market.CalculateADX(klines3m, adxPeriod)
	atr3m := market.ExportCalculateATR(klines3m, l3m.ATRPeriod)
	snap.ADX3m = adx3m.ADX
	snap.ATR3m = atr3m

	ema9 := market.ExportCalculateEMA(klines3m, 9)
	lastClose := klines3m[len(klines3m)-1].Close
	switch {
	case lastClose > ema9*1.001:
		snap.TriggerBias3m = "long"
	case lastClose < ema9*0.999:
		snap.TriggerBias3m = "short"
	default:
		snap.TriggerBias3m = "flat"
	}

	rangingBelow := l15.ADXRangingBelow
	trendAbove := l15.ADXTrendAbove
	if rangingBelow <= 0 {
		rangingBelow = 20
	}
	if trendAbove <= 0 {
		trendAbove = 25
	}

	reasonPart := fmt.Sprintf("3m ADX=%.1f ATR=%.4f", adx3m.ADX, atr3m)
	if snap.Reason != "" {
		snap.Reason += " | "
	}
	switch {
	case adx3m.ADX > 0 && adx3m.ADX < rangingBelow:
		snap.Reason += reasonPart + " 微震荡"
		return market.RegimeVerdictRange
	case adx3m.ADX >= trendAbove:
		snap.Reason += fmt.Sprintf("%s 微趋势(%s)", reasonPart, snap.TriggerBias3m)
		return market.RegimeVerdictTrend
	default:
		snap.Reason += reasonPart + " 扳机未决"
		return market.RegimeVerdictInconclusive
	}
}

func applyRegime3mTriggerFilter(
	snap *market.MultiTFRegimeSnapshot,
	l3m store.RegimeLayer3m,
	bias3m market.MultiTFRegimeVerdict,
) {
	if !l3m.Enabled || snap == nil || !snap.Decisive {
		return
	}
	if bias3m == market.RegimeVerdictInconclusive {
		snap.Decisive = false
		snap.Reason += " | 3m 扳机未决，暂不切"
		return
	}
	wantRange := snap.Verdict == market.RegimeVerdictRange
	gotRange := bias3m == market.RegimeVerdictRange
	if wantRange != gotRange {
		snap.Decisive = false
		snap.Reason += " | 3m 与 1H/15m 判定冲突，暂不切"
		return
	}
	snap.Reason += " | 3m扳机同向确认"
}
