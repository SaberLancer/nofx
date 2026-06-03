package market

// KlineOptions controls exchange-specific market data sourcing.
type KlineOptions struct {
	// Simulated enables OKX demo-trading market data (x-simulated-trading: 1).
	// When true, OKX klines/ticks must not fall back to live Binance/CoinAnk data.
	Simulated bool
}
