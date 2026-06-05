#!/usr/bin/env python3
"""Compute ADX(14) for ds-flash losing trades window. Uses Binance futures klines."""
import json
import urllib.request
from datetime import datetime, timezone, timedelta

BJ = timezone(timedelta(hours=8))


def to_ms(s: str) -> int:
    return int(datetime.strptime(s, "%Y-%m-%d %H:%M:%S").replace(tzinfo=BJ).timestamp() * 1000)


def bj(ts: int) -> str:
    return datetime.fromtimestamp(ts / 1000, BJ).strftime("%m-%d %H:%M")


def fetch_binance(symbol: str, interval: str, start_ms: int, end_ms: int) -> list:
    klines = []
    cur = start_ms
    while cur < end_ms:
        url = (
            f"https://fapi.binance.com/fapi/v1/klines?symbol={symbol}"
            f"&interval={interval}&startTime={cur}&endTime={end_ms}&limit=1500"
        )
        with urllib.request.urlopen(url, timeout=30) as r:
            batch = json.loads(r.read())
        if not batch:
            break
        for row in batch:
            klines.append(
                {
                    "ts": int(row[0]),
                    "o": float(row[1]),
                    "h": float(row[2]),
                    "l": float(row[3]),
                    "c": float(row[4]),
                }
            )
        cur = int(batch[-1][0]) + 1
        if len(batch) < 1500:
            break
    return klines


def calc_adx_series(klines: list, period: int = 14) -> list:
    """Return (bar_open_ts, adx, +di, -di) for each bar with valid ADX."""
    n = len(klines)
    if n < period * 2:
        return []

    tr = [0.0] * n
    pdm = [0.0] * n
    mdm = [0.0] * n
    for i in range(1, n):
        h, l, pc = klines[i]["h"], klines[i]["l"], klines[i - 1]["c"]
        up = klines[i]["h"] - klines[i - 1]["h"]
        down = klines[i - 1]["l"] - klines[i]["l"]
        pdm[i] = up if up > down and up > 0 else 0
        mdm[i] = down if down > up and down > 0 else 0
        tr[i] = max(h - l, abs(h - pc), abs(l - pc))

    def wilder_seed(vals, p, start):
        s = sum(vals[start : start + p])
        return s

    series = []
    atr = wilder_seed(tr, period, 1)
    sp = wilder_seed(pdm, period, 1)
    sm = wilder_seed(mdm, period, 1)
    dx_buf = []

    for i in range(period, n):
        if i > period:
            atr = atr - atr / period + tr[i]
            sp = sp - sp / period + pdm[i]
            sm = sm - sm / period + mdm[i]
        if atr == 0:
            pdi = mdi = dx = 0.0
        else:
            pdi = 100 * sp / atr
            mdi = 100 * sm / atr
            denom = pdi + mdi
            dx = 100 * abs(pdi - mdi) / denom if denom else 0.0
        dx_buf.append(dx)
        if len(dx_buf) < period:
            continue
        if len(dx_buf) == period:
            adx = sum(dx_buf) / period
        else:
            adx = (adx * (period - 1) + dx) / period
        series.append((klines[i]["ts"], adx, pdi, mdi))
    return series


def lookup(series, ts):
    val = pdi = mdi = None
    for t, v, p, m in series:
        if t <= ts:
            val, pdi, mdi = v, p, m
        else:
            break
    return val, pdi, mdi


def market_feat(adx):
    if adx is None:
        return "N/A"
    if adx < 20:
        return "震荡(ADX<20)"
    if adx < 25:
        return "弱趋势(20-25)"
    if adx < 40:
        return "中等趋势(25-40)"
    return "强趋势(>=40)"


TRADES = [
    (1, "SOLUSDT", "SOL", "多", "2026-06-04 23:15:00", "2026-06-04 23:22:30"),
    (2, "SOLUSDT", "SOL", "多", "2026-06-05 00:44:00", "2026-06-05 01:32:30"),
    (3, "SOLUSDT", "SOL", "空", "2026-06-05 01:41:00", "2026-06-05 01:52:30"),
    (4, "SOLUSDT", "SOL", "多", "2026-06-05 01:58:00", "2026-06-05 04:07:30"),
    (5, "ETHUSDT", "ETH", "多", "2026-06-05 02:33:00", "2026-06-05 05:41:10"),
    (6, "SOLUSDT", "SOL", "空", "2026-06-05 06:27:00", "2026-06-05 07:04:31"),
    (7, "ETHUSDT", "ETH", "空", "2026-06-05 06:32:00", "2026-06-05 07:04:32"),
]


def main():
    # 15m: 1 day back; 1h: 5 days back for ADX(14) warm-up
    warm_end = to_ms("2026-06-05 09:00:00")
    warm_15m = to_ms("2026-06-04 12:00:00")
    warm_1h = to_ms("2026-06-01 00:00:00")
    cache = {}
    for sym in ["SOLUSDT", "ETHUSDT"]:
        cache[(sym, "15m")] = fetch_binance(sym, "15m", warm_15m, warm_end)
        cache[(sym, "1h")] = fetch_binance(sym, "1h", warm_1h, warm_end)
        for iv in ["15m", "1h"]:
            kl = cache[(sym, iv)]
            print(f"{sym} {iv}: {len(kl)} bars, {bj(kl[0]['ts'])} ~ {bj(kl[-1]['ts'])}")

    print()
    print("ADX(14) Wilder | 数据源: Binance USDT永续 (本环境 OKX 公开 API 403)")
    print("与策略 ETHUSDT/SOLUSDT 同源; 主周期 15m, 辅周期 1h")
    print()

    for bar in ["15m", "1h"]:
        print(f"=== {bar} ===")
        print("| # | 品种 | 方向 | 开仓ADX | 平仓ADX | 持仓均ADX | +DI@开 | -DI@开 | 特征 |")
        print("|---|------|------|---------|---------|-----------|--------|--------|------|")
        for num, symbol, sym, side, ot, ct in TRADES:
            series = calc_adx_series(cache[(symbol, bar)], 14)
            oms, cms = to_ms(ot), to_ms(ct)
            ao, ap, am = lookup(series, oms)
            ac, _, _ = lookup(series, cms)
            vals = [v for t, v, _, _ in series if oms <= t <= cms]
            if not vals and ao is not None and ac is not None:
                vals = [ao, ac]
            avg = sum(vals) / len(vals) if vals else None

            def f(x):
                return f"{x:.1f}" if x is not None else "N/A"

            print(
                f"| {num} | {sym} | {side} | {f(ao)} | {f(ac)} | {f(avg)} | {f(ap)} | {f(am)} | {market_feat(ao)} |"
            )
        print()

    print("=== #4 SOL多 15m ADX 持仓轨迹 ===")
    series = calc_adx_series(cache[("SOLUSDT", "15m")], 14)
    oms, cms = to_ms("2026-06-05 01:58:00"), to_ms("2026-06-05 04:07:30")
    pts = [(t, v) for t, v, _, _ in series if oms <= t <= cms]
    step = max(1, len(pts) // 10)
    for i in range(0, len(pts), step):
        t, v = pts[i]
        print(f"  {bj(t)}  ADX={v:.1f}")
    if pts:
        print(f"  {bj(pts[-1][0])}  ADX={pts[-1][1]:.1f} (平仓附近)")


if __name__ == "__main__":
    main()
