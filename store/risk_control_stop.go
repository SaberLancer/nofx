package store

// Defaults for price-based stop-loss distance (underlying % move, not margin PnL%).
const (
	DefaultBtcEthMinStopLossDistPct    = 0.5
	DefaultAltcoinMinStopLossDistPct   = 0.8
	DefaultStructStopWickBufferPct     = 0.15
	MinStopLossDistPctFloor            = 0.1
	MaxStopLossDistPctCeil             = 5.0
	MinStructStopWickBufferPctFloor    = 0.05
	MaxStructStopWickBufferPctCeil     = 1.0
)

func (r RiskControlConfig) EffectiveBtcEthMinStopLossDistPct() float64 {
	if r.BtcEthMinStopLossDistPct > 0 {
		return r.BtcEthMinStopLossDistPct
	}
	return DefaultBtcEthMinStopLossDistPct
}

func (r RiskControlConfig) EffectiveAltcoinMinStopLossDistPct() float64 {
	if r.AltcoinMinStopLossDistPct > 0 {
		return r.AltcoinMinStopLossDistPct
	}
	return DefaultAltcoinMinStopLossDistPct
}

func (r RiskControlConfig) EffectiveStructStopWickBufferPct() float64 {
	if r.StructStopWickBufferPct > 0 {
		return r.StructStopWickBufferPct
	}
	return DefaultStructStopWickBufferPct
}

// EffectiveEnforceStructStop defaults to true when unset (zero value).
func (r RiskControlConfig) EffectiveEnforceStructStop() bool {
	if r.EnforceStructStop == nil {
		return true
	}
	return *r.EnforceStructStop
}

func (r RiskControlConfig) OpenProtectionParams() (minRR, btcEthMinSL, altMinSL float64) {
	minRR = r.MinRiskRewardRatio
	if minRR <= 0 {
		minRR = 3.0
	}
	return minRR, r.EffectiveBtcEthMinStopLossDistPct(), r.EffectiveAltcoinMinStopLossDistPct()
}
