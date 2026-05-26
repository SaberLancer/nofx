package kernel

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var rePctNumber = regexp.MustCompile(`([+\-−]?\d+(?:\.\d+)?)\s*%`)

// underlyingPriceChangePct is raw price move % (no margin/leverage), for comparison only.
func underlyingPriceChangePct(pos PositionInfo) float64 {
	if pos.EntryPrice <= 0 {
		return 0
	}
	if strings.EqualFold(pos.Side, "short") {
		return (pos.EntryPrice - pos.MarkPrice) / pos.EntryPrice * 100
	}
	return (pos.MarkPrice - pos.EntryPrice) / pos.EntryPrice * 100
}

func symbolMentioned(text, symbol string) bool {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	base := strings.TrimSuffix(sym, "USDT")
	upper := strings.ToUpper(text)
	if strings.Contains(upper, sym) {
		return true
	}
	if base != "" && strings.Contains(upper, base) {
		return true
	}
	return false
}

func formatPctSigned(v float64) string {
	return fmt.Sprintf("%+.2f%%", v)
}

// FixReasoningPositionPnLPct rewrites CoT percentages that match underlying price change
// but not system margin PnL%, so UI and reasoning stay aligned with position cards.
func FixReasoningPositionPnLPct(reasoning string, positions []PositionInfo) string {
	if reasoning == "" || len(positions) == 0 {
		return reasoning
	}

	out := reasoning
	for _, pos := range positions {
		systemPct := pos.UnrealizedPnLPct
		pricePct := underlyingPriceChangePct(pos)
		if math.Abs(systemPct-pricePct) < 0.15 {
			continue
		}
		if !symbolMentioned(out, pos.Symbol) {
			continue
		}

		loc := strings.Index(strings.ToUpper(out), strings.ToUpper(pos.Symbol))
		if loc < 0 {
			base := strings.TrimSuffix(strings.ToUpper(pos.Symbol), "USDT")
			loc = strings.Index(strings.ToUpper(out), base)
		}
		if loc < 0 {
			continue
		}

		start := loc - 120
		if start < 0 {
			start = 0
		}
		end := loc + 220
		if end > len(out) {
			end = len(out)
		}
		window := out[start:end]

		matches := rePctNumber.FindAllStringSubmatchIndex(window, -1)
		if len(matches) == 0 {
			continue
		}

		// Replace from end to start to preserve indices
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			numStart := start + m[2]
			numEnd := start + m[3]
			raw := out[numStart:numEnd]
			raw = strings.ReplaceAll(raw, "−", "-")
			var parsed float64
			if _, err := fmt.Sscanf(raw, "%f", &parsed); err != nil {
				continue
			}

			windowLower := strings.ToLower(window)
			claimsMarginPnL := strings.Contains(windowLower, "margin pnl") ||
				strings.Contains(window, "Margin PnL") ||
				strings.Contains(window, "保证金盈亏") ||
				strings.Contains(window, "未实现盈亏")

			// Replace when value matches price-change % but not system margin PnL%
			shouldReplace := math.Abs(parsed-pricePct) < 0.25 &&
				math.Abs(systemPct-pricePct) > 0.15 &&
				math.Abs(parsed-systemPct) > 0.15

			if !shouldReplace && !claimsMarginPnL {
				continue
			}
			if !shouldReplace && claimsMarginPnL {
				// Mislabeled "Margin PnL%" that is actually price-change %
				if math.Abs(parsed-pricePct) >= math.Abs(parsed-systemPct) {
					shouldReplace = math.Abs(parsed-pricePct) < 0.3
				}
			}
			if !shouldReplace {
				continue
			}

			replacement := fmt.Sprintf("%+.2f", systemPct)
			out = out[:numStart] + replacement + out[numEnd:]
		}
	}

	return out
}

// AppendMandatoryPositionPnLQuotes adds copy-paste PnL% lines for the reasoning section.
func AppendMandatoryPositionPnLQuotes(sb *strings.Builder, positions []PositionInfo, lang Language) {
	if len(positions) == 0 {
		return
	}

	if lang == LangChinese {
		sb.WriteString("## 思维链必须引用的持仓 PnL%（禁止自算涨跌幅）\n")
		sb.WriteString("描述「未实现盈亏」「浮盈/浮亏」时，**只能**抄写下列 **Margin PnL%**，不得用 (Current−Entry)/Entry 替代：\n")
	} else {
		sb.WriteString("## Mandatory PnL% for <reasoning> (do not use price-change %)\n")
		sb.WriteString("When stating unrealized P&L, copy **only** these **Margin PnL%** values (not underlying price change):\n")
	}

	for _, pos := range positions {
		pricePct := underlyingPriceChangePct(pos)
		sb.WriteString(fmt.Sprintf("- %s %s: Margin PnL%% = %s",
			pos.Symbol, strings.ToUpper(pos.Side), formatPctSigned(pos.UnrealizedPnLPct)))
		if math.Abs(pricePct-pos.UnrealizedPnLPct) >= 0.15 {
			if lang == LangChinese {
				sb.WriteString(fmt.Sprintf("（标的价格变动约 %s，**不得**当作未实现盈亏%%）", formatPctSigned(pricePct)))
			} else {
				sb.WriteString(fmt.Sprintf(" (underlying price change ~%s — **not** unrealized PnL%%)", formatPctSigned(pricePct)))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}
