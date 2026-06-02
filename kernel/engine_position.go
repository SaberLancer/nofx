package kernel

import (
	"fmt"
	"nofx/logger"
	"strings"
)

// ============================================================================
// Decision Validation
// ============================================================================

// validateDecisions validates all decisions; any failure fails the whole batch (legacy).
func validateDecisions(
	decisions []Decision,
	accountEquity float64,
	btcEthLeverage, altcoinLeverage int,
	btcEthPosRatio, altcoinPosRatio float64,
	protection OpenProtectionParams,
	marketPrices map[string]float64,
) error {
	for i := range decisions {
		if err := validateDecision(&decisions[i], accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio, protection, marketPrices); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

// sanitizeDecisions downgrades invalid per-symbol decisions to wait so other symbols can still execute.
func sanitizeDecisions(
	decisions []Decision,
	accountEquity float64,
	btcEthLeverage, altcoinLeverage int,
	btcEthPosRatio, altcoinPosRatio float64,
	protection OpenProtectionParams,
	marketPrices map[string]float64,
) []string {
	var notes []string
	for i := range decisions {
		if err := validateDecision(&decisions[i], accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio, protection, marketPrices); err != nil {
			sym := decisions[i].Symbol
			prev := decisions[i].Action
			note := fmt.Sprintf("%s: %s validation failed → wait (%v)", sym, prev, err)
			logger.Warnf("⚠️ [Decision Sanitize] %s", note)
			downgradeFailedDecisionToWait(&decisions[i], err.Error())
			notes = append(notes, note)
		}
	}
	return notes
}

func downgradeFailedDecisionToWait(d *Decision, reason string) {
	d.Action = "wait"
	d.Leverage = 0
	d.PositionSizeUSD = 0
	d.StopLoss = 0
	d.TakeProfit = 0
	d.CloseRatio = 0
	d.RiskUSD = 0
	if strings.TrimSpace(d.Reasoning) == "" {
		d.Reasoning = "CODE: downgraded to wait — " + reason
	} else {
		d.Reasoning = d.Reasoning + " | CODE: downgraded to wait — " + reason
	}
}

func validateDecision(
	d *Decision,
	accountEquity float64,
	btcEthLeverage, altcoinLeverage int,
	btcEthPosRatio, altcoinPosRatio float64,
	protection OpenProtectionParams,
	marketPrices map[string]float64,
) error {
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	if d.Action == "open_long" || d.Action == "open_short" {
		maxLeverage := altcoinLeverage
		posRatio := altcoinPosRatio
		maxPositionValue := accountEquity * posRatio
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage
			posRatio = btcEthPosRatio
			maxPositionValue = accountEquity * posRatio
		}

		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			logger.Infof("⚠️  [Leverage Fallback] %s leverage exceeded (%dx > %dx), auto-adjusting to limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		const minPositionSizeGeneral = 12.0
		const minPositionSizeBTCETH = 60.0

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		tolerance := maxPositionValue * 0.01
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH single coin position value cannot exceed %.0f USDT (%.1fx account equity), actual: %.0f", maxPositionValue, posRatio, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("altcoin single coin position value cannot exceed %.0f USDT (%.1fx account equity), actual: %.0f", maxPositionValue, posRatio, d.PositionSizeUSD)
			}
		}
		entryPrice := ResolveEntryPrice(d.Action, d.StopLoss, d.TakeProfit, marketPrices[d.Symbol])
		if err := ValidateOpenProtection(d.Action, d.Symbol, entryPrice, d.StopLoss, d.TakeProfit, protection); err != nil {
			return err
		}
	}

	return nil
}
