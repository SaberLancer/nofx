#!/usr/bin/env python3
import sys
sys.path.insert(0, r"e:\OKXAgent\nofx\scripts")
from calc_adx_trades_okx import *

inst = "SOL-USDT-SWAP"
kl15 = fetch_okx_range(inst, "15m", to_ms("2026-06-04 12:00:00"), to_ms("2026-06-05 09:00:00"), True)
kl1h = fetch_okx_range(inst, "1H", to_ms("2026-05-28 00:00:00"), to_ms("2026-06-05 09:00:00"), True)
s15 = calc_adx_series(kl15, 14)
s1h = calc_adx_series(kl1h, 14)

print("OKX simulated SOL — ADX vs 肉眼横盘区间\n")

opens = [
    ("#1 多 开", "2026-06-04 23:15:00"),
    ("#2 多 开", "2026-06-05 00:44:00"),
    ("#3 空 开", "2026-06-05 01:41:00"),
    ("#4 多 开", "2026-06-05 01:58:00"),
    ("#6 空 开", "2026-06-05 06:27:00"),
]

print("15m @开仓时刻:")
print("| 标记 | 时间 | ADX | +DI | -DI | 近14根振幅% | 解读 |")
print("|------|------|-----|-----|-----|------------|------|")
for label, tstr in opens:
    ts = to_ms(tstr)
    adx, p, m = lookup(s15, ts)
    window = [k for k in kl15 if ts - 14 * 900000 <= k["ts"] <= ts]
    if window:
        hi = max(k["h"] for k in window)
        lo = min(k["l"] for k in window)
        mid = window[-1]["c"]
        amp = (hi - lo) / mid * 100 if mid else 0
    else:
        amp = 0
    if adx is None:
        note = "N/A"
    elif adx < 20:
        note = "震荡(与图一致)"
    elif adx < 25:
        note = "边界/偏弱"
    else:
        note = "数值偏高(滞后)"
    print(f"| {label} | {tstr[5:16]} | {adx:.1f} | {p:.1f} | {m:.1f} | {amp:.2f}% | {note} |")

print("\n1h @开仓时刻:")
print("| 标记 | 时间 | ADX | +DI | -DI | 近14根振幅% | 解读 |")
print("|------|------|-----|-----|-----|------------|------|")
for label, tstr in opens:
    ts = to_ms(tstr)
    adx, p, m = lookup(s1h, ts)
    window = [k for k in kl1h if ts - 14 * 3600000 <= k["ts"] <= ts]
    if window:
        hi = max(k["h"] for k in window)
        lo = min(k["l"] for k in window)
        mid = window[-1]["c"]
        amp = (hi - lo) / mid * 100 if mid else 0
    else:
        amp = 0
    if adx is None:
        note = "N/A"
    elif adx < 25:
        note = "震荡(与图一致)"
    elif adx < 40 and amp < 3:
        note = "ADX滞后(当前已横盘)"
    else:
        note = "ADX仍高(前几根大波动)"
    print(f"| {label} | {tstr[5:16]} | {adx:.1f} | {p:.1f} | {m:.1f} | {amp:.2f}% | {note} |")

print("\n15m ADX 衰减（横盘段 23:15 后每30分钟）:")
ts0 = to_ms("2026-06-04 23:15:00")
for extra in range(0, 481, 30):
    ts = ts0 + extra * 60 * 1000
    adx, p, m = lookup(s15, ts)
    if adx:
        print(f"  {bj(ts)}  ADX={adx:.1f}  +DI={p:.1f}  -DI={m:.1f}")
