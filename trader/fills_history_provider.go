package trader

import "time"

// FillsHistoryClosedPnLProvider is implemented by exchanges that reconstruct
// closed positions from historical fill/delegation records (e.g. OKX fills-history).
type FillsHistoryClosedPnLProvider interface {
	GetClosedPnLFromFillsHistory(startTime time.Time, limit int) ([]ClosedPnLRecord, error)
}
