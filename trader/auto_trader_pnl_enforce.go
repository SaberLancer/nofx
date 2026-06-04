package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	"strings"
	"time"
)

const (
	pnlEnforceTierStop       = 100
	pnlEnforceTierPeakPull   = 90
	pnlEnforceTierLock2      = 2
	pnlEnforceTierLock1      = 1
	pnlEnforceLock2CloseRatio = 0.4
)

// pnlEnforcePlan is the outcome of comparing live Margin PnL% to strategy thresholds.
type pnlEnforcePlan struct {
	CloseRatio float64 // 0 = close all
	Tier       int
	Rule       string
}

func evaluatePositionPnLAction(pnl, peak float64, appliedTier int, rc store.RiskControlConfig) (pnlEnforcePlan, bool) {
	stopP := rc.EffectiveStopLossPnLPct()
	if pnl <= stopP {
		return pnlEnforcePlan{
			CloseRatio: 0,
			Tier:       pnlEnforceTierStop,
			Rule:       fmt.Sprintf("stop_loss: pnl %+.2f%% <= %+.1f%%", pnl, stopP),
		}, true
	}

	peakMin := rc.EffectivePeakMinForPullback()
	pullPts := rc.EffectivePeakPullbackPts()
	if peak >= peakMin && peak-pnl >= pullPts {
		return pnlEnforcePlan{
			CloseRatio: 0,
			Tier:       pnlEnforceTierPeakPull,
			Rule:       fmt.Sprintf("peak_pullback: peak %+.2f%% → %+.2f%% (%.1f pp ≥ %.1f)", peak, pnl, peak-pnl, pullPts),
		}, true
	}

	lock2 := rc.EffectiveLockProfitSecondPnLPct()
	if pnl >= lock2 && appliedTier < pnlEnforceTierLock2 {
		return pnlEnforcePlan{
			CloseRatio: pnlEnforceLock2CloseRatio,
			Tier:       pnlEnforceTierLock2,
			Rule:       fmt.Sprintf("lock_tier2: pnl %+.2f%% >= %+.1f%%", pnl, lock2),
		}, true
	}

	lock1 := rc.EffectiveLockProfitPnLPct()
	ratio := rc.EffectiveLockProfitReduceRatio()
	if pnl >= lock1 && appliedTier < pnlEnforceTierLock1 {
		return pnlEnforcePlan{
			CloseRatio: ratio,
			Tier:       pnlEnforceTierLock1,
			Rule:       fmt.Sprintf("lock_tier1: pnl %+.2f%% >= %+.1f%%", pnl, lock1),
		}, true
	}

	return pnlEnforcePlan{}, false
}

// positionInfosFromRaw builds Margin PnL% snapshots from exchange position maps.
func (at *AutoTrader) positionInfosFromRaw(positions []map[string]interface{}) []kernel.PositionInfo {
	var infos []kernel.PositionInfo
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		exchangePositionID := store.ExchangePositionIDFromMap(pos)
		posKey := at.pnlEnforcePosKey(exchangePositionID, symbol, side)
		at.UpdatePeakPnL(exchangePositionID, symbol, side, pnlPct)
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		infos = append(infos, kernel.PositionInfo{
			Symbol:             symbol,
			Side:               side,
			EntryPrice:         entryPrice,
			MarkPrice:          markPrice,
			Quantity:           quantity,
			Leverage:           leverage,
			UnrealizedPnL:      unrealizedPnl,
			UnrealizedPnLPct:   pnlPct,
			PeakPnLPct:         peakPnlPct,
			LiquidationPrice:   liquidationPrice,
			MarginUsed:         marginUsed,
			ExchangePositionID: exchangePositionID,
		})
	}
	return infos
}

// positionInfosForPnLEnforce builds Margin PnL% position snapshots from the exchange.
func (at *AutoTrader) positionInfosForPnLEnforce() ([]kernel.PositionInfo, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, err
	}
	return at.positionInfosFromRaw(positions), nil
}

func positionsEnforceFingerprint(infos []kernel.PositionInfo) string {
	var b strings.Builder
	for _, p := range infos {
		fmt.Fprintf(&b, "%s|%s|%.6f|%.4f|%.2f;", p.Symbol, p.Side, p.Quantity, p.MarkPrice, p.UnrealizedPnLPct)
	}
	return b.String()
}

