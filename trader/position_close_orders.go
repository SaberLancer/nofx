package trader

import (
	"time"

	"nofx/trader/types"
)

// PositionCloseOrdersProvider is implemented by exchanges that can list close orders
// for a closed position window (e.g. OKX fills-history grouped by ordId).
type PositionCloseOrdersProvider interface {
	GetPositionCloseOrders(symbol, side string, entryTime, exitTime time.Time, limit int) ([]types.PositionCloseOrderRecord, error)
}
