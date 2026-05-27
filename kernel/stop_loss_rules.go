package kernel

import (
	"fmt"
	"nofx/store"
	"strings"
)

// AppendConfiguredStopLossRules injects user-tuned stop-loss / structure rules into the system prompt.
func AppendConfiguredStopLossRules(sb *strings.Builder, rc store.RiskControlConfig, lang Language, primaryTF, longerTF string) {
	btcMin := rc.EffectiveBtcEthMinStopLossDistPct()
	altMin := rc.EffectiveAltcoinMinStopLossDistPct()
	bufPct := rc.EffectiveStructStopWickBufferPct()
	minRR := rc.MinRiskRewardRatio
	if minRR <= 0 {
		minRR = 3.0
	}
	structTF := strings.TrimSpace(primaryTF)
	if structTF == "" {
		structTF = strings.TrimSpace(longerTF)
	}
	if structTF == "" {
		structTF = "4h"
	}

	sb.WriteString("## Stop-Loss & Structure (Mandatory on Open)\n\n")
	if lang == LangChinese {
		sb.WriteString("适用于所有 `open_long` / `open_short`。`stop_loss` / `take_profit` 必须为**具体价格数字**，禁止公式。\n\n")
		sb.WriteString("### 最小止损价距（相对入场价，标的涨跌幅）\n\n")
		sb.WriteString(fmt.Sprintf("- BTCUSDT、ETHUSDT：止损价距 **≥ %.2f%%**\n", btcMin))
		sb.WriteString(fmt.Sprintf("- 其他山寨：止损价距 **≥ %.2f%%**\n", altMin))
		sb.WriteString(fmt.Sprintf("- 盈亏比（价距）≥ **1:%.1f**；先定合法止损，再定止盈；**禁止**为凑盈亏比把止损缩到最小距离以内\n\n", minRR))
		if rc.EffectiveEnforceStructStop() {
			sb.WriteString("### 结构位（必须与决策周期一致）\n\n")
			sb.WriteString(fmt.Sprintf("- 主要依据时间框架：**%s**（策略主周期）；勿单独用更短周期微型高低点充当结构位\n", structTF))
			sb.WriteString(fmt.Sprintf("- 空头：止损放在该周期最近有效摆动高点之上，再加 **%.2f%%** 防插针缓冲\n", bufPct))
			sb.WriteString(fmt.Sprintf("- 多头：止损放在该周期最近有效摆动低点之下，再加 **%.2f%%** 缓冲\n", bufPct))
			sb.WriteString("- 若大周期看空但止损只能贴在更短周期小高点 → 对该标的 `wait`\n\n")
			sb.WriteString("思维链须写明：结构位来源与周期、止损价距%、是否满足最小距离。\n\n")
		}
		sb.WriteString("不满足最小价距或盈亏比时，必须对该标的 `wait`，不得开仓。\n\n")
		return
	}

	sb.WriteString("Applies to all `open_long` / `open_short`. `stop_loss` / `take_profit` must be **numeric prices**, not formulas.\n\n")
	sb.WriteString("### Minimum stop distance (% of entry, underlying move)\n\n")
	sb.WriteString(fmt.Sprintf("- BTCUSDT, ETHUSDT: **≥ %.2f%%**\n", btcMin))
	sb.WriteString(fmt.Sprintf("- Other altcoins: **≥ %.2f%%**\n", altMin))
	sb.WriteString(fmt.Sprintf("- Price R:R ≥ **1:%.1f**; set valid SL first, then TP; never tighten SL below minimum to fake R:R\n\n", minRR))
	if rc.EffectiveEnforceStructStop() {
		sb.WriteString("### Structural placement\n\n")
		sb.WriteString(fmt.Sprintf("- Primary structure timeframe: **%s**; do not use sub-TF micro highs/lows alone\n", structTF))
		sb.WriteString(fmt.Sprintf("- Short SL: above valid swing high + **%.2f%%** wick buffer\n", bufPct))
		sb.WriteString(fmt.Sprintf("- Long SL: below valid swing low + **%.2f%%** buffer\n", bufPct))
		sb.WriteString("- Timeframe mismatch → `wait`\n\n")
		sb.WriteString("In reasoning: cite structure level, SL distance %, and whether minimum distance is met.\n\n")
	}
	sb.WriteString("If minimum distance or R:R fails, use `wait` for that symbol.\n\n")
}
