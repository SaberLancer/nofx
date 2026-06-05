import type { Exchange } from '../types'

export interface BacktestKlineSource {
  kline_exchange: string
  kline_simulated: boolean
  label: string
}

/** Match backend hydrateBacktestKlineConfig: first enabled OKX account, else Binance live. */
export function resolveBacktestKlineSource(
  exchanges: Exchange[] | undefined,
  language: string
): BacktestKlineSource {
  const list = exchanges ?? []
  const okx = list.find((e) => e.enabled && e.exchange_type?.toLowerCase() === 'okx')
  if (okx) {
    const sim = !!okx.testnet
    const label =
      language === 'zh'
        ? sim
          ? 'OKX 模拟盘 K 线'
          : 'OKX 实盘 K 线'
        : sim
          ? 'OKX demo klines'
          : 'OKX live klines'
    return { kline_exchange: 'okx', kline_simulated: sim, label }
  }
  const label = language === 'zh' ? 'Binance 实盘 K 线' : 'Binance live klines'
  return { kline_exchange: 'binance', kline_simulated: false, label }
}
