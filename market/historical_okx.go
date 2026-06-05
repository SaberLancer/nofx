package market

import (
	"fmt"
	"strconv"
	"time"
)

const okxHistoryCandlesURL = "https://www.okx.com/api/v5/market/history-candles"

// GetKlinesRangeOKX fetches swap candle history from OKX (demo when simulated=true).
func GetKlinesRangeOKX(symbol string, timeframe string, start, end time.Time, simulated bool) ([]Kline, error) {
	symbol = Normalize(symbol)
	if _, err := NormalizeTimeframe(timeframe); err != nil {
		return nil, err
	}
	bar, err := mapTimeframeToOKXBar(timeframe)
	if err != nil {
		return nil, err
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	barMs := timeframeDurationMs(timeframe)

	seen := make(map[int64]Kline)
	var after int64

	for page := 0; page < 50; page++ {
		url := fmt.Sprintf("%s?instId=%s&bar=%s&limit=300", okxHistoryCandlesURL, okxInstID(symbol), bar)
		if after > 0 {
			url += "&after=" + strconv.FormatInt(after, 10)
		}

		body, err := okxPublicGet(url, simulated, 20*time.Second)
		if err != nil {
			return nil, err
		}
		batch, err := parseOKXCandlesJSON(body)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		oldest := batch[0].OpenTime
		for _, k := range batch {
			if k.OpenTime < startMs-barMs*5 {
				continue
			}
			if k.OpenTime > endMs+barMs {
				continue
			}
			seen[k.OpenTime] = k
			if k.OpenTime < oldest {
				oldest = k.OpenTime
			}
		}

		if oldest <= startMs-barMs*30 {
			break
		}
		after = oldest
		if len(batch) < 300 {
			break
		}
	}

	if len(seen) == 0 {
		return nil, fmt.Errorf("no okx klines for %s %s in range", symbol, timeframe)
	}

	out := make([]Kline, 0, len(seen))
	for _, k := range seen {
		if k.OpenTime >= startMs && k.OpenTime <= endMs+barMs {
			out = append(out, k)
		}
	}
	sortKlinesByOpenTime(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no okx klines for %s %s after range filter", symbol, timeframe)
	}
	return out, nil
}

func timeframeDurationMs(tf string) int64 {
	d, err := TFDuration(tf)
	if err != nil {
		return 900_000
	}
	return d.Milliseconds()
}

func sortKlinesByOpenTime(klines []Kline) {
	for i := 1; i < len(klines); i++ {
		key := klines[i]
		j := i - 1
		for j >= 0 && klines[j].OpenTime > key.OpenTime {
			klines[j+1] = klines[j]
			j--
		}
		klines[j+1] = key
	}
}
