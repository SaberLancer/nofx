package trader

import (
	"fmt"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
)

const (
	protectionMaxAttempts = 3
	protectionRetryDelay    = 500 * time.Millisecond
)

// unprotectedPosition tracks a live position missing exchange SL/TP orders.
type unprotectedPosition struct {
	Symbol       string
	Side         string
	Quantity     float64
	StopLoss     float64
	TakeProfit   float64
	StopLossOK   bool
	TakeProfitOK bool
	LastError    string
	Since        time.Time
}

func (at *AutoTrader) protectionKey(symbol, side string) string {
	return strings.ToUpper(symbol) + "_" + strings.ToLower(side)
}

func (at *AutoTrader) markUnprotected(info unprotectedPosition) {
	if at.unprotectedPositions == nil {
		at.unprotectedPositions = make(map[string]unprotectedPosition)
	}
	at.unprotectedMu.Lock()
	at.unprotectedPositions[at.protectionKey(info.Symbol, info.Side)] = info
	at.unprotectedMu.Unlock()
}

func (at *AutoTrader) clearUnprotected(symbol, side string) {
	at.unprotectedMu.Lock()
	delete(at.unprotectedPositions, at.protectionKey(symbol, side))
	at.unprotectedMu.Unlock()
}

func (at *AutoTrader) isUnprotected(symbol, side string) (unprotectedPosition, bool) {
	at.unprotectedMu.RLock()
	defer at.unprotectedMu.RUnlock()
	info, ok := at.unprotectedPositions[at.protectionKey(symbol, side)]
	return info, ok
}

func (at *AutoTrader) retryCall(label string, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= protectionMaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			logger.Infof("  ⚠ %s attempt %d/%d failed: %v", label, attempt, protectionMaxAttempts, err)
			if attempt < protectionMaxAttempts {
				time.Sleep(protectionRetryDelay)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d attempts: %w", label, protectionMaxAttempts, lastErr)
}

// applyOpenPositionProtection sets exchange SL/TP with retries; marks unprotected on failure.
// Respects EnableStopLoss / EnableTakeProfit from the strategy risk control config.
func (at *AutoTrader) applyOpenPositionProtection(
	decision *kernel.Decision,
	symbol, positionSide string,
	quantity float64,
	actionRecord *store.DecisionAction,
) {
	rc := at.strategyEngine.GetRiskControlConfig()
	slEnabled := rc.EffectiveEnableStopLoss()
	tpEnabled := rc.EffectiveEnableTakeProfit()

	sideLower := strings.ToLower(positionSide)
	// If SL disabled, treat as already OK; same for TP.
	slOK := !slEnabled || decision.StopLoss <= 0
	tpOK := !tpEnabled || decision.TakeProfit <= 0
	var notes []string

	if slEnabled && decision.StopLoss > 0 {
		err := at.retryCall("set stop loss", func() error {
			return at.trader.SetStopLoss(symbol, positionSide, quantity, decision.StopLoss)
		})
		if err != nil {
			slOK = false
			notes = append(notes, fmt.Sprintf("SL: %v", err))
		} else {
			slOK = true
			logger.Infof("  ✓ Stop loss set: %.4f", decision.StopLoss)
		}
	}

	if tpEnabled && decision.TakeProfit > 0 {
		err := at.retryCall("set take profit", func() error {
			return at.trader.SetTakeProfit(symbol, positionSide, quantity, decision.TakeProfit)
		})
		if err != nil {
			tpOK = false
			notes = append(notes, fmt.Sprintf("TP: %v", err))
		} else {
			tpOK = true
			logger.Infof("  ✓ Take profit set: %.4f", decision.TakeProfit)
		}
	}

	if slOK && tpOK {
		at.clearUnprotected(symbol, sideLower)
		return
	}

	note := strings.Join(notes, "; ")
	actionRecord.Unprotected = true
	actionRecord.ProtectionNote = note

	at.markUnprotected(unprotectedPosition{
		Symbol:       symbol,
		Side:         sideLower,
		Quantity:     quantity,
		StopLoss:     decision.StopLoss,
		TakeProfit:   decision.TakeProfit,
		StopLossOK:   slOK,
		TakeProfitOK: tpOK,
		LastError:    note,
		Since:        time.Now().UTC(),
	})

	at.logErrorf("🚨 UNPROTECTED POSITION: %s %s — exchange SL/TP not fully set (%s). Will retry each cycle.",
		symbol, sideLower, note)
}

// retryUnprotectedPositions attempts to attach missing SL/TP on known unprotected positions.
func (at *AutoTrader) retryUnprotectedPositions() {
	at.unprotectedMu.RLock()
	if len(at.unprotectedPositions) == 0 {
		at.unprotectedMu.RUnlock()
		return
	}
	pending := make([]unprotectedPosition, 0, len(at.unprotectedPositions))
	for _, info := range at.unprotectedPositions {
		pending = append(pending, info)
	}
	at.unprotectedMu.RUnlock()

	rc := at.strategyEngine.GetRiskControlConfig()
	slEnabled := rc.EffectiveEnableStopLoss()
	tpEnabled := rc.EffectiveEnableTakeProfit()

	for _, info := range pending {
		posSide := "LONG"
		if strings.ToLower(info.Side) == "short" {
			posSide = "SHORT"
		}

		qty := info.Quantity
		if liveQty, ok := at.livePositionQuantity(info.Symbol, info.Side); ok && liveQty > 0 {
			qty = liveQty
		} else if liveQty == 0 {
			at.clearUnprotected(info.Symbol, info.Side)
			continue
		}

		updated := info
		if slEnabled && !info.StopLossOK && info.StopLoss > 0 {
			if err := at.retryCall("retry stop loss", func() error {
				return at.trader.SetStopLoss(info.Symbol, posSide, qty, info.StopLoss)
			}); err != nil {
				updated.LastError = err.Error()
			} else {
				updated.StopLossOK = true
				at.logInfof("  ✓ Recovered stop loss for %s %s @ %.4f", info.Symbol, info.Side, info.StopLoss)
			}
		} else if !slEnabled {
			updated.StopLossOK = true
		}

		if tpEnabled && !info.TakeProfitOK && info.TakeProfit > 0 {
			if err := at.retryCall("retry take profit", func() error {
				return at.trader.SetTakeProfit(info.Symbol, posSide, qty, info.TakeProfit)
			}); err != nil {
				updated.LastError = err.Error()
			} else {
				updated.TakeProfitOK = true
				at.logInfof("  ✓ Recovered take profit for %s %s @ %.4f", info.Symbol, info.Side, info.TakeProfit)
			}
		} else if !tpEnabled {
			updated.TakeProfitOK = true
		}

		if updated.StopLossOK && updated.TakeProfitOK {
			at.clearUnprotected(info.Symbol, info.Side)
			at.logInfof("🛡️ Position protection restored: %s %s", info.Symbol, info.Side)
			continue
		}
		updated.Quantity = qty
		at.markUnprotected(updated)
	}
}

func (at *AutoTrader) livePositionQuantity(symbol, side string) (float64, bool) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return 0, false
	}
	for _, pos := range positions {
		sym, _ := pos["symbol"].(string)
		if sym != symbol {
			continue
		}
		posSide, _ := pos["side"].(string)
		if strings.ToLower(posSide) != strings.ToLower(side) {
			continue
		}
		qty, _ := pos["positionAmt"].(float64)
		if qty < 0 {
			qty = -qty
		}
		return qty, true
	}
	return 0, true
}
