package store

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// PositionLiveKey builds a stable symbol_side key for matching exchange positions to DB rows.
func PositionLiveKey(symbol, side string) string {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	return sym + "_" + strings.ToLower(normalizePositionSide(side))
}

// ExchangePositionIDFromMap reads exchange position id from a generic exchange position map.
func ExchangePositionIDFromMap(pos map[string]interface{}) string {
	if pos == nil {
		return ""
	}
	for _, key := range []string{"exchangePositionId", "exchangePositionID", "posId"} {
		if id, ok := pos[key].(string); ok {
			id = strings.TrimSpace(id)
			if id != "" {
				return id
			}
		}
	}
	return ""
}

// PeakCacheKey builds the in-memory peak / PnL-enforce cache key for a live position.
func PeakCacheKey(exchangePositionID, symbol, side string) string {
	id := strings.TrimSpace(exchangePositionID)
	if id != "" && !isSyntheticExchangePositionID(id) {
		return "pos:" + id
	}
	return PositionLiveKey(symbol, side)
}

// BuildLiveExchangePositionIDs maps live symbol_side keys to official exchange position ids.
func BuildLiveExchangePositionIDs(positions []map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for _, pos := range positions {
		symbol, ok1 := pos["symbol"].(string)
		side, ok2 := pos["side"].(string)
		if !ok1 || !ok2 {
			continue
		}
		qty := 0.0
		if amt, ok := pos["positionAmt"].(float64); ok {
			qty = math.Abs(amt)
		}
		if qty <= 0 {
			continue
		}
		exPosID := ExchangePositionIDFromMap(pos)
		if exPosID == "" || isSyntheticExchangePositionID(exPosID) {
			continue
		}
		out[PositionLiveKey(symbol, side)] = exPosID
	}
	return out
}

// BuildLivePeakCacheKeys returns peak-cache keys for currently live exchange positions.
func BuildLivePeakCacheKeys(positions []map[string]interface{}) map[string]bool {
	keys := make(map[string]bool)
	for _, pos := range positions {
		symbol, ok1 := pos["symbol"].(string)
		side, ok2 := pos["side"].(string)
		if !ok1 || !ok2 {
			continue
		}
		qty := 0.0
		if amt, ok := pos["positionAmt"].(float64); ok {
			qty = math.Abs(amt)
		}
		if qty <= 0 {
			continue
		}
		keys[PeakCacheKey(ExchangePositionIDFromMap(pos), symbol, side)] = true
	}
	return keys
}

// BuildLivePositionKeys returns live position keys from generic exchange position maps.
func BuildLivePositionKeys(positions []map[string]interface{}) map[string]bool {
	keys := make(map[string]bool)
	for _, pos := range positions {
		symbol, ok1 := pos["symbol"].(string)
		side, ok2 := pos["side"].(string)
		if !ok1 || !ok2 {
			continue
		}
		qty := 0.0
		if amt, ok := pos["positionAmt"].(float64); ok {
			qty = math.Abs(amt)
		}
		if qty <= 0 {
			continue
		}
		keys[PositionLiveKey(symbol, side)] = true
	}
	return keys
}

// ResetOpenPositionPeakPnLPct clears peak margin PnL% on all OPEN rows for symbol+side.
func (s *PositionStore) ResetOpenPositionPeakPnLPct(traderID, symbol, side string) error {
	side = normalizePositionSide(side)
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	nowMs := time.Now().UTC().UnixMilli()

	q := s.db.Model(&TraderPosition{}).
		Where("trader_id = ? AND side = ? AND status = ?", traderID, side, "OPEN")
	if strings.HasSuffix(sym, "USDT") {
		base := strings.TrimSuffix(sym, "USDT")
		q = q.Where("symbol = ? OR symbol = ?", sym, base)
	} else {
		q = q.Where("symbol = ?", sym)
	}
	return q.Updates(map[string]interface{}{
		"peak_pnl_pct": 0,
		"updated_at":   nowMs,
	}).Error
}

