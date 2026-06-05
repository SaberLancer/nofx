package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/store"
	"strings"
	"time"
)

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPrompt builds the static System Prompt (strategy + rules only).
// Account-equity USDT caps live in the user prompt (## Current Session Limits) so consecutive
// calls can share an identical system prefix for provider prompt-cache hits.
func (e *StrategyEngine) BuildSystemPrompt(variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// 0. Data Dictionary & Schema (ensure AI understands all fields)
	lang := e.GetLanguage()
	schemaPrompt := GetSchemaPromptForIndicators(lang, &e.config.Indicators)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# You are a professional cryptocurrency trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on provided market data.\n\n")
	}

	// 2. Trading mode variant
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		if riskControl.EffectiveEnableStopLoss() || riskControl.EffectiveEnableTakeProfit() {
			sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
		} else {
			sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions; stop-loss/take-profit are disabled in this strategy — do not require SL/TP on open\n\n")
		}
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open when enabled data sources in this strategy align; do not cite disabled indicators\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
	}

	// 3. Hard constraints (risk control) — ratios only; per-cycle USDT caps in user prompt
	btcEthPosValueRatio := positionValueRatioBTCETH(riskControl)
	altcoinPosValueRatio := positionValueRatioAltcoin(riskControl)

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", e.config.EffectiveMaxPositions()))
	if lang == LangChinese {
		sb.WriteString(fmt.Sprintf("- 仓位名义上限（山寨）: 净值 × %.1fx — **本周期 USDT 上限见用户消息「## 本周期仓位上限」**\n", altcoinPosValueRatio))
		sb.WriteString(fmt.Sprintf("- 仓位名义上限（BTC/ETH）: 净值 × %.1fx — **本周期 USDT 上限见用户消息「## 本周期仓位上限」**\n", btcEthPosValueRatio))
	} else {
		sb.WriteString(fmt.Sprintf("- Position value cap (Altcoins): equity × %.1fx — **exact max USDT in user message \"## Current Session Limits\"**\n", altcoinPosValueRatio))
		sb.WriteString(fmt.Sprintf("- Position value cap (BTC/ETH): equity × %.1fx — **exact max USDT in user message \"## Current Session Limits\"**\n", btcEthPosValueRatio))
	}
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))

	slEnabled := riskControl.EffectiveEnableStopLoss()
	tpEnabled := riskControl.EffectiveEnableTakeProfit()
	if slEnabled || tpEnabled {
		sb.WriteString("## CODE ENFORCED (Open protection — stop_loss / take_profit):\n")
		if slEnabled {
			sb.WriteString(fmt.Sprintf("- BTC/ETH min stop distance: ≥%.2f%% | Altcoins: ≥%.2f%% (underlying price move)\n",
				riskControl.EffectiveBtcEthMinStopLossDistPct(), riskControl.EffectiveAltcoinMinStopLossDistPct()))
		}
		if slEnabled && tpEnabled {
			sb.WriteString(fmt.Sprintf("- Min risk-reward on open: ≥1:%.1f\n", riskControl.MinRiskRewardRatio))
		}
		sb.WriteString("\n")
	} else if lang == LangChinese {
		sb.WriteString("## 止损止盈（策略已关闭，后端不校验）\n")
		sb.WriteString("- 开仓勿填 `stop_loss` / `take_profit`；勿因最小止损价距或盈亏比不足而 `wait`。\n\n")
	} else {
		sb.WriteString("## Stop-Loss / Take-Profit (disabled — not validated on open)\n")
		sb.WriteString("- Omit `stop_loss` / `take_profit` on open; do not `wait` due to min SL distance or R:R.\n\n")
	}

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	if slEnabled && tpEnabled {
		sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	}
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))

	primaryTF := e.config.Indicators.Klines.PrimaryTimeframe
	longerTF := e.config.Indicators.Klines.LongerTimeframe
	AppendConfiguredStopLossRules(&sb, riskControl, lang, primaryTF, longerTF)

	AppendConfiguredPnLThresholds(&sb, riskControl, lang)

	// Position sizing guidance (USDT caps are per-cycle in user prompt)
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString("Calculate `position_size_usd` from your confidence and the **USDT caps** in user message \"## Current Session Limits\" / \"## 本周期仓位上限\":\n")
	sb.WriteString("- High confidence (≥85): Use 80-100%% of the applicable max position value (USDT)\n")
	sb.WriteString("- Medium confidence (70-84): Use 50-80%% of the applicable max position value (USDT)\n")
	sb.WriteString("- Low confidence (60-69): Use 30-50%% of the applicable max position value (USDT)\n")
	sb.WriteString("- **DO NOT** use available_balance as position_size_usd; use the session USDT caps for that symbol type (BTC/ETH vs altcoin)\n\n")

	// 4. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = Overtrading\n")
		sb.WriteString("- Single position hold time ≥ 30-60 minutes\n")
		sb.WriteString("If you find yourself trading every period → standards too low; if closing positions < 30 minutes → too impatient.\n\n")
	}

	// 5. Entry standards — indicator list is always generated from strategy config
	e.appendConfiguredEntryStandards(&sb, lang, riskControl.MinConfidence)

	// 6. Reduce standards (editable)
	if promptSections.ReduceStandards != "" {
		sb.WriteString(promptSections.ReduceStandards)
		sb.WriteString("\n\n")
	} else {
		if lang == LangChinese {
			sb.WriteString("# 🪓 减仓标准（锁盈 / 降风险）\n\n")
			sb.WriteString("以**当前持仓 Margin PnL%** 为准（见上文「持仓盈利率风控」数值）。\n")
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%%：开始锁盈（close_ratio 建议 %.2f）\n",
				riskControl.EffectiveLockProfitPnLPct(), riskControl.EffectiveLockProfitReduceRatio()))
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%%：进一步减仓并收紧保护\n\n",
				riskControl.EffectiveLockProfitSecondPnLPct()))
		} else {
			sb.WriteString("# 🪓 Reduce Standards (Lock Profit / De-risk)\n\n")
			sb.WriteString("Use **current position Margin PnL%** (see Position PnL% Risk Rules above).\n")
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%%: start lock (close_ratio ~%.2f)\n",
				riskControl.EffectiveLockProfitPnLPct(), riskControl.EffectiveLockProfitReduceRatio()))
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%%: further reduce and tighten protection\n\n",
				riskControl.EffectiveLockProfitSecondPnLPct()))
		}
	}

	// 7. Exit standards (editable)
	if promptSections.ExitStandards != "" {
		sb.WriteString(promptSections.ExitStandards)
		sb.WriteString("\n\n")
	} else {
		if lang == LangChinese {
			sb.WriteString("# 🧯 平仓标准（纪律性退出）\n\n")
			sb.WriteString("触发条件均指**当前持仓 Margin PnL%**。\n")
			sb.WriteString("- 逻辑失效/反转确认 → 平仓\n")
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%% 且出现反转信号 → 保护利润平仓\n",
				riskControl.EffectiveExitProtectPnLPct()))
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≤ %+.1f%% → 止损平仓\n\n",
				riskControl.EffectiveStopLossPnLPct()))
		} else {
			sb.WriteString("# 🧯 Exit Standards (Disciplined Close)\n\n")
			sb.WriteString("Triggers use **current Margin PnL%**.\n")
			sb.WriteString("- Invalidation/reversal confirmed → close\n")
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≥ %+.1f%% with reversal → protect profit, close\n",
				riskControl.EffectiveExitProtectPnLPct()))
			sb.WriteString(fmt.Sprintf("- Margin PnL%% ≤ %+.1f%% → stop loss close\n\n",
				riskControl.EffectiveStopLossPnLPct()))
		}
	}

	// 8. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → Should we take profit/stop-loss\n")
		sb.WriteString("2. Scan candidate coins + multi-timeframe → Are there strong signals\n")
		sb.WriteString("3. Write chain of thought first, then output structured JSON\n\n")
	}

	// Reasoning: position P&L must match system data (margin-based %), not raw price change
	if lang == LangChinese {
		sb.WriteString("# 思维链中的持仓盈亏（强制）\n\n")
		sb.WriteString("在 `<reasoning>` 中分析**已有持仓**的盈亏时：\n")
		sb.WriteString("- **必须**使用「## Current Positions」每行的 **Margin PnL%**，以及下方「思维链必须引用的持仓 PnL%」中的数值（相对保证金、已含杠杆）。\n")
		sb.WriteString("- **禁止**把标的价格涨跌幅 (Current−Entry)/Entry 写成「未实现盈亏」；二者常相差约杠杆倍数（如 5x 时 PnL% ≈ 价格变动% × 5）。\n")
		sb.WriteString("- 若需提及标的价格变动，必须单独写「标的价格变动约 +x.xx%」，并标明**不是** Margin PnL%。\n")
		sb.WriteString("- K 线表时间为**北京时间（UTC+8）**；思维链描述行情时刻也必须使用**北京时间**，并以标记 `<- current` 的最新 K 线为准。\n\n")
	} else {
		sb.WriteString("# Chain-of-Thought: Position P&L (Mandatory)\n\n")
		sb.WriteString("When analyzing **existing positions** inside `<reasoning>`:\n")
		sb.WriteString("- You **MUST** use **Margin PnL%** on each `## Current Positions` line and in `## Mandatory PnL% for <reasoning>` (return on margin, leverage included).\n")
		sb.WriteString("- You **MUST NOT** report raw `(Current − Entry) / Entry` as \"unrealized PnL\"; with 5x leverage, Margin PnL% is often ~5× the price-change %.\n")
		sb.WriteString("- If you mention underlying price movement, label it separately and state it is **not** Margin PnL%.\n")
		sb.WriteString("- K-line tables use **Beijing Time (UTC+8)**; cite bar times in Beijing time and use the bar marked `<- current` as live price action.\n\n")
	}

	// 9. Output format
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your chain of thought analysis...\n")
	sb.WriteString("- Briefly analyze your thinking process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	// Illustrative numbers only (not account-specific) to keep system prompt byte-stable
	if slEnabled && tpEnabled {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": 50000, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
			riskControl.BTCETHMaxLeverage))
	} else if slEnabled {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": 50000, \"stop_loss\": 97000, \"confidence\": 85, \"risk_usd\": 300},\n",
			riskControl.BTCETHMaxLeverage))
	} else if tpEnabled {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": 50000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
			riskControl.BTCETHMaxLeverage))
	} else {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": 50000, \"confidence\": 85, \"risk_usd\": 300},\n",
			riskControl.BTCETHMaxLeverage))
	}
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"close_ratio\": 0.5}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- Optional for closing: `close_ratio` (0-1). If omitted or ≥1, close all.\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString(openRequiredFieldsDescription(slEnabled, tpEnabled, lang))
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n\n")

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n")
	}

	return sb.String()
}

