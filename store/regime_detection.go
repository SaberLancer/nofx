package store

import (
	"encoding/json"

	"nofx/market"
)

// RegimeLayer1H — 【1H 层】大局 Bias：定今日主方向与震荡/趋势日。
type RegimeLayer1H struct {
	Enabled         bool    `json:"enabled"`
	EMAFast         int     `json:"ema_fast"`
	EMASlow         int     `json:"ema_slow"`
	ADXPeriod       int     `json:"adx_period"`
	ADXRangingBelow float64 `json:"adx_ranging_below"` // ADX < 此值 → 震荡日
	ADXTrendAbove   float64 `json:"adx_trend_above"`   // ADX > 此值 → 趋势日
	KlineCount      int     `json:"kline_count"`
}

// RegimeLayer15m — 【15m 层】决策台：灰区确认与结构/波动判断。
type RegimeLayer15m struct {
	Enabled          bool    `json:"enabled"`
	EMAFast          int     `json:"ema_fast"`
	EMASlow          int     `json:"ema_slow"`
	ADXPeriod        int     `json:"adx_period"`
	ADXRangingBelow  float64 `json:"adx_ranging_below"`
	ADXTrendAbove    float64 `json:"adx_trend_above"`
	RSIPeriod        int     `json:"rsi_period"`
	BBPeriod         int     `json:"bb_period"`
	BBWThinBelowPct  float64 `json:"bbw_thin_below_pct"` // 布林带宽低于此 → 偏震荡
	ATRPeriod        int     `json:"atr_period"`
	VolumeMAPeriod   int     `json:"volume_ma_period"`
	MaxRangePct      float64 `json:"max_range_pct"`
	RangeLookback    int     `json:"range_lookback"`
	SwingLookback    int     `json:"swing_lookback"`
	KlineCount       int     `json:"kline_count"`
}

// RegimeLayer3m — 【3m 层】扳机：入场周期与止损参考（写入交易员配置，供策略执行参考）。
type RegimeLayer3m struct {
	Enabled           bool    `json:"enabled"`
	ATRPeriod         int     `json:"atr_period"`
	ATRSLMultiplier   float64 `json:"atr_sl_multiplier"` // 止损 = N × ATR
	UseStructureSL    bool    `json:"use_structure_sl"`    // 亦可放在 15m 结构外+buffer
	StructureTimeframe string `json:"structure_timeframe"` // 结构止损参考周期，默认 15m
	KlineCount        int     `json:"kline_count"`
}

// RegimeDetectionConfig — 交易员第 4 步多周期市场状态检测（与策略模板无关）。
type RegimeDetectionConfig struct {
	Layer1H  RegimeLayer1H  `json:"layer_1h"`
	Layer15m RegimeLayer15m `json:"layer_15m"`
	Layer3m  RegimeLayer3m  `json:"layer_3m"`
}

func DefaultRegimeDetectionConfig() RegimeDetectionConfig {
	return RegimeDetectionConfig{
		Layer1H: RegimeLayer1H{
			Enabled:         true,
			EMAFast:         20,
			EMASlow:         50,
			ADXPeriod:       14,
			ADXRangingBelow: 20,
			ADXTrendAbove:   25,
			KlineCount:      60,
		},
		Layer15m: RegimeLayer15m{
			Enabled:         true,
			EMAFast:         20,
			EMASlow:         50,
			ADXPeriod:       14,
			ADXRangingBelow: 22,
			ADXTrendAbove:   25,
			RSIPeriod:       14,
			BBPeriod:        20,
			BBWThinBelowPct: 3,
			ATRPeriod:       14,
			VolumeMAPeriod:  20,
			MaxRangePct:     4.5,
			RangeLookback:   14,
			SwingLookback:   12,
			KlineCount:      50,
		},
		Layer3m: RegimeLayer3m{
			Enabled:            true,
			ATRPeriod:          14,
			ATRSLMultiplier:    1.5,
			UseStructureSL:     true,
			StructureTimeframe: "15m",
			KlineCount:         40,
		},
	}
}

func (l *RegimeLayer1H) Normalize() {
	def := DefaultRegimeDetectionConfig().Layer1H
	if l.EMAFast <= 0 {
		l.EMAFast = def.EMAFast
	}
	if l.EMASlow <= 0 {
		l.EMASlow = def.EMASlow
	}
	if l.ADXPeriod <= 0 {
		l.ADXPeriod = def.ADXPeriod
	}
	if l.ADXRangingBelow <= 0 {
		l.ADXRangingBelow = def.ADXRangingBelow
	}
	if l.ADXTrendAbove <= 0 {
		l.ADXTrendAbove = def.ADXTrendAbove
	}
	if l.ADXTrendAbove <= l.ADXRangingBelow {
		l.ADXTrendAbove = l.ADXRangingBelow + 5
	}
	if l.KlineCount < 30 {
		l.KlineCount = def.KlineCount
	}
	if l.KlineCount > 200 {
		l.KlineCount = 200
	}
}

