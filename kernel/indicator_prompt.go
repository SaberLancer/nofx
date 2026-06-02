package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/store"
	"strings"
)

// AppendConfiguredIndicatorDataGuide lists only indicators/data enabled in strategy config.
func AppendConfiguredIndicatorDataGuide(sb *strings.Builder, ind store.IndicatorConfig, coin store.CoinSourceConfig, lang Language) {
	if lang == LangChinese {
		sb.WriteString("## 本周期可用数据（由策略配置生成）\n\n")
		sb.WriteString("**仅**可使用下列数据；未列出项本策略**未提供**，禁止在思维链中引用（勿编造 RSI/MACD/EMA 等未启用指标）。\n\n")
	} else {
		sb.WriteString("## Available Data This Strategy (from config)\n\n")
		sb.WriteString("Use **only** the items below; anything not listed is **not provided** — do not cite or invent it in reasoning.\n\n")
	}

	lines := configuredIndicatorLines(ind, coin, lang)
	if len(lines) == 0 {
		if lang == LangChinese {
			sb.WriteString("- （未启用任何行情指标，仅账户/持仓信息）\n\n")
		} else {
			sb.WriteString("- (No market indicators enabled; account/position info only)\n\n")
		}
		return
	}
	for _, line := range lines {
		sb.WriteString(line)
	}
	sb.WriteString("\n")
}

func configuredIndicatorLines(ind store.IndicatorConfig, coin store.CoinSourceConfig, lang Language) []string {
	var lines []string
	k := ind.Klines
	primaryTF := strings.TrimSpace(k.PrimaryTimeframe)
	if primaryTF == "" {
		primaryTF = "3m"
	}
	count := k.PrimaryCount
	if count <= 0 {
		count = 30
	}

	if len(k.SelectedTimeframes) > 0 {
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- K线（多周期）: %v，每周期约 %d 根 OHLCV", k.SelectedTimeframes, count))
		} else {
			lines = append(lines, fmt.Sprintf("- K-lines (multi-TF): %v, ~%d OHLCV bars each", k.SelectedTimeframes, count))
		}
	} else {
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- K线主周期: %s，%d 根 OHLCV", primaryTF, count))
		} else {
			lines = append(lines, fmt.Sprintf("- K-line primary: %s, %d OHLCV bars", primaryTF, count))
		}
		if k.EnableMultiTimeframe && strings.TrimSpace(k.LongerTimeframe) != "" {
			longerCount := k.LongerCount
			if longerCount <= 0 {
				longerCount = count
			}
			if lang == LangChinese {
				lines = append(lines, fmt.Sprintf("- K线长周期: %s，%d 根", k.LongerTimeframe, longerCount))
			} else {
				lines = append(lines, fmt.Sprintf("- K-line longer TF: %s, %d bars", k.LongerTimeframe, longerCount))
			}
		}
	}

	if ind.EnableRawKlines || len(lines) > 0 {
		// Raw OHLCV is always fetched when klines exist; note only if explicit or implied
		if ind.EnableRawKlines {
			if lang == LangChinese {
				lines = append(lines, "- 原始 K 线 OHLCV 表（按周期展示）")
			} else {
				lines = append(lines, "- Raw OHLCV table per timeframe")
			}
		}
	}

	if ind.EnableEMA {
		periods := effectiveIntPeriods(ind.EMAPeriods, 20, 50)
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- EMA 周期: %v", periods))
		} else {
			lines = append(lines, fmt.Sprintf("- EMA periods: %v", periods))
		}
	}
	if ind.EnableMACD {
		if lang == LangChinese {
			lines = append(lines, "- MACD")
		} else {
			lines = append(lines, "- MACD")
		}
	}
	if ind.EnableRSI {
		periods := effectiveIntPeriods(ind.RSIPeriods, 7, 14)
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- RSI 周期: %v", periods))
		} else {
			lines = append(lines, fmt.Sprintf("- RSI periods: %v", periods))
		}
	}
	if ind.EnableATR {
		periods := effectiveIntPeriods(ind.ATRPeriods, 14)
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- ATR 周期: %v", periods))
		} else {
			lines = append(lines, fmt.Sprintf("- ATR periods: %v", periods))
		}
	}
	if ind.EnableBOLL {
		periods := effectiveIntPeriods(ind.BOLLPeriods, 20)
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 布林带 BOLL 周期: %v（上/中/下轨）", periods))
		} else {
			lines = append(lines, fmt.Sprintf("- Bollinger Bands periods: %v (upper/mid/lower)", periods))
		}
	}
	if ind.EnableVolume {
		if lang == LangChinese {
			lines = append(lines, "- 成交量 Volume")
		} else {
			lines = append(lines, "- Volume")
		}
	}
	if ind.EnableOI {
		if lang == LangChinese {
			line := "- 持仓量 OI（Latest/Average）"
			if ind.EnableFundingRate {
				line += "；资金费率 Funding Rate"
			}
			lines = append(lines, line)
		} else {
			line := "- Open Interest (latest/avg)"
			if ind.EnableFundingRate {
				line += "; funding rate"
			}
			lines = append(lines, line)
		}
	} else if ind.EnableFundingRate {
		if lang == LangChinese {
			lines = append(lines, "- 资金费率 Funding Rate")
		} else {
			lines = append(lines, "- Funding rate")
		}
	}

	if ind.EnableQuantData {
		var parts []string
		if ind.EnableQuantOI {
			if lang == LangChinese {
				parts = append(parts, "量化 OI 变化")
			} else {
				parts = append(parts, "quant OI change")
			}
		}
		if ind.EnableQuantNetflow {
			if lang == LangChinese {
				parts = append(parts, "资金流向 Netflow")
			} else {
				parts = append(parts, "netflow")
			}
		}
		if len(parts) > 0 {
			if lang == LangChinese {
				lines = append(lines, "- 量化数据: "+strings.Join(parts, "、"))
			} else {
				lines = append(lines, "- Quant data: "+strings.Join(parts, ", "))
			}
		}
	}

	if ind.EnableOIRanking {
		dur := ind.OIRankingDuration
		if dur == "" {
			dur = "1h"
		}
		limit := ind.OIRankingLimit
		if limit <= 0 {
			limit = 10
		}
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 全市场 OI 排名（%s，Top %d）", dur, limit))
		} else {
			lines = append(lines, fmt.Sprintf("- Market-wide OI ranking (%s, top %d)", dur, limit))
		}
	}
	if ind.EnableNetFlowRanking {
		dur := ind.NetFlowRankingDuration
		if dur == "" {
			dur = "1h"
		}
		limit := ind.NetFlowRankingLimit
		if limit <= 0 {
			limit = 10
		}
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 资金流向排名（%s，Top %d）", dur, limit))
		} else {
			lines = append(lines, fmt.Sprintf("- Netflow ranking (%s, top %d)", dur, limit))
		}
	}
	if ind.EnablePriceRanking {
		dur := ind.PriceRankingDuration
		if dur == "" {
			dur = "1h"
		}
		limit := ind.PriceRankingLimit
		if limit <= 0 {
			limit = 10
		}
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 涨跌幅排名（%s，Top %d）", dur, limit))
		} else {
			lines = append(lines, fmt.Sprintf("- Price change ranking (%s, top %d)", dur, limit))
		}
	}

	for _, src := range ind.ExternalDataSources {
		if strings.TrimSpace(src.Name) == "" {
			continue
		}
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 外部数据: %s", src.Name))
		} else {
			lines = append(lines, fmt.Sprintf("- External: %s", src.Name))
		}
	}

	if coin.UseAI500 || coin.UseOITop {
		var tags []string
		if coin.UseAI500 {
			tags = append(tags, "AI500")
		}
		if coin.UseOITop {
			tags = append(tags, "OI_Top")
		}
		if lang == LangChinese {
			lines = append(lines, fmt.Sprintf("- 候选币标签: %s（若有）", strings.Join(tags, "、")))
		} else {
			lines = append(lines, fmt.Sprintf("- Candidate tags: %s (if present)", strings.Join(tags, ", ")))
		}
	}

	return lines
}

