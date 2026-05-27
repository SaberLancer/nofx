package kernel

import (
	"fmt"
	"nofx/store"
	"strings"
)

// OpenProtectionFromRiskControl builds validation params from strategy risk_control.
func OpenProtectionFromRiskControl(rc store.RiskControlConfig) OpenProtectionParams {
	minRR, btc, alt := rc.OpenProtectionParams()
	return OpenProtectionParams{
		MinRiskRewardRatio:            minRR,
		BtcEthMinStopLossDistancePct:  btc,
		AltcoinMinStopLossDistancePct: alt,
		StopLossEnabled:               rc.EffectiveEnableStopLoss(),
		TakeProfitEnabled:             rc.EffectiveEnableTakeProfit(),
	}
}

// OpenProtectionParams validates stop_loss / take_profit on open actions.
type OpenProtectionParams struct {
	MinRiskRewardRatio            float64
	BtcEthMinStopLossDistancePct  float64
	AltcoinMinStopLossDistancePct float64
	StopLossEnabled               bool
	TakeProfitEnabled             bool
}

func minStopLossDistancePct(symbol string, p OpenProtectionParams) float64 {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "BTCUSDT" || sym == "ETHUSDT" {
		return p.BtcEthMinStopLossDistancePct
	}
	return p.AltcoinMinStopLossDistancePct
}

// ResolveEntryPrice prefers live mark price; falls back to SL/TP midpoint.
func ResolveEntryPrice(action string, stopLoss, takeProfit, marketPrice float64) float64 {
	if marketPrice > 0 {
		return marketPrice
	}
	if stopLoss > 0 && takeProfit > 0 {
		return (stopLoss + takeProfit) / 2
	}
	return 0
}

func stopLossDistancePct(action string, entry, stopLoss float64) float64 {
	if entry <= 0 || stopLoss <= 0 {
		return 0
	}
	switch action {
	case "open_long":
		if stopLoss >= entry {
			return 0
		}
		return (entry - stopLoss) / entry * 100
	case "open_short":
		if stopLoss <= entry {
			return 0
		}
		return (stopLoss - entry) / entry * 100
	default:
		return 0
	}
}

func takeProfitDistancePct(action string, entry, takeProfit float64) float64 {
	if entry <= 0 || takeProfit <= 0 {
		return 0
	}
	switch action {
	case "open_long":
		if takeProfit <= entry {
			return 0
		}
		return (takeProfit - entry) / entry * 100
	case "open_short":
		if takeProfit >= entry {
			return 0
		}
		return (entry - takeProfit) / entry * 100
	default:
		return 0
	}
}

// ValidateOpenProtection enforces min SL distance and min risk-reward on opens.
// Validation is skipped for whichever of SL/TP is disabled in the risk config.
func ValidateOpenProtection(action, symbol string, entry, stopLoss, takeProfit float64, p OpenProtectionParams) error {
	if action != "open_long" && action != "open_short" {
		return nil
	}

	// If both SL and TP are disabled, skip all validation.
	if !p.StopLossEnabled && !p.TakeProfitEnabled {
		return nil
	}

	if entry <= 0 {
		return fmt.Errorf("%s: cannot validate stop_loss without entry price", symbol)
	}

	if p.StopLossEnabled {
		if stopLoss <= 0 {
			return fmt.Errorf("%s: stop_loss is required on open (stop loss is enabled)", symbol)
		}
		slPct := stopLossDistancePct(action, entry, stopLoss)
		minPct := minStopLossDistancePct(symbol, p)
		if slPct <= 0 {
			return fmt.Errorf("%s: stop_loss on wrong side of entry for %s", symbol, action)
		}
		if slPct+1e-9 < minPct {
			return fmt.Errorf("%s: stop_loss distance %.2f%% < minimum %.2f%%", symbol, slPct, minPct)
		}

		if p.TakeProfitEnabled {
			if takeProfit <= 0 {
				return fmt.Errorf("%s: take_profit is required on open (take profit is enabled)", symbol)
			}
			tpPct := takeProfitDistancePct(action, entry, takeProfit)
			if tpPct <= 0 {
				return fmt.Errorf("%s: take_profit on wrong side of entry for %s", symbol, action)
			}
			minRR := p.MinRiskRewardRatio
			if minRR <= 0 {
				minRR = 3.0
			}
			rr := tpPct / slPct
			if rr+1e-9 < minRR {
				return fmt.Errorf("%s: risk-reward %.2f < minimum 1:%.1f (tp %.2f%% / sl %.2f%%)", symbol, rr, minRR, tpPct, slPct)
			}
		}
	}

	return nil
}
