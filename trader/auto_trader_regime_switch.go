package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"sync"
)

type marketRegime string

const (
	regimeTrend       marketRegime = "trend"
	regimeOscillation marketRegime = "oscillation"
)

type regimeSwitchFields struct {
	enabled               bool
	trendStrategyID       string
	oscillationStrategyID string
	confirmCycles         int
	pending               marketRegime
	pendingCount          int
	active                marketRegime
	detection             store.RegimeDetectionConfig
	lastExecLogLines      []string
	mu                    sync.Mutex
}

func (at *AutoTrader) applyRegimeSwitchConfig(
	enabled bool,
	trendID, oscillationID string,
	confirmCycles int,
	detection store.RegimeDetectionConfig,
) {
	if at == nil {
		return
	}
	if confirmCycles < 1 {
		confirmCycles = 2
	}
	normDetection := detection.Normalize()
	at.regimeSwitch.mu.Lock()
	changed := at.regimeSwitch.enabled != enabled ||
		at.regimeSwitch.trendStrategyID != trendID ||
		at.regimeSwitch.oscillationStrategyID != oscillationID ||
		at.regimeSwitch.confirmCycles != confirmCycles ||
		at.regimeSwitch.detection != normDetection
	at.regimeSwitch.enabled = enabled
	at.regimeSwitch.trendStrategyID = trendID
	at.regimeSwitch.oscillationStrategyID = oscillationID
	at.regimeSwitch.confirmCycles = confirmCycles
	at.regimeSwitch.detection = normDetection
	if changed {
		at.regimeSwitch.pending = ""
		at.regimeSwitch.pendingCount = 0
		at.regimeSwitch.active = ""
		if enabled {
			l1h := normDetection.Layer1H
			l15 := normDetection.Layer15m
			l3m := normDetection.Layer3m
			at.logInfof("📐 多周期检测: 1H ADX灰区%.0f–%.0f | 15m确认=%v | 3m扳机=%v SL=%.1fxATR",
				l1h.ADXRangingBelow, l1h.ADXTrendAbove, l15.Enabled, l3m.Enabled, l3m.ATRSLMultiplier)
		}
	}
	at.regimeSwitch.mu.Unlock()
}

func (at *AutoTrader) storeRegimeExecutionLog(lines []string) {
	if at == nil {
		return
	}
	at.regimeSwitch.mu.Lock()
	at.regimeSwitch.lastExecLogLines = lines
	at.regimeSwitch.mu.Unlock()
}

func (at *AutoTrader) flushRegimeExecutionLog(record *store.DecisionRecord) {
	if at == nil || record == nil {
		return
	}
	at.regimeSwitch.mu.Lock()
	lines := at.regimeSwitch.lastExecLogLines
	at.regimeSwitch.mu.Unlock()
	if len(lines) == 0 {
		return
	}
	record.ExecutionLog = append(record.ExecutionLog, lines...)
}

func (at *AutoTrader) strategyDisplayName(strategyID string) string {
	if at == nil || at.store == nil || strategyID == "" {
		return strategyID
	}
	strategy, err := at.store.Strategy().Get(at.userID, strategyID)
	if err != nil || strategy == nil || strategy.Name == "" {
		return strategyID
	}
	return strategy.Name
}

func (at *AutoTrader) currentStrategyDisplayName() string {
	if at == nil {
		return ""
	}
	at.strategyMu.RLock()
	name := at.strategyName
	id := at.strategyID
	at.strategyMu.RUnlock()
	if name != "" {
		return name
	}
	return at.strategyDisplayName(id)
}

