package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/security"
	"strconv"
	"time"
)

const (
	okxOpenInterestURL = "https://www.okx.com/api/v5/public/open-interest"
	okxFundingRateURL  = "https://www.okx.com/api/v5/public/funding-rate"
)

func getOpenInterestOKX(symbol string) (*OIData, error) {
	instID := okxInstID(symbol)
	url := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxOpenInterestURL, instID)

	resp, err := security.SafeGet(url, 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx open interest api returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseOKXOpenInterestJSON(body)
}

func parseOKXOpenInterestJSON(body []byte) (*OIData, error) {
	var payload struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OI    string `json:"oi"`
			OICcy string `json:"oiCcy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Code != "" && payload.Code != "0" {
		return nil, fmt.Errorf("okx open interest API error: code=%s msg=%s", payload.Code, payload.Msg)
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("okx open interest response is empty")
	}

	row := payload.Data[0]
	// oiCcy is base-coin amount, aligned with Binance openInterest semantics.
	oi, err := strconv.ParseFloat(row.OICcy, 64)
	if err != nil || oi <= 0 {
		oi, err = strconv.ParseFloat(row.OI, 64)
		if err != nil {
			return nil, fmt.Errorf("okx open interest parse failed: %w", err)
		}
	}

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999,
	}, nil
}

func getFundingRateOKX(symbol string) (float64, error) {
	instID := okxInstID(symbol)
	url := fmt.Sprintf("%s?instId=%s", okxFundingRateURL, instID)

	resp, err := security.SafeGet(url, 15*time.Second)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("okx funding rate api returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseOKXFundingRateJSON(body)
}

func parseOKXFundingRateJSON(body []byte) (float64, error) {
	var payload struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			FundingRate string `json:"fundingRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	if payload.Code != "" && payload.Code != "0" {
		return 0, fmt.Errorf("okx funding rate API error: code=%s msg=%s", payload.Code, payload.Msg)
	}
	if len(payload.Data) == 0 {
		return 0, fmt.Errorf("okx funding rate response is empty")
	}

	rate, err := strconv.ParseFloat(payload.Data[0].FundingRate, 64)
	if err != nil {
		return 0, fmt.Errorf("okx funding rate parse failed: %w", err)
	}
	return rate, nil
}

func fundingRateCacheKey(exchange, symbol string) string {
	return NormalizeKlineExchange(exchange) + ":" + Normalize(symbol)
}

func getOpenInterestData(symbol, exchange string) (*OIData, error) {
	exchange = NormalizeKlineExchange(exchange)
	if exchange == "okx" {
		if data, err := getOpenInterestOKX(symbol); err == nil {
			logger.Infof("OI source: OKX direct (%s)", symbol)
			return data, nil
		} else {
			logger.Infof("⚠️ OKX OI failed for %s, falling back to Binance: %v", symbol, err)
		}
	}
	return getOpenInterestBinance(symbol)
}

func getFundingRate(symbol, exchange string) (float64, error) {
	exchange = NormalizeKlineExchange(exchange)
	cacheKey := fundingRateCacheKey(exchange, symbol)

	if cached, ok := fundingRateMap.Load(cacheKey); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			return cache.Rate, nil
		}
	}

	if exchange == "okx" {
		rate, okxErr := getFundingRateOKX(symbol)
		if okxErr == nil {
			logger.Infof("Funding rate source: OKX direct (%s)", symbol)
			fundingRateMap.Store(cacheKey, &FundingRateCache{
				Rate:      rate,
				UpdatedAt: time.Now(),
			})
			return rate, nil
		}
		logger.Infof("⚠️ OKX funding rate failed for %s, falling back to Binance: %v", symbol, okxErr)
	}

	rate, err := getFundingRateBinance(symbol)
	if err != nil {
		return 0, err
	}
	fundingRateMap.Store(cacheKey, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})
	return rate, nil
}
