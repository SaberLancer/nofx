package trader

import "nofx/trader/history"

// RebuildPositionsFromOrderActions reconstructs closed positions from exchange fills
// that carry open_/close_ order actions (OKX fills-history).
func RebuildPositionsFromOrderActions(trades []TradeRecord) []ClosedPnLRecord {
	return history.RebuildPositionsFromOrderActions(trades)
}
