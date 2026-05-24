package market

import "time"

// TickSide indicates aggressive trade direction on the tape.
type TickSide string

const (
	TickSideBuy  TickSide = "buy"
	TickSideSell TickSide = "sell"
)

// RawTick is one exchange trade print (TICK-level data).
type RawTick struct {
	Symbol    string
	Price     float64
	Quantity  float64
	Side      TickSide
	Timestamp time.Time
}

// TrendDirection is the pre-decision output used to gate AI analysis.
type TrendDirection string

const (
	TrendNone  TrendDirection = "none"
	TrendLong  TrendDirection = "long"
	TrendShort TrendDirection = "short"
)

// TrendSignal summarizes real-time tick trend for one symbol.
type TrendSignal struct {
	Symbol        string
	Direction     TrendDirection
	BuyPressure   float64 // buy volume / total volume in window
	SellPressure  float64
	MomentumPct   float64 // price change % over window
	TickCount     int
	WindowSeconds int
	UpdatedAt     time.Time
}
