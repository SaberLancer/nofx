// 技术指标计算工具

export interface Kline {
  time: number
  open: number
  high: number
  low: number
  close: number
  volume?: number
}

// 简单移动平均线 (SMA)
export function calculateSMA(data: Kline[], period: number): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []

  for (let i = period - 1; i < data.length; i++) {
    let sum = 0
    for (let j = 0; j < period; j++) {
      sum += data[i - j].close
    }
    result.push({
      time: data[i].time,
      value: sum / period,
    })
  }

  return result
}

// 指数移动平均线 (EMA)
export function calculateEMA(data: Kline[], period: number): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []
  const multiplier = 2 / (period + 1)

  // 第一个EMA值使用SMA
  let ema = 0
  for (let i = 0; i < period; i++) {
    ema += data[i].close
  }
  ema = ema / period
  result.push({ time: data[period - 1].time, value: ema })

  // 后续EMA值
  for (let i = period; i < data.length; i++) {
    ema = (data[i].close - ema) * multiplier + ema
    result.push({ time: data[i].time, value: ema })
  }

  return result
}

// MACD 指标
export interface MACDData {
  time: number
  macd: number
  signal: number
  histogram: number
}

export function calculateMACD(
  data: Kline[],
  fastPeriod = 12,
  slowPeriod = 26,
  signalPeriod = 9
): MACDData[] {
  const fastEMA = calculateEMA(data, fastPeriod)
  const slowEMA = calculateEMA(data, slowPeriod)

  // 计算MACD线
  const macdLine: Array<{ time: number; value: number }> = []
  for (let i = 0; i < slowEMA.length; i++) {
    const fastValue = fastEMA.find(e => e.time === slowEMA[i].time)
    if (fastValue) {
      macdLine.push({
        time: slowEMA[i].time,
        value: fastValue.value - slowEMA[i].value,
      })
    }
  }

  // 计算信号线（MACD的EMA）
  const signalLine = calculateEMAFromValues(macdLine, signalPeriod)

  // 生成MACD数据
  const result: MACDData[] = []
  for (let i = 0; i < signalLine.length; i++) {
    const macdValue = macdLine.find(m => m.time === signalLine[i].time)
    if (macdValue) {
      result.push({
        time: signalLine[i].time,
        macd: macdValue.value,
        signal: signalLine[i].value,
        histogram: macdValue.value - signalLine[i].value,
      })
    }
  }

  return result
}

// 从值数组计算EMA（辅助函数）
function calculateEMAFromValues(
  data: Array<{ time: number; value: number }>,
  period: number
): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []
  const multiplier = 2 / (period + 1)

  if (data.length < period) return []

  // 第一个EMA值使用SMA
  let ema = 0
  for (let i = 0; i < period; i++) {
    ema += data[i].value
  }
  ema = ema / period
  result.push({ time: data[period - 1].time, value: ema })

  // 后续EMA值
  for (let i = period; i < data.length; i++) {
    ema = (data[i].value - ema) * multiplier + ema
    result.push({ time: data[i].time, value: ema })
  }

  return result
}

// RSI 指标
export function calculateRSI(data: Kline[], period = 14): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []

  if (data.length < period + 1) return []

  // 计算价格变化
  const changes: number[] = []
  for (let i = 1; i < data.length; i++) {
    changes.push(data[i].close - data[i - 1].close)
  }

  // 计算初始平均涨跌幅
  let avgGain = 0
  let avgLoss = 0
  for (let i = 0; i < period; i++) {
    if (changes[i] > 0) {
      avgGain += changes[i]
    } else {
      avgLoss += Math.abs(changes[i])
    }
  }
  avgGain = avgGain / period
  avgLoss = avgLoss / period

  // 计算RSI
  for (let i = period; i < changes.length; i++) {
    const currentChange = changes[i]

    if (currentChange > 0) {
      avgGain = (avgGain * (period - 1) + currentChange) / period
      avgLoss = (avgLoss * (period - 1)) / period
    } else {
      avgGain = (avgGain * (period - 1)) / period
      avgLoss = (avgLoss * (period - 1) + Math.abs(currentChange)) / period
    }

    const rs = avgGain / avgLoss
    const rsi = 100 - 100 / (1 + rs)

    result.push({
      time: data[i + 1].time,
      value: rsi,
    })
  }

  return result
}

// 布林带
export interface BollingerBands {
  time: number
  upper: number
  middle: number
  lower: number
}

