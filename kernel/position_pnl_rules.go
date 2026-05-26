package kernel

import (
	"fmt"
	"nofx/store"
	"strings"
)

// AppendConfiguredPnLThresholds writes strategy risk-control PnL% rules (margin-based).
func AppendConfiguredPnLThresholds(sb *strings.Builder, rc store.RiskControlConfig, lang Language) {
	lock1 := rc.EffectiveLockProfitPnLPct()
	lock2 := rc.EffectiveLockProfitSecondPnLPct()
	ratio := rc.EffectiveLockProfitReduceRatio()
	exitP := rc.EffectiveExitProtectPnLPct()
	stopP := rc.EffectiveStopLossPnLPct()
	peakMin := rc.EffectivePeakMinForPullback()
	pullPts := rc.EffectivePeakPullbackPts()

	if lang == LangChinese {
		sb.WriteString("## 持仓盈利率风控（Margin PnL%，相对保证金、已含杠杆）\n\n")
		sb.WriteString("减仓/平仓判断**必须**使用「当前持仓」的 **Margin PnL%**，不得用标的涨跌幅。\n\n")
		sb.WriteString(fmt.Sprintf("- **锁盈（第一档）**：Margin PnL%% ≥ %+.1f%% → 开始锁盈，建议 `close_ratio=%.2f` 部分减仓或上移止损\n", lock1, ratio))
		sb.WriteString(fmt.Sprintf("- **锁盈（第二档）**：Margin PnL%% ≥ %+.1f%% → 再减仓 30%%~50%%，收紧止损保护剩余仓位\n", lock2))
		sb.WriteString(fmt.Sprintf("- **保护利润平仓**：Margin PnL%% ≥ %+.1f%% 且出现反转/失效信号 → 果断平仓\n", exitP))
		sb.WriteString(fmt.Sprintf("- **亏损止损**：Margin PnL%% ≤ %+.1f%% 且无修复迹象 → 平仓止损\n", stopP))
		sb.WriteString(fmt.Sprintf("- **峰值回撤**：Peak PnL%% ≥ %+.1f%% 且从峰值回撤 ≥ %.1f 个百分点 → 减仓/平仓\n\n", peakMin, pullPts))
	} else {
		sb.WriteString("## Position PnL% Risk Rules (Margin PnL%, leverage included)\n\n")
		sb.WriteString("Use **Margin PnL%** from Current Positions for reduce/exit — not raw price change.\n\n")
		sb.WriteString(fmt.Sprintf("- **Lock profit (tier 1)**: Margin PnL%% ≥ %+.1f%% → start lock; suggest `close_ratio=%.2f`\n", lock1, ratio))
		sb.WriteString(fmt.Sprintf("- **Lock profit (tier 2)**: Margin PnL%% ≥ %+.1f%% → reduce 30%%~50%% more, tighten stop\n", lock2))
		sb.WriteString(fmt.Sprintf("- **Protect gains**: Margin PnL%% ≥ %+.1f%% + reversal/invalidation → close\n", exitP))
		sb.WriteString(fmt.Sprintf("- **Stop loss**: Margin PnL%% ≤ %+.1f%% without recovery → close\n", stopP))
		sb.WriteString(fmt.Sprintf("- **Peak pullback**: Peak PnL%% ≥ %+.1f%% and pullback ≥ %.1f pp from peak → reduce/exit\n\n", peakMin, pullPts))
	}
}

