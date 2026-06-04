package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

const tickConflictPressureGap = 0.20

// sanitizeOpenDecisionsAgainstTick downgrades open_long/open_short to wait when tick flow
// strongly conflicts with AI direction (e.g. 95% sell pressure but AI opens long).
func (at *AutoTrader) sanitizeOpenDecisionsAgainstTick(decisions []kernel.Decision) []string {
	if !at.preDecisionEnabled() || at.preDecisionTracker == nil || len(decisions) == 0 {
		return nil
	}

	cfg := at.preDecisionConfig()
	var notes []string
	for i := range decisions {
		d := &decisions[i]
		if d.Action != "open_long" && d.Action != "open_short" {
			continue
		}
		signal, ok := at.preDecisionTracker.Signal(d.Symbol)
		if !ok {
			continue
		}
		if conflict, reason := tickConflictsWithOpen(d.Action, signal, cfg); conflict {
			note := fmt.Sprintf("%s: tick vs AI conflict → wait (%s)", d.Symbol, reason)
			at.logWarnf("⚠️ [Tick Conflict] %s", note)
			kernelDowngradeToWait(d, reason)
			notes = append(notes, note)
		}
	}
	return notes
}

func tickConflictsWithOpen(action string, signal market.TrendSignal, cfg store.PreDecisionConfig) (bool, string) {
	minOpp := cfg.MinSellPressure
	if cfg.MinBuyPressure > minOpp {
		minOpp = cfg.MinBuyPressure
	}
	if minOpp < 0.60 {
		minOpp = 0.60
	}

	switch action {
	case "open_long":
		if signal.Direction == market.TrendShort {
			return true, fmt.Sprintf("tick short sell=%.1f%% momentum=%+.3f%%",
				signal.SellPressure*100, signal.MomentumPct)
		}
		if signal.SellPressure >= minOpp && signal.SellPressure-signal.BuyPressure >= tickConflictPressureGap {
			return true, fmt.Sprintf("tick sell pressure %.1f%% > buy %.1f%%",
				signal.SellPressure*100, signal.BuyPressure*100)
		}
	case "open_short":
		if signal.Direction == market.TrendLong {
			return true, fmt.Sprintf("tick long buy=%.1f%% momentum=%+.3f%%",
				signal.BuyPressure*100, signal.MomentumPct)
		}
		if signal.BuyPressure >= minOpp && signal.BuyPressure-signal.SellPressure >= tickConflictPressureGap {
			return true, fmt.Sprintf("tick buy pressure %.1f%% > sell %.1f%%",
				signal.BuyPressure*100, signal.SellPressure*100)
		}
	}
	return false, ""
}

func kernelDowngradeToWait(d *kernel.Decision, reason string) {
	prev := d.Action
	d.Action = "wait"
	d.Leverage = 0
	d.PositionSizeUSD = 0
	d.StopLoss = 0
	d.TakeProfit = 0
	d.CloseRatio = 0
	d.RiskUSD = 0
	msg := fmt.Sprintf("CODE: downgraded %s → wait — tick conflict: %s", prev, reason)
	if d.Reasoning == "" {
		d.Reasoning = msg
	} else {
		d.Reasoning = d.Reasoning + " | " + msg
	}
}
