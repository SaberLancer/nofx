import type { BacktestTradeEvent, DecisionRecord } from '../types'

export function actionHighlightKey(symbol: string, action: string): string {
  return `${symbol}:${action}`
}

/** Resolve the decision action string for a backtest trade row. */
export function tradeActionForBacktest(trade: BacktestTradeEvent): string {
  if (trade.liquidation || trade.close_reason === 'liquidation') {
    return trade.side === 'short' ? 'close_short' : 'close_long'
  }
  return trade.action
}

export function isAutomaticClose(trade: BacktestTradeEvent): boolean {
  const reason = trade.close_reason
  return (
    trade.liquidation ||
    reason === 'stop_loss' ||
    reason === 'take_profit' ||
    reason === 'liquidation'
  )
}

export function findMatchingActionIndex(
  decision: DecisionRecord,
  symbol: string,
  action: string
): number {
  if (!decision.decisions?.length) return -1
  return decision.decisions.findIndex(
    (d) => d.symbol === symbol && d.action === action
  )
}

export function closeActionForSide(side: string): string {
  return side.toUpperCase() === 'LONG' ? 'close_long' : 'close_short'
}

export function openActionForSide(side: string): string {
  return side.toUpperCase() === 'LONG' ? 'open_long' : 'open_short'
}

/** Find the decision cycle whose timestamp is closest to an event time. */
export function findDecisionForOperation(
  decisions: DecisionRecord[],
  symbol: string,
  action: string,
  eventTimeMs: number,
  maxSkewMs = 60 * 60 * 1000
): DecisionRecord | null {
  let best: DecisionRecord | null = null
  let bestDelta = Infinity

  for (const record of decisions) {
    const hasAction = record.decisions?.some(
      (d) => d.symbol === symbol && d.action === action
    )
    if (!hasAction) continue

    const ts = new Date(record.timestamp).getTime()
    if (Number.isNaN(ts)) continue

    const delta = Math.abs(ts - eventTimeMs)
    if (delta < bestDelta && delta <= maxSkewMs) {
      bestDelta = delta
      best = record
    }
  }

  return best
}