func effectiveIntPeriods(custom []int, defaults ...int) []int {
	if len(custom) > 0 {
		return custom
	}
	return defaults
}

// appendConfiguredEntryStandards writes entry section from strategy config (no generic indicator defaults).
func (e *StrategyEngine) appendConfiguredEntryStandards(sb *strings.Builder, lang Language, minConfidence int) {
	if e.config.PromptSections.EntryStandards != "" {
		sb.WriteString(e.config.PromptSections.EntryStandards)
		sb.WriteString("\n\n")
	} else if lang == LangChinese {
		sb.WriteString("# 🎯 入场分析要求\n\n")
		sb.WriteString("- 仅根据下方「本周期可用数据」中**已启用**的指标论证，不得引用未提供数据。\n")
		sb.WriteString("- 信号不足或数据矛盾时，对该标的使用 `wait`。\n")
	} else {
		sb.WriteString("# 🎯 Entry Analysis Requirements\n\n")
		sb.WriteString("- Argue entries **only** from enabled data listed below; never cite disabled indicators.\n")
		sb.WriteString("- Use `wait` when setup is unclear or data conflicts.\n")
	}
	AppendConfiguredIndicatorDataGuide(sb, e.config.Indicators, e.config.CoinSource, lang)
	if lang == LangChinese {
		sb.WriteString(fmt.Sprintf("**开仓信心度 ≥ %d**（%d 以下请 wait）。\n\n", minConfidence, minConfidence))
	} else {
		sb.WriteString(fmt.Sprintf("**Confidence ≥ %d** to open (%d below → wait).\n\n", minConfidence, minConfidence))
	}
}

// formatBTCMarketSnapshot formats BTC header for user prompt using only enabled indicators.
func (e *StrategyEngine) formatBTCMarketSnapshot(btc *market.Data) string {
	ind := e.config.Indicators
	var parts []string
	parts = append(parts, fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%)", btc.CurrentPrice, btc.PriceChange1h, btc.PriceChange4h))
	if ind.EnableMACD {
		parts = append(parts, fmt.Sprintf("MACD: %.4f", btc.CurrentMACD))
	}
	if ind.EnableRSI {
		rsi := btc.CurrentRSI7
		if len(ind.RSIPeriods) > 0 && ind.RSIPeriods[0] == 14 {
			// prefer 14 if configured; snapshot uses CurrentRSI7 from primary series
		}
		parts = append(parts, fmt.Sprintf("RSI: %.2f", rsi))
	}
	if ind.EnableEMA {
		parts = append(parts, fmt.Sprintf("EMA20: %.2f", btc.CurrentEMA20))
	}
	return strings.Join(parts, " | ") + "\n\n"
}