func positionValueRatioBTCETH(riskControl store.RiskControlConfig) float64 {
	r := riskControl.BTCETHMaxPositionValueRatio
	if r <= 0 {
		return 5.0
	}
	return r
}

func positionValueRatioAltcoin(riskControl store.RiskControlConfig) float64 {
	r := riskControl.AltcoinMaxPositionValueRatio
	if r <= 0 {
		return store.DefaultAltcoinMaxPositionValueRatio
	}
	return r
}

// appendCurrentSessionLimits writes per-cycle equity-based USDT caps (user prompt; dynamic).
func (e *StrategyEngine) appendCurrentSessionLimits(sb *strings.Builder, accountEquity float64) {
	if accountEquity <= 0 {
		return
	}
	riskControl := e.config.RiskControl
	btcRatio := positionValueRatioBTCETH(riskControl)
	altRatio := positionValueRatioAltcoin(riskControl)
	btcMax := accountEquity * btcRatio
	altMax := accountEquity * altRatio
	lang := e.GetLanguage()

	if lang == LangChinese {
		sb.WriteString("## 本周期仓位上限（开仓 position_size_usd 必须 ≤ 对应上限）\n\n")
		sb.WriteString(fmt.Sprintf("- 账户净值: %.2f USDT\n", accountEquity))
		sb.WriteString(fmt.Sprintf("- 山寨币名义上限: %.0f USDT (= 净值 × %.1fx)\n", altMax, altRatio))
		sb.WriteString(fmt.Sprintf("- BTC/ETH 名义上限: %.0f USDT (= 净值 × %.1fx)\n", btcMax, btcRatio))
		sb.WriteString(fmt.Sprintf("- 高信心(≥85) BTC/ETH 参考: 约 %.0f–%.0f USDT\n\n", btcMax*0.8, btcMax))
	} else {
		sb.WriteString("## Current Session Limits (position_size_usd must not exceed the cap for that symbol type)\n\n")
		sb.WriteString(fmt.Sprintf("- Account equity: %.2f USDT\n", accountEquity))
		sb.WriteString(fmt.Sprintf("- Altcoin max position value: %.0f USDT (= equity × %.1fx)\n", altMax, altRatio))
		sb.WriteString(fmt.Sprintf("- BTC/ETH max position value: %.0f USDT (= equity × %.1fx)\n", btcMax, btcRatio))
		sb.WriteString(fmt.Sprintf("- High confidence (≥85) BTC/ETH reference: ~%.0f–%.0f USDT\n\n", btcMax*0.8, btcMax))
	}
}

