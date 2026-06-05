package types

import (
	"fmt"
	"math"
	"strings"
)

// DedupeClosedPnLRecords merges duplicate exchange closed-position rows from pagination
// overlap (same posId + same uTime). Different close events for the same posId (e.g.
// partial then full) are kept separate to match OKX App history.
func DedupeClosedPnLRecords(records []ClosedPnLRecord) []ClosedPnLRecord {
	if len(records) <= 1 {
		return records
	}

	bestByKey := make(map[string]ClosedPnLRecord, len(records))
	order := make([]string, 0, len(records))

	for _, rec := range records {
		key := closedPnLRecordKey(rec)
		if existing, ok := bestByKey[key]; ok {
			if preferClosedPnLRecord(rec, existing) {
				bestByKey[key] = rec
			}
			continue
		}
		bestByKey[key] = rec
		order = append(order, key)
	}

	out := make([]ClosedPnLRecord, 0, len(order))
	for _, key := range order {
		out = append(out, bestByKey[key])
	}
	return out
}

func closedPnLRecordKey(rec ClosedPnLRecord) string {
	id := strings.TrimSpace(rec.ExchangeID)
	if id != "" {
		return fmt.Sprintf("pos:%s|%d", id, rec.ExitTime.UnixMilli())
	}
	symbol := strings.ToUpper(strings.TrimSpace(rec.Symbol))
	side := strings.ToUpper(strings.TrimSpace(rec.Side))
	return fmt.Sprintf("%s|%s|%d|%.6f|%.4f|%.4f",
		symbol, side, rec.ExitTime.UnixMilli(), rec.Quantity, rec.EntryPrice, rec.ExitPrice)
}

func preferClosedPnLRecord(a, b ClosedPnLRecord) bool {
	rank := func(closeType string) int {
		switch strings.ToLower(strings.TrimSpace(closeType)) {
		case "liquidation":
			return 4
		case "full":
			return 3
		case "partial":
			return 2
		default:
			return 1
		}
	}
	if rank(a.CloseType) != rank(b.CloseType) {
		return rank(a.CloseType) > rank(b.CloseType)
	}
	aPnL := math.Abs(a.RealizedPnL)
	bPnL := math.Abs(b.RealizedPnL)
	if math.Abs(aPnL-bPnL) > 0.01 {
		return aPnL > bPnL
	}
	return a.ExitTime.After(b.ExitTime)
}
