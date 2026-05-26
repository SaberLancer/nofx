package store

// Effective PnL% thresholds for reduce/exit (margin-based unrealized PnL %).
// Zero in JSON means "use product default".

func (r RiskControlConfig) EffectiveLockProfitPnLPct() float64 {
	if r.LockProfitPnLPct > 0 {
		return r.LockProfitPnLPct
	}
	return 8
}

func (r RiskControlConfig) EffectiveLockProfitSecondPnLPct() float64 {
	if r.LockProfitSecondPnLPct > 0 {
		return r.LockProfitSecondPnLPct
	}
	return 12
}

func (r RiskControlConfig) EffectiveLockProfitReduceRatio() float64 {
	if r.LockProfitReduceRatio > 0 && r.LockProfitReduceRatio <= 1 {
		return r.LockProfitReduceRatio
	}
	return 0.3
}

func (r RiskControlConfig) EffectiveExitProtectPnLPct() float64 {
	if r.ExitProtectPnLPct > 0 {
		return r.ExitProtectPnLPct
	}
	return 10
}

func (r RiskControlConfig) EffectiveStopLossPnLPct() float64 {
	if r.StopLossPnLPct < 0 {
		return r.StopLossPnLPct
	}
	return -5
}

func (r RiskControlConfig) EffectivePeakMinForPullback() float64 {
	if r.PeakMinForPullback > 0 {
		return r.PeakMinForPullback
	}
	return 10
}

func (r RiskControlConfig) EffectivePeakPullbackPts() float64 {
	if r.PeakPullbackPts > 0 {
		return r.PeakPullbackPts
	}
	return 4
}