func openRequiredFieldsDescription(slEnabled, tpEnabled bool, lang Language) string {
	baseZH := "- 开仓必填：leverage、position_size_usd、confidence、risk_usd"
	baseEN := "- Required when opening: leverage, position_size_usd, confidence, risk_usd"
	switch {
	case slEnabled && tpEnabled:
		if lang == LangChinese {
			return baseZH + "、stop_loss、take_profit\n"
		}
		return baseEN + ", stop_loss, take_profit\n"
	case slEnabled:
		if lang == LangChinese {
			return baseZH + "、stop_loss（勿填 take_profit）\n"
		}
		return baseEN + ", stop_loss (omit take_profit)\n"
	case tpEnabled:
		if lang == LangChinese {
			return baseZH + "、take_profit（勿填 stop_loss）\n"
		}
		return baseEN + ", take_profit (omit stop_loss)\n"
	default:
		if lang == LangChinese {
			return baseZH + "（**勿**填 stop_loss / take_profit）\n"
		}
		return baseEN + " (do **not** include stop_loss or take_profit)\n"
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status (Beijing time — matches K-line tables and AI reasoning)
	refTime := DecisionReferenceTime(ctx)
	sb.WriteString(FormatDecisionContextTimeLine(refTime, e.GetLanguage(), ctx.CallCount, ctx.RuntimeMinutes))
	sb.WriteString("\n\n")

	// Per-cycle limits (dynamic) — immediately after time line, before market data
	e.appendCurrentSessionLimits(&sb, ctx.Account.TotalEquity)

	// BTC market (only show indicators enabled in strategy config)
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(e.formatBTCMarketSnapshot(btcData))
	}

	// Account information
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.EntryTime, order.ExitTime, order.HoldDuration))
		}
		sb.WriteString("\n")
	}

	// Historical trading statistics (helps AI understand past performance)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		// Get language from strategy config
		lang := e.GetLanguage()

		// Win/Loss ratio
		var winLossRatio float64
		if ctx.TradingStats.AvgLoss > 0 {
			winLossRatio = ctx.TradingStats.AvgWin / ctx.TradingStats.AvgLoss
		}

		if lang == LangChinese {
			sb.WriteString("## 历史交易统计\n")
			sb.WriteString(fmt.Sprintf("总交易: %d 笔 | 盈利因子: %.2f | 夏普比率: %.2f | 盈亏比: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("表现: 良好 - 保持当前策略\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
			} else {
				sb.WriteString("表现: 正常 - 有优化空间\n")
			}
		} else {
			sb.WriteString("## Historical Trading Statistics\n")
			sb.WriteString(fmt.Sprintf("Total Trades: %d | Profit Factor: %.2f | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("Performance: GOOD - maintain current strategy\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
			} else {
				sb.WriteString("Performance: NORMAL - room for optimization\n")
			}
		}
		sb.WriteString("\n")
	}

	// Position information
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
		AppendMandatoryPositionPnLQuotes(&sb, ctx.Positions, e.GetLanguage())
		AppendPositionPnLActionGuide(&sb, ctx.Positions, e.config.RiskControl, e.GetLanguage())
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Candidate coins (exclude coins already in positions to avoid duplicate data)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		// Normalize symbol to handle both "ETH" and "ETHUSDT" formats
		normalizedSymbol := market.Normalize(pos.Symbol)
		positionSymbols[normalizedSymbol] = true
	}

	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Get language for market data formatting
	nofxosLang := nofxos.LangEnglish
	if e.GetLanguage() == LangChinese {
		nofxosLang = nofxos.LangChinese
	}

	// OI Ranking data (market-wide open interest changes)
	if ctx.OIRankingData != nil {
		sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
	}

	// NetFlow Ranking data (market-wide fund flow)
	if ctx.NetFlowRankingData != nil {
		sb.WriteString(nofxos.FormatNetFlowRankingForAI(ctx.NetFlowRankingData, nofxosLang))
	}

	// Price Ranking data (market-wide gainers/losers)
	if ctx.PriceRankingData != nil {
		sb.WriteString(nofxos.FormatPriceRankingForAI(ctx.PriceRankingData, nofxosLang))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")

	return sb.String()
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	holdingDuration := ""
	if pos.UpdateTime > 0 {
		nowMs := time.Now().UnixMilli()
		if ctx != nil && ctx.ReferenceTimeMs > 0 {
			nowMs = ctx.ReferenceTimeMs
		}
		durationMs := nowMs - pos.UpdateTime
		durationMin := durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | Holding Duration %d min", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | Holding Duration %dh %dm", durationHour, durationMinRemainder)
		}
	}

	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Position Value %.2f USDT | Margin PnL%% %+.2f%% | PnL Amount %+.2f USDT | Peak PnL%% %.2f%% | Leverage %dx | Margin %.0f | Liq Price %.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		// Multiple signal source combination
		hasAI500 := false
		hasOITop := false
		hasOILow := false
		hasHyperAll := false
		hasHyperMain := false
		for _, s := range sources {
			switch s {
			case "ai500":
				hasAI500 = true
			case "oi_top":
				hasOITop = true
			case "oi_low":
				hasOILow = true
			case "hyper_all":
				hasHyperAll = true
			case "hyper_main":
				hasHyperMain = true
			}
		}
		if hasAI500 && hasOITop {
			return " (AI500+OI_Top dual signal)"
		}
		if hasAI500 && hasOILow {
			return " (AI500+OI_Low dual signal)"
		}
		if hasOITop && hasOILow {
			return " (OI_Top+OI_Low)"
		}
		if hasHyperMain && hasAI500 {
			return " (HyperMain+AI500)"
		}
		if hasHyperAll || hasHyperMain {
			return " (Hyperliquid)"
		}
		return " (Multiple sources)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top OI increase)"
		case "oi_low":
			return " (OI_Low OI decrease)"
		case "static":
			return " (Manual selection)"
		case "hyper_all":
			return " (Hyperliquid All)"
		case "hyper_main":
			return " (Hyperliquid Top20)"
		}
	}
	return ""
}

