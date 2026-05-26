/** Extract symbol → margin PnL% from user prompt mandatory block or position lines. */
export function parseMarginPnLFromPrompt(inputPrompt: string): Map<string, number> {
  const map = new Map<string, number>()
  if (!inputPrompt) return map

  const mandatory = inputPrompt.match(
    /-\s*(\S+)\s+(LONG|SHORT):\s*Margin PnL%?\s*=\s*([+\-]?\d+(?:\.\d+)?)%/gi
  )
  if (mandatory) {
    for (const line of mandatory) {
      const m = line.match(
        /-\s*(\S+)\s+(LONG|SHORT):\s*Margin PnL%?\s*=\s*([+\-]?\d+(?:\.\d+)?)%/i
      )
      if (m) {
        map.set(m[1].toUpperCase(), parseFloat(m[3]))
      }
    }
  }

  const rows = inputPrompt.matchAll(
    /(\S+USDT)\s+(LONG|SHORT)[^\n]*Margin PnL%?\s*([+\-]?\d+(?:\.\d+)?)%/gi
  )
  for (const m of rows) {
    map.set(m[1].toUpperCase(), parseFloat(m[3]))
  }

  return map
}

function underlyingPriceChangePct(
  entry: number,
  mark: number,
  side: string
): number {
  if (entry <= 0) return 0
  const isShort = side.toUpperCase() === 'SHORT'
  if (isShort) return ((entry - mark) / entry) * 100
  return ((mark - entry) / entry) * 100
}

function symbolMentioned(text: string, symbol: string): boolean {
  const sym = symbol.toUpperCase()
  const base = sym.replace(/USDT$/i, '')
  const upper = text.toUpperCase()
  return upper.includes(sym) || (base.length > 0 && upper.includes(base))
}

/** Align CoT text with margin PnL% (same logic as backend FixReasoningPositionPnLPct). */
export function fixReasoningPnLDisplay(
  cot: string,
  marginPnLBySymbol: Map<string, number>,
  prices?: Map<string, { entry: number; mark: number; side: string }>
): string {
  if (!cot || marginPnLBySymbol.size === 0) return cot

  let out = cot

  for (const [symbol, systemPct] of marginPnLBySymbol) {
    let pricePct = 0
    if (prices?.has(symbol)) {
      const p = prices.get(symbol)!
      pricePct = underlyingPriceChangePct(p.entry, p.mark, p.side)
    }
    if (Math.abs(systemPct - pricePct) < 0.15) continue
    if (!symbolMentioned(out, symbol)) continue

    const symKey = symbol.toUpperCase()
    const base = symKey.replace(/USDT$/, '')
    let loc = out.toUpperCase().indexOf(symKey)
    if (loc < 0) loc = out.toUpperCase().indexOf(base)
    if (loc < 0) continue

    const start = Math.max(0, loc - 120)
    const end = Math.min(out.length, loc + 220)
    const window = out.slice(start, end)

    const matches: { numStart: number; numEnd: number; parsed: number }[] = []
    let m: RegExpExecArray | null
    const localRe = /([+\-−]?\d+(?:\.\d+)?)\s*%/g
    while ((m = localRe.exec(window)) !== null) {
      const raw = m[1].replace('−', '-')
      const parsed = parseFloat(raw)
      if (!Number.isNaN(parsed)) {
        matches.push({
          numStart: start + m.index + (m[0].indexOf(m[1])),
          numEnd: start + m.index + m[0].indexOf(m[1]) + m[1].length,
          parsed,
        })
      }
    }

    for (let i = matches.length - 1; i >= 0; i--) {
      const { numStart, numEnd, parsed } = matches[i]
      if (pricePct !== 0) {
        if (Math.abs(parsed - pricePct) > 0.2 || Math.abs(parsed - systemPct) <= 0.2) {
          continue
        }
        if (Math.abs(parsed - pricePct) >= Math.abs(parsed - systemPct)) {
          continue
        }
      } else if (Math.abs(parsed - systemPct) > 0.25) {
        continue
      }
      const replacement =
        systemPct >= 0
          ? `+${systemPct.toFixed(2)}`
          : systemPct.toFixed(2)
      out = out.slice(0, numStart) + replacement + out.slice(numEnd)
    }
  }

  return out
}
