import { useMemo, useState, type CSSProperties } from 'react'
import { motion } from 'framer-motion'
import { ArrowDown, ArrowUp, ArrowUpDown, Filter, RotateCcw } from 'lucide-react'
import type { BacktestTradeEvent } from '../../types'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'

type ActionFilter = 'all' | 'open' | 'close' | 'liquidated'
type TimeSortOrder = 'newest' | 'oldest'
type SortKey = 'time' | 'pnl'
type SortDir = 'asc' | 'desc'

const selectStyle: CSSProperties = {
  background: '#1E2329',
  border: '1px solid #2B3139',
  color: '#EAECEF',
}

function formatActionLabel(action: string): string {
  return action.replace(/_/g, ' ').toUpperCase()
}

function matchesActionFilter(trade: BacktestTradeEvent, filter: ActionFilter): boolean {
  switch (filter) {
    case 'open':
      return trade.action.includes('open')
    case 'close':
      return trade.action.includes('close')
    case 'liquidated':
      return trade.liquidation || trade.action === 'liquidated'
    default:
      return true
  }
}

function sortTrades(
  list: BacktestTradeEvent[],
  sortKey: SortKey,
  sortDir: SortDir
): BacktestTradeEvent[] {
  const sorted = [...list]
  if (sortKey === 'pnl') {
    sorted.sort((a, b) =>
      sortDir === 'desc'
        ? b.realized_pnl - a.realized_pnl
        : a.realized_pnl - b.realized_pnl
    )
    return sorted
  }
  sorted.sort((a, b) => (sortDir === 'desc' ? b.ts - a.ts : a.ts - b.ts))
  return sorted
}

function SortablePnlHeader({
  sortKey,
  sortDir,
  onSort,
}: {
  sortKey: SortKey
  sortDir: SortDir
  onSort: () => void
}) {
  const { language } = useLanguage()
  const active = sortKey === 'pnl'
  const title = active
    ? sortDir === 'desc'
      ? t('backtestTrades.sortPnlHigh', language)
      : t('backtestTrades.sortPnlLow', language)
    : t('backtestTrades.colPnl', language)

  return (
    <button
      type="button"
      onClick={onSort}
      title={title}
      className="inline-flex items-center justify-end gap-1 w-full font-medium whitespace-nowrap transition-colors hover:text-[#EAECEF]"
      style={{ color: active ? '#F0B90B' : '#848E9C' }}
    >
      {t('backtestTrades.colPnl', language)}
      {active ? (
        sortDir === 'desc' ? (
          <ArrowDown className="w-3.5 h-3.5" />
        ) : (
          <ArrowUp className="w-3.5 h-3.5" />
        )
      ) : (
        <ArrowUpDown className="w-3.5 h-3.5 opacity-60" />
      )}
    </button>
  )
}