// ============================================================================
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// Clearly label the coin symbol
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))

	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	}

	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	}

	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	}

	sb.WriteString("\n\n")

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				e.formatTimeframeSeriesData(&sb, tfData, indicators)
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	if len(data.Klines) > 0 {
		sb.WriteString("Time(Beijing)   Open      High      Low       Close     Volume\n")
		for i, k := range data.Klines {
			timeStr := FormatBeijingKlineTimeMs(k.Time)
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.4f %-9.4f %-9.4f %-9.4f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if indicators.EnableVolume && len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}

	if indicators.EnableEMA {
		if len(data.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
		}
		if len(data.EMA50Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}

	if indicators.EnableRSI {
		if len(data.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
		}
		if len(data.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
		}
	}

	if indicators.EnableATR && data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL Upper: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL Middle: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL Lower: %s\n", formatFloatSlice(data.BOLLLower)))
	}

	sb.WriteString("\n")
}

func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	indicators := e.config.Indicators
	if !indicators.EnableQuantOI && !indicators.EnableQuantNetflow {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.4f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if indicators.EnableQuantNetflow && data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if data.Netflow.Institution.Future != nil && len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Institution.Spot != nil && len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if data.Netflow.Personal.Future != nil && len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Personal.Spot != nil && len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if indicators.EnableQuantOI && len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.4f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}
