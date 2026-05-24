package market

import (
	"fmt"
	"strings"
	"time"
)

const defaultTickFetchTimeout = 10 * time.Second

// FetchRecentTicks loads recent public trades for a symbol from the bound exchange.
// Supported: okx, binance (and aliases that share Binance futures tape).
func FetchRecentTicks(exchange, symbol string, limit int) ([]RawTick, error) {
	if limit <= 0 {
		limit = 100
	}
	symbol = Normalize(symbol)
	exchange = strings.ToLower(strings.TrimSpace(exchange))

	switch exchange {
	case "okx":
		return fetchOKXRecentTicks(symbol, limit)
	case "binance", "bybit", "bitget", "gate", "kucoin", "aster", "indodax":
		// Public aggTrades on Binance futures is used as a liquid reference tape
		// when the bound exchange has no dedicated tick adapter yet.
		return fetchBinanceAggTrades(symbol, limit)
	default:
		return fetchBinanceAggTrades(symbol, limit)
	}
}

func okxInstID(symbol string) string {
	base := strings.TrimSuffix(Normalize(symbol), "USDT")
	return fmt.Sprintf("%s-USDT-SWAP", base)
}
