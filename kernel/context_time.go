package kernel

import (
	"fmt"
	"time"
)

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*3600)

// ToBeijing converts an instant to Asia/Shanghai (UTC+8).
func ToBeijing(t time.Time) time.Time {
	return t.In(beijingLocation)
}

// FormatBeijingDateTime formats full datetime in Beijing time.
func FormatBeijingDateTime(t time.Time) string {
	return ToBeijing(t).Format("2006-01-02 15:04:05")
}

// FormatBeijingKlineTime formats kline bar time (MM-DD HH:mm) in Beijing time.
func FormatBeijingKlineTime(t time.Time) string {
	return ToBeijing(t).Format("01-02 15:04")
}

// FormatBeijingKlineTimeMs formats millisecond timestamp as Beijing kline time.
func FormatBeijingKlineTimeMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return FormatBeijingKlineTime(time.UnixMilli(ms))
}

// FormatDecisionContextTime formats the decision reference instant (Beijing only).
func FormatDecisionContextTime(t time.Time, lang Language) string {
	bj := FormatBeijingDateTime(t)
	if lang == LangChinese {
		return fmt.Sprintf("北京时间 %s", bj)
	}
	return fmt.Sprintf("Beijing Time %s", bj)
}

// FormatDecisionContextTimeLine builds the standard decision header time line.
func FormatDecisionContextTimeLine(t time.Time, lang Language, callCount, runtimeMinutes int) string {
	if lang == LangChinese {
		return fmt.Sprintf("决策时刻: %s | 周期: #%d | 运行: %d 分钟",
			FormatDecisionContextTime(t, lang), callCount, runtimeMinutes)
	}
	return fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes",
		FormatDecisionContextTime(t, lang), callCount, runtimeMinutes)
}

// DecisionReferenceTime resolves the effective decision instant from context.
func DecisionReferenceTime(ctx *Context) time.Time {
	if ctx != nil && ctx.ReferenceTimeMs > 0 {
		return time.UnixMilli(ctx.ReferenceTimeMs).UTC()
	}
	return time.Now().UTC()
}