func (at *AutoTrader) maybeSwitchStrategyByRegime() bool {
	if at == nil || at.store == nil {
		return false
	}

	at.regimeSwitch.mu.Lock()
	enabled := at.regimeSwitch.enabled
	trendID := at.regimeSwitch.trendStrategyID
	oscillationID := at.regimeSwitch.oscillationStrategyID
	confirmCycles := at.regimeSwitch.confirmCycles
	detection := at.regimeSwitch.detection
	at.regimeSwitch.mu.Unlock()

	if !enabled || trendID == "" || oscillationID == "" {
		at.storeRegimeExecutionLog(nil)
		return false
	}

	trendName := at.strategyDisplayName(trendID)
	oscName := at.strategyDisplayName(oscillationID)

	symbol := at.regimeDetectionSymbolEarly()
	if symbol == "" {
		at.logWarnf("⚠️ Regime switch skipped: no symbol for detection")
		at.storeRegimeExecutionLog([]string{"regime-detect: skipped no_symbol"})
		return false
	}

	snap := at.detectMultiTFRegimeSnapshot(symbol, detection)
	if !snap.Decisive {
		at.logInfof("📊 市场状态未决，暂不切换 [%s] %s", symbol, snap.Reason)
		at.storeRegimeExecutionLog([]string{
			fmt.Sprintf("regime-detect: inconclusive symbol=%s %s | %s",
				symbol, regimeDetectMetrics(snap), snap.Reason),
		})
		return false
	}

	detected := regimeTrend
	targetStrategyID := trendID
	targetName := trendName
	verdictLabel := "trend"
	label := "趋势"
	if snap.IsOscillating {
		detected = regimeOscillation
		targetStrategyID = oscillationID
		targetName = oscName
		verdictLabel = "oscillation"
		label = "震荡"
	}

	at.strategyMu.RLock()
	currentStrategyID := at.strategyID
	currentName := at.strategyName
	at.strategyMu.RUnlock()
	if currentName == "" {
		currentName = at.strategyDisplayName(currentStrategyID)
	}

	if currentStrategyID == targetStrategyID {
		at.regimeSwitch.mu.Lock()
		at.regimeSwitch.pending = ""
		at.regimeSwitch.pendingCount = 0
		at.regimeSwitch.active = detected
		at.regimeSwitch.mu.Unlock()
		at.storeRegimeExecutionLog([]string{
			fmt.Sprintf("regime-detect: verdict=%s symbol=%s %s strategy=%s | %s",
				verdictLabel, symbol, regimeDetectMetrics(snap), currentName, snap.Reason),
		})
		return false
	}

	at.regimeSwitch.mu.Lock()
	if at.regimeSwitch.pending == detected {
		at.regimeSwitch.pendingCount++
	} else {
		at.regimeSwitch.pending = detected
		at.regimeSwitch.pendingCount = 1
	}
	count := at.regimeSwitch.pendingCount
	at.regimeSwitch.mu.Unlock()

	if count < confirmCycles {
		at.logInfof("📊 市场状态切换待确认: %s (%d/%d) [%s] 1H_ADX=%.1f 15m_ADX=%.1f 3m_ADX=%.1f %s",
			label, count, confirmCycles, symbol, snap.ADX1H, snap.ADX15m, snap.ADX3m, snap.Reason)
		at.storeRegimeExecutionLog([]string{
			fmt.Sprintf("regime-switch: pending %s symbol=%s confirm=%d/%d target=%s %s | %s",
				verdictLabel, symbol, count, confirmCycles, targetName, regimeDetectMetrics(snap), snap.Reason),
		})
		return false
	}

	if err := at.store.Trader().UpdateStrategyID(at.userID, at.id, targetStrategyID); err != nil {
		at.logWarnf("⚠️ 市场状态策略切换失败: %v", err)
		at.storeRegimeExecutionLog([]string{
			fmt.Sprintf("regime-switch: failed %s symbol=%s target=%s error=%v",
				verdictLabel, symbol, targetName, err),
		})
		return false
	}

	at.regimeSwitch.mu.Lock()
	at.regimeSwitch.pending = ""
	at.regimeSwitch.pendingCount = 0
	at.regimeSwitch.active = detected
	at.regimeSwitch.mu.Unlock()

	at.logInfof("🔀 市场状态确认为%s → 自动切换策略 [%s] (1H_ADX=%.1f, 3m_ADX=%.1f, %s)", label, symbol, snap.ADX1H, snap.ADX3m, snap.Reason)
	at.storeRegimeExecutionLog([]string{
		fmt.Sprintf("regime-switch: confirmed %s symbol=%s switched_to=%s %s | %s",
			verdictLabel, symbol, targetName, regimeDetectMetrics(snap), snap.Reason),
	})
	return true
}

