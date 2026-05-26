package backtest

import (
	"fmt"
	"strings"

	"nofx/logger"
)

// checkStopLossTakeProfit closes positions when simulated SL/TP levels are touched on the decision bar.
// Uses bar high/low when available; falls back to mark price.
func (r *Runner) checkStopLossTakeProfit(ts int64, priceMap map[string]float64, cycle int) ([]TradeEvent, string, error) {
	positions := append([]*position(nil), r.account.Positions()...)
	events := make([]TradeEvent, 0)
	var noteBuilder strings.Builder

	for _, pos := range positions {
		if pos.StopLoss <= 0 && pos.TakeProfit <= 0 {
			continue
		}

		mark := priceMap[pos.Symbol]
		if mark <= 0 {
			continue
		}

		barLow, barHigh := mark, mark
		if curr, _ := r.feed.decisionBarSnapshot(pos.Symbol, ts); curr != nil {
			if curr.Low > 0 {
				barLow = curr.Low
			}
			if curr.High > 0 {
				barHigh = curr.High
			}
		}

		trigger, execPrice, reason := evaluateProtectionTrigger(pos, barLow, barHigh, mark)
		if !trigger {
			continue
		}

		evt, err := r.closePositionAt(pos.Symbol, pos.Side, pos.Quantity, execPrice, mark, ts, cycle, reason)
		if err != nil {
			return nil, "", err
		}
		events = append(events, evt)
		noteBuilder.WriteString(fmt.Sprintf("%s %s %s @ %.4f; ", pos.Symbol, pos.Side, reason, evt.ExitPrice))
		logger.Infof("📊 Backtest [%s] %s %s %s triggered @ %.4f (entry %.4f)",
			r.cfg.RunID, pos.Symbol, pos.Side, reason, evt.ExitPrice, evt.EntryPrice)
	}

	if len(events) == 0 {
		return events, "", nil
	}
	return events, strings.TrimSuffix(noteBuilder.String(), "; "), nil
}

func evaluateProtectionTrigger(pos *position, barLow, barHigh, mark float64) (trigger bool, execPrice float64, reason string) {
	if pos.Side == "long" {
		slHit := pos.StopLoss > 0 && barLow <= pos.StopLoss
		tpHit := pos.TakeProfit > 0 && barHigh >= pos.TakeProfit
		if slHit && tpHit {
			return true, pos.StopLoss, "stop_loss"
		}
		if slHit {
			return true, pos.StopLoss, "stop_loss"
		}
		if tpHit {
			return true, pos.TakeProfit, "take_profit"
		}
		return false, 0, ""
	}

	// short
	slHit := pos.StopLoss > 0 && barHigh >= pos.StopLoss
	tpHit := pos.TakeProfit > 0 && barLow <= pos.TakeProfit
	if slHit && tpHit {
		return true, pos.StopLoss, "stop_loss"
	}
	if slHit {
		return true, pos.StopLoss, "stop_loss"
	}
	if tpHit {
		return true, pos.TakeProfit, "take_profit"
	}
	return false, 0, ""
}

func (r *Runner) closePositionAt(symbol, side string, qty, execPrice, markPrice float64, ts int64, cycle int, closeReason string) (TradeEvent, error) {
	posBefore, _ := r.account.GetPosition(symbol, side)
	entryPrice := 0.0
	if posBefore != nil {
		entryPrice = posBefore.EntryPrice
	}

	posLev := r.account.positionLeverage(symbol, side)
	realized, fee, finalPrice, err := r.account.Close(symbol, side, qty, execPrice)
	if err != nil {
		return TradeEvent{}, err
	}

	action := "close_long"
	if side == "short" {
		action = "close_short"
	}

	slippage := 0.0
	if side == "long" {
		slippage = markPrice - finalPrice
	} else {
		slippage = finalPrice - markPrice
	}

	note := fmt.Sprintf("%s @ %.4f", closeReason, finalPrice)
	return TradeEvent{
		Timestamp:     ts,
		Symbol:        symbol,
		Action:        action,
		Side:          side,
		Quantity:      qty,
		Price:         finalPrice,
		EntryPrice:    entryPrice,
		ExitPrice:     finalPrice,
		CloseReason:   closeReason,
		Fee:           fee,
		Slippage:      slippage,
		OrderValue:    finalPrice * qty,
		RealizedPnL:   realized - fee,
		Leverage:      posLev,
		Cycle:         cycle,
		PositionAfter: r.remainingPosition(symbol, side),
		Note:          note,
	}, nil
}
