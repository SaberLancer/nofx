import { describe, expect, it } from 'vitest'
import { calculateADX, calculateADXAtBar, type Kline } from './indicators'

function makeChoppyKlines(n: number, base: number): Kline[] {
  const out: Kline[] = []
  for (let i = 0; i < n; i++) {
    const wave = (i % 4) * 0.15
    const price = base + wave
    out.push({
      time: i,
      open: price,
      high: price + 0.2,
      low: price - 0.2,
      close: price + 0.05,
    })
  }
  return out
}

describe('calculateADX', () => {
  it('returns empty when insufficient bars', () => {
    expect(calculateADX([], 14)).toEqual([])
    expect(calculateADX(makeChoppyKlines(20, 100), 14)).toEqual([])
  })

  it('returns low ADX on choppy range series', () => {
    const series = calculateADX(makeChoppyKlines(60, 100), 14)
    expect(series.length).toBeGreaterThan(0)
    const last = series[series.length - 1]
    expect(last.adx).toBeGreaterThan(0)
    expect(last.adx).toBeLessThan(25)
  })

  it('calculateADXAtBar uses trailing window', () => {
    const klines = makeChoppyKlines(80, 100)
    const full = calculateADX(klines, 14)
    const windowed = calculateADXAtBar(klines, klines.length - 1, 14, 60)
    expect(windowed).not.toBeNull()
    expect(full.length).toBeGreaterThan(0)
    // 60-bar window vs full series may differ; both should be positive
    expect(windowed!.adx).toBeGreaterThan(0)
  })
})
