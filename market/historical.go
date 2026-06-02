package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	binanceFuturesKlinesURL = "https://fapi.binance.com/fapi/v1/klines"
	binanceMaxKlineLimit    = 1500
)

// GetKlinesRange fetches K-line series within specified time range (closed interval), returns data sorted by time in ascending order.
func GetKlinesRange(symbol string, timeframe string, start, end time.Time) ([]Kline, error) {
	symbol = Normalize(symbol)
	normTF, err := NormalizeTimeframe(timeframe)
	if err != nil {
		return nil, err
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	var all []Kline
	cursor := startMs

	client := &http.Client{Timeout: 15 * time.Second}

	for cursor < endMs {
		req, err := http.NewRequest("GET", binanceFuturesKlinesURL, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Set("symbol", symbol)
		q.Set("interval", normTF)
		q.Set("limit", fmt.Sprintf("%d", binanceMaxKlineLimit))
		q.Set("startTime", fmt.Sprintf("%d", cursor))
		q.Set("endTime", fmt.Sprintf("%d", endMs))
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("binance klines api returned status %d: %s", resp.StatusCode, string(body))
		}

		batch, err := parseBinanceKlinesJSON(body)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)

		last := batch[len(batch)-1]
		cursor = last.CloseTime + 1

		// If returned quantity is less than request limit, reached the end, can exit early.
		if len(batch) < binanceMaxKlineLimit {
			break
		}
	}

	return all, nil
}

// GetKlinesRecent fetches the latest N klines from Binance USDT-M futures.
// Used as a live-trading fallback when CoinAnk is rate-limited or unavailable.
func GetKlinesRecent(symbol string, timeframe string, limit int) ([]Kline, error) {
	symbol = Normalize(symbol)
	normTF, err := NormalizeTimeframe(timeframe)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > binanceMaxKlineLimit {
		limit = binanceMaxKlineLimit
	}

	req, err := http.NewRequest("GET", binanceFuturesKlinesURL, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("symbol", symbol)
	q.Set("interval", normTF)
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance klines api returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseBinanceKlinesJSON(body)
}

func parseBinanceKlinesJSON(body []byte) ([]Kline, error) {
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("binance klines response is empty")
	}

	klines := make([]Kline, len(raw))
	for i, item := range raw {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}
	return klines, nil
}
