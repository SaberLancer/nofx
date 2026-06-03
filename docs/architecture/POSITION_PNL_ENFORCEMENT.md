# Position PnL Enforcement (Code-Executed)

**Language:** [English](POSITION_PNL_ENFORCEMENT.md) | [中文](POSITION_PNL_ENFORCEMENT.zh-CN.md)

This document describes how the **live auto-trader** (non-grid strategies) automatically reduces or closes positions based on **margin-based unrealized PnL %**. It reflects the current codebase.

**Implementation:** `trader/auto_trader_pnl_enforce.go`, `trader/auto_trader_loop.go` (`buildTradingContext`)

---

## When checks run

Checks run **immediately after** a fresh position snapshot is built, while the trader is **running**:

1. **Primary:** each AI scan cycle (on start, then every configured **scan interval**, often ~3 minutes).
2. **Secondary:** `GET /api/account` and internal account snapshot calls (dashboard often polls ~every 5 seconds).

**Not triggered:** positions-only API, stopped trader, grid-trading mode.

**Debounce:** identical snapshot within **800ms** is skipped.

There is **no** dedicated one-minute background monitor.

---

## Data sources

| Data | Source |
|------|--------|
| Quantity, mark price, unrealized PnL, leverage | Exchange REST (`GetPositions`) |
| Balance / equity | Exchange `GetBalance()` |
| Margin PnL % | `(unrealized PnL / (qty × mark / leverage)) × 100` — estimated margin, not exchange `margin` field |
| Peak PnL % | In-memory cache + DB `trader_positions`, updated each sample |
| Thresholds | Strategy risk control in DB; defaults in `store/risk_control_pnl.go` |

Exchange liquidation price is **display-only** for this feature; live trading does not simulate exchange liquidation.

---

## Enforced rules (priority order)

| Priority | Rule | Default trigger | Action |
|----------|------|-----------------|--------|
| 1 | Stop loss | Margin PnL % ≤ **-5%** | Full close |
| 2 | Peak pullback | Peak ≥ **10%**, drop from peak ≥ **4** pp | Full close |
| 3 | Lock tier 2 | PnL % ≥ **12%**, tier 2 not applied | **40%** reduce (hardcoded) |
| 4 | Lock tier 1 | PnL % ≥ **8%**, tier 1 not applied | **30%** reduce (configurable ratio) |

**Prompt-only (not code-enforced):** “protect gains on reversal” style rules; exchange liquidation.

---

## Execution

`close_long` / `close_short` via `executeDecisionWithRecord`; quantity from live exchange positions; reason tag `[CODE ENFORCED PnL]`.

---

## Configuration example (placeholders)

```json
{
  "risk_control": {
    "stop_loss_pnl_pct": -5,
    "lock_profit_pnl_pct": 8,
    "lock_profit_reduce_ratio": 0.3,
    "lock_profit_second_pnl_pct": 12,
    "peak_min_for_pullback": 10,
    "peak_pullback_pts": 4
  }
}
```

Use **your API Key**, **your Secret Key**, **your Passphrase** in the UI — never commit real credentials to docs or repos.

---

[← Architecture index](README.md) · [Strategy module](STRATEGY_MODULE.md)
