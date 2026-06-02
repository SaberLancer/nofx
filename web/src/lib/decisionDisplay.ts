import type { DecisionAction, DecisionRecord } from '../types'

const UNAVAILABLE_ACTION = 'unavailable'

/** Build cycle display rows: all configured candidates first, then any extra decisions. */
export function buildCycleDecisionItems(decision: DecisionRecord): DecisionAction[] {
  const candidates = decision.candidate_coins ?? []
  const decisions = decision.decisions ?? []

  if (
    decision.pre_decision_skipped &&
    decisions.length === 0 &&
    candidates.length > 0
  ) {
    const gateReason = inferPreDecisionReason(decision.execution_log)
    return candidates.map((sym) => ({
      symbol: sym,
      action: 'wait',
      quantity: 0,
      leverage: 0,
      price: 0,
      order_id: 0,
      timestamp: decision.timestamp,
      success: true,
      reasoning: gateReason,
    }))
  }

  if (candidates.length === 0) {
    return decisions
  }

  const bySymbol = new Map<string, DecisionAction>()
  for (const action of decisions) {
    const existing = bySymbol.get(action.symbol)
    if (!existing || (existing.action === UNAVAILABLE_ACTION && action.action !== UNAVAILABLE_ACTION)) {
      bySymbol.set(action.symbol, action)
    }
  }

  const items: DecisionAction[] = []
  for (const sym of candidates) {
    const existing = bySymbol.get(sym)
    if (existing) {
      items.push(existing)
      bySymbol.delete(sym)
      continue
    }
    items.push({
      symbol: sym,
      action: UNAVAILABLE_ACTION,
      quantity: 0,
      leverage: 0,
      price: 0,
      order_id: 0,
      timestamp: decision.timestamp,
      success: false,
      error: inferUnavailableReason(decision.execution_log, sym),
    })
  }

  for (const action of bySymbol.values()) {
    items.push(action)
  }

  return items
}

function inferUnavailableReason(log: string[] | undefined, symbol: string): string {
  if (!log?.length) {
    return ''
  }
  const prefix = `⚠️ ${symbol} unavailable: `
  for (const line of log) {
    if (line.startsWith(prefix)) {
      return line.slice(prefix.length)
    }
  }
  return ''
}

function inferPreDecisionReason(log: string[] | undefined): string {
  if (!log?.length) {
    return ''
  }
  for (const line of log) {
    if (line.startsWith('pre-decision:')) {
      return line
    }
  }
  return ''
}

export function isUnavailableAction(action: DecisionAction): boolean {
  return action.action === UNAVAILABLE_ACTION
}
