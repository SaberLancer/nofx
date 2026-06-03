import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { createPortal } from 'react-dom'
import useSWR from 'swr'
import { Brain } from 'lucide-react'
import { api } from '../../lib/api'
import {
  closeActionForSide,
  findDecisionForOperation,
  isLikelySystemClose,
  openActionForSide,
} from '../../lib/decisionTradeMatch'
import { useLanguage } from '../../contexts/LanguageContext'
import { t, type Language } from '../../i18n/translations'
import { MetricTooltip } from '../common/MetricTooltip'
import { formatPrice, formatQuantity } from '../../utils/format'
import { NofxSelect } from '../ui/select'
import { DecisionDetailModal } from './DecisionDetailModal'
import { calcCloseROIPct, calcOperationCloseROIPct, getDisplayPnL } from './utils'
import type {
  DecisionRecord,
  HistoricalPosition,
  PositionCloseOperation,
  TraderStats,
  SymbolStats,
  DirectionStats,
} from '../../types'

interface PositionHistoryProps {
  traderId: string
  /** When open positions decrease, history refetches immediately (position closed). */
  openPositionCount?: number
  /** Fingerprint of open positions (symbol/side/qty) — refresh history after close/reduce. */
  openPositionsKey?: string
}

/** SWR cache key — use with globalMutate after manual close. */
export function positionHistorySWRKey(traderId: string) {
  return `position-history-${traderId}`
}

type TimeRangePreset = 'all' | '7d' | '30d' | '90d' | 'custom'

function positionExitTimeMs(position: HistoricalPosition): number {
  const v = position.exit_time
  if (v === undefined || v === null || v === '') return 0
  if (typeof v === 'number') return v
  const ms = Date.parse(String(v))
  return Number.isNaN(ms) ? 0 : ms
}

function dateInputToStartMs(dateStr: string): number {
  if (!dateStr) return NaN
  return new Date(`${dateStr}T00:00:00`).getTime()
}

function dateInputToEndMs(dateStr: string): number {
  if (!dateStr) return NaN
  return new Date(`${dateStr}T23:59:59.999`).getTime()
}

function resolveTimeRangeMs(
  preset: TimeRangePreset,
  dateFrom: string,
  dateTo: string
): { fromMs: number; toMs: number } | null {
  const now = Date.now()
  switch (preset) {
    case '7d':
      return { fromMs: now - 7 * 24 * 60 * 60 * 1000, toMs: now }
    case '30d':
      return { fromMs: now - 30 * 24 * 60 * 60 * 1000, toMs: now }
    case '90d':
      return { fromMs: now - 90 * 24 * 60 * 60 * 1000, toMs: now }
    case 'custom': {
      const fromMs = dateInputToStartMs(dateFrom)
      const toMs = dateInputToEndMs(dateTo)
      if (!Number.isFinite(fromMs) && !Number.isFinite(toMs)) return null
      return {
        fromMs: Number.isFinite(fromMs) ? fromMs : 0,
        toMs: Number.isFinite(toMs) ? toMs : now,
      }
    }
    default:
      return null
  }
}

// Format number with proper decimals (for large numbers)
function formatNumber(value: number, decimals: number = 2): string {
  if (Math.abs(value) >= 1000000) {
    return (value / 1000000).toFixed(2) + 'M'
  }
  if (Math.abs(value) >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  }
  return value.toFixed(decimals)
}

// Format duration from minutes
function formatDuration(minutes: number): string {
  if (!minutes || minutes <= 0) return '-'
  if (minutes < 60) return `${minutes.toFixed(0)}m`
  if (minutes < 1440) return `${(minutes / 60).toFixed(1)}h`
  return `${(minutes / 1440).toFixed(1)}d`
}

import { formatBeijingDateTime } from '../../utils/format'

// Format date (API may return RFC3339 string or Unix ms number) — always Beijing time
function formatDate(dateVal: string | number | undefined): string {
  return formatBeijingDateTime(dateVal) ?? '-'
}

// Stats Card Component with formula tooltip
function StatCard({
  title,
  value,
  suffix,
  color,
  icon,
  subtitle,
  metricKey,
  language = 'en',
}: {
  title: string
  value: string | number
  suffix?: string
  color?: string
  icon: string
  subtitle?: string
  metricKey?: string
  language?: string
}) {
  return (
    <div
      className="rounded-lg p-4 transition-all duration-200 hover:scale-[1.02]"
      style={{
        background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        border: '1px solid #2B3139',
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
      }}
    >
      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">{icon}</span>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {title}
        </span>
        {metricKey && (
          <MetricTooltip metricKey={metricKey} language={language} size={12} />
        )}
      </div>
      <div className="flex items-baseline gap-1">
        <span
          className="text-xl font-bold font-mono"
          style={{ color: color || '#EAECEF' }}
        >
          {value}
        </span>
        {suffix && (
          <span className="text-sm" style={{ color: '#848E9C' }}>
            {suffix}
          </span>
        )}
      </div>
      {subtitle && (
        <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
          {subtitle}
        </div>
      )}
    </div>
  )
}

