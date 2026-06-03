package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nofx/logger"
	"nofx/market"
	"nofx/provider/alpaca"
	"nofx/provider/hyperliquid"
	"nofx/provider/twelvedata"

	"github.com/gin-gonic/gin"
)

// handleKlines K-line data (supports multiple exchanges via coinank)
func (s *Server) handleKlines(c *gin.Context) {
	// Get query parameters
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	interval := c.DefaultQuery("interval", "5m")
	exchange := c.DefaultQuery("exchange", "binance") // Default to binance for backward compatibility
	limitStr := c.DefaultQuery("limit", "1000")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 1000
	}

	// Coinank API has a maximum limit of 1500 klines per request
	if limit > 1500 {
		limit = 1500
	}

	var klines []market.Kline
	exchangeLower := strings.ToLower(exchange)

	// Route to appropriate data source based on exchange type
	switch exchangeLower {
	case "alpaca":
		// US Stocks via Alpaca
		klines, err = s.getKlinesFromAlpaca(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from Alpaca", err)
			return
		}
	case "forex", "metals":
		// Forex and Metals via Twelve Data
		klines, err = s.getKlinesFromTwelveData(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from TwelveData", err)
			return
		}
	case "hyperliquid", "hyperliquid-xyz", "xyz":
		// Hyperliquid native API - supports both crypto perps and stock perps (xyz dex)
		klines, err = s.getKlinesFromHyperliquid(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from Hyperliquid", err)
			return
		}
	default:
		// Crypto exchanges: OKX direct (live/simulated) → CoinAnk → Binance fallback
		symbol = market.Normalize(symbol)
		opts := market.KlineOptions{Simulated: parseKlineSimulatedQuery(c)}
		klines, err = market.GetExchangeKlines(symbol, interval, exchange, limit, opts)
		if err != nil {
			SafeInternalError(c, "Get klines", err)
			return
		}
	}

	c.JSON(http.StatusOK, klines)
}

func parseKlineSimulatedQuery(c *gin.Context) bool {
	for _, key := range []string{"simulated", "testnet"} {
		switch strings.ToLower(strings.TrimSpace(c.Query(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

// getKlinesFromAlpaca fetches kline data from Alpaca API for US stocks
func (s *Server) getKlinesFromAlpaca(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Alpaca client
	client := alpaca.NewClient()

	// Map interval to Alpaca timeframe format
	timeframe := alpaca.MapTimeframe(interval)

	// Fetch bars from Alpaca
	ctx := context.Background()
	bars, err := client.GetBars(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("alpaca API error: %w", err)
	}

	// Convert Alpaca bars to market.Kline format
	klines := make([]market.Kline, len(bars))
	for i, bar := range bars {
		klines[i] = market.Kline{
			OpenTime:    bar.Timestamp.UnixMilli(),
			Open:        bar.Open,
			High:        bar.High,
			Low:         bar.Low,
			Close:       bar.Close,
			Volume:      float64(bar.Volume),             // share count
			QuoteVolume: float64(bar.Volume) * bar.Close, // turnover = shares * close price (USD)
			CloseTime:   bar.Timestamp.UnixMilli(),
		}
	}

	return klines, nil
}

// getKlinesFromTwelveData fetches kline data from Twelve Data API for forex and metals
func (s *Server) getKlinesFromTwelveData(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Twelve Data client
	client := twelvedata.NewClient()

	// Map interval to Twelve Data timeframe format
	timeframe := twelvedata.MapTimeframe(interval)

	// Fetch time series from Twelve Data
	ctx := context.Background()
	result, err := client.GetTimeSeries(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("twelvedata API error: %w", err)
	}

	// Convert Twelve Data bars to market.Kline format
	// Note: Twelve Data returns bars in reverse order (newest first)
	klines := make([]market.Kline, len(result.Values))
	for i, bar := range result.Values {
		open, high, low, close, volume, timestamp, err := twelvedata.ParseBar(bar)
		if err != nil {
			logger.Warnf("⚠️ Failed to parse TwelveData bar: %v", err)
			continue
		}

		// Reverse order: put oldest first
		idx := len(result.Values) - 1 - i
		klines[idx] = market.Kline{
			OpenTime:  timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: timestamp,
		}
	}

	return klines, nil
}

// getKlinesFromHyperliquid fetches kline data from Hyperliquid API
// Supports both crypto perps (default dex) and stock perps/forex/commodities (xyz dex)
func (s *Server) getKlinesFromHyperliquid(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Hyperliquid client
	client := hyperliquid.NewClient()

	// Map interval to Hyperliquid format
	timeframe := hyperliquid.MapTimeframe(interval)

	// Fetch candles from Hyperliquid
	// FormatCoinForAPI will automatically add xyz: prefix for stock perps
	ctx := context.Background()
	candles, err := client.GetCandles(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("hyperliquid API error: %w", err)
	}

	// Convert Hyperliquid candles to market.Kline format
	klines := make([]market.Kline, len(candles))
	for i, candle := range candles {
		open, _ := strconv.ParseFloat(candle.Open, 64)
		high, _ := strconv.ParseFloat(candle.High, 64)
		low, _ := strconv.ParseFloat(candle.Low, 64)
		close, _ := strconv.ParseFloat(candle.Close, 64)
		volume, _ := strconv.ParseFloat(candle.Volume, 64)

		klines[i] = market.Kline{
			OpenTime:    candle.OpenTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close,
			Volume:      volume,         // contract quantity
			QuoteVolume: volume * close, // turnover (USD)
			CloseTime:   candle.CloseTime,
		}
	}

	return klines, nil
}

// handleSymbols returns available symbols for a given exchange
func (s *Server) handleSymbols(c *gin.Context) {
	exchange := c.DefaultQuery("exchange", "hyperliquid")

	type SymbolInfo struct {
		Symbol      string `json:"symbol"`
		Name        string `json:"name"`
		Category    string `json:"category"` // crypto, stock, forex, commodity, index
		MaxLeverage int    `json:"maxLeverage,omitempty"`
	}

	var symbols []SymbolInfo

	switch strings.ToLower(exchange) {
	case "hyperliquid", "hyperliquid-xyz", "xyz":
		// Fetch symbols from Hyperliquid
		client := hyperliquid.NewClient()
		ctx := context.Background()

		// Get crypto perps from default dex
		if exchange == "hyperliquid" || exchange == "hyperliquid-xyz" {
			mids, err := client.GetAllMids(ctx)
			if err == nil {
				for symbol := range mids {
					// Skip spot tokens (start with @)
					if strings.HasPrefix(symbol, "@") {
						continue
					}
					symbols = append(symbols, SymbolInfo{
						Symbol:   symbol,
						Name:     symbol,
						Category: "crypto",
					})
				}
			}
		}

		// Get xyz dex symbols (stocks, forex, commodities)
		xyzMids, err := client.GetAllMidsXYZ(ctx)
		if err == nil {
			for symbol := range xyzMids {
				// Remove xyz: prefix for display
				displaySymbol := strings.TrimPrefix(symbol, "xyz:")
				category := "stock"
				if displaySymbol == "GOLD" || displaySymbol == "SILVER" {
					category = "commodity"
				} else if displaySymbol == "EUR" || displaySymbol == "JPY" {
					category = "forex"
				} else if displaySymbol == "XYZ100" {
					category = "index"
				}
				symbols = append(symbols, SymbolInfo{
					Symbol:   displaySymbol,
					Name:     displaySymbol,
					Category: category,
				})
			}
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange for symbol listing"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exchange": exchange,
		"symbols":  symbols,
		"count":    len(symbols),
	})
}
