package trader

import (
	"nofx/store"
	"testing"
)

func TestEvaluatePositionPnLAction_StopLoss(t *testing.T) {
	rc := store.RiskControlConfig{}
	plan, ok := evaluatePositionPnLAction(-6, 5, 0, rc)
	if !ok || plan.Tier != pnlEnforceTierStop || plan.CloseRatio != 0 {
		t.Fatalf("expected stop loss full close, got %+v ok=%v", plan, ok)
	}
}

func TestEvaluatePositionPnLAction_PeakPullback(t *testing.T) {
	rc := store.RiskControlConfig{}
	plan, ok := evaluatePositionPnLAction(6, 12, 0, rc)
	if !ok || plan.Tier != pnlEnforceTierPeakPull {
		t.Fatalf("expected peak pullback, got %+v ok=%v", plan, ok)
	}
}

func TestEvaluatePositionPnLAction_LockTiers(t *testing.T) {
	rc := store.RiskControlConfig{}

	plan, ok := evaluatePositionPnLAction(9, 9, 0, rc)
	if !ok || plan.Tier != pnlEnforceTierLock1 || plan.CloseRatio != 0.3 {
		t.Fatalf("expected lock tier1, got %+v ok=%v", plan, ok)
	}

	plan, ok = evaluatePositionPnLAction(13, 13, 1, rc)
	if !ok || plan.Tier != pnlEnforceTierLock2 || plan.CloseRatio != pnlEnforceLock2CloseRatio {
		t.Fatalf("expected lock tier2, got %+v ok=%v", plan, ok)
	}

	_, ok = evaluatePositionPnLAction(13, 13, 2, rc)
	if ok {
		t.Fatal("expected no action when tier2 already applied")
	}
}

func TestEvaluatePositionPnLAction_StopBeforeLock(t *testing.T) {
	rc := store.RiskControlConfig{}
	plan, ok := evaluatePositionPnLAction(-6, 15, 0, rc)
	if !ok || plan.Tier != pnlEnforceTierStop {
		t.Fatalf("stop loss should win over peak lock, got %+v", plan)
	}
}
