package kernel

import (
	"testing"

	"nofx/market"
)

func TestRestrictCandidatesToPositions(t *testing.T) {
	ctx := &Context{
		Positions: []PositionInfo{
			{Symbol: "ETHUSDT"},
		},
		CandidateCoins: []CandidateCoin{
			{Symbol: "BTCUSDT"},
			{Symbol: "ETHUSDT"},
			{Symbol: "SOLUSDT"},
		},
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {Symbol: "BTCUSDT"},
			"ETHUSDT": {Symbol: "ETHUSDT"},
		},
	}
	RestrictCandidatesToPositions(ctx)
	if len(ctx.CandidateCoins) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(ctx.CandidateCoins))
	}
	if ctx.CandidateCoins[0].Symbol != "ETHUSDT" {
		t.Fatalf("expected ETHUSDT, got %s", ctx.CandidateCoins[0].Symbol)
	}
	if _, ok := ctx.MarketDataMap["BTCUSDT"]; ok {
		t.Fatal("BTC market data should be removed")
	}
}
