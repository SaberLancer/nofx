package kernel

import (
	"fmt"
	"nofx/store"
	"strings"
)

// AppendConfiguredStopLossRules injects stop-loss / take-profit rules when enabled in risk_control.
func AppendConfiguredStopLossRules(sb *strings.Builder, rc store.RiskControlConfig, lang Language, primaryTF, longerTF string) {
	slOn := rc.EffectiveEnableStopLoss()
	tpOn := rc.EffectiveEnableTakeProfit()
	if !slOn && !tpOn {
		appendStopLossTakeProfitDisabledNotice(sb, lang)
		return
	}

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
		appendOpenSLTPFieldRulesZH(sb, slOn, tpOn)
		if !slOn {
			sb.WriteString("### 最小止损价距\n\n")
			sb.WriteString("- 本策略**未启用**止损，后端不校验最小止损价距；思维链**勿**以此作为 `wait` 理由。\n\n")
		} else {
			sb.WriteString("### 最小止损价距（相对入场价，标的涨跌幅）\n\n")
			sb.WriteString(fmt.Sprintf("- BTCUSDT、ETHUSDT：止损价距 **≥ %.2f%%**\n", btcMin))
			sb.WriteString(fmt.Sprintf("- 其他山寨：止损价距 **≥ %.2f%%**\n", altMin))
			if tpOn {
				sb.WriteString(fmt.Sprintf("- 盈亏比（价距）≥ **1:%.1f**；先定合法止损，再定止盈；**禁止**为凑盈亏比把止损缩到最小距离以内\n\n", minRR))
			} else {
				sb.WriteString("- 本策略**未启用**止盈，后端不校验开仓盈亏比。\n\n")
			}
		}
		if slOn && rc.EffectiveEnforceStructStop() {
			sb.WriteString("### 结构位（必须与决策周期一致）\n\n")
			sb.WriteString(fmt.Sprintf("- 主要依据时间框架：**%s**（策略主周期）；勿单独用更短周期微型高低点充当结构位\n", structTF))
			sb.WriteString(fmt.Sprintf("- 空头：止损放在该周期最近有效摆动高点之上，再加 **%.2f%%** 防插针缓冲\n", bufPct))
			sb.WriteString(fmt.Sprintf("- 多头：止损放在该周期最近有效摆动低点之下，再加 **%.2f%%** 缓冲\n", bufPct))
			sb.WriteString("- 若大周期看空但止损只能贴在更短周期小高点 → 对该标的 `wait`\n\n")
			sb.WriteString("思维链须写明：结构位来源与周期、止损价距%、是否满足最小距离。\n\n")
		}
		if slOn {
			appendStopLossReasoningRulesZH(sb)
			if tpOn {
				sb.WriteString("不满足最小价距或盈亏比时，必须对该标的 `wait`，不得开仓。\n\n")
			} else {
				sb.WriteString("不满足最小价距时，必须对该标的 `wait`，不得开仓。\n\n")
			}
		}
		return
	}

	sb.WriteString("Applies to all `open_long` / `open_short`.\n\n")
	appendOpenSLTPFieldRulesEN(sb, slOn, tpOn)
	if !slOn {
		sb.WriteString("### Minimum stop distance\n\n")
		sb.WriteString("- Stop-loss is **disabled**; backend does not enforce min SL distance. Do **not** use it as a `wait` reason in CoT.\n\n")
	} else {
		sb.WriteString("### Minimum stop distance (% of entry, underlying move)\n\n")
		sb.WriteString(fmt.Sprintf("- BTCUSDT, ETHUSDT: **≥ %.2f%%**\n", btcMin))
		sb.WriteString(fmt.Sprintf("- Other altcoins: **≥ %.2f%%**\n", altMin))
		if tpOn {
			sb.WriteString(fmt.Sprintf("- Price R:R ≥ **1:%.1f**; set valid SL first, then TP; never tighten SL below minimum to fake R:R\n\n", minRR))
		} else {
			sb.WriteString("- Take-profit is **disabled**; backend does not enforce open R:R.\n\n")
		}
	}
	if slOn && rc.EffectiveEnforceStructStop() {
		sb.WriteString("### Structural placement\n\n")
		sb.WriteString(fmt.Sprintf("- Primary structure timeframe: **%s**; do not use sub-TF micro highs/lows alone\n", structTF))
		sb.WriteString(fmt.Sprintf("- Short SL: above valid swing high + **%.2f%%** wick buffer\n", bufPct))
		sb.WriteString(fmt.Sprintf("- Long SL: below valid swing low + **%.2f%%** buffer\n", bufPct))
		sb.WriteString("- Timeframe mismatch → `wait`\n\n")
		sb.WriteString("In reasoning: cite structure level, SL distance %, and whether minimum distance is met.\n\n")
	}
	if slOn {
		appendStopLossReasoningRulesEN(sb)
		if tpOn {
			sb.WriteString("If minimum distance or R:R fails, use `wait` for that symbol.\n\n")
		} else {
			sb.WriteString("If minimum distance fails, use `wait` for that symbol.\n\n")
		}
	}
}

