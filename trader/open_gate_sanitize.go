package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func (at *AutoTrader) riskControlConfig() store.RiskControlConfig {
	if at == nil || at.config.StrategyConfig == nil {
		return store.RiskControlConfig{}
	}
	return at.config.StrategyConfig.RiskControl
}

func (at *AutoTrader) primaryTimeframe() string {
	if at.config.StrategyConfig != nil {
		if tf := at.config.StrategyConfig.Indicators.Klines.PrimaryTimeframe; tf != "" {
			return tf
		}
	}
	return "15m"
}

// sanitizeOpenDecisionsGate applies oscillation pause and tick-vs-AI open gates before execution.
func (at *AutoTrader) sanitizeOpenDecisionsGate(ctx *kernel.Context, decisions []kernel.Decision) []string {
	if len(decisions) == 0 {
		return nil
	}

	rc := at.riskControlConfig()
	primaryTF := at.primaryTimeframe()
	var notes []string

	for i := range decisions {
		d := &decisions[i]
		if d.Action != "open_long" && d.Action != "open_short" {
			continue
		}

		if rc.EffectiveOscillationGateEnabled() {
			snap := at.detectOscillationForSymbol(ctx, d.Symbol, primaryTF, rc)
			if snap.IsOscillating {
				note := fmt.Sprintf("%s: oscillating market → wait (%s)", d.Symbol, snap.Reason)
				at.logWarnf("⚠️ [Oscillation Gate] %s", note)
				downgradeOpenToWait(d, "oscillation", snap.Reason)
				notes = append(notes, note)
				continue
			}
		}

		if !at.preDecisionEnabled() || at.preDecisionTracker == nil {
			continue
		}
		signal, ok := at.preDecisionTracker.Signal(d.Symbol)
		if !ok {
			continue
		}

		cfg := at.preDecisionConfig()
		level, reason := evaluateTickConflict(d.Action, signal, cfg)
		switch level {
		case tickConflictBlock:
			note := fmt.Sprintf("%s: tick vs AI severe conflict → wait (%s)", d.Symbol, reason)
			at.logWarnf("⚠️ [Tick Conflict] %s", note)
			downgradeOpenToWait(d, "tick conflict", reason)
			notes = append(notes, note)
		case tickConflictReduce:
			ratio := cfg.EffectiveTickConflictReduceRatio()
			oldSize := d.PositionSizeUSD
			d.PositionSizeUSD *= ratio
			note := fmt.Sprintf("%s: tick vs AI mild conflict → size %.0f→%.0f USDT (×%.0f%%, %s)",
				d.Symbol, oldSize, d.PositionSizeUSD, ratio*100, reason)
			at.logWarnf("⚠️ [Tick Conflict] %s", note)
			appendOpenGateReason(d, fmt.Sprintf("size reduced ×%.0f%%: %s", ratio*100, reason))
			notes = append(notes, note)
		}
	}
	return notes
}

func (at *AutoTrader) detectOscillationForSymbol(ctx *kernel.Context, symbol, primaryTF string, rc store.RiskControlConfig) market.OscillationSnapshot {
	if ctx == nil || ctx.MarketDataMap == nil {
		return market.OscillationSnapshot{}
	}
	data, ok := ctx.MarketDataMap[symbol]
	if !ok || data == nil {
		return market.OscillationSnapshot{}
	}

	var tfData *market.TimeframeSeriesData
	if data.TimeframeData != nil {
		if v, ok := data.TimeframeData[primaryTF]; ok && v != nil && len(v.Klines) >= 28 {
			tfData = v
		} else {
			for _, v := range data.TimeframeData {
				if v != nil && len(v.Klines) >= 28 {
					tfData = v
					break
				}
			}
		}
	}
	if tfData == nil || len(tfData.Klines) < 28 {
		return market.OscillationSnapshot{}
	}

	params := market.OscillationParams{
		MaxADX:            rc.EffectiveOscillationMaxADX(),
		MaxRangePct:       rc.EffectiveOscillationMaxRangePct(),
		RangeLookback:     rc.EffectiveOscillationRangeLookback(),
		SwingLookback:     rc.EffectiveOscillationSwingLookback(),
		SwingTolerancePct: market.DefaultOscillationParams().SwingTolerancePct,
		ADXLagMax:         rc.EffectiveOscillationADXLagMax(),
	}
	return market.DetectOscillation(market.KlinesFromBars(tfData.Klines), params)
}

func downgradeOpenToWait(d *kernel.Decision, kind, reason string) {
	prev := d.Action
	d.Action = "wait"
	d.Leverage = 0
	d.PositionSizeUSD = 0
	d.StopLoss = 0
	d.TakeProfit = 0
	d.CloseRatio = 0
	d.RiskUSD = 0
	msg := fmt.Sprintf("CODE: downgraded %s → wait — %s: %s", prev, kind, reason)
	appendOpenGateReason(d, msg)
}

func appendOpenGateReason(d *kernel.Decision, msg string) {
	if d.Reasoning == "" {
		d.Reasoning = msg
	} else {
		d.Reasoning = d.Reasoning + " | " + msg
	}
}