// AppendPositionPnLActionGuide compares live Margin PnL% to configured thresholds.
func AppendPositionPnLActionGuide(sb *strings.Builder, positions []PositionInfo, rc store.RiskControlConfig, lang Language) {
	if len(positions) == 0 {
		return
	}

	lock1 := rc.EffectiveLockProfitPnLPct()
	lock2 := rc.EffectiveLockProfitSecondPnLPct()
	ratio := rc.EffectiveLockProfitReduceRatio()
	exitP := rc.EffectiveExitProtectPnLPct()
	stopP := rc.EffectiveStopLossPnLPct()
	peakMin := rc.EffectivePeakMinForPullback()
	pullPts := rc.EffectivePeakPullbackPts()

	if lang == LangChinese {
		sb.WriteString("## 本轮持仓 → 减仓/平仓动作指引（按当前 Margin PnL%）\n\n")
	} else {
		sb.WriteString("## This Cycle: Reduce/Exit Actions (by current Margin PnL%)\n\n")
	}

	for _, pos := range positions {
		pnl := pos.UnrealizedPnLPct
		peak := pos.PeakPnLPct
		side := strings.ToUpper(pos.Side)

		var actions []string

		if pnl <= stopP {
			actions = append(actions, fmt.Sprintf("STOP: PnL%% %+.2f%% ≤ %+.1f%% → consider full close (stop loss)", pnl, stopP))
		}
		if peak >= peakMin && peak-pnl >= pullPts {
			actions = append(actions, fmt.Sprintf("PEAK PULLBACK: Peak %+.2f%% → now %+.2f%% (drawdown %.1f pp ≥ %.1f) → reduce or exit", peak, pnl, peak-pnl, pullPts))
		}
		if pnl >= exitP {
			actions = append(actions, fmt.Sprintf("PROFIT PROTECT: PnL%% %+.2f%% ≥ %+.1f%% → on reversal signals, close (do not wait)", pnl, exitP))
		}
		if pnl >= lock2 {
			actions = append(actions, fmt.Sprintf("LOCK TIER 2: PnL%% %+.2f%% ≥ %+.1f%% → reduce 30%%~50%% (close_ratio 0.3~0.5), tighten SL", pnl, lock2))
		} else if pnl >= lock1 {
			actions = append(actions, fmt.Sprintf("LOCK TIER 1: PnL%% %+.2f%% ≥ %+.1f%% → start lock profit (close_ratio=%.2f) or move SL to breakeven", pnl, lock1, ratio))
		} else {
			actions = append(actions, fmt.Sprintf("HOLD: PnL%% %+.2f%% < lock tier 1 (%+.1f%%) — no lock-profit reduce required yet", pnl, lock1))
		}

		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("### %s %s | 当前 Margin PnL%% %+.2f%% | 峰值 %+.2f%%\n", pos.Symbol, side, pnl, peak))
			for _, a := range actions {
				sb.WriteString("- ")
				if strings.HasPrefix(a, "HOLD") {
					sb.WriteString("⏳ ")
				} else {
					sb.WriteString("⚡ ")
				}
				sb.WriteString(translateActionHint(a, lang))
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString(fmt.Sprintf("### %s %s | Margin PnL%% %+.2f%% | Peak %+.2f%%\n", pos.Symbol, side, pnl, peak))
			for _, a := range actions {
				sb.WriteString("- ")
				if strings.HasPrefix(a, "HOLD") {
					sb.WriteString("⏳ ")
				} else {
					sb.WriteString("⚡ ")
				}
				sb.WriteString(a)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
}

func translateActionHint(en string, lang Language) string {
	if lang != LangChinese {
		return en
	}
	repl := []struct{ from, to string }{
		{"STOP:", "止损："},
		{"PEAK PULLBACK:", "峰值回撤："},
		{"PROFIT PROTECT:", "保护利润："},
		{"LOCK TIER 2:", "锁盈二档："},
		{"LOCK TIER 1:", "锁盈一档："},
		{"HOLD:", "持有："},
		{"consider full close (stop loss)", "考虑全部平仓（止损）"},
		{"reduce or exit", "减仓或平仓"},
		{"on reversal signals, close (do not wait)", "出现反转信号则平仓（勿观望）"},
		{"reduce 30%~50%", "减仓 30%~50%"},
		{"start lock profit", "开始锁盈"},
		{"no lock-profit reduce required yet", "尚未达到锁盈线，无需为锁盈减仓"},
		{"drawdown", "回撤"},
		{"pp", "个百分点"},
	}
	out := en
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.from, r.to)
	}
	return out
}
