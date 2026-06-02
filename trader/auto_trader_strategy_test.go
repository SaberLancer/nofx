package trader

import (
	"nofx/store"
	"testing"
	"time"
)

func TestReloadStrategyConfigIfNeeded(t *testing.T) {
	dbPath := t.TempDir() + "/strategy_hot_reload.db"
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	userID := "user-hot-reload"
	strategy := &store.Strategy{
		ID:          "strategy-hot-reload",
		UserID:      userID,
		Name:        "Hot Reload Test",
		Description: "test",
		Config:      `{"language":"zh","risk_control":{"lock_profit_pnl_pct":4}}`,
	}
	if err := st.Strategy().Create(strategy); err != nil {
		t.Fatalf("Strategy().Create: %v", err)
	}

	loaded, err := st.Strategy().Get(userID, strategy.ID)
	if err != nil {
		t.Fatalf("Strategy().Get: %v", err)
	}

	cfg, err := loaded.ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	at := &AutoTrader{
		id:                "trader-hot-reload",
		userID:            userID,
		store:             st,
		strategyID:        strategy.ID,
		strategyName:      loaded.Name,
		strategyUpdatedAt: loaded.UpdatedAt,
		config: AutoTraderConfig{
			StrategyConfig:    cfg,
			StrategyID:        strategy.ID,
			StrategyName:      loaded.Name,
			StrategyUpdatedAt: loaded.UpdatedAt,
		},
	}

	// Unchanged strategy should not rebuild engine.
	at.strategyEngine = nil
	at.reloadStrategyConfigIfNeeded()
	if at.strategyEngine != nil {
		t.Fatal("expected no reload when strategy unchanged")
	}

	// Update strategy config in DB.
	time.Sleep(10 * time.Millisecond)
	updatedCfg := *cfg
	updatedCfg.RiskControl.LockProfitPnLPct = 8
	if err := loaded.SetConfig(&updatedCfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	loaded.Name = "Hot Reload Test v2"
	if err := st.Strategy().Update(loaded); err != nil {
		t.Fatalf("Strategy().Update: %v", err)
	}

	reloaded, err := st.Strategy().Get(userID, strategy.ID)
	if err != nil {
		t.Fatalf("Strategy().Get after update: %v", err)
	}
	if !reloaded.UpdatedAt.After(at.strategyUpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance after update")
	}

	at.reloadStrategyConfigIfNeeded()
	if at.strategyEngine == nil {
		t.Fatal("expected strategy engine to be rebuilt after update")
	}
	if got := at.GetStrategyConfig().RiskControl.LockProfitPnLPct; got != 8 {
		t.Fatalf("expected lock_profit_pnl_pct=8, got %v", got)
	}
	if at.strategyName != "Hot Reload Test v2" {
		t.Fatalf("expected strategy name update, got %q", at.strategyName)
	}
}