export interface ADXPoint {
  time: number
  adx: number
  plusDI: number
  minusDI: number
}

/** Wilder ADX(period) time series — matches backend market.CalculateADX smoothing. */
export function calculateADX(data: Kline[], period = 14): ADXPoint[] {
  if (period <= 0) period = 14
  const n = data.length
  if (n < period * 2) return []

  const tr = new Array<number>(n).fill(0)
  const pdm = new Array<number>(n).fill(0)
  const mdm = new Array<number>(n).fill(0)

  for (let i = 1; i < n; i++) {
    const up = data[i].high - data[i - 1].high
    const down = data[i - 1].low - data[i].low
    if (up > down && up > 0) pdm[i] = up
    if (down > up && down > 0) mdm[i] = down
    const h = data[i].high
    const l = data[i].low
    const pc = data[i - 1].close
    tr[i] = Math.max(h - l, Math.max(Math.abs(h - pc), Math.abs(l - pc)))
  }

  let sumTR = 0
  let sumPDM = 0
  let sumMDM = 0
  for (let i = 1; i <= period; i++) {
    sumTR += tr[i]
    sumPDM += pdm[i]
    sumMDM += mdm[i]
  }

  let atr = sumTR
  let sp = sumPDM
  let sm = sumMDM

  const dxBuf: number[] = []
  const diBuf: Array<{ plus: number; minus: number }> = []

  for (let i = period; i < n; i++) {
    if (i > period) {
      atr = atr - atr / period + tr[i]
      sp = sp - sp / period + pdm[i]
      sm = sm - sm / period + mdm[i]
    }
    let plusDI = 0
    let minusDI = 0
    let dx = 0
    if (atr !== 0) {
      plusDI = (100 * sp) / atr
      minusDI = (100 * sm) / atr
      const denom = plusDI + minusDI
      if (denom > 0) {
        dx = (100 * Math.abs(plusDI - minusDI)) / denom
      }
    }
    dxBuf.push(dx)
    diBuf.push({ plus: plusDI, minus: minusDI })
  }

  if (dxBuf.length < period) return []

  const result: ADXPoint[] = []
  let adx = 0
  for (let i = 0; i < period; i++) {
    adx += dxBuf[i]
  }
  adx /= period

  for (let i = period - 1; i < dxBuf.length; i++) {
    if (i > period - 1) {
      adx = (adx * (period - 1) + dxBuf[i]) / period
    }
    const klineIdx = period + i
    result.push({
      time: data[klineIdx].time,
      adx,
      plusDI: diBuf[i].plus,
      minusDI: diBuf[i].minus,
    })
  }

  return result
}

/** ADX at bar endIdx using a trailing window (matches backend regime detection kline_count). */
export function calculateADXAtBar(
  klines: Kline[],
  endIdx: number,
  period = 14,
  klineCount = 60
): ADXPoint | null {
  if (endIdx < 0 || endIdx >= klines.length) return null
  const start = Math.max(0, endIdx - klineCount + 1)
  const window = klines.slice(start, endIdx + 1)
  const pts = calculateADX(window, period)
  if (pts.length === 0) return null
  const last = pts[pts.length - 1]
  return { ...last, time: klines[endIdx].time }
}

/** Rolling regime-style ADX series (each point uses trailing klineCount bars). */
export function calculateRollingADXSeries(
  klines: Kline[],
  period = 14,
  klineCount = 60
): ADXPoint[] {
  const result: ADXPoint[] = []
  for (let i = 0; i < klines.length; i++) {
    const pt = calculateADXAtBar(klines, i, period, klineCount)
    if (pt) result.push(pt)
  }
  return result
}

export function calculateBollingerBands(
  data: Kline[],
  period = 20,
  stdDev = 2
): BollingerBands[] {
  const result: BollingerBands[] = []

  for (let i = period - 1; i < data.length; i++) {
    // 计算SMA
    let sum = 0
    for (let j = 0; j < period; j++) {
      sum += data[i - j].close
    }
    const sma = sum / period

    // 计算标准差
    let variance = 0
    for (let j = 0; j < period; j++) {
      variance += Math.pow(data[i - j].close - sma, 2)
    }
    const std = Math.sqrt(variance / period)

    result.push({
      time: data[i].time,
      upper: sma + stdDev * std,
      middle: sma,
      lower: sma - stdDev * std,
    })
  }

  return result
}