// Symbol Stats Row
function SymbolStatsRow({ stat }: { stat: SymbolStats }) {
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'
  const winRateColor =
    winRate >= 60 ? '#0ECB81' : winRate >= 40 ? '#F0B90B' : '#F6465D'

  return (
    <div
      className="flex items-center justify-between p-3 rounded-lg transition-all duration-200 hover:bg-white/5"
      style={{ borderBottom: '1px solid #2B3139' }}
    >
      <div className="flex items-center gap-3">
        <span className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
          {(stat.symbol || '').replace('USDT', '')}
        </span>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {stat.total_trades || 0} trades
        </span>
      </div>
      <div className="flex items-center gap-6">
        <div className="text-right">
          <div className="text-xs" style={{ color: '#848E9C' }}>
            Win Rate
          </div>
          <div className="font-mono font-semibold" style={{ color: winRateColor }}>
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div className="text-right min-w-[80px]">
          <div className="text-xs" style={{ color: '#848E9C' }}>
            P&L
          </div>
          <div className="font-mono font-semibold" style={{ color: pnlColor }}>
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
      </div>
    </div>
  )
}

// Direction Stats Card
function DirectionStatsCard({ stat, language }: { stat: DirectionStats; language: Language }) {
  const isLong = (stat.side || '').toLowerCase() === 'long'
  const iconColor = isLong ? '#0ECB81' : '#F6465D'
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const tradeCount = stat.trade_count || 0
  const avgPnl = stat.avg_pnl || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'

  return (
    <div
      className="rounded-lg p-4"
      style={{
        background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        border: `1px solid ${iconColor}33`,
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xl">{isLong ? '📈' : '📉'}</span>
        <span
          className="font-bold uppercase"
          style={{ color: iconColor }}
        >
          {stat.side || 'Unknown'}
        </span>
      </div>
      <div className="grid grid-cols-4 gap-4">
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.trades', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
            {tradeCount}
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.winRate', language)}
          </div>
          <div
            className="font-mono font-semibold"
            style={{
              color:
                winRate >= 60
                  ? '#0ECB81'
                  : winRate >= 40
                    ? '#F0B90B'
                    : '#F6465D',
            }}
          >
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.totalPnL', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: pnlColor }}>
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.avgPnL', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: avgPnl >= 0 ? '#0ECB81' : '#F6465D' }}>
            {avgPnl >= 0 ? '+' : ''}
            {formatNumber(avgPnl)}
          </div>
        </div>
      </div>
    </div>
  )
}

type PositionDecisionKind = 'open' | 'close'

function mergeCloseOperations(operations: PositionCloseOperation[]): PositionCloseOperation[] {
  if (!operations || operations.length === 0) return []

  const toMs = (v: string | number | undefined): number => {
    if (v === undefined || v === null || v === '') return Number.POSITIVE_INFINITY
    if (typeof v === 'number') return v
    const ms = Date.parse(v)
    return Number.isNaN(ms) ? Number.POSITIVE_INFINITY : ms
  }

  const toSec = (v: string | number | undefined): number => {
    const ms = toMs(v)
    if (!Number.isFinite(ms)) return Number.MAX_SAFE_INTEGER
    return Math.floor(ms / 1000)
  }

  const groups = new Map<string, PositionCloseOperation[]>()
  for (const op of operations) {
    const ex = String(op.exchange_order_id || '').trim()
    const action = String(op.order_action || '').trim()
    const posSide = String(op.position_side || '').trim()
    const status = String(op.status || '').trim()

    // Prefer exchange order id grouping (OKX ordId) so same-second orders stay separate.
    const key = ex !== ''
      ? `ord:${ex}|action:${action}|posSide:${posSide}`
      : (() => {
          const timeSec = toSec(op.filled_at || op.created_at)
          return `sec:${timeSec}|action:${action}|posSide:${posSide}|status:${status}|exType:none`
        })()

    const arr = groups.get(key)
    if (arr) arr.push(op)
    else groups.set(key, [op])
  }

  const merged: PositionCloseOperation[] = []
  for (const [, ops] of groups) {
    if (ops.length === 0) continue

    let sumExecQty = 0
    let sumFilledQty = 0
    let sumQty = 0
    let sumFee = 0
    let sumPnl = 0
    let weightedPriceSum = 0
    let weightedAvgFillSum = 0

    let earliestCreatedAt = ops[0].created_at
    let latestFilledAt = ops[0].filled_at

    let status = ops[0].status
    if (ops.some((o) => String(o.status || '').toUpperCase() === 'FILLED')) {
      status = 'FILLED'
    }

    for (const op of ops) {
      const execQty = Number(op.exec_quantity || 0)
      const filledQty = Number(op.filled_quantity || 0)
      const qty = Number(op.quantity || 0)

      sumExecQty += execQty
      sumFilledQty += filledQty
      sumQty += qty
      sumFee += Number(op.fee || 0)
      sumPnl += Number(op.realized_pnl || 0)

      const execPrice = Number(op.exec_price || 0)
      const avgFillPrice = Number(op.avg_fill_price || 0)
      weightedPriceSum += execPrice * execQty
      weightedAvgFillSum += avgFillPrice * execQty

      if (toMs(op.created_at) < toMs(earliestCreatedAt)) earliestCreatedAt = op.created_at
      if (toMs(op.filled_at) > toMs(latestFilledAt)) latestFilledAt = op.filled_at
    }

    const avgExecPrice = sumExecQty > 0 ? weightedPriceSum / sumExecQty : Number(ops[0].exec_price || 0)
    const avgAvgFillPrice =
      sumExecQty > 0 ? weightedAvgFillSum / sumExecQty : Number(ops[0].avg_fill_price || 0)

    const first = ops[0]
    merged.push({
      ...first,
      // Keep the row identity stable for React keys.
      id: first.id,
      status,
      quantity: sumQty,
      filled_quantity: sumFilledQty,
      exec_quantity: sumExecQty,
      price: avgExecPrice,
      exec_price: avgExecPrice,
      avg_fill_price: avgAvgFillPrice,
      fee: sumFee,
      realized_pnl: sumPnl,
      created_at: earliestCreatedAt,
      filled_at: latestFilledAt,
    })
  }

  merged.sort(
    (a, b) =>
      toMs(b.filled_at || b.created_at) - toMs(a.filled_at || a.created_at)
  )
  return merged
}

// Position Row Component
function PositionRow({
  position,
  language,
  onViewDecision,
  onViewCloseDetails,
}: {
  position: HistoricalPosition
  language: Language
  onViewDecision: (position: HistoricalPosition, kind: PositionDecisionKind) => void
  onViewCloseDetails: (position: HistoricalPosition) => void
}) {
  const side = position.side || ''
  const isLong = side.toUpperCase() === 'LONG'
  const displayPnl = getDisplayPnL(position)
  const isProfitable = displayPnl >= 0
  const sideColor = isLong ? '#0ECB81' : '#F6465D'
  const pnlColor = isProfitable ? '#0ECB81' : '#F6465D'

  // Calculate holding time
  const entryTime = position.entry_time ? new Date(position.entry_time).getTime() : 0
  const exitTime = position.exit_time ? new Date(position.exit_time).getTime() : 0
  const holdingMinutes = entryTime && exitTime && exitTime > entryTime ? (exitTime - entryTime) / 60000 : 0

  const entryPrice = position.entry_price || 0
  const exitPrice = position.exit_price || 0
  const pnlPct = calcCloseROIPct(position)

  const maxQty = position.entry_quantity || position.quantity || 0
  const closeQty = position.quantity || maxQty

  return (
    <tr
      className="cursor-pointer transition-all duration-200 hover:bg-white/5"
      style={{ borderBottom: '1px solid #2B3139' }}
      onClick={() => onViewCloseDetails(position)}
    >
      {/* Symbol */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-2">
          <span className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
            {(position.symbol || '').replace('USDT', '')}
          </span>
          <span
            className="px-2 py-0.5 rounded text-xs font-semibold uppercase"
            style={{
              background: `${sideColor}22`,
              color: sideColor,
              border: `1px solid ${sideColor}44`,
            }}
          >
            {side}
          </span>
        </div>
      </td>

      {/* Entry Price */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatPrice(entryPrice)}
      </td>

      {/* Exit Price */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatPrice(exitPrice)}
      </td>

      {/* Max Holding Quantity */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatQuantity(maxQty)}
      </td>

      {/* Close Quantity */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#848E9C' }}>
        {formatQuantity(closeQty)}
      </td>

      {/* Position Value (Entry Price * Max Qty) */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatNumber(entryPrice * maxQty)}
      </td>

      {/* P&L (net, matches OKX App) */}
      <td className="py-3 px-4 text-right">
        <div className="font-mono font-semibold" style={{ color: pnlColor }}>
          {isProfitable ? '+' : ''}
          {formatNumber(displayPnl)}
        </div>
        <div className="text-xs" style={{ color: pnlColor }}>
          {pnlPct >= 0 ? '+' : ''}
          {pnlPct.toFixed(2)}%
        </div>
      </td>

      {/* Fee - show more precision for small fees */}
      <td className="py-3 px-4 text-right font-mono text-xs" style={{ color: '#848E9C' }}>
        -{((position.fee || 0) < 0.01 && (position.fee || 0) > 0)
          ? (position.fee || 0).toFixed(4)
          : (position.fee || 0).toFixed(2)}
      </td>

      {/* Duration */}
      <td className="py-3 px-4 text-center text-sm" style={{ color: '#848E9C' }}>
        {formatDuration(holdingMinutes)}
      </td>

      {/* Exit Time */}
      <td className="py-3 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
        {formatDate(position.exit_time)}
      </td>

      {/* AI decisions */}
      <td className="py-3 px-4 text-center whitespace-nowrap">
        <div className="inline-flex items-center gap-1">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onViewDecision(position, 'open')
            }}
            title={t('positionHistory.viewOpenDecision', language)}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/10"
            style={{ color: '#0ECB81', border: '1px solid rgba(14, 203, 129, 0.35)' }}
          >
            <Brain className="w-3.5 h-3.5" />
            <span>{language === 'zh' ? '开' : 'In'}</span>
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onViewDecision(position, 'close')
            }}
            title={t('positionHistory.viewCloseDecision', language)}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/10"
            style={{ color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.35)' }}
          >
            <Brain className="w-3.5 h-3.5" />
            <span>{language === 'zh' ? '平' : 'Out'}</span>
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onViewCloseDetails(position)
            }}
            title={t('positionHistory.viewCloseDetail', language)}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/10"
            style={{ color: '#F0B90B', border: '1px solid rgba(240, 185, 11, 0.35)' }}
          >
            <span>{language === 'zh' ? '详' : 'Ops'}</span>
          </button>
        </div>
      </td>
    </tr>
  )
}

