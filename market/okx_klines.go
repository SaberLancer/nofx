package market

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const okxCandlesURL = "https://www.okx.com/api/v5/market/candles"

// GetKlinesRecentOKX fetches the latest N swap candles from OKX public market API.
// When simulated is true, requests demo-trading klines via x-simulated-trading: 1.
func GetKlinesRecentOKX(symbol string, timeframe string, limit int, simulated bool) ([]Kline, error) {
	symbol = Normalize(symbol)
	if _, err := NormalizeTimeframe(timeframe); err != nil {
		return nil, err
	}
	bar, err := mapTimeframeToOKXBar(timeframe)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 300 {
		limit = 300
	}

	instID := okxInstID(symbol)
	url := fmt.Sprintf("%s?instId=%s&bar=%s&limit=%d", okxCandlesURL, instID, bar, limit)

	body, err := okxPublicGet(url, simulated, 15*time.Second)
	if err != nil {
		return nil, err
	}

	return parseOKXCandlesJSON(body)
}

func mapTimeframeToOKXBar(timeframe string) (string, error) {
	switch timeframe {
	case "1m":
		return "1m", nil
	case "3m":
		return "3m", nil
	case "5m":
		return "5m", nil
	case "15m":
		return "15m", nil
	case "30m":
		return "30m", nil
	case "1h":
		return "1H", nil
	case "2h":
		return "2H", nil
	case "4h":
		return "4H", nil
	case "6h":
		return "6H", nil
	case "12h":
		return "12H", nil
	case "1d":
		return "1D", nil
	default:
		return "", fmt.Errorf("unsupported OKX bar interval: %s", timeframe)
	}
}

func parseOKXCandlesJSON(body []byte) ([]Kline, error) {
	var payload struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Code != "" && payload.Code != "0" {
		return nil, fmt.Errorf("okx candles API error: code=%s msg=%s", payload.Code, payload.Msg)
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("okx candles response is empty")
	}

	// OKX returns newest first; convert to ascending time order.
	rows := payload.Data
	klines := make([]Kline, len(rows))
	for i, row := range rows {
		if len(row) < 6 {
			return nil, fmt.Errorf("okx candles row too short")
		}
		openTime, err := parseInt64String(row[0])
		if err != nil {
			return nil, err
		}
		open, err := parseFloatString(row[1])
		if err != nil {
			return nil, err
		}
		high, err := parseFloatString(row[2])
		if err != nil {
			return nil, err
		}
		low, err := parseFloatString(row[3])
		if err != nil {
			return nil, err
		}
		close, err := parseFloatString(row[4])
		if err != nil {
			return nil, err
		}
		volume, err := parseFloatString(row[5])
		if err != nil {
			return nil, err
		}

		idx := len(rows) - 1 - i
		klines[idx] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: openTime,
		}
	}
	return klines, nil
}

func parseFloatString(v string) (float64, error) {
	return parseFloat(v)
}

func parseInt64String(v string) (int64, error) {
	return strconv.ParseInt(v, 10, 64)
}

// NormalizeKlineExchange maps trader exchange types to kline source identifiers.
func NormalizeKlineExchange(exchange string) string {
	switch strings.ToLower(strings.TrimSpace(exchange)) {
	case "okx", "okex":
		return "okx"
	case "binance":
		return "binance"
	case "bybit":
		return "bybit"
	case "bitget":
		return "bitget"
	case "gate":
		return "gate"
	case "hyperliquid":
		return "hyperliquid"
	case "aster":
		return "aster"
	default:
		return "binance"
	}
}
