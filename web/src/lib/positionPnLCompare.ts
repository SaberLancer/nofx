import { parseMarginPnLFromPrompt } from './fixReasoningPnL'

export interface LivePositionPnL {
  symbol: string
  unrealized_pnl_pct: number
}

/** Symbols mentioned in decision actions or input prompt. */
export function symbolsInDecision(
  decision: { decisions?: { symbol: string }[]; input_prompt?: string }
): string[] {
  const set = new Set<string>()
  for (const a of decision.decisions ?? []) {
    if (a.symbol) set.add(a.symbol.toUpperCase())
  }
  const prompt = decision.input_prompt ?? ''
  const re = /(BTC|ETH|SOL|DOGE|BNB|XRP|ADA|AVAX|LINK|DOT|MATIC|LTC|BCH|ETC|TRX|OP|ARB|APT|SUI|SEI|TON|NEAR|ATOM|FIL|UNI|AAVE|MKR|CRV|PEPE|SHIB|WIF|BONK)[A-Z]*USDT/gi
  let m: RegExpExecArray | null
  while ((m = re.exec(prompt)) !== null) {
    const sym = m[0].toUpperCase()
    if (!sym.endsWith('USDT')) continue
    set.add(sym)
  }
  return Array.from(set)
}

export function decisionTimePnLMap(inputPrompt: string): Map<string, number> {
  return parseMarginPnLFromPrompt(inputPrompt)
}

export function livePnLMap(positions: LivePositionPnL[] | undefined): Map<string, number> {
  const m = new Map<string, number>()
  for (const p of positions ?? []) {
    if (p.symbol) m.set(p.symbol.toUpperCase(), p.unrealized_pnl_pct)
  }
  return m
}

export function pnlDiffSignificant(a: number, b: number, threshold = 0.08): boolean {
  return Math.abs(a - b) >= threshold
}
