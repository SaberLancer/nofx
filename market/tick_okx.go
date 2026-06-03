package market

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const okxTradesURL = "https://www.okx.com/api/v5/market/trades"

func fetchOKXRecentTicks(symbol string, limit int, simulated bool) ([]RawTick, error) {
	instID := okxInstID(symbol)
	url := fmt.Sprintf("%s?instId=%s&limit=%d", okxTradesURL, instID, limit)

	body, err := okxPublicGet(url, simulated, defaultTickFetchTimeout)
	if err != nil {
		return nil, fmt.Errorf("okx trades request failed: %w", err)
	}

	var payload struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Px   string `json:"px"`
			Sz   string `json:"sz"`
			Side string `json:"side"`
			Ts   string `json:"ts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("okx trades decode failed: %w", err)
	}
	if payload.Code != "" && payload.Code != "0" {
		return nil, fmt.Errorf("okx trades API error: code=%s msg=%s", payload.Code, payload.Msg)
	}

	out := make([]RawTick, 0, len(payload.Data))
	for _, row := range payload.Data {
		price, err := strconv.ParseFloat(row.Px, 64)
		if err != nil {
			continue
		}
		qty, err := strconv.ParseFloat(row.Sz, 64)
		if err != nil {
			continue
		}
		tsMs, err := strconv.ParseInt(row.Ts, 10, 64)
		if err != nil {
			continue
		}
		side := TickSideSell
		if strings.EqualFold(row.Side, "buy") {
			side = TickSideBuy
		}
		out = append(out, RawTick{
			Symbol:    Normalize(symbol),
			Price:     price,
			Quantity:  qty,
			Side:      side,
			Timestamp: time.UnixMilli(tsMs),
		})
	}
	return out, nil
}
