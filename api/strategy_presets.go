package api

import (
	"fmt"

	"nofx/logger"
	"nofx/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type strategyPresetDef struct {
	name        string
	description string
	isActive    bool
	applyConfig func(*store.StrategyConfig)
}

type strategyPresetLocale struct {
	shortTerm      strategyPresetI18n
	ultraShortTerm strategyPresetI18n
}

type strategyPresetI18n struct {
	name, description string
}

func strategyPresetLocales(lang string) strategyPresetLocale {
	locales := map[string]strategyPresetLocale{
		"zh": {
			shortTerm: strategyPresetI18n{
				name:        "短线策略",
				description: "日内短线模板。主周期 15m，多周期 5m/15m/1h，适中杠杆与较快锁盈，适合 15–90 分钟持仓。",
			},
			ultraShortTerm: strategyPresetI18n{
				name:        "超短线策略",
				description: "超短线/剥头皮模板。主周期 3m，多周期 1m/3m/5m/15m，启用 TICK 前置门禁，更快锁盈与更高交易频率。",
			},
		},
		"en": {
			shortTerm: strategyPresetI18n{
				name:        "Short-Term Strategy",
				description: "Intraday short-term template. Primary 15m, TFs 5m/15m/1h, moderate leverage and faster profit locks, ~15–90 min holds.",
			},
			ultraShortTerm: strategyPresetI18n{
				name:        "Ultra Short-Term Strategy",
				description: "Scalping template. Primary 3m, TFs 1m/3m/5m/15m, pre-decision tick gate enabled, faster locks and higher trade frequency.",
			},
		},
		"id": {
			shortTerm: strategyPresetI18n{
				name:        "Strategi Jangka Pendek",
				description: "Template intraday. Primary 15m, TF 5m/15m/1h, leverage moderat, lock profit lebih cepat, hold ~15–90 menit.",
			},
			ultraShortTerm: strategyPresetI18n{
				name:        "Strategi Ultra Pendek",
				description: "Template scalping. Primary 3m, TF 1m/3m/5m/15m, pre-decision tick gate aktif, lock profit cepat.",
			},
		},
	}
	if locale, ok := locales[lang]; ok {
		return locale
	}
	return locales["en"]
}

func applyShortTermStrategyConfig(c *store.StrategyConfig) {
	c.Indicators.Klines.PrimaryTimeframe = "15m"
	c.Indicators.Klines.LongerTimeframe = "1h"
	c.Indicators.Klines.SelectedTimeframes = []string{"5m", "15m", "1h"}
	c.Indicators.Klines.EnableMultiTimeframe = true

	c.RiskControl.MinConfidence = 72
	c.RiskControl.MinRiskRewardRatio = 2.5
	c.RiskControl.LockProfitPnLPct = 6
	c.RiskControl.LockProfitSecondPnLPct = 10
	c.RiskControl.ExitProtectPnLPct = 8
	c.RiskControl.StopLossPnLPct = -4
	c.RiskControl.PeakMinForPullback = 8
	c.RiskControl.PeakPullbackPts = 3

	applyShortTermPromptSections(c)
}

func applyUltraShortTermStrategyConfig(c *store.StrategyConfig) {
	c.Indicators.Klines.PrimaryTimeframe = "3m"
	c.Indicators.Klines.LongerTimeframe = "15m"
	c.Indicators.Klines.SelectedTimeframes = []string{"1m", "3m", "5m", "15m"}
	c.Indicators.Klines.EnableMultiTimeframe = true

	c.RiskControl.MaxPositions = 4
	c.RiskControl.BTCETHMaxLeverage = 5
	c.RiskControl.AltcoinMaxLeverage = 5
	c.RiskControl.MinConfidence = 68
	c.RiskControl.MinRiskRewardRatio = 2.0
	c.RiskControl.LockProfitPnLPct = 4
	c.RiskControl.LockProfitSecondPnLPct = 7
	c.RiskControl.ExitProtectPnLPct = 6
	c.RiskControl.StopLossPnLPct = -3
	c.RiskControl.PeakMinForPullback = 6
	c.RiskControl.PeakPullbackPts = 2

	c.PreDecision.Enabled = true
	c.PreDecision.PollIntervalSec = 5
	c.PreDecision.WindowSec = 45
	c.PreDecision.MinTicks = 15
	c.PreDecision.MinMomentumPct = 0.02
	c.PreDecision.AlwaysWhenPositions = true

	applyUltraShortTermPromptSections(c)
}

