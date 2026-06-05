package market

import (
	"fmt"
	"strings"
	"time"
)

// KlineRangeOptions selects the market data source for historical range fetches.
type KlineRangeOptions struct {
	// Exchange: okx, binance, etc. Empty defaults to okx when simulated is set, else binance.
	Exchange string
	// Simulated enables OKX demo-trading klines (x-simulated-trading: 1).
	Simulated bool
}

// GetKlinesRange fetches ascending OHLC klines between start and end (inclusive).
func GetKlinesRange(symbol string, timeframe string, start, end time.Time, opts KlineRangeOptions) ([]Kline, error) {
	ex := NormalizeKlineExchange(opts.Exchange)
	if ex == "" {
		if opts.Simulated {
			ex = "okx"
		} else {
			ex = "binance"
		}
	}
	switch ex {
	case "okx":
		return GetKlinesRangeOKX(symbol, timeframe, start, end, opts.Simulated)
	case "binance":
		return getKlinesRangeBinance(symbol, timeframe, start, end)
	default:
		return nil, fmt.Errorf("historical klines not supported for exchange %q", ex)
	}
}

// ResolveKlineRangeOptions picks exchange/simulated from explicit values or an OKX account testnet flag.
func ResolveKlineRangeOptions(exchange string, simulated *bool, okxTestnet bool) KlineRangeOptions {
	ex := strings.TrimSpace(exchange)
	if ex != "" {
		sim := false
		if simulated != nil {
			sim = *simulated
		}
		return KlineRangeOptions{Exchange: NormalizeKlineExchange(ex), Simulated: sim}
	}
	if okxTestnet {
		return KlineRangeOptions{Exchange: "okx", Simulated: true}
	}
	// Default: OKX live (aligns with primary CEX in this product); caller may override to binance.
	return KlineRangeOptions{Exchange: "okx", Simulated: false}
}
