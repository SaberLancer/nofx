#!/usr/bin/env python3
import json
import sqlite3
from pathlib import Path

STRATEGY_ID = "fa85fe6d-d7b9-4542-8f54-89997d6e3b2d"
TRADER_ID = "001112e4_cbb38523-c1a8-4fb5-baa3-9598bb0d85a2_deepseek_1779584497"
DB = Path(__file__).resolve().parents[1] / "data" / "data.db"
OUT = Path(__file__).resolve().parents[1] / "scripts" / "export_custom_jiji"

def main():
    conn = sqlite3.connect(DB)
    row = conn.execute(
        "SELECT name, description, config FROM strategies WHERE id=?",
        (STRATEGY_ID,),
    ).fetchone()
    cfg = json.loads(row[2])
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "config.json").write_text(
        json.dumps(cfg, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    sp = conn.execute(
        "SELECT system_prompt FROM decision_records WHERE trader_id=? ORDER BY timestamp DESC LIMIT 1",
        (TRADER_ID,),
    ).fetchone()
    if sp and sp[0]:
        (OUT / "system_prompt_latest.txt").write_text(sp[0], encoding="utf-8")
    summary = {
        "strategy_id": STRATEGY_ID,
        "name": row[0],
        "description": row[1],
        "trader": "逐仓积极",
        "trader_id": TRADER_ID,
    }
    (OUT / "summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    conn.close()
    print(f"exported to {OUT}")

if __name__ == "__main__":
    main()
