package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
	"time"
)

const defaultClosedPositionLookback = 30 * 24 * time.Hour

// SyncClosedPositionsFromExchange imports exchange closed-position history into trader_positions.
func SyncClosedPositionsFromExchange(
	trader Trader,
	traderID, exchangeID, exchangeType string,
	st *store.Store,
	lookback time.Duration,
	limit int,
) error {
	if st == nil || trader == nil {
		return nil
	}
	if lookback <= 0 {
		lookback = defaultClosedPositionLookback
	}
	if limit <= 0 {
		limit = 100
	}

	posStore := st.Position()
	closedCount, err := posStore.CountClosedPositions(traderID)
	if err != nil {
		return err
	}

	var start time.Time
	if closedCount == 0 {
		start = time.Time{}
	} else {
		lastExitMs, err := posStore.GetLastClosedPositionTime(traderID)
		if err != nil {
			return err
		}
		start = time.UnixMilli(lastExitMs).UTC().Add(-2 * time.Hour)
		minStart := time.Now().UTC().Add(-lookback)
		if start.Before(minStart) {
			start = minStart
		}
	}

	records, err := trader.GetClosedPnL(start, limit)
	if err != nil {
		return fmt.Errorf("get closed PnL: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	storeRecords := make([]store.ClosedPnLRecord, 0, len(records))
	for _, r := range records {
		symbol := strings.ToUpper(strings.TrimSpace(r.Symbol))
		if symbol == "" {
			continue
		}
		storeRecords = append(storeRecords, store.ClosedPnLRecord{
			Symbol:          symbol,
			Side:            r.Side,
			EntryPrice:      r.EntryPrice,
			ExitPrice:       r.ExitPrice,
			Quantity:        r.Quantity,
			MaxOpenQuantity: r.MaxOpenQuantity,
			RealizedPnL:     r.RealizedPnL,
			Fee:         r.Fee,
			Leverage:    r.Leverage,
			EntryTime:   r.EntryTime.UTC().UnixMilli(),
			ExitTime:    r.ExitTime.UTC().UnixMilli(),
			OrderID:     r.OrderID,
			CloseType:   r.CloseType,
			ExchangeID:  r.ExchangeID,
		})
	}

	created, skipped, err := posStore.SyncClosedPositions(traderID, exchangeID, exchangeType, storeRecords)
	if err != nil {
		return err
	}
	logger.Infof("📜 Closed position sync [%s]: fetched %d, created %d, skipped %d",
		traderID, len(records), created, skipped)
	return nil
}
