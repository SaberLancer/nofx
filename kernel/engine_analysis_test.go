package kernel

import "testing"

func TestValidateJSONFormatAllowsCommaInStringValues(t *testing.T) {
	jsonContent := `[{"symbol": "BTCUSDT", "action": "wait", "confidence": 85, "reason": "Market in tight consolidation 76,685-76,830. RSI 43.61."}]`
	if err := validateJSONFormat(jsonContent); err != nil {
		t.Fatalf("expected valid JSON with comma-formatted prices in reason string, got: %v", err)
	}
}

func TestValidateJSONFormatRejectsThousandSeparatorInNumbers(t *testing.T) {
	jsonContent := `[{"symbol": "BTCUSDT", "action": "wait", "confidence": 85, "stop_loss": 76,685}]`
	if err := validateJSONFormat(jsonContent); err == nil {
		t.Fatal("expected thousand separator in numeric field to be rejected")
	}
}

func TestExtractDecisionsWithCommaInReason(t *testing.T) {
	response := `<reasoning>analysis</reasoning>
<decision>
` + "```json\n" + `[{"symbol": "BTCUSDT", "action": "wait", "confidence": 85, "reason": "Range 76,685-76,830"}]` + "\n```\n</decision>"

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "BTCUSDT" || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decisions: %+v", decisions)
	}
	if decisions[0].Reasoning == "" {
		t.Fatal("expected reasoning from reason alias")
	}
}
