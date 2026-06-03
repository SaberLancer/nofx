package trader

import (
	"fmt"
	"nofx/logger"
	"strings"
)

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

func peakCacheKey(symbol, side string) string {
	return symbol + "_" + strings.ToLower(strings.TrimSpace(side))
}

// loadPeakPnLFromStore hydrates in-memory peak cache from open positions in DB (survives restarts).
func (at *AutoTrader) loadPeakPnLFromStore() {
	if at.store == nil {
		return
	}
	positions, err := at.store.Position().GetOpenPositions(at.id)
	if err != nil {
		at.logWarnf("⚠️ Failed to load peak PnL from store: %v", err)
		return
	}
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()
	for _, pos := range positions {
		if pos == nil || pos.PeakPnLPct == 0 {
			continue
		}
		key := peakCacheKey(pos.Symbol, pos.Side)
		if pos.PeakPnLPct > at.peakPnLCache[key] {
			at.peakPnLCache[key] = pos.PeakPnLPct
		}
	}
}

// UpdatePeakPnL updates peak profit on every PnL sample (no spike filtering). Persists to DB; cleared only on full close.
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	posKey := peakCacheKey(symbol, side)

	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	peak := at.peakPnLCache[posKey]
	if currentPnLPct > peak {
		peak = currentPnLPct
	}

	if at.store != nil {
		storedPeak, err := at.store.Position().SyncOpenPositionPeakPnLPct(at.id, symbol, side, currentPnLPct)
		if err != nil {
			at.logWarnf("⚠️ Failed to persist peak PnL for %s %s: %v", symbol, side, err)
		} else if storedPeak > peak {
			peak = storedPeak
		}
	}

	at.peakPnLCache[posKey] = peak
}

// ClearPeakPnLCache clears in-memory peak cache after a full close (DB peak remains on the closed row).
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	delete(at.peakPnLCache, peakCacheKey(symbol, side))
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 5.0 // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 1.0 // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.EffectiveMaxPositions()

	if currentPositionCount >= maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] Already at max positions (%d/%d)", currentPositionCount, maxPositions)
	}
	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}
