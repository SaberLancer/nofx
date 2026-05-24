package trader

import (
	"nofx/logger"
	"nofx/market"
	"time"
)

func (at *AutoTrader) startPreDecisionMonitor() {
	cfg := at.preDecisionConfig()
	if !cfg.Enabled {
		return
	}

	at.ensurePreDecisionTracker()
	if at.config.StrategyConfig != nil {
		at.preDecisionTracker.TrackSymbols(at.config.StrategyConfig.CoinSource.StaticCoins)
	}

	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		interval := time.Duration(cfg.PollIntervalSec) * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		at.logInfof("📡 Pre-decision tick monitor started (poll=%v window=%ds)", interval, cfg.WindowSec)

		for {
			select {
			case <-ticker.C:
				at.pollPreDecisionTicks()
			case <-at.stopMonitorCh:
				at.logInfof("⏹ Pre-decision tick monitor stopped")
				return
			}
		}
	}()
}

func (at *AutoTrader) pollPreDecisionTicks() {
	if !at.preDecisionEnabled() || at.preDecisionTracker == nil {
		return
	}

	symbols := at.currentPreDecisionSymbols()
	if len(symbols) == 0 {
		return
	}
	at.preDecisionTracker.TrackSymbols(symbols)

	limit := at.preDecisionConfig().MinTicks
	if limit < 100 {
		limit = 100
	}

	for _, symbol := range symbols {
		ticks, err := market.FetchRecentTicks(at.exchange, symbol, limit)
		if err != nil {
			at.logWarnf("pre-decision tick fetch failed for %s: %v", symbol, err)
			continue
		}
		at.preDecisionTracker.IngestBatch(ticks)
	}
}

func (at *AutoTrader) currentPreDecisionSymbols() []string {
	if at.config.StrategyConfig == nil {
		return nil
	}
	symbols := append([]string(nil), at.config.StrategyConfig.CoinSource.StaticCoins...)

	if at.store != nil {
		positions, err := at.trader.GetPositions()
		if err != nil {
			logger.Infof("%s pre-decision: failed to load positions for symbol watch: %v", at.logTag(), err)
		} else {
			for _, pos := range positions {
				if symbol, ok := pos["symbol"].(string); ok {
					symbols = append(symbols, symbol)
				}
			}
		}
	}

	seen := make(map[string]struct{}, len(symbols))
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = market.Normalize(symbol)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}