function TradeTable({
  trades,
  sortKey,
  sortDir,
  onPnlSort,
}: {
  trades: BacktestTradeEvent[]
  sortKey: SortKey
  sortDir: SortDir
  onPnlSort: () => void
}) {
  const { language } = useLanguage()

  if (trades.length === 0) {
    return (
      <div className="py-12 text-center" style={{ color: '#5E6673' }}>
        {t('backtestTrades.noTrades', language)}
      </div>
    )
  }

  return (
    <div
      className="rounded-lg overflow-hidden"
      style={{ border: '1px solid #2B3139' }}
    >
      <div className="overflow-auto max-h-[min(70vh,720px)]">
        <table className="w-full text-sm">
          <thead
            className="sticky top-0 z-10"
            style={{ background: '#1E2329' }}
          >
            <tr className="text-left text-xs" style={{ color: '#848E9C' }}>
              <th className="px-3 py-2 font-medium whitespace-nowrap">
                {t('backtestTrades.colTime', language)}
              </th>
              <th className="px-3 py-2 font-medium whitespace-nowrap">
                {t('backtestTrades.colSymbol', language)}
              </th>
              <th className="px-3 py-2 font-medium whitespace-nowrap">
                {t('backtestTrades.colAction', language)}
              </th>
              <th className="px-3 py-2 font-medium text-right whitespace-nowrap">
                {t('backtestTrades.colQty', language)}
              </th>
              <th className="px-3 py-2 font-medium text-right whitespace-nowrap">
                {t('backtestTrades.colPrice', language)}
              </th>
              <th className="px-3 py-2 text-right whitespace-nowrap">
                <SortablePnlHeader
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onSort={onPnlSort}
                />
              </th>
              <th className="px-3 py-2 font-medium text-right whitespace-nowrap">
                {t('backtestTrades.colFee', language)}
              </th>
            </tr>
          </thead>
          <tbody>
            {trades.map((trade, idx) => {
              const isOpen = trade.action.includes('open')
              const pnlColor =
                trade.realized_pnl >= 0 ? '#0ECB81' : '#F6465D'
              const rowBg =
                idx % 2 === 0 ? 'transparent' : 'rgba(255,255,255,0.02)'

              return (
                <tr
                  key={`${trade.ts}-${trade.symbol}-${trade.action}-${idx}`}
                  className="border-t"
                  style={{ background: rowBg, borderColor: '#2B3139' }}
                >
                  <td
                    className="px-3 py-2 font-mono text-xs whitespace-nowrap"
                    style={{ color: '#848E9C' }}
                  >
                    {new Date(trade.ts).toLocaleString()}
                  </td>
                  <td
                    className="px-3 py-2 font-mono font-bold whitespace-nowrap"
                    style={{ color: '#EAECEF' }}
                  >
                    {trade.symbol.replace('USDT', '')}
                  </td>
                  <td className="px-3 py-2 whitespace-nowrap">
                    <span
                      className="px-2 py-0.5 rounded text-xs font-medium"
                      style={{
                        background: isOpen
                          ? 'rgba(14, 203, 129, 0.15)'
                          : 'rgba(246, 70, 93, 0.15)',
                        color: isOpen ? '#0ECB81' : '#F6465D',
                      }}
                    >
                      {formatActionLabel(trade.action)}
                    </span>
                    {trade.leverage ? (
                      <span
                        className="ml-1 text-xs"
                        style={{ color: '#5E6673' }}
                      >
                        {trade.leverage}x
                      </span>
                    ) : null}
                  </td>
                  <td
                    className="px-3 py-2 font-mono text-right whitespace-nowrap"
                    style={{ color: '#EAECEF' }}
                  >
                    {trade.qty.toFixed(4)}
                  </td>
                  <td
                    className="px-3 py-2 font-mono text-right whitespace-nowrap"
                    style={{ color: '#EAECEF' }}
                  >
                    ${trade.price.toFixed(2)}
                  </td>
                  <td
                    className="px-3 py-2 font-mono text-right font-bold whitespace-nowrap"
                    style={{ color: pnlColor }}
                  >
                    {trade.realized_pnl >= 0 ? '+' : ''}
                    {trade.realized_pnl.toFixed(2)}
                  </td>
                  <td
                    className="px-3 py-2 font-mono text-right whitespace-nowrap"
                    style={{ color: '#848E9C' }}
                  >
                    {trade.fee.toFixed(4)}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

interface BacktestTradesTabProps {
  trades: BacktestTradeEvent[] | undefined
}

export function BacktestTradesTab({ trades }: BacktestTradesTabProps) {
  const { language } = useLanguage()
  const allTrades = trades ?? []

  const symbolOptions = useMemo(() => {
    const set = new Set<string>()
    for (const trade of allTrades) {
      if (trade.symbol) {
        set.add(trade.symbol)
      }
    }
    return Array.from(set).sort()
  }, [allTrades])

  const [symbolFilter, setSymbolFilter] = useState<string>('all')
  const [actionFilter, setActionFilter] = useState<ActionFilter>('all')
  const [timeSort, setTimeSort] = useState<TimeSortOrder>('newest')
  const [sortKey, setSortKey] = useState<SortKey>('time')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const handleTimeSortChange = (order: TimeSortOrder) => {
    setTimeSort(order)
    setSortKey('time')
    setSortDir(order === 'newest' ? 'desc' : 'asc')
  }

  const handlePnlSort = () => {
    if (sortKey !== 'pnl') {
      setSortKey('pnl')
      setSortDir('desc')
      return
    }
    setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))
  }

  const filteredTrades = useMemo(() => {
    const list = allTrades.filter((trade) => {
      if (symbolFilter !== 'all' && trade.symbol !== symbolFilter) {
        return false
      }
      return matchesActionFilter(trade, actionFilter)
    })
    return sortTrades(list, sortKey, sortDir)
  }, [allTrades, symbolFilter, actionFilter, sortKey, sortDir])

  const hasActiveFilters =
    symbolFilter !== 'all' ||
    actionFilter !== 'all' ||
    timeSort !== 'newest' ||
    sortKey === 'pnl'

  const resetFilters = () => {
    setSymbolFilter('all')
    setActionFilter('all')
    setTimeSort('newest')
    setSortKey('time')
    setSortDir('desc')
  }

  const showingText = t('backtestTrades.showing', language)
    .replace('{shown}', String(filteredTrades.length))
    .replace('{total}', String(allTrades.length))

  return (
    <motion.div
      key="trades"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="space-y-4"
    >
      <div
        className="flex flex-wrap items-end gap-3 p-4 rounded-lg"
        style={{ background: '#1E2329', border: '1px solid #2B3139' }}
      >
        <div className="flex items-center gap-2 mr-1">
          <Filter size={16} style={{ color: '#F0B90B' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            {t('backtestTrades.filterSymbol', language)}
          </span>
        </div>

        <label className="flex flex-col gap-1">
          <span className="text-xs" style={{ color: '#848E9C' }}>
            {t('backtestTrades.filterSymbol', language)}
          </span>
          <select
            value={symbolFilter}
            onChange={(e) => setSymbolFilter(e.target.value)}
            className="px-3 py-1.5 rounded text-sm min-w-[140px]"
            style={selectStyle}
          >
            <option value="all">
              {t('backtestTrades.allSymbols', language)}
            </option>
            {symbolOptions.map((sym) => (
              <option key={sym} value={sym}>
                {sym}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs" style={{ color: '#848E9C' }}>
            {t('backtestTrades.filterAction', language)}
          </span>
          <select
            value={actionFilter}
            onChange={(e) => setActionFilter(e.target.value as ActionFilter)}
            className="px-3 py-1.5 rounded text-sm min-w-[120px]"
            style={selectStyle}
          >
            <option value="all">
              {t('backtestTrades.allActions', language)}
            </option>
            <option value="open">
              {t('backtestTrades.actionOpen', language)}
            </option>
            <option value="close">
              {t('backtestTrades.actionClose', language)}
            </option>
            <option value="liquidated">
              {t('backtestTrades.actionLiquidated', language)}
            </option>
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs" style={{ color: '#848E9C' }}>
            —
          </span>
          <select
            value={timeSort}
            onChange={(e) =>
              handleTimeSortChange(e.target.value as TimeSortOrder)
            }
            className="px-3 py-1.5 rounded text-sm min-w-[120px]"
            style={{
              ...selectStyle,
              borderColor: sortKey === 'time' ? '#F0B90B' : '#2B3139',
            }}
          >
            <option value="newest">
              {t('backtestTrades.sortNewest', language)}
            </option>
            <option value="oldest">
              {t('backtestTrades.sortOldest', language)}
            </option>
          </select>
        </label>

        {hasActiveFilters && (
          <button
            type="button"
            onClick={resetFilters}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-sm transition-colors hover:opacity-90"
            style={{
              background: '#2B3139',
              color: '#EAECEF',
              border: '1px solid #474D57',
            }}
          >
            <RotateCcw size={14} />
            {t('backtestTrades.resetFilters', language)}
          </button>
        )}

        <span className="text-xs ml-auto" style={{ color: '#5E6673' }}>
          {showingText}
          {sortKey === 'pnl' && (
            <span style={{ color: '#F0B90B' }}>
              {' '}
              ·{' '}
              {sortDir === 'desc'
                ? t('backtestTrades.sortPnlHigh', language)
                : t('backtestTrades.sortPnlLow', language)}
            </span>
          )}
        </span>
      </div>

      <TradeTable
        trades={filteredTrades}
        sortKey={sortKey}
        sortDir={sortDir}
        onPnlSort={handlePnlSort}
      />
    </motion.div>
  )
}
