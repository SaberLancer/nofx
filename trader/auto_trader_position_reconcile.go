package trader

import (
	"nofx/store"
)

// reconcileOpenPositionsWithExchange closes stale local OPEN rows that are not on the exchange,
// binds live exchange posIds, and clears peak / enforce state for keys that are no longer live.
func (at *AutoTrader) reconcileOpenPositionsWithExchange(positions []map[string]interface{}) int {
	if at.store == nil {
		return 0
	}
	if positions == nil {
		var err error
		positions, err = at.trader.GetPositions()
		if err != nil {
			at.logWarnf("⚠️ Reconcile open positions: failed to get exchange positions: %v", err)
			return 0
		}
	}

	liveKeys := store.BuildLivePositionKeys(positions)
	livePosIDs := store.BuildLiveExchangePositionIDs(positions)
	peakKeys := store.BuildLivePeakCacheKeys(positions)

	closed, err := at.store.Position().ReconcileOpenPositions(at.id, liveKeys, livePosIDs)
	if err != nil {
		at.logWarnf("⚠️ Reconcile open positions: %v", err)
		return 0
	}
	if closed > 0 {
		at.logInfof("🧹 Reconciled %d stale OPEN position row(s) with exchange", closed)
	}

	if at.exchangeID != "" {
		if err := at.store.Position().BindLiveExchangePositionIDs(at.id, at.exchangeID, positions); err != nil {
			at.logWarnf("⚠️ Bind live exchange position ids: %v", err)
		}
	}

	at.peakPnLCacheMutex.Lock()
	for key := range at.peakPnLCache {
		if !peakKeys[key] {
			delete(at.peakPnLCache, key)
		}
	}
	at.peakPnLCacheMutex.Unlock()

	at.pnlEnforceTierMu.RLock()
	staleTierKeys := make([]string, 0)
	for key := range at.pnlEnforceTier {
		if !peakKeys[key] {
			staleTierKeys = append(staleTierKeys, key)
		}
	}
	at.pnlEnforceTierMu.RUnlock()
	for _, key := range staleTierKeys {
		at.clearPnLEnforceTierByKey(key)
	}

	return closed
}

func (at *AutoTrader) clearPnLEnforceTierByKey(posKey string) {
	at.pnlEnforceTierMu.Lock()
	delete(at.pnlEnforceTier, posKey)
	at.pnlEnforceTierMu.Unlock()
}