func applyShortTermPromptSections(c *store.StrategyConfig) {
	if c.Language == "zh" {
		c.PromptSections.TradingFrequency = `# ⏱️ 交易频率（短线）

- 目标：每天约 4–8 笔，每小时约 0.3–0.6 笔
- 单笔持仓建议 15–90 分钟；低于 10 分钟平仓需有明确失效信号
- 避免在同一币种 15 分钟内反复开平`
		c.PromptSections.EntryStandards = `# 🎯 入场标准（短线）

- 主周期 15m 趋势与 5m 动量需同向；1h 不得明显逆势
- 仅在关键位突破/回踩确认且盈亏比达标时开仓
- 横盘震荡、信号矛盾、刚平仓又追单 → 观望`
		return
	}
	c.PromptSections.TradingFrequency = `# ⏱️ Trading Frequency (Short-Term)

- Target: ~4–8 trades/day (~0.3–0.6/hour)
- Typical hold 15–90 minutes; sub-10m exits need clear invalidation
- Avoid re-entering the same symbol within 15 minutes`
	c.PromptSections.EntryStandards = `# 🎯 Entry Standards (Short-Term)

- 15m trend and 5m momentum aligned; 1h must not strongly oppose
- Enter on confirmed breakout/pullback with acceptable R:R
- Chop, conflicting signals, or immediate re-entry after close → wait`
}

func applyUltraShortTermPromptSections(c *store.StrategyConfig) {
	if c.Language == "zh" {
		c.PromptSections.TradingFrequency = `# ⏱️ 交易频率（超短线）

- 目标：每天约 8–15 笔；单周期频繁交易需有 TICK 趋势支撑
- 单笔持仓建议 5–30 分钟；无扩展动能应快速减仓或离场
- 前置门禁未通过时不要强行调用 AI 开仓`
		c.PromptSections.EntryStandards = `# 🎯 入场标准（超短线）

- 1m/3m 动量与 5m/15m 方向一致；禁止逆更长周期强趋势硬做
- 优先高流动性主流币；波动过小或点差/滑点不划算时观望
- 快进快出：入场即设止损止盈，盈亏比可略低于长线但仍需 ≥ 配置下限`
		return
	}
	c.PromptSections.TradingFrequency = `# ⏱️ Trading Frequency (Ultra Short-Term)

- Target: ~8–15 trades/day when tick gate confirms direction
- Typical hold 5–30 minutes; exit if momentum stalls
- Do not force AI entries when pre-decision gate is skipped`
	c.PromptSections.EntryStandards = `# 🎯 Entry Standards (Ultra Short-Term)

- 1m/3m momentum aligned with 5m/15m; no fighting strong higher-TF trend
- Prefer liquid majors; skip when volatility/spread is poor
- Quick in/out: set SL/TP on entry; R:R may be tighter but must meet configured minimum`
}

func shortTermPresetDefinitions(lang string) []strategyPresetDef {
	locale := strategyPresetLocales(lang)
	return []strategyPresetDef{
		{
			name:        locale.shortTerm.name,
			description: locale.shortTerm.description,
			isActive:    false,
			applyConfig: applyShortTermStrategyConfig,
		},
		{
			name:        locale.ultraShortTerm.name,
			description: locale.ultraShortTerm.description,
			isActive:    false,
			applyConfig: applyUltraShortTermStrategyConfig,
		},
	}
}

func buildStrategyFromPreset(userID string, def strategyPresetDef, configLang string) (*store.Strategy, error) {
	config := store.GetDefaultStrategyConfig(configLang)
	def.applyConfig(&config)

	strategy := &store.Strategy{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        def.name,
		Description: def.description,
		IsActive:    def.isActive,
		IsDefault:   false,
	}
	if err := strategy.SetConfig(&config); err != nil {
		return nil, fmt.Errorf("failed to set config for strategy %q: %w", def.name, err)
	}
	return strategy, nil
}

// ensureShortTermPresetStrategies creates 短线/超短线 templates if the user does not already have them.
func (s *Server) ensureShortTermPresetStrategies(userID, lang string) error {
	if userID == "" {
		return nil
	}
	existing, err := s.store.Strategy().List(userID)
	if err != nil {
		return err
	}
	hasName := make(map[string]bool, len(existing))
	for _, st := range existing {
		hasName[st.Name] = true
	}

	configLang := lang
	if lang == "id" {
		configLang = "en"
	}

	var toCreate []*store.Strategy
	for _, def := range shortTermPresetDefinitions(lang) {
		if hasName[def.name] {
			continue
		}
		strategy, err := buildStrategyFromPreset(userID, def, configLang)
		if err != nil {
			return err
		}
		toCreate = append(toCreate, strategy)
	}
	if len(toCreate) == 0 {
		return nil
	}

	return s.store.Transaction(func(tx *gorm.DB) error {
		for _, strategy := range toCreate {
			if err := tx.Create(strategy).Error; err != nil {
				return fmt.Errorf("failed to create strategy %q: %w", strategy.Name, err)
			}
			logger.Infof("  ✓ Ensured preset strategy: %s (user=%s)", strategy.Name, userID)
		}
		return nil
	})
}
