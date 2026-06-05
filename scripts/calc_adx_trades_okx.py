#!/usr/bin/env python3
"""ADX(14) for ds-flash losing trades using OKX klines (simulated=demo)."""
import json
import urllib.request
from datetime import datetime, timezone, timedelta

BJ = timezone(timedelta(hours=8))
OKX_HISTORY = "https://www.okx.com/api/v5/market/history-candles"
OKX_CANDLES = "https://www.okx.com/api/v5/market/candles"

TRADES = [
    (1, "SOL-USDT-SWAP", "SOL", "多", "2026-06-04 23:15:00", "2026-06-04 23:22:30"),
    (2, "SOL-USDT-SWAP", "SOL", "多", "2026-06-05 00:44:00", "2026-06-05 01:32:30"),
    (3, "SOL-USDT-SWAP", "SOL", "空", "2026-06-05 01:41:00", "2026-06-05 01:52:30"),
    (4, "SOL-USDT-SWAP", "SOL", "多", "2026-06-05 01:58:00", "2026-06-05 04:07:30"),
    (5, "ETH-USDT-SWAP", "ETH", "多", "2026-06-05 02:33:00", "2026-06-05 05:41:10"),
    (6, "SOL-USDT-SWAP", "SOL", "空", "2026-06-05 06:27:00", "2026-06-05 07:04:31"),
    (7, "ETH-USDT-SWAP", "ETH", "空", "2026-06-05 06:32:00", "2026-06-05 07:04:32"),
]


def to_ms(s: str) -> int:
    return int(datetime.strptime(s, "%Y-%m-%d %H:%M:%S").replace(tzinfo=BJ).timestamp() * 1000)


def bj(ts: int) -> str:
    return datetime.fromtimestamp(ts / 1000, BJ).strftime("%m-%d %H:%M")


def okx_get(url: str, simulated: bool) -> dict:
    req = urllib.request.Request(
        url,
        headers={
            "User-Agent": "nofx-adx-script/1.0",
            "x-simulated-trading": "1" if simulated else "0",
        },
    )
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


def tf_bar(tf: str) -> str:
    return {"15m": "15m", "1h": "1H", "3m": "3m"}.get(tf, tf)


def fetch_okx_range(inst: str, bar: str, start_ms: int, end_ms: int, simulated: bool) -> list:
    """Paginate OKX history-candles using `after` (older pages), same as OKX docs."""
    seen = {}
    bar_ms = {"3m": 180_000, "15m": 900_000, "1H": 3_600_000, "1h": 3_600_000}.get(bar, 900_000)
    after = None

    for _ in range(20):
        url = f"{OKX_HISTORY}?instId={inst}&bar={bar}&limit=300"
        if after is not None:
            url += f"&after={after}"
        payload = okx_get(url, simulated)
        if payload.get("code") != "0":
            raise RuntimeError(f"OKX {payload.get('code')} {payload.get('msg')}")
        rows = payload.get("data") or []
        if not rows:
            break
        for row in rows:
            ts = int(row[0])
            seen[ts] = {
                "ts": ts,
                "o": float(row[1]),
                "h": float(row[2]),
                "l": float(row[3]),
                "c": float(row[4]),
            }
        oldest = int(rows[-1][0])
        if oldest <= start_ms - bar_ms * 30:
            break
        after = oldest
        if len(rows) < 300:
            break

    out = sorted(seen.values(), key=lambda x: x["ts"])
    return [k for k in out if start_ms - bar_ms * 30 <= k["ts"] <= end_ms + bar_ms]


def calc_adx_series(klines: list, period: int = 14) -> list:
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
        return sum(vals[start : start + p])

    series = []
    atr = wilder_seed(tr, period, 1)
    sp = wilder_seed(pdm, period, 1)
    sm = wilder_seed(mdm, period, 1)
    dx_buf = []
    adx = 0.0

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


def feat(adx):
    if adx is None:
        return "N/A"
    if adx < 20:
        return "震荡"
    if adx < 25:
        return "弱趋势"
    if adx < 40:
        return "中等趋势"
    return "强趋势"


def print_table(title: str, trades, cache, bar: str):
    print(f"\n=== {title} | {bar} ===")
    print("| # | 品种 | 方向 | 开仓ADX | 平仓ADX | 持仓均ADX | +DI | -DI | 特征 |")
    print("|---|------|------|---------|---------|-----------|-----|-----|------|")
    for num, inst, sym, side, ot, ct in trades:
        series = calc_adx_series(cache[(inst, bar)], 14)
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
            f"| {num} | {sym} | {side} | {f(ao)} | {f(ac)} | {f(avg)} | {f(ap)} | {f(am)} | {feat(ao)} |"
        )


def close_at(series: list, ts_ms: int):
    val = None
    for k in series:
        if k["ts"] <= ts_ms:
            val = k["c"]
        else:
            break
    return val


def compare_sim_vs_live(cache_sim, cache_live, inst: str, ts_ms: int):
    sc = close_at(cache_sim.get((inst, "15m"), []), ts_ms)
    lc = close_at(cache_live.get((inst, "15m"), []), ts_ms)
    if sc is None or lc is None:
        return None, None
    return sc, lc


def main():
    warm_end = to_ms("2026-06-05 09:00:00")
    warm_15m = to_ms("2026-06-04 10:00:00")
    warm_1h = to_ms("2026-06-01 00:00:00")

    cache_sim = {}
    cache_live = {}
    for inst in ["SOL-USDT-SWAP", "ETH-USDT-SWAP"]:
        for bar in ["15m", "1h"]:
            b = tf_bar(bar)
            cache_sim[(inst, bar)] = fetch_okx_range(inst, b, warm_15m if bar == "15m" else warm_1h, warm_end, True)
            cache_live[(inst, bar)] = fetch_okx_range(inst, b, warm_15m if bar == "15m" else warm_1h, warm_end, False)
            print(
                f"{inst} {bar} sim={len(cache_sim[(inst, bar)])} live={len(cache_live[(inst, bar)])} "
                f"({bj(cache_sim[(inst, bar)][0]['ts']) if cache_sim[(inst, bar)] else '?'}"
                f" ~ {bj(cache_sim[(inst, bar)][-1]['ts']) if cache_sim[(inst, bar)] else '?'})"
            )

    print("\n--- OKX 模拟 vs 实盘 K 线收盘价抽样（15m，开仓时刻）---")
    print("| # | 品种 | 模拟收盘 | 实盘收盘 | 价差 |")
    print("|---|------|----------|----------|------|")
    for num, inst, sym, side, ot, _ in TRADES:
        ts = to_ms(ot)
        sc, lc = compare_sim_vs_live(cache_sim, cache_live, inst, ts)
        if sc is None:
            diff = "N/A"
        else:
            diff = f"{sc - lc:+.4f}"
            sc, lc = f"{sc:.4f}", f"{lc:.4f}"
        print(f"| {num} | {sym} | {sc} | {lc} | {diff} |")

    print("\nADX(14) Wilder | 数据源: OKX 公开 K 线 + x-simulated-trading:1（与 NOFX 模拟盘一致）")
    print("说明: 此前 Binance 分析作废；以下以模拟环境 K 线为准。")
    print_table("OKX 模拟盘", TRADES, cache_sim, "15m")
    print_table("OKX 模拟盘", TRADES, cache_sim, "1h")

    # Trade 4 trajectory on sim 15m
    print("\n=== #4 SOL多 | OKX模拟 15m ADX 轨迹 ===")
    series = calc_adx_series(cache_sim[("SOL-USDT-SWAP", "15m")], 14)
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