function isCloseOrderAction(action: string | undefined): boolean {
  const normalized = String(action || '').trim().toLowerCase()
  return normalized === 'close_long' || normalized === 'close_short'
}

function formatCloseAction(action: string, language: Language): string {
  const normalized = String(action || '').trim().toLowerCase()
  if (language === 'zh') {
    if (normalized === 'close_long') return '平多'
    if (normalized === 'close_short') return '平空'
    if (normalized === 'open_long') return '开多'
    if (normalized === 'open_short') return '开空'
  }
  if (normalized === 'close_long') return 'Close Long'
  if (normalized === 'close_short') return 'Close Short'
  if (normalized === 'open_long') return 'Open Long'
  if (normalized === 'open_short') return 'Open Short'
  return action || '-'
}

function PositionCloseDetailModal({
  open,
  language,
  position,
  operations,
  loading,
  error,
  onClose,
}: {
  open: boolean
  language: Language
  position: HistoricalPosition | null
  operations: PositionCloseOperation[]
  loading: boolean
  error: string | null
  onClose: () => void
}) {
  if (!open || !position) return null

  const mergedOperations = mergeCloseOperations(operations)

  const totalReduced = mergedOperations
    .filter((op) => isCloseOrderAction(op.order_action))
    .reduce((sum, op) => sum + (op.exec_quantity || 0), 0)
  const initialQty = position.entry_quantity || position.quantity || 0
  const reducedRatio = initialQty > 0 ? (totalReduced / initialQty) * 100 : 0

  const modal = (
    <div
      className="fixed inset-0 z-[90] flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="w-full max-w-4xl max-h-[90vh] flex flex-col rounded-xl border overflow-hidden"
        style={{ background: '#12161C', borderColor: '#2B3139' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <div className="flex items-center justify-between px-5 py-4 border-b" style={{ borderColor: '#2B3139' }}>
          <div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.closeDetailTitle', language)}
            </div>
            <div className="mt-1 font-semibold" style={{ color: '#EAECEF' }}>
              {(position.symbol || '').replace('USDT', '')} {position.side}
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 rounded text-sm"
            style={{ background: '#1E2329', color: '#EAECEF', border: '1px solid #2B3139' }}
          >
            {t('decisionModal.close', language)}
          </button>
        </div>

        <div className="px-5 py-4 text-sm" style={{ color: '#848E9C' }}>
          {t('positionHistory.closeDetailSummary', language, {
            initial: formatQuantity(initialQty),
            reduced: formatQuantity(totalReduced),
            pct: reducedRatio.toFixed(1),
          })}
        </div>

        <div className="flex-1 min-h-0 overflow-auto border-t" style={{ borderColor: '#2B3139' }}>
          {loading ? (
            <div className="p-6 text-center" style={{ color: '#848E9C' }}>
              {t('positionHistory.loadingCloseDetail', language)}
            </div>
          ) : error ? (
            <div className="p-6 text-center" style={{ color: '#F6465D' }}>
              {error}
            </div>
          ) : operations.length === 0 ? (
            <div className="p-6 text-center" style={{ color: '#848E9C' }}>
              {t('positionHistory.noCloseDetail', language)}
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr style={{ background: '#0B0E11' }}>
                  <th className="py-2 px-4 text-left text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailAction', language)}
                  </th>
                  <th className="py-2 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailQty', language)}
                  </th>
                  <th className="py-2 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailPrice', language)}
                  </th>
                  <th className="py-2 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailFee', language)}
                  </th>
                  <th className="py-2 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailPnl', language)}
                  </th>
                  <th className="py-2 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
                    {t('positionHistory.closeDetailRoi', language)}
                  </th>
                </tr>
              </thead>
              <tbody>
                {mergedOperations.map((op, idx) => {
                  const pnl = Number(op.realized_pnl ?? 0)
                  const fee = Number(op.fee ?? 0)
                  const isProfitable = pnl >= 0
                  const pnlColor = isProfitable ? '#F6465D' : '#0ECB81'
                  const roiPct = calcOperationCloseROIPct(position, op)
                  const roiProfitable = roiPct != null && roiPct >= 0
                  const roiColor = roiProfitable ? '#F6465D' : '#0ECB81'
                  return (
                  <tr
                    key={`${op.exchange_order_id || 'noex'}-${op.order_action || ''}-${op.position_side || ''}-${idx}`}
                    style={{ borderTop: '1px solid #2B3139' }}
                  >
                    <td className="py-2 px-4 text-xs" style={{ color: '#F0B90B' }}>
                      {formatCloseAction(op.order_action, language)}
                    </td>
                    <td className="py-2 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
                      {formatQuantity(op.exec_quantity || 0)}
                    </td>
                    <td className="py-2 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
                      {formatPrice(op.exec_price || op.avg_fill_price || 0)}
                    </td>
                    <td className="py-2 px-4 text-right font-mono text-xs" style={{ color: '#848E9C' }}>
                      {fee !== 0 ? `-${Math.abs(fee).toFixed(fee < 0.01 && fee > 0 ? 4 : 2)}` : '-'}
                    </td>
                    <td className="py-2 px-4 text-right font-mono" style={{ color: pnl !== 0 ? pnlColor : '#848E9C' }}>
                      {pnl !== 0 ? `${isProfitable ? '+' : ''}${formatNumber(pnl)}` : '-'}
                    </td>
                    <td className="py-2 px-4 text-right font-mono text-xs" style={{ color: roiPct != null ? roiColor : '#848E9C' }}>
                      {roiPct != null
                        ? `${roiPct >= 0 ? '+' : ''}${roiPct.toFixed(2)}%`
                        : '-'}
                    </td>
                  </tr>
                )})}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )

  return createPortal(modal, document.body)
}

export function PositionHistory({ traderId, openPositionCount, openPositionsKey }: PositionHistoryProps) {
  const { language } = useLanguage()
  const [positions, setPositions] = useState<HistoricalPosition[]>([])
  const [stats, setStats] = useState<TraderStats | null>(null)
  const [symbolStats, setSymbolStats] = useState<SymbolStats[]>([])
  const [directionStats, setDirectionStats] = useState<DirectionStats[]>([])
  const prevOpenCountRef = useRef<number | undefined>(undefined)
  const prevPositionsKeyRef = useRef<string | undefined>(undefined)

  // Pagination state
  const [pageSize, setPageSize] = useState<number>(20)
  const [currentPage, setCurrentPage] = useState<number>(1)

  // Filter state
  const [filterSymbol, setFilterSymbol] = useState<string>('all')
  const [filterSide, setFilterSide] = useState<string>('all')
  const [filterTimePreset, setFilterTimePreset] = useState<TimeRangePreset>('all')
  const [filterDateFrom, setFilterDateFrom] = useState('')
  const [filterDateTo, setFilterDateTo] = useState('')
  const [sortBy, setSortBy] = useState<'time' | 'pnl' | 'pnl_pct'>('time')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  const historyFetchLimit = Math.max(200, pageSize * 5)

  const {
    data: historyData,
    error: swrError,
    isLoading,
    mutate: refreshHistory,
  } = useSWR(
    traderId ? [positionHistorySWRKey(traderId), historyFetchLimit] : null,
    ([, limit]) => api.getPositionHistory(traderId, limit as number, true),
    {
      refreshInterval: 5000,
      revalidateOnFocus: true,
      dedupingInterval: 0,
    }
  )

  useEffect(() => {
    if (!historyData) return
    setPositions(historyData.positions || [])
    setStats(historyData.stats)
    setSymbolStats(historyData.symbol_stats || [])
    setDirectionStats(historyData.direction_stats || [])
  }, [historyData])

  // Refresh when a position is fully closed (open count drops).
  useEffect(() => {
    if (
      traderId &&
      prevOpenCountRef.current !== undefined &&
      openPositionCount !== undefined &&
      openPositionCount < prevOpenCountRef.current
    ) {
      refreshHistory()
    }
    prevOpenCountRef.current = openPositionCount
  }, [openPositionCount, traderId, refreshHistory])

  // Refresh after close/reduce changes open position size (partial reduce included).
  useEffect(() => {
    if (!traderId || openPositionsKey === undefined) return
    if (
      prevPositionsKeyRef.current !== undefined &&
      prevPositionsKeyRef.current !== openPositionsKey
    ) {
      const timer = window.setTimeout(() => {
        refreshHistory()
      }, 1200)
      prevPositionsKeyRef.current = openPositionsKey
      return () => window.clearTimeout(timer)
    }
    prevPositionsKeyRef.current = openPositionsKey
  }, [openPositionsKey, traderId, refreshHistory])

  const loading = isLoading && !historyData
  const error = swrError
    ? swrError instanceof Error
      ? swrError.message
      : 'Failed to load history'
    : null

  const [decisionsCache, setDecisionsCache] = useState<DecisionRecord[] | null>(
    null
  )
  const [modalOpen, setModalOpen] = useState(false)
  const [modalLoading, setModalLoading] = useState(false)
  const [modalError, setModalError] = useState<
    'not_found' | 'system' | 'fetch' | null
  >(null)
  const [modalDecision, setModalDecision] = useState<DecisionRecord | null>(
    null
  )
  const [modalMeta, setModalMeta] = useState<{
    symbol: string
    action: string
    kind: PositionDecisionKind
  } | null>(null)
  const [opsModalOpen, setOpsModalOpen] = useState(false)
  const [opsModalLoading, setOpsModalLoading] = useState(false)
  const [opsModalError, setOpsModalError] = useState<string | null>(null)
  const [opsModalPosition, setOpsModalPosition] = useState<HistoricalPosition | null>(null)
  const [opsModalData, setOpsModalData] = useState<PositionCloseOperation[]>([])

  const loadDecisions = useCallback(async () => {
    if (decisionsCache) return decisionsCache
    const list = await api.getDecisions(traderId)
    setDecisionsCache(list)
    return list
  }, [traderId, decisionsCache])

  const handleViewDecision = useCallback(
    async (position: HistoricalPosition, kind: PositionDecisionKind) => {
      const action =
        kind === 'open'
          ? openActionForSide(position.side)
          : closeActionForSide(position.side)
      const eventTime =
        kind === 'open'
          ? new Date(position.entry_time).getTime()
          : new Date(position.exit_time).getTime()

      setModalMeta({
        symbol: position.symbol,
        action,
        kind,
      })
      setModalOpen(true)
      setModalLoading(true)
      setModalError(null)
      setModalDecision(null)

      try {
        const list = await loadDecisions()
        const match = findDecisionForOperation(
          list,
          position.symbol,
          action,
          eventTime
        )
        if (match) {
          setModalDecision(match)
        } else if (kind === 'close' && isLikelySystemClose(
          list,
          position.symbol,
          position.side,
          eventTime
        )) {
          setModalError('system')
        } else {
          setModalError('not_found')
        }
      } catch {
        setModalError('fetch')
      } finally {
        setModalLoading(false)
      }
    },
    [loadDecisions]
  )

  const closeModal = useCallback(() => {
    setModalOpen(false)
    setModalMeta(null)
    setModalDecision(null)
    setModalError(null)
  }, [])

  const handleViewCloseDetails = useCallback(
    async (position: HistoricalPosition) => {
      setOpsModalPosition(position)
      setOpsModalOpen(true)
      setOpsModalLoading(true)
      setOpsModalError(null)
      setOpsModalData([])
      try {
        const res = await api.getPositionCloseOperations(traderId, {
          symbol: position.symbol,
          side: position.side,
          entryTime: position.entry_time,
          exitTime: position.exit_time,
          limit: 100,
        })
        setOpsModalData(res.operations || [])
      } catch (err) {
        const msg = err instanceof Error ? err.message : t('positionHistory.closeDetailFetchFailed', language)
        setOpsModalError(msg)
      } finally {
        setOpsModalLoading(false)
      }
    },
    [traderId, language]
  )

  // Get unique symbols for filter
  const uniqueSymbols = useMemo(() => {
    const symbols = new Set(positions.map((p) => p.symbol))
    return Array.from(symbols).sort()
  }, [positions])

  // Filtered and sorted positions (before pagination)
  const filteredAndSortedPositions = useMemo(() => {
    let result = [...positions]

    // Apply filters
    if (filterSymbol !== 'all') {
      result = result.filter((p) => p.symbol === filterSymbol)
    }
    if (filterSide !== 'all') {
      result = result.filter(
        (p) => (p.side || '').toUpperCase() === filterSide.toUpperCase()
      )
    }

    const timeRange = resolveTimeRangeMs(filterTimePreset, filterDateFrom, filterDateTo)
    if (timeRange) {
      result = result.filter((p) => {
        const exitMs = positionExitTimeMs(p)
        if (exitMs <= 0) return false
        return exitMs >= timeRange.fromMs && exitMs <= timeRange.toMs
      })
    }

    // Apply sorting
    result.sort((a, b) => {
      let comparison = 0
      switch (sortBy) {
        case 'time':
          comparison =
            new Date(a.exit_time || 0).getTime() - new Date(b.exit_time || 0).getTime()
          break
        case 'pnl':
          comparison = getDisplayPnL(a) - getDisplayPnL(b)
          break
        case 'pnl_pct':
          comparison = calcCloseROIPct(a) - calcCloseROIPct(b)
          break
      }
      return sortOrder === 'desc' ? -comparison : comparison
    })

    return result
  }, [positions, filterSymbol, filterSide, filterTimePreset, filterDateFrom, filterDateTo, sortBy, sortOrder])

  // Pagination calculations
  const totalFilteredCount = filteredAndSortedPositions.length
  const totalPages = Math.ceil(totalFilteredCount / pageSize)

  // Reset to page 1 when filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [filterSymbol, filterSide, filterTimePreset, filterDateFrom, filterDateTo, sortBy, sortOrder, pageSize])

  // Paginated positions (for display)
  const paginatedPositions = useMemo(() => {
    const startIndex = (currentPage - 1) * pageSize
    return filteredAndSortedPositions.slice(startIndex, startIndex + pageSize)
  }, [filteredAndSortedPositions, currentPage, pageSize])

  // For backwards compatibility, keep filteredPositions as the paginated result
  const filteredPositions = paginatedPositions

  // Calculate profit/loss ratio (avg win / avg loss)
  const profitLossRatio = useMemo(() => {
    if (!stats) return 0
    const avgWin = stats.avg_win || 0
    const avgLoss = stats.avg_loss || 0
    if (avgLoss === 0) return avgWin > 0 ? Infinity : 0
    return avgWin / avgLoss
  }, [stats])

  if (loading) {
    return (
      <div
        className="flex items-center justify-center p-12"
        style={{ color: '#848E9C' }}
      >
        <div className="animate-spin mr-3">
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24">
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        </div>
        {t('positionHistory.loading', language)}
      </div>
    )
  }

  if (error) {
    return (
      <div
        className="rounded-lg p-6 text-center"
        style={{
          background: 'rgba(246, 70, 93, 0.1)',
          border: '1px solid rgba(246, 70, 93, 0.3)',
          color: '#F6465D',
        }}
      >
        {error}
      </div>
    )
  }

  if (positions.length === 0) {
    return (
      <div
        className="rounded-lg p-12 text-center"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
        }}
      >
        <div className="text-4xl mb-4">📊</div>
        <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
          {t('positionHistory.noHistory', language)}
        </div>
        <div style={{ color: '#848E9C' }}>
          {t('positionHistory.noHistoryDesc', language)}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Overall Stats - Row 1: Core Metrics */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
          <StatCard
            icon="📊"
            title={t('positionHistory.totalTrades', language)}
            value={stats.total_trades || 0}
            subtitle={t('positionHistory.winLoss', language, { win: stats.win_trades || 0, loss: stats.loss_trades || 0 })}
            language={language}
          />
          <StatCard
            icon="🎯"
            title={t('positionHistory.winRate', language)}
            value={(stats.win_rate || 0).toFixed(1)}
            suffix="%"
            color={
              (stats.win_rate || 0) >= 60
                ? '#0ECB81'
                : (stats.win_rate || 0) >= 40
                  ? '#F0B90B'
                  : '#F6465D'
            }
            metricKey="win_rate"
            language={language}
          />
          <StatCard
            icon="💰"
            title={t('positionHistory.totalPnL', language)}
            value={((stats.total_pnl || 0) >= 0 ? '+' : '') + formatNumber(stats.total_pnl || 0)}
            color={(stats.total_pnl || 0) >= 0 ? '#0ECB81' : '#F6465D'}
            subtitle={`${t('positionHistory.fee', language)}: -${formatNumber(stats.total_fee || 0)}`}
            metricKey="total_return"
            language={language}
          />
          <StatCard
            icon="📈"
            title={t('positionHistory.profitFactor', language)}
            value={(stats.profit_factor || 0).toFixed(2)}
            color={(stats.profit_factor || 0) >= 1.5 ? '#0ECB81' : (stats.profit_factor || 0) >= 1 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.profitFactorDesc', language)}
            metricKey="profit_factor"
            language={language}
          />
          <StatCard
            icon="⚖️"
            title={t('positionHistory.plRatio', language)}
            value={profitLossRatio === Infinity ? '∞' : profitLossRatio.toFixed(2)}
            color={profitLossRatio >= 1.5 ? '#0ECB81' : profitLossRatio >= 1 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.plRatioDesc', language)}
            metricKey="expectancy"
            language={language}
          />
        </div>
      )}

      {/* Overall Stats - Row 2: Advanced Metrics */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
          <StatCard
            icon="📉"
            title={t('positionHistory.sharpeRatio', language)}
            value={(stats.sharpe_ratio || 0).toFixed(2)}
            color={(stats.sharpe_ratio || 0) >= 1 ? '#0ECB81' : (stats.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.sharpeRatioDesc', language)}
            metricKey="sharpe_ratio"
            language={language}
          />
          <StatCard
            icon="🔻"
            title={t('positionHistory.maxDrawdown', language)}
            value={(stats.max_drawdown_pct || 0).toFixed(1)}
            suffix="%"
            color={(stats.max_drawdown_pct || 0) <= 10 ? '#0ECB81' : (stats.max_drawdown_pct || 0) <= 20 ? '#F0B90B' : '#F6465D'}
            metricKey="max_drawdown"
            language={language}
          />
          <StatCard
            icon="🏆"
            title={t('positionHistory.avgWin', language)}
            value={'+' + formatNumber(stats.avg_win || 0)}
            color="#0ECB81"
            metricKey="avg_trade_pnl"
            language={language}
          />
          <StatCard
            icon="💸"
            title={t('positionHistory.avgLoss', language)}
            value={'-' + formatNumber(stats.avg_loss || 0)}
            color="#F6465D"
            language={language}
          />
          <StatCard
            icon="💵"
            title={t('positionHistory.netPnL', language)}
            value={((stats.total_pnl || 0) - (stats.total_fee || 0) >= 0 ? '+' : '') + formatNumber((stats.total_pnl || 0) - (stats.total_fee || 0))}
            color={(stats.total_pnl || 0) - (stats.total_fee || 0) >= 0 ? '#0ECB81' : '#F6465D'}
            subtitle={t('positionHistory.netPnLDesc', language)}
            language={language}
          />
        </div>
      )}

      {/* Direction Stats */}
      {directionStats.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {directionStats.map((stat) => (
            <DirectionStatsCard key={stat.side} stat={stat} language={language} />
          ))}
        </div>
      )}

      {/* Symbol Performance */}
      {symbolStats.length > 0 && (
        <div
          className="rounded-lg p-4"
          style={{
            background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
            border: '1px solid #2B3139',
          }}
        >
          <div className="flex items-center gap-2 mb-4">
            <span className="text-lg">🏅</span>
            <span className="font-semibold" style={{ color: '#EAECEF' }}>
              {t('positionHistory.symbolPerformance', language)}
            </span>
          </div>
          <div className="space-y-1">
            {symbolStats.slice(0, 10).map((stat) => (
              <SymbolStatsRow key={stat.symbol} stat={stat} />
            ))}
          </div>
        </div>
      )}

      {/* Position List */}
      <div
        className="rounded-lg overflow-hidden"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
        }}
      >
        {/* Filters */}
        <div
          className="flex flex-wrap items-center gap-4 p-4"
          style={{ borderBottom: '1px solid #2B3139' }}
        >
          <div className="flex items-center gap-2">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.symbol', language)}:
            </span>
            <NofxSelect
              value={filterSymbol}
              onChange={(val) => setFilterSymbol(val)}
              options={[
                { value: 'all', label: t('positionHistory.allSymbols', language) },
                ...uniqueSymbols.map(s => ({ value: s, label: (s || '').replace('USDT', '') }))
              ]}
              className="rounded px-3 py-1.5 text-sm"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>

          <div className="flex items-center gap-2">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.side', language)}:
            </span>
            <div className="flex rounded overflow-hidden" style={{ border: '1px solid #2B3139' }}>
              {['all', 'LONG', 'SHORT'].map((side) => (
                <button
                  key={side}
                  onClick={() => setFilterSide(side)}
                  className="px-3 py-1.5 text-sm capitalize transition-colors"
                  style={{
                    background: filterSide === side ? '#2B3139' : 'transparent',
                    color: filterSide === side ? '#EAECEF' : '#848E9C',
                  }}
                >
                  {side === 'all' ? t('positionHistory.all', language) : side}
                </button>
              ))}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.timeRange', language)}:
            </span>
            <div className="flex flex-wrap rounded overflow-hidden" style={{ border: '1px solid #2B3139' }}>
              {(
                [
                  ['all', 'timeAll'],
                  ['7d', 'time7d'],
                  ['30d', 'time30d'],
                  ['90d', 'time90d'],
                  ['custom', 'timeCustom'],
                ] as const
              ).map(([preset, labelKey]) => (
                <button
                  key={preset}
                  type="button"
                  onClick={() => setFilterTimePreset(preset)}
                  className="px-3 py-1.5 text-sm transition-colors"
                  style={{
                    background: filterTimePreset === preset ? '#2B3139' : 'transparent',
                    color: filterTimePreset === preset ? '#EAECEF' : '#848E9C',
                  }}
                >
                  {t(`positionHistory.${labelKey}`, language)}
                </button>
              ))}
            </div>
            {filterTimePreset === 'custom' && (
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {t('positionHistory.timeFrom', language)}
                </span>
                <input
                  type="date"
                  value={filterDateFrom}
                  onChange={(e) => setFilterDateFrom(e.target.value)}
                  className="rounded px-2 py-1 text-sm"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                    colorScheme: 'dark',
                  }}
                />
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {t('positionHistory.timeTo', language)}
                </span>
                <input
                  type="date"
                  value={filterDateTo}
                  onChange={(e) => setFilterDateTo(e.target.value)}
                  className="rounded px-2 py-1 text-sm"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                    colorScheme: 'dark',
                  }}
                />
              </div>
            )}
          </div>

          <div className="flex items-center gap-2 ml-auto">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.sort', language)}:
            </span>
            <NofxSelect
              value={`${sortBy}-${sortOrder}`}
              onChange={(val) => {
                const [by, order] = val.split('-') as ['time' | 'pnl' | 'pnl_pct', 'asc' | 'desc']
                setSortBy(by)
                setSortOrder(order)
              }}
              options={[
                { value: 'time-desc', label: t('positionHistory.latestFirst', language) },
                { value: 'time-asc', label: t('positionHistory.oldestFirst', language) },
                { value: 'pnl-desc', label: t('positionHistory.highestPnL', language) },
                { value: 'pnl-asc', label: t('positionHistory.lowestPnL', language) },
              ]}
              className="rounded px-3 py-1.5 text-sm"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>
        </div>

        {/* Table */}
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr style={{ background: '#0B0E11' }}>
                <th
                  className="py-3 px-4 text-left text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.symbol', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.entry', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.exit', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.maxQty', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.qty', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.value', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.pnl', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.fee', language)}
                </th>
                <th
                  className="py-3 px-4 text-center text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.duration', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.closedAt', language)}
                </th>
                <th
                  className="py-3 px-4 text-center text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.colDecision', language)}
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredPositions.length === 0 ? (
                <tr>
                  <td
                    colSpan={11}
                    className="py-10 text-center text-sm"
                    style={{ color: '#848E9C' }}
                  >
                    {positions.length > 0
                      ? t('positionHistory.noFilterResults', language)
                      : t('positionHistory.noHistory', language)}
                  </td>
                </tr>
              ) : (
                filteredPositions.map((position) => (
                  <PositionRow
                    key={position.id}
                    position={position}
                    language={language}
                    onViewDecision={handleViewDecision}
                    onViewCloseDetails={handleViewCloseDetails}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Footer with Pagination */}
        <div
          className="flex flex-wrap items-center justify-between gap-4 p-4 text-sm"
          style={{ borderTop: '1px solid #2B3139', color: '#848E9C' }}
        >
          {/* Left: Count info */}
          <div className="flex items-center gap-4">
            <span>
              {t('positionHistory.showingPositions', language, { count: totalFilteredCount, total: positions.length })}
            </span>
            {totalFilteredCount > 0 && (
              <span>
                {t('positionHistory.totalPnL', language)}:{' '}
                <span
                  style={{
                    color:
                      filteredAndSortedPositions.reduce((sum, p) => sum + getDisplayPnL(p), 0) >= 0
                        ? '#0ECB81'
                        : '#F6465D',
                  }}
                >
                  {filteredAndSortedPositions.reduce((sum, p) => sum + getDisplayPnL(p), 0) >= 0
                    ? '+'
                    : ''}
                  {formatNumber(
                    filteredAndSortedPositions.reduce((sum, p) => sum + getDisplayPnL(p), 0)
                  )}
                </span>
              </span>
            )}
          </div>

          {/* Right: Pagination controls */}
          <div className="flex items-center gap-3">
            {/* Page size selector */}
            <div className="flex items-center gap-2">
              <span className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '每页' : 'Per page'}:
              </span>
              <NofxSelect
                value={pageSize}
                onChange={(val) => setPageSize(Number(val))}
                options={[
                  { value: 20, label: '20' },
                  { value: 50, label: '50' },
                  { value: 100, label: '100' },
                ]}
                className="rounded px-2 py-1 text-sm"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>

            {/* Page navigation */}
            {totalPages > 1 && (
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setCurrentPage(1)}
                  disabled={currentPage === 1}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  «
                </button>
                <button
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  ‹
                </button>
                <span className="px-3 text-xs" style={{ color: '#EAECEF' }}>
                  {currentPage} / {totalPages}
                </span>
                <button
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  ›
                </button>
                <button
                  onClick={() => setCurrentPage(totalPages)}
                  disabled={currentPage === totalPages}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  »
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      <DecisionDetailModal
        open={modalOpen}
        onClose={closeModal}
        language={language}
        title={
          modalMeta
            ? t('decisionModal.titleLive', language)
                .replace('{symbol}', modalMeta.symbol.replace('USDT', ''))
                .replace(
                  '{action}',
                  modalMeta.kind === 'open'
                    ? t('positionHistory.viewOpenDecision', language)
                    : t('positionHistory.viewCloseDecision', language)
                )
            : ''
        }
        loading={modalLoading}
        errorKind={modalError}
        decision={modalDecision}
        highlightSymbol={modalMeta?.symbol}
        highlightAction={modalMeta?.action}
      />
      <PositionCloseDetailModal
        open={opsModalOpen}
        language={language}
        position={opsModalPosition}
        operations={opsModalData}
        loading={opsModalLoading}
        error={opsModalError}
        onClose={() => setOpsModalOpen(false)}
      />
    </div>
  )
}
