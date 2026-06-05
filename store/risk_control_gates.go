package store

import "math"

func (r RiskControlConfig) EffectivePeakMaxAbsolutePct() float64 {
	if r.PeakMaxAbsolutePct > 0 {
		return r.PeakMaxAbsolutePct
	}
	return 200
}

func (r RiskControlConfig) EffectivePeakMaxJumpPts() float64 {
	if r.PeakMaxJumpPts > 0 {
		return r.PeakMaxJumpPts
	}
	return 30
}

func (r RiskControlConfig) EffectiveOscillationGateEnabled() bool {
	if r.OscillationGateEnabled != nil {
		return *r.OscillationGateEnabled
	}
	return true
}

func (r RiskControlConfig) EffectiveOscillationMaxADX() float64 {
	if r.OscillationMaxADX > 0 {
		return r.OscillationMaxADX
	}
	return 22
}

func (r RiskControlConfig) EffectiveOscillationMaxRangePct() float64 {
	if r.OscillationMaxRangePct > 0 {
		return r.OscillationMaxRangePct
	}
	return 4.5
}

func (r RiskControlConfig) EffectiveOscillationRangeLookback() int {
	if r.OscillationRangeLookback > 0 {
		return r.OscillationRangeLookback
	}
	return 14
}

func (r RiskControlConfig) EffectiveOscillationSwingLookback() int {
	if r.OscillationSwingLookback > 0 {
		return r.OscillationSwingLookback
	}
	return 12
}

func (r RiskControlConfig) EffectiveOscillationADXLagMax() float64 {
	if r.OscillationADXLagMax > 0 {
		return r.OscillationADXLagMax
	}
	return 30
}

func (c PreDecisionConfig) EffectiveTickConflictBlockGap() float64 {
	if c.TickConflictBlockGap > 0 {
		return c.TickConflictBlockGap
	}
	return 0.20
}

func (c PreDecisionConfig) EffectiveTickConflictReduceGap() float64 {
	if c.TickConflictReduceGap > 0 {
		return c.TickConflictReduceGap
	}
	return 0.10
}

func (c PreDecisionConfig) EffectiveTickConflictReduceRatio() float64 {
	if c.TickConflictReduceRatio > 0 && c.TickConflictReduceRatio <= 1 {
		return c.TickConflictReduceRatio
	}
	return 0.5
}

// SanitizePeakPnLPct rejects absurd stored peaks (ghost rows, bad bind) relative to live PnL%.
func SanitizePeakPnLPct(storedPeak, currentPnLPct, maxAbs, maxJump float64) float64 {
	if storedPeak <= 0 {
		return 0
	}
	if maxAbs <= 0 {
		maxAbs = 200
	}
	if maxJump <= 0 {
		maxJump = 30
	}
	if storedPeak > maxAbs {
		return math.Max(currentPnLPct, 0)
	}
	if storedPeak-currentPnLPct > maxJump {
		return math.Max(currentPnLPct, 0)
	}
	return storedPeak
}
