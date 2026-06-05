package trader

import (
	"nofx/kernel"
	"time"
)

// reloadStrategyConfigIfNeeded loads the latest strategy from DB when it changed.
// Running traders pick up prompt / risk / indicator edits on the next decision cycle.
func (at *AutoTrader) reloadStrategyConfigIfNeeded() {
	if at == nil || at.store == nil || at.userID == "" {
		return
	}

	strategyID := at.strategyID
	if strategyID == "" {
		return
	}

	if traderRec, err := at.store.Trader().GetByID(at.id); err == nil {
		if traderRec.UserID != "" && traderRec.UserID != at.userID {
			at.logWarnf("⚠️ Strategy hot-reload skipped: trader user mismatch")
			return
		}
		at.applyRegimeSwitchConfig(
			traderRec.RegimeSwitchEnabled,
			traderRec.TrendStrategyID,
			traderRec.OscillationStrategyID,
			traderRec.RegimeConfirmCycles,
			traderRec.RegimeDetection(),
		)
		if traderRec.StrategyID != "" && traderRec.StrategyID != strategyID {
			at.logInfof("📎 Strategy binding changed: %s → %s", strategyID, traderRec.StrategyID)
			strategyID = traderRec.StrategyID
		}
	}

	strategy, err := at.store.Strategy().Get(at.userID, strategyID)
	if err != nil {
		at.logWarnf("⚠️ Strategy hot-reload skipped: failed to load strategy %s: %v", strategyID, err)
		return
	}

	if strategyID == at.strategyID && !strategy.UpdatedAt.After(at.strategyUpdatedAt) {
		return
	}

	cfg, err := strategy.ParseConfig()
	if err != nil {
		at.logWarnf("⚠️ Strategy hot-reload skipped: parse error: %v", err)
		return
	}
	cfg.NormalizeProductSchema()
	cfg.ClampLimits()

	at.strategyMu.Lock()
	defer at.strategyMu.Unlock()

	at.strategyID = strategyID
	at.strategyUpdatedAt = strategy.UpdatedAt
	at.strategyName = strategy.Name
	at.config.StrategyConfig = cfg
	at.strategyEngine = kernel.NewStrategyEngine(cfg, at.config.Claw402WalletKey)

	at.logInfof("🔄 Strategy hot-reloaded: %s (updated %s)", strategy.Name, strategy.UpdatedAt.Format(time.RFC3339))
}
