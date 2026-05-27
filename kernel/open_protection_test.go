package kernel

import (
	"testing"
)

func TestValidateOpenProtection_SOLShort(t *testing.T) {
	p := OpenProtectionParams{
		MinRiskRewardRatio:            3.0,
		BtcEthMinStopLossDistancePct:  0.5,
		AltcoinMinStopLossDistancePct: 0.8,
	}
	entry := 84.1032

	// Too tight SL (like user's screenshot) should fail
	err := ValidateOpenProtection("open_short", "SOLUSDT", entry, 84.60, 82.50, p)
	if err == nil {
		t.Fatal("expected error for SL distance < 0.8%")
	}

	// Valid wider SL (+0.8% SL, ~2.5% TP → RR > 3)
	err = ValidateOpenProtection("open_short", "SOLUSDT", entry, 84.78, 82.00, p)
	if err != nil {
		t.Fatalf("expected valid protection, got %v", err)
	}
}

func TestValidateOpenProtection_RR(t *testing.T) {
	p := OpenProtectionParams{
		MinRiskRewardRatio:            3.0,
		AltcoinMinStopLossDistancePct: 0.8,
	}
	entry := 100.0
	// SL 0.8%, TP only 1% → RR 1.25 < 3
	err := ValidateOpenProtection("open_long", "SOLUSDT", entry, 99.2, 101.0, p)
	if err == nil {
		t.Fatal("expected R:R validation error")
	}
}