func appendStopLossTakeProfitDisabledNotice(sb *strings.Builder, lang Language) {
	if lang == LangChinese {
		sb.WriteString("## 止损止盈（策略已关闭）\n\n")
		sb.WriteString("- 本策略已**关闭**止损与止盈：回测/实盘**不会**下单 SL/TP，后端**不会**校验最小止损价距或开仓盈亏比。\n")
		sb.WriteString("- 开仓 JSON **勿**填写 `stop_loss` / `take_profit`（可省略）。\n")
		sb.WriteString("- 思维链**禁止**因「最小止损距离」「盈亏比不足」将标的判为 `wait`；按技术面、信心度与其它硬约束决策。\n\n")
		return
	}
	sb.WriteString("## Stop-Loss / Take-Profit (Disabled in Strategy)\n\n")
	sb.WriteString("- Stop-loss and take-profit are **off**: no SL/TP orders in backtest/live; backend **does not** enforce min SL distance or open R:R.\n")
	sb.WriteString("- On open, **omit** `stop_loss` and `take_profit` from JSON.\n")
	sb.WriteString("- In CoT, **do not** reject entries because min SL distance or R:R is insufficient; decide on setup quality, confidence, and other hard rules.\n\n")
}

func appendOpenSLTPFieldRulesZH(sb *strings.Builder, slOn, tpOn bool) {
	switch {
	case slOn && tpOn:
		sb.WriteString("适用于所有 `open_long` / `open_short`。`stop_loss` 与 `take_profit` 必须为**具体价格数字**，禁止公式。\n\n")
	case slOn:
		sb.WriteString("适用于所有 `open_long` / `open_short`。`stop_loss` 必须为**具体价格数字**，禁止公式；**勿**填 `take_profit`。\n\n")
	case tpOn:
		sb.WriteString("适用于所有 `open_long` / `open_short`。`take_profit` 必须为**具体价格数字**，禁止公式；**勿**填 `stop_loss`。\n\n")
	}
}

func appendOpenSLTPFieldRulesEN(sb *strings.Builder, slOn, tpOn bool) {
	switch {
	case slOn && tpOn:
		sb.WriteString("`stop_loss` and `take_profit` must be **numeric prices**, not formulas.\n\n")
	case slOn:
		sb.WriteString("`stop_loss` must be a **numeric price**; omit `take_profit`.\n\n")
	case tpOn:
		sb.WriteString("`take_profit` must be a **numeric price**; omit `stop_loss`.\n\n")
	}
}

func appendStopLossReasoningRulesZH(sb *strings.Builder) {
	sb.WriteString("### 止损逻辑（思维链必遵，禁止自相矛盾）\n\n")
	sb.WriteString("- **做空**：止损在入场价**上方**；**做多**：止损在入场价**下方**\n")
	sb.WriteString("- **止损价距越大 → 越不易被扫**；价距越小或止损越贴近结构位 → 越易被正常回踩扫损\n")
	sb.WriteString("- **禁止**写「止损高于近期高点所以更容易被扫」：止损高于结构高点 = 离入场更远，应是**更难**被扫，不是更容易\n")
	sb.WriteString("- 空单常见扫损原因：止损贴在主周期**摆动高点**一带，反弹**测前高**即触发——不是说止损离高点越远越容易亏\n")
	sb.WriteString("- 最小价距与结构位：结构位过近 → 价距 < 最小要求 → 对该标的 `wait`；结构位过远 → 止损过宽、盈亏比难达标 → 对该标的 `wait`\n")
	sb.WriteString("- 涉及开平仓时，思维链**按序**写出：**入场价 → 结构位（摆动高/低）→ 止损价 → 止损价距%**（并写明是否 ≥ 最小价距、盈亏比是否达标）\n\n")
}

func appendStopLossReasoningRulesEN(sb *strings.Builder) {
	sb.WriteString("### Stop-loss reasoning (mandatory in CoT; no contradictions)\n\n")
	sb.WriteString("- **Short**: stop_loss **above** entry; **long**: stop_loss **below** entry\n")
	sb.WriteString("- **Larger SL distance → harder to stop out**; tighter SL near structure → easier stop-out on routine retests\n")
	sb.WriteString("- **Never** claim \"SL above recent high → easier to stop out\": farther SL is **harder** to hit, not easier\n")
	sb.WriteString("- Common short stop-out: SL near swing **high**; a bounce **retesting the high** triggers SL — not because SL is far above the high\n")
	sb.WriteString("- Min distance vs structure: swing too close → distance < minimum → `wait`; swing too far → SL too wide, R:R fails → `wait`\n")
	sb.WriteString("- When opening/closing, CoT must list in order: **entry → structure level (swing high/low) → stop_loss price → SL distance %** (and whether minimum distance & R:R are met)\n\n")
}