func (l *RegimeLayer15m) Normalize() {
	def := DefaultRegimeDetectionConfig().Layer15m
	if l.EMAFast <= 0 {
		l.EMAFast = def.EMAFast
	}
	if l.EMASlow <= 0 {
		l.EMASlow = def.EMASlow
	}
	if l.ADXPeriod <= 0 {
		l.ADXPeriod = def.ADXPeriod
	}
	if l.ADXRangingBelow <= 0 {
		l.ADXRangingBelow = def.ADXRangingBelow
	}
	if l.ADXTrendAbove <= 0 {
		l.ADXTrendAbove = def.ADXTrendAbove
	}
	if l.RSIPeriod <= 0 {
		l.RSIPeriod = def.RSIPeriod
	}
	if l.BBPeriod <= 0 {
		l.BBPeriod = def.BBPeriod
	}
	if l.BBWThinBelowPct <= 0 {
		l.BBWThinBelowPct = def.BBWThinBelowPct
	}
	if l.ATRPeriod <= 0 {
		l.ATRPeriod = def.ATRPeriod
	}
	if l.VolumeMAPeriod <= 0 {
		l.VolumeMAPeriod = def.VolumeMAPeriod
	}
	if l.MaxRangePct <= 0 {
		l.MaxRangePct = def.MaxRangePct
	}
	if l.RangeLookback <= 0 {
		l.RangeLookback = def.RangeLookback
	}
	if l.SwingLookback <= 0 {
		l.SwingLookback = def.SwingLookback
	}
	if l.KlineCount < 28 {
		l.KlineCount = def.KlineCount
	}
	if l.KlineCount > 200 {
		l.KlineCount = 200
	}
}

func (l *RegimeLayer3m) Normalize() {
	def := DefaultRegimeDetectionConfig().Layer3m
	if l.ATRPeriod <= 0 {
		l.ATRPeriod = def.ATRPeriod
	}
	if l.ATRSLMultiplier <= 0 {
		l.ATRSLMultiplier = def.ATRSLMultiplier
	}
	if l.StructureTimeframe == "" {
		l.StructureTimeframe = def.StructureTimeframe
	}
	if _, err := market.NormalizeTimeframe(l.StructureTimeframe); err != nil {
		l.StructureTimeframe = def.StructureTimeframe
	}
	if l.KlineCount < 20 {
		l.KlineCount = def.KlineCount
	}
	if l.KlineCount > 200 {
		l.KlineCount = 200
	}
}

func (c RegimeDetectionConfig) Normalize() RegimeDetectionConfig {
	out := c
	out.Layer1H.Normalize()
	out.Layer15m.Normalize()
	out.Layer3m.Normalize()
	return out
}

func (c RegimeDetectionConfig) OscillationParams15m() market.OscillationParams {
	l := c.Layer15m
	return market.OscillationParams{
		MaxADX:            l.ADXRangingBelow,
		MaxRangePct:       l.MaxRangePct,
		RangeLookback:     l.RangeLookback,
		SwingLookback:     l.SwingLookback,
		SwingTolerancePct: market.DefaultOscillationParams().SwingTolerancePct,
		ADXLagMax:         l.ADXTrendAbove,
	}
}

// legacyFromFlat maps pre-multi-TF flat columns to the new schema.
func legacyFromFlat(t *Trader) RegimeDetectionConfig {
	cfg := DefaultRegimeDetectionConfig()
	if t == nil {
		return cfg
	}
	if t.RegimeDetectionMaxADX > 0 {
		cfg.Layer15m.ADXRangingBelow = t.RegimeDetectionMaxADX
	}
	if t.RegimeDetectionMaxRangePct > 0 {
		cfg.Layer15m.MaxRangePct = t.RegimeDetectionMaxRangePct
	}
	if t.RegimeDetectionRangeLookback > 0 {
		cfg.Layer15m.RangeLookback = t.RegimeDetectionRangeLookback
	}
	if t.RegimeDetectionSwingLookback > 0 {
		cfg.Layer15m.SwingLookback = t.RegimeDetectionSwingLookback
	}
	if t.RegimeDetectionADXLagMax > 0 {
		cfg.Layer15m.ADXTrendAbove = t.RegimeDetectionADXLagMax
	}
	if t.RegimeDetectionKlineCount > 0 {
		cfg.Layer15m.KlineCount = t.RegimeDetectionKlineCount
	}
	return cfg.Normalize()
}

func (t *Trader) RegimeDetection() RegimeDetectionConfig {
	if t == nil {
		return DefaultRegimeDetectionConfig()
	}
	if t.RegimeDetectionJSON != "" {
		var cfg RegimeDetectionConfig
		if err := json.Unmarshal([]byte(t.RegimeDetectionJSON), &cfg); err == nil {
			return cfg.Normalize()
		}
	}
	return legacyFromFlat(t)
}

func (t *Trader) ApplyRegimeDetection(cfg RegimeDetectionConfig) {
	if t == nil {
		return
	}
	norm := cfg.Normalize()
	if data, err := json.Marshal(norm); err == nil {
		t.RegimeDetectionJSON = string(data)
	}
	// Keep legacy flat columns in sync with 15m layer for DB browsers / rollback.
	t.RegimeDetectionTimeframe = "15m"
	t.RegimeDetectionKlineCount = norm.Layer15m.KlineCount
	t.RegimeDetectionMaxADX = norm.Layer15m.ADXRangingBelow
	t.RegimeDetectionMaxRangePct = norm.Layer15m.MaxRangePct
	t.RegimeDetectionRangeLookback = norm.Layer15m.RangeLookback
	t.RegimeDetectionSwingLookback = norm.Layer15m.SwingLookback
	t.RegimeDetectionADXLagMax = norm.Layer15m.ADXTrendAbove
}