// onPositionsUpdated runs code-enforced PnL rules when live positions were just refreshed.
// Returns how many positions were acted on.
func (at *AutoTrader) onPositionsUpdated(infos []kernel.PositionInfo, source string, record *store.DecisionRecord) int {
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		return 0
	}

	at.strategyMu.RLock()
	engine := at.strategyEngine
	at.strategyMu.RUnlock()
	if engine == nil || at.isGridTradingMode() {
		return 0
	}

	fp := positionsEnforceFingerprint(infos)
	at.pnlEnforceCheckMu.Lock()
	if fp == at.lastPositionsEnforceFP && time.Since(at.lastPositionsEnforceAt) < 800*time.Millisecond {
		at.pnlEnforceCheckMu.Unlock()
		return 0
	}
	at.lastPositionsEnforceFP = fp
	at.lastPositionsEnforceAt = time.Now()
	at.pnlEnforceCheckMu.Unlock()

	if record == nil {
		record = &store.DecisionRecord{
			ExecutionLog: []string{fmt.Sprintf("[PnL on position update] source=%s", source)},
			Success:      true,
		}
	} else if source != "" {
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("[PnL on position update] source=%s", source))
	}

	acted := at.enforcePositionPnLRulesFromPositions(infos, engine.GetRiskControlConfig(), record)
	if acted > 0 && record != nil && source != "ai_cycle" {
		if err := at.saveDecision(record); err != nil {
			at.logWarnf("⚠️ PnL enforce acted (%s) but failed to save decision: %v", source, err)
		}
	}
	return acted
}

// onPositionsUpdatedRaw is a convenience wrapper for exchange position maps.
func (at *AutoTrader) onPositionsUpdatedRaw(positions []map[string]interface{}, source string, record *store.DecisionRecord) int {
	return at.onPositionsUpdated(at.positionInfosFromRaw(positions), source, record)
}

func (at *AutoTrader) isGridTradingMode() bool {
	cfg := at.GetStrategyConfig()
	return cfg != nil && cfg.StrategyType == "grid_trading"
}

func (at *AutoTrader) pnlEnforcePosKey(exchangePositionID, symbol, side string) string {
	return store.PeakCacheKey(exchangePositionID, symbol, side)
}

func (at *AutoTrader) getPnLEnforceTier(posKey string) int {
	at.pnlEnforceTierMu.RLock()
	defer at.pnlEnforceTierMu.RUnlock()
	return at.pnlEnforceTier[posKey]
}

func (at *AutoTrader) setPnLEnforceTier(posKey string, tier int) {
	at.pnlEnforceTierMu.Lock()
	defer at.pnlEnforceTierMu.Unlock()
	if at.pnlEnforceTier == nil {
		at.pnlEnforceTier = make(map[string]int)
	}
	if tier > at.pnlEnforceTier[posKey] {
		at.pnlEnforceTier[posKey] = tier
	}
}

func (at *AutoTrader) clearPnLEnforceTier(exchangePositionID, symbol, side string) {
	posKey := at.pnlEnforcePosKey(exchangePositionID, symbol, side)
	at.pnlEnforceTierMu.Lock()
	delete(at.pnlEnforceTier, posKey)
	at.pnlEnforceTierMu.Unlock()
}

// enforcePositionPnLRules applies strategy Margin PnL% rules without waiting for AI.
// Returns the number of positions acted on this cycle.
func (at *AutoTrader) enforcePositionPnLRules(ctx *kernel.Context, record *store.DecisionRecord) int {
	if ctx == nil || at.isGridTradingMode() {
		return 0
	}
	at.strategyMu.RLock()
	engine := at.strategyEngine
	at.strategyMu.RUnlock()
	if engine == nil {
		return 0
	}
	return at.enforcePositionPnLRulesFromPositions(ctx.Positions, engine.GetRiskControlConfig(), record)
}

