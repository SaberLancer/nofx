package types

import "time"

// PositionCloseOrderRecord is one aggregated close/reduce order for a historical position.
type PositionCloseOrderRecord struct {
	ExchangeOrderID string
	OrderAction     string
	PositionSide    string
	ExecQuantity    float64
	ExecPrice       float64
	Fee             float64
	RealizedPnL     float64
	FilledAt        time.Time
	Status          string
}