// ReconcileOpenPositions closes DB OPEN rows that are absent on the exchange, and dedupes
// multiple OPEN rows for the same live symbol+side. When livePosIDs is provided, prefers
// the row whose exchange_position_id matches the live exchange position.
func (s *PositionStore) ReconcileOpenPositions(traderID string, liveKeys map[string]bool, livePosIDs map[string]string) (int, error) {
	opens, err := s.GetOpenPositions(traderID)
	if err != nil {
		return 0, err
	}
	if len(opens) == 0 {
		return 0, nil
	}

	keepID := make(map[string]int64)
	for _, pos := range opens {
		if pos == nil {
			continue
		}
		key := PositionLiveKey(pos.Symbol, pos.Side)
		if !liveKeys[key] {
			continue
		}
		livePosID := ""
		if livePosIDs != nil {
			livePosID = livePosIDs[key]
		}
		if id, ok := keepID[key]; !ok || preferOpenPositionForLive(findOpenByID(opens, id), pos, livePosID) {
			keepID[key] = pos.ID
		}
	}

	closed := 0
	nowMs := time.Now().UTC().UnixMilli()
	for _, pos := range opens {
		if pos == nil {
			continue
		}
		key := PositionLiveKey(pos.Symbol, pos.Side)
		if !liveKeys[key] {
			if err := s.closeReconciledOpen(pos, nowMs, "reconcile_ghost"); err != nil {
				return closed, err
			}
			closed++
			continue
		}
		if keepID[key] != pos.ID {
			if err := s.closeReconciledOpen(pos, nowMs, "reconcile_duplicate"); err != nil {
				return closed, err
			}
			closed++
		}
	}
	return closed, nil
}

// BindOpenPositionExchangePositionID links the newest OPEN row for symbol+side to the live exchange posId.
func (s *PositionStore) BindOpenPositionExchangePositionID(traderID, exchangeID, symbol, side, exchangePositionID string) error {
	exchangePositionID = strings.TrimSpace(exchangePositionID)
	if exchangePositionID == "" || exchangeID == "" || isSyntheticExchangePositionID(exchangePositionID) {
		return nil
	}

	existing, err := s.GetOpenPositionByExchangePositionID(exchangeID, exchangePositionID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	pos, err := s.GetOpenPositionBySymbol(traderID, symbol, side)
	if err != nil {
		return err
	}
	if pos == nil {
		return nil
	}
	if pos.ExchangePositionID == exchangePositionID {
		return nil
	}
	if pos.ExchangePositionID != "" && !isSyntheticExchangePositionID(pos.ExchangePositionID) {
		return nil
	}

	nowMs := time.Now().UTC().UnixMilli()
	updates := map[string]interface{}{
		"exchange_position_id": exchangePositionID,
		"peak_pnl_pct":         0,
		"updated_at":           nowMs,
	}
	if strings.TrimSpace(pos.ExchangeID) == "" {
		updates["exchange_id"] = exchangeID
	}
	return s.db.Model(&TraderPosition{}).Where("id = ?", pos.ID).Updates(updates).Error
}

// BindLiveExchangePositionIDs binds all live exchange positions to their OPEN DB rows.
func (s *PositionStore) BindLiveExchangePositionIDs(traderID, exchangeID string, positions []map[string]interface{}) error {
	for _, pos := range positions {
		symbol, ok1 := pos["symbol"].(string)
		side, ok2 := pos["side"].(string)
		if !ok1 || !ok2 {
			continue
		}
		qty := 0.0
		if amt, ok := pos["positionAmt"].(float64); ok {
			qty = math.Abs(amt)
		}
		if qty <= 0 {
			continue
		}
		exPosID := ExchangePositionIDFromMap(pos)
		if exPosID == "" {
			continue
		}
		if err := s.BindOpenPositionExchangePositionID(traderID, exchangeID, symbol, side, exPosID); err != nil {
			return err
		}
	}
	return nil
}

func findOpenByID(opens []*TraderPosition, id int64) *TraderPosition {
	for _, pos := range opens {
		if pos != nil && pos.ID == id {
			return pos
		}
	}
	return nil
}

func preferOpenPositionForLive(current, candidate *TraderPosition, liveExchangePosID string) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}

	liveExchangePosID = strings.TrimSpace(liveExchangePosID)
	if liveExchangePosID != "" {
		curMatch := strings.TrimSpace(current.ExchangePositionID) == liveExchangePosID
		candMatch := strings.TrimSpace(candidate.ExchangePositionID) == liveExchangePosID
		if candMatch && !curMatch {
			return true
		}
		if curMatch && !candMatch {
			return false
		}
	}

	curOfficial := !isSyntheticExchangePositionID(current.ExchangePositionID)
	candOfficial := !isSyntheticExchangePositionID(candidate.ExchangePositionID)
	if candOfficial && !curOfficial {
		return true
	}
	if curOfficial && !candOfficial {
		return false
	}

	return candidate.EntryTime > current.EntryTime
}

func (s *PositionStore) closeReconciledOpen(pos *TraderPosition, exitTimeMs int64, reason string) error {
	if pos == nil {
		return nil
	}
	exitPrice := pos.EntryPrice
	if exitPrice <= 0 && pos.ExitPrice > 0 {
		exitPrice = pos.ExitPrice
	}
	if err := s.ClosePositionFully(pos.ID, exitPrice, "", exitTimeMs, pos.RealizedPnL, pos.Fee, reason); err != nil {
		return fmt.Errorf("close reconciled open id=%d: %w", pos.ID, err)
	}
	return nil
}
