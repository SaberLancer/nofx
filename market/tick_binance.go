package market

import (
	"encoding/json"
	"fmt"
	"nofx/security"
	"strconv"
	"time"
)

const binanceAggTradesURL = "https://fapi.binance.com/fapi/v1/aggTrades"

func fetchBinanceAggTradesWindow(symbol string, start, end time.Time, limit int) ([]RawTick, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	url := fmt.Sprintf("%s?symbol=%s&startTime=%d&endTime=%d&limit=%d",
		binanceAggTradesURL, Normalize(symbol), start.UnixMilli(), end.UnixMilli(), limit)

	resp, err := security.SafeGet(url, defaultTickFetchTimeout)
	if err != nil {
		return nil, fmt.Errorf("binance aggTrades request failed: %w", err)
	}
	defer resp.Body.Close()

	var rows []struct {
		Price        string `json:"p"`
		Quantity     string `json:"q"`
		Timestamp    int64  `json:"T"`
		IsBuyerMaker bool   `json:"m"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("binance aggTrades decode failed: %w", err)
	}

	out := make([]RawTick, 0, len(rows))
	for _, row := range rows {
		price, err := strconv.ParseFloat(row.Price, 64)
		if err != nil {
			continue
		}
		qty, err := strconv.ParseFloat(row.Quantity, 64)
		if err != nil {
			continue
		}
		side := TickSideBuy
		if row.IsBuyerMaker {
			side = TickSideSell
		}
		ts := time.UnixMilli(row.Timestamp)
		if ts.Before(start) || ts.After(end) {
			continue
		}
		out = append(out, RawTick{
			Symbol:    Normalize(symbol),
			Price:     price,
			Quantity:  qty,
			Side:      side,
			Timestamp: ts,
		})
	}
	return out, nil
}

func fetchBinanceAggTrades(symbol string, limit int) ([]RawTick, error) {
	url := fmt.Sprintf("%s?symbol=%s&limit=%d", binanceAggTradesURL, Normalize(symbol), limit)

	resp, err := security.SafeGet(url, defaultTickFetchTimeout)
	if err != nil {
		return nil, fmt.Errorf("binance aggTrades request failed: %w", err)
	}
	defer resp.Body.Close()

	var rows []struct {
		Price        string `json:"p"`
		Quantity     string `json:"q"`
		Timestamp    int64  `json:"T"`
		IsBuyerMaker bool   `json:"m"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("binance aggTrades decode failed: %w", err)
	}

	out := make([]RawTick, 0, len(rows))
	for _, row := range rows {
		price, err := strconv.ParseFloat(row.Price, 64)
		if err != nil {
			continue
		}
		qty, err := strconv.ParseFloat(row.Quantity, 64)
		if err != nil {
			continue
		}
		side := TickSideBuy
		if row.IsBuyerMaker {
			side = TickSideSell
		}
		out = append(out, RawTick{
			Symbol:    Normalize(symbol),
			Price:     price,
			Quantity:  qty,
			Side:      side,
			Timestamp: time.UnixMilli(row.Timestamp),
		})
	}
	return out, nil
}
