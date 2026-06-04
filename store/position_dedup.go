package store

import (
	"fmt"
	"math"
	"strings"
)

const closedExitTimeToleranceMs = int64(5 * 60 * 1000) // 5 minutes

func quantitiesClose(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	const absTol = 0.0001
	if math.Abs(a-b) <= absTol {
		return true
	}
	maxQ := math.Max(a, b)
	return math.Abs(a-b)/maxQ <= 0.001
}

func closedPositionMatchKey(symbol, side string, exitTimeMs int64, quantity float64) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = strings.ToUpper(strings.TrimSpace(side))
	// Bucket exit time to the minute to tolerate ms drift between sync paths.
	minute := exitTimeMs / 60000
	return fmt.Sprintf("%s|%s|%d|%.6f", symbol, side, minute, quantity)
}

// FindMatchingClosedPosition finds an existing CLOSED row for the same logical trade.
func (s *PositionStore) FindMatchingClosedPosition(traderID, symbol, side string, exitTimeMs int64, quantity float64) (*TraderPosition, error) {
	if traderID == "" || symbol == "" || side == "" || exitTimeMs <= 0 || quantity <= 0 {
		return nil, nil
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = strings.ToUpper(strings.TrimSpace(side))
	if side == "BUY" {
		side = "LONG"
	} else if side == "SELL" {
		side = "SHORT"
	}

	var candidates []TraderPosition
	err := s.db.Where(
		"trader_id = ? AND symbol = ? AND side = ? AND status = ? AND exit_time BETWEEN ? AND ?",
		traderID, symbol, side, "CLOSED",
		exitTimeMs-closedExitTimeToleranceMs,
		exitTimeMs+closedExitTimeToleranceMs,
	).Order("exit_time DESC").Find(&candidates).Error
	if err != nil {
		return nil, err
	}

	for i := range candidates {
		qty := candidates[i].EntryQuantity
		if qty <= 0 {
			qty = candidates[i].Quantity
		}
		if quantitiesClose(qty, quantity) {
			pos := candidates[i]
			if pos.EntryQuantity == 0 {
				pos.EntryQuantity = pos.Quantity
			}
			return &pos, nil
		}
	}
	return nil, nil
}

func isSyntheticExchangePositionID(id string) bool {
	id = strings.TrimSpace(id)
	return id == "" ||
		strings.HasPrefix(id, "sync_") ||
		strings.HasPrefix(id, "close_only_") ||
		strings.HasPrefix(id, "snapshot_")
}

// IsSyntheticExchangePositionID reports whether an exchange_position_id is a local placeholder, not an exchange posId.
func IsSyntheticExchangePositionID(id string) bool {
	return isSyntheticExchangePositionID(id)
}

// preferClosedPosition returns true if a should replace b when deduplicating display rows.
func preferClosedPosition(a, b *TraderPosition) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}

	aPnL := math.Abs(a.RealizedPnL)
	bPnL := math.Abs(b.RealizedPnL)
	if aPnL > bPnL+0.01 {
		return true
	}
	if bPnL > aPnL+0.01 {
		return false
	}

	aOfficial := !isSyntheticExchangePositionID(a.ExchangePositionID)
	bOfficial := !isSyntheticExchangePositionID(b.ExchangePositionID)
	if aOfficial && !bOfficial {
		return true
	}
	if bOfficial && !aOfficial {
		return false
	}

	return a.ID > b.ID
}

// DedupeClosedPositions merges duplicate CLOSED rows (same symbol/side/exit/qty) for UI/history.
func DedupeClosedPositions(positions []*TraderPosition) []*TraderPosition {
	if len(positions) <= 1 {
		return positions
	}

	bestByKey := make(map[string]*TraderPosition, len(positions))
	order := make([]string, 0, len(positions))

	for _, pos := range positions {
		if pos == nil {
			continue
		}
		qty := pos.EntryQuantity
		if qty <= 0 {
			qty = pos.Quantity
		}
		key := closedPositionMatchKey(pos.Symbol, pos.Side, pos.ExitTime, qty)
		if existing, ok := bestByKey[key]; ok {
			if preferClosedPosition(pos, existing) {
				bestByKey[key] = pos
			}
		} else {
			bestByKey[key] = pos
			order = append(order, key)
		}
	}

	out := make([]*TraderPosition, 0, len(order))
	for _, key := range order {
		if pos := bestByKey[key]; pos != nil {
			out = append(out, pos)
		}
	}
	return out
}