func (at *AutoTrader) enforcePositionPnLRulesFromPositions(positions []kernel.PositionInfo, rc store.RiskControlConfig, record *store.DecisionRecord) int {
	if len(positions) == 0 {
		return 0
	}

	acted := 0
	for _, pos := range positions {
		posKey := at.pnlEnforcePosKey(pos.ExchangePositionID, pos.Symbol, pos.Side)
		at.UpdatePeakPnL(pos.ExchangePositionID, pos.Symbol, pos.Side, pos.UnrealizedPnLPct)

		at.peakPnLCacheMutex.RLock()
		peak := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()
		if peak < pos.UnrealizedPnLPct {
			peak = pos.UnrealizedPnLPct
		}

		appliedTier := at.getPnLEnforceTier(posKey)
		plan, ok := evaluatePositionPnLAction(pos.UnrealizedPnLPct, peak, appliedTier, rc)
		if !ok {
			continue
		}

		action := "close_long"
		if pos.Side == "short" {
			action = "close_short"
		}

		reason := fmt.Sprintf("[CODE ENFORCED PnL] %s %s | %s", pos.Symbol, pos.Side, plan.Rule)
		at.logWarnf("🔒 %s", reason)

		decision := &kernel.Decision{
			Symbol:     pos.Symbol,
			Action:     action,
			CloseRatio: plan.CloseRatio,
			Price:      pos.MarkPrice, // fast path: skip slow market fetch for code-enforced closes
			Reasoning:  reason,
		}

		actionRecord := &store.DecisionAction{
			Action:    action,
			Symbol:    pos.Symbol,
			Reasoning: reason,
			Timestamp: time.Now().UTC(),
		}

		if err := at.executeDecisionWithRecord(decision, actionRecord); err != nil {
			at.alertPnLEnforceFailure(pos, reason, err, record, actionRecord)
			if record != nil {
				record.Decisions = append(record.Decisions, *actionRecord)
			}
			continue
		}

		actionRecord.Success = true
		at.setPnLEnforceTier(posKey, plan.Tier)
		if plan.CloseRatio <= 0 || plan.CloseRatio >= 1 {
			at.clearPnLEnforceTier(pos.ExchangePositionID, pos.Symbol, pos.Side)
			at.ClearPeakPnLCache(pos.ExchangePositionID, pos.Symbol, pos.Side)
		}

		acted++
		if record != nil {
			ratioNote := "full close"
			if plan.CloseRatio > 0 && plan.CloseRatio < 1 {
				ratioNote = fmt.Sprintf("partial close ratio=%.2f", plan.CloseRatio)
			}
			record.ExecutionLog = append(record.ExecutionLog, reason+" → "+ratioNote)
			record.Decisions = append(record.Decisions, *actionRecord)
		}
	}

	if acted > 0 {
		logger.Infof("%s 🔒 Auto PnL enforcement acted on %d position(s) (independent of AI)", at.logTag(), acted)
	}
	return acted
}

func (at *AutoTrader) alertPnLEnforceFailure(
	pos kernel.PositionInfo,
	reason string,
	err error,
	record *store.DecisionRecord,
	actionRecord *store.DecisionAction,
) {
	actionRecord.Success = false
	actionRecord.Error = err.Error()

	stillOpen := at.exchangeHasPosition(pos.Symbol, pos.Side)
	alert := fmt.Sprintf("⚠️ ALERT forced close FAILED %s %s: %v", pos.Symbol, pos.Side, err)
	if stillOpen {
		alert += " — position STILL OPEN on exchange (UI/OKX may diverge)"
	}
	at.logErrorf("❌ %s | %s", reason, alert)

	if record != nil {
		record.Success = false
		if record.ErrorMessage == "" {
			record.ErrorMessage = fmt.Sprintf("PnL enforce failed %s %s: %v", pos.Symbol, pos.Side, err)
		}
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("⚠️ ALERT PnL enforce failed %s %s: %v", pos.Symbol, pos.Side, err))
		if stillOpen {
			record.ExecutionLog = append(record.ExecutionLog,
				fmt.Sprintf("⚠️ ALERT %s %s still open on exchange after failed enforce close", pos.Symbol, pos.Side))
		}
	}
}

func (at *AutoTrader) exchangeHasPosition(symbol, side string) bool {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return false
	}
	wantSide := strings.ToLower(strings.TrimSpace(side))
	for _, p := range positions {
		sym, ok1 := p["symbol"].(string)
		ps, ok2 := p["side"].(string)
		if !ok1 || !ok2 || sym != symbol {
			continue
		}
		if strings.ToLower(ps) != wantSide {
			continue
		}
		qty := 0.0
		if amt, ok := p["positionAmt"].(float64); ok {
			if amt < 0 {
				amt = -amt
			}
			qty = amt
		}
		return qty > 0
	}
	return false
}

// refreshTradingContextAfterPnLEnforce rebuilds ctx so AI sees post-enforcement positions and balances.
func (at *AutoTrader) refreshTradingContextAfterPnLEnforce(acted int, record *store.DecisionRecord) (*kernel.Context, error) {
	if acted <= 0 {
		return nil, fmt.Errorf("no PnL enforce actions to refresh for")
	}
	fresh, _, err := at.buildTradingContext(nil)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Refreshed trading context after auto PnL enforcement (%d position action(s))", acted)
	at.logInfof("🔄 %s", msg)
	if record != nil {
		record.ExecutionLog = append(record.ExecutionLog, msg)
		record.AccountState = store.AccountSnapshot{
			TotalBalance:          fresh.Account.TotalEquity,
			AvailableBalance:      fresh.Account.AvailableBalance,
			TotalUnrealizedProfit: fresh.Account.UnrealizedPnL,
			PositionCount:         fresh.Account.PositionCount,
			InitialBalance:        at.initialBalance,
		}
	}
	return fresh, nil
}
