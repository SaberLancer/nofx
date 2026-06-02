package kernel

import (
	"nofx/store"
	"strings"
	"testing"
)

func TestSanitizeDecisions_RRFailureDowngradesToWait(t *testing.T) {
	protection := OpenProtectionParams{
		MinRiskRewardRatio:            3.0,
		BtcEthMinStopLossDistancePct:  2.0,
		AltcoinMinStopLossDistancePct: 2.0,
		StopLossEnabled:               true,
		TakeProfitEnabled:             true,
	}
	marketPrices := map[string]float64{
		"DOGEUSDT": 0.1006,
		"BTCUSDT":  100000,
	}
	decisions := []Decision{
		{
			Symbol:          "DOGEUSDT",
			Action:          "open_long",
			Leverage:        5,
			PositionSizeUSD: 100,
			StopLoss:        0.0986,
			TakeProfit:      0.1066, // tp/sl ~ 2.92 < 3
		},
		{
			Symbol:          "BTCUSDT",
			Action:          "open_short",
			Leverage:        10,
			PositionSizeUSD: 5000,
			StopLoss:        102000,
			TakeProfit:      94000,
		},
	}

	notes := sanitizeDecisions(decisions, 10000, 10, 7, 5, 2, protection, marketPrices)
	if len(notes) != 1 {
		t.Fatalf("expected 1 sanitize note, got %d: %v", len(notes), notes)
	}
	if !strings.Contains(notes[0], "DOGEUSDT") {
		t.Fatalf("expected DOGE in note, got %q", notes[0])
	}
	if decisions[0].Action != "wait" {
		t.Fatalf("DOGE should be wait, got %s", decisions[0].Action)
	}
	if decisions[0].StopLoss != 0 || decisions[0].TakeProfit != 0 {
		t.Fatalf("open fields should be cleared on wait")
	}
	if decisions[1].Action != "open_short" {
		t.Fatalf("BTC should remain open_short, got %s", decisions[1].Action)
	}
}

func TestParseFullDecisionResponse_PartialSanitizeNoError(t *testing.T) {
	protection := OpenProtectionFromRiskControl(store.RiskControlConfig{
		MinRiskRewardRatio:           3,
		BtcEthMinStopLossDistPct:     2,
		AltcoinMinStopLossDistPct:    2,
		BTCETHMaxLeverage:            10,
		AltcoinMaxLeverage:           7,
	})
	resp := `<reasoning>test</reasoning>
<decision>
` + "```json\n" + `[
  {"symbol":"DOGEUSDT","action":"open_long","leverage":5,"position_size_usd":100,"stop_loss":0.0986,"take_profit":0.1066,"confidence":70},
  {"symbol":"BTCUSDT","action":"wait"}
]
` + "```\n</decision>"

	fd, err := parseFullDecisionResponse(resp, 10000, 10, 7, 5, 2, protection, map[string]float64{"DOGEUSDT": 0.1006})
	if err != nil {
		t.Fatalf("parse should succeed with sanitize: %v", err)
	}
	if len(fd.ValidationNotes) != 1 {
		t.Fatalf("expected validation note, got %v", fd.ValidationNotes)
	}
	if fd.Decisions[0].Action != "wait" {
		t.Fatalf("expected DOGE wait, got %s", fd.Decisions[0].Action)
	}
}