func regimeDetectMetrics(snap market.MultiTFRegimeSnapshot) string {
	return fmt.Sprintf(
		"adx_1h=%.1f adx_15m=%.1f adx_3m=%.1f atr_3m=%.4f bbw_15m_pct=%.2f",
		snap.ADX1H, snap.ADX15m, snap.ADX3m, snap.ATR3m, snap.BBW15mPct,
	)
}

func (at *AutoTrader) detectMultiTFRegimeSnapshot(symbol string, detection store.RegimeDetectionConfig) market.MultiTFRegimeSnapshot {
	detection = detection.Normalize()
	l1h := detection.Layer1H
	l15 := detection.Layer15m
	l3m := detection.Layer3m

	klineExchange := market.NormalizeKlineExchange(at.exchange)
	opts := at.klineOptions()

	var klines1h, klines15m, klines3m []market.Kline

	timeframes := make([]string, 0, 3)
	klineCount := 30
	primaryTF := "15m"
	if l1h.Enabled {
		timeframes = append(timeframes, "1h")
		if l1h.KlineCount > klineCount {
			klineCount = l1h.KlineCount
		}
		primaryTF = "1h"
	}
	if l15.Enabled {
		timeframes = append(timeframes, "15m")
		if l15.KlineCount > klineCount {
			klineCount = l15.KlineCount
		}
		if !l1h.Enabled {
			primaryTF = "15m"
		}
	}
	if l3m.Enabled {
		timeframes = append(timeframes, "3m")
		if l3m.KlineCount > klineCount {
			klineCount = l3m.KlineCount
		}
		if !l1h.Enabled && !l15.Enabled {
			primaryTF = "3m"
		}
	}

	if len(timeframes) > 0 {
		data, err := market.GetWithTimeframesOptions(symbol, timeframes, primaryTF, klineCount, klineExchange, opts)
		if err == nil && data != nil && data.TimeframeData != nil {
			if tf, ok := data.TimeframeData["1h"]; ok && tf != nil {
				klines1h = market.KlinesFromBars(tf.Klines)
			}
			if tf, ok := data.TimeframeData["15m"]; ok && tf != nil {
				klines15m = market.KlinesFromBars(tf.Klines)
			}
			if tf, ok := data.TimeframeData["3m"]; ok && tf != nil {
				klines3m = market.KlinesFromBars(tf.Klines)
			}
		}
	}

	return detectMultiTFRegime(klines1h, klines15m, klines3m, detection)
}

func (at *AutoTrader) regimeDetectionSymbolEarly() string {
	if at == nil || at.trader == nil {
		return ""
	}
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			symbol, _ := pos["symbol"].(string)
			qty, _ := pos["positionAmt"].(float64)
			if symbol != "" && qty != 0 {
				return symbol
			}
		}
	}
	return at.firstTrendStrategyCandidateSymbol()
}

func (at *AutoTrader) firstTrendStrategyCandidateSymbol() string {
	if at == nil || at.store == nil || at.userID == "" {
		return ""
	}
	at.regimeSwitch.mu.Lock()
	trendID := at.regimeSwitch.trendStrategyID
	at.regimeSwitch.mu.Unlock()
	if trendID == "" {
		return ""
	}
	strategy, err := at.store.Strategy().Get(at.userID, trendID)
	if err != nil {
		return ""
	}
	cfg, err := strategy.ParseConfig()
	if err != nil {
		return ""
	}
	cfg.NormalizeProductSchema()
	engine := kernel.NewStrategyEngine(cfg, at.config.Claw402WalletKey)
	coins, err := engine.GetCandidateCoins()
	if err != nil || len(coins) == 0 {
		return ""
	}
	return coins[0].Symbol
}

func (at *AutoTrader) regimeDetectionSymbol(ctx *kernel.Context) string {
	if ctx == nil {
		return at.regimeDetectionSymbolEarly()
	}
	if len(ctx.Positions) > 0 && ctx.Positions[0].Symbol != "" {
		return ctx.Positions[0].Symbol
	}
	if len(ctx.CandidateCoins) > 0 && ctx.CandidateCoins[0].Symbol != "" {
		return ctx.CandidateCoins[0].Symbol
	}
	for sym := range ctx.MarketDataMap {
		return sym
	}
	return at.regimeDetectionSymbolEarly()
}
