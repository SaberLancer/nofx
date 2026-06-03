import { useCallback, useEffect, useRef, useState } from 'react'
import type { Position } from '../types'

export type PositionPnlHistoryEntry = {
  recorded_at: number
  unrealized_pnl_pct: number
  unrealized_pnl: number
  mark_price: number
}

const MAX_ENTRIES_PER_POSITION = 500
const MIN_PCT_CHANGE = 0.01
const STORAGE_PREFIX = 'nofx-pos-pnl-hist:'

export function buildPositionPnlKey(pos: Pick<Position, 'symbol' | 'side' | 'open_time'>) {
  const side = pos.side?.toLowerCase() || 'unknown'
  const openTime = pos.open_time ?? 0
  return `${pos.symbol}:${side}:${openTime}`
}

function loadFromStorage(traderId: string): Record<string, PositionPnlHistoryEntry[]> {
  if (!traderId || typeof sessionStorage === 'undefined') return {}
  try {
    const raw = sessionStorage.getItem(STORAGE_PREFIX + traderId)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Record<string, PositionPnlHistoryEntry[]>
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function saveToStorage(traderId: string, data: Record<string, PositionPnlHistoryEntry[]>) {
  if (!traderId || typeof sessionStorage === 'undefined') return
  try {
    sessionStorage.setItem(STORAGE_PREFIX + traderId, JSON.stringify(data))
  } catch {
    // quota or private mode — ignore
  }
}

function shouldAppend(
  last: PositionPnlHistoryEntry | undefined,
  pct: number,
  now: number
): boolean {
  if (!last) return true
  if (Math.abs(pct - last.unrealized_pnl_pct) >= MIN_PCT_CHANGE) return true
  // 收益率未变时，每 60s 记一条心跳，便于对齐刷新节奏
  if (now - last.recorded_at >= 60_000) return true
  return false
}

function trimEntries(entries: PositionPnlHistoryEntry[]): PositionPnlHistoryEntry[] {
  if (entries.length <= MAX_ENTRIES_PER_POSITION) return entries
  return entries.slice(entries.length - MAX_ENTRIES_PER_POSITION)
}

/**
 * 随仪表盘持仓轮询累积 Margin PnL% 历史（按 symbol+side+open_time 区分）。
 */
export function usePositionPnlHistory(
  traderId: string | undefined,
  positions: Position[] | undefined
) {
  const [historyByKey, setHistoryByKey] = useState<Record<string, PositionPnlHistoryEntry[]>>(
    () => (traderId ? loadFromStorage(traderId) : {})
  )
  const historyRef = useRef(historyByKey)
  historyRef.current = historyByKey

  useEffect(() => {
    if (!traderId) {
      setHistoryByKey({})
      return
    }
    const stored = loadFromStorage(traderId)
    setHistoryByKey(stored)
    historyRef.current = stored
  }, [traderId])

  useEffect(() => {
    if (!traderId || !positions?.length) return

    const now = Date.now()
    let changed = false
    const next = { ...historyRef.current }

    for (const pos of positions) {
      if (!pos.symbol || !pos.quantity) continue
      const key = buildPositionPnlKey(pos)
      const pct = pos.unrealized_pnl_pct ?? 0
      const list = next[key] ? [...next[key]] : []
      const last = list[list.length - 1]

      if (!shouldAppend(last, pct, now)) continue

      list.push({
        recorded_at: now,
        unrealized_pnl_pct: pct,
        unrealized_pnl: pos.unrealized_pnl ?? 0,
        mark_price: pos.mark_price ?? 0,
      })
      next[key] = trimEntries(list)
      changed = true
    }

    if (!changed) return
    historyRef.current = next
    setHistoryByKey(next)
    saveToStorage(traderId, next)
  }, [traderId, positions])

  const appendSnapshot = useCallback(
    (pos: Position, force = false) => {
      if (!traderId || !pos.symbol || !pos.quantity) return
      const now = Date.now()
      const key = buildPositionPnlKey(pos)
      const pct = pos.unrealized_pnl_pct ?? 0
      const next = { ...historyRef.current }
      const list = next[key] ? [...next[key]] : []
      const last = list[list.length - 1]

      if (!force && !shouldAppend(last, pct, now)) return

      if (
        !force &&
        last &&
        last.unrealized_pnl_pct === pct &&
        last.unrealized_pnl === (pos.unrealized_pnl ?? 0) &&
        now - last.recorded_at < 1000
      ) {
        return
      }

      list.push({
        recorded_at: now,
        unrealized_pnl_pct: pct,
        unrealized_pnl: pos.unrealized_pnl ?? 0,
        mark_price: pos.mark_price ?? 0,
      })
      next[key] = trimEntries(list)
      historyRef.current = next
      setHistoryByKey(next)
      saveToStorage(traderId, next)
    },
    [traderId]
  )

  const getHistory = useCallback(
    (pos: Pick<Position, 'symbol' | 'side' | 'open_time'>) => {
      const key = buildPositionPnlKey(pos)
      const list = historyByKey[key] ?? []
      return [...list].sort((a, b) => b.recorded_at - a.recorded_at)
    },
    [historyByKey]
  )

  return { getHistory, recordSnapshot: appendSnapshot }
}
