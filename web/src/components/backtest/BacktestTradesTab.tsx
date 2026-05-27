import {
  useCallback,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from 'react'
import { motion } from 'framer-motion'
import { ArrowDown, ArrowUp, ArrowUpDown, Brain, Filter, RotateCcw } from 'lucide-react'
import type { BacktestTradeEvent, DecisionRecord } from '../../types'
import { api } from '../../lib/api'
import {
  findMatchingActionIndex,
  isAutomaticClose,
  tradeActionForBacktest,
} from '../../lib/decisionTradeMatch'
import { DecisionDetailModal } from '../trader/DecisionDetailModal'
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

/** Fills parent flex area and scrolls children when content overflows. */
function TradeListScrollArea({ children }: { children: ReactNode }) {
  const shellRef = useRef<HTMLDivElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const shell = shellRef.current
    const scroll = scrollRef.current
    if (!shell || !scroll) return

    const sync = () => {
      const h = shell.clientHeight
      if (h > 0) {
        scroll.style.height = `${h}px`
        scroll.style.maxHeight = `${h}px`
      }
    }

    sync()
    const ro = new ResizeObserver(sync)
    ro.observe(shell)
    return () => ro.disconnect()
  }, [children])

  return (
    <div
      ref={shellRef}
      className="flex-1 min-h-0 flex flex-col overflow-hidden rounded-lg"
      style={{ border: '1px solid #2B3139' }}
    >
      <div ref={scrollRef} className="overflow-y-auto overscroll-contain">
        {children}
      </div>
    </div>
  )
}

function formatActionLabel(action: string, closeReason?: string): string {
  if (closeReason === 'stop_loss') return 'STOP LOSS'
  if (closeReason === 'take_profit') return 'TAKE PROFIT'
  if (closeReason === 'liquidation') return 'LIQUIDATED'
  return action.replace(/_/g, ' ').toUpperCase()
}

function matchesActionFilter(trade: BacktestTradeEvent, filter: ActionFilter): boolean {
  switch (filter) {
    case 'open':
      return trade.action.includes('open')
    case 'close':
      return (
        trade.action.includes('close') &&
        trade.close_reason !== 'stop_loss' &&
        trade.close_reason !== 'take_profit'
      )
    case 'liquidated':
      return trade.liquidation || trade.action === 'liquidated' || trade.close_reason === 'liquidation'
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
  onActionClick,
}: {
  trades: BacktestTradeEvent[]
  sortKey: SortKey
  sortDir: SortDir
  onPnlSort: () => void
  onActionClick: (trade: BacktestTradeEvent) => void
}) {
  const { language } = useLanguage()

  if (trades.length === 0) {
    return (
      <TradeListScrollArea>
        <div className="py-12 text-center" style={{ color: '#5E6673' }}>
          {t('backtestTrades.noTrades', language)}
        </div>
      </TradeListScrollArea>
    )
  }

  return (
    <TradeListScrollArea>
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
                {t('backtestTrades.colEntry', language)}
              </th>
              <th className="px-3 py-2 font-medium text-right whitespace-nowrap">
                {t('backtestTrades.colExit', language)}
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
              const entryPx =
                trade.entry_price && trade.entry_price > 0
                  ? trade.entry_price
                  : isOpen
                    ? trade.price
                    : 0
              const exitPx =
                trade.exit_price && trade.exit_price > 0
                  ? trade.exit_price
                  : !isOpen
                    ? trade.price
                    : 0
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
                    <button
                      type="button"
                      onClick={() => onActionClick(trade)}
                      title={t('backtestTrades.viewDecision', language)}
                      className="inline-flex items-center gap-1 rounded transition-opacity hover:opacity-90"
                    >
                      <span
                        className="px-2 py-0.5 rounded text-xs font-medium"
                        style={{
                          background: isOpen
                            ? 'rgba(14, 203, 129, 0.15)'
                            : 'rgba(246, 70, 93, 0.15)',
                          color: isOpen ? '#0ECB81' : '#F6465D',
                          border: '1px solid transparent',
                        }}
                      >
                        {formatActionLabel(trade.action, trade.close_reason)}
                      </span>
                      <Brain
                        className="w-3.5 h-3.5 shrink-0"
                        style={{ color: '#F0B90B' }}
                      />
                    </button>
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
                    {entryPx > 0 ? `$${entryPx.toFixed(2)}` : '—'}
                  </td>
                  <td
                    className="px-3 py-2 font-mono text-right whitespace-nowrap"
                    style={{ color: '#EAECEF' }}
                  >
                    {exitPx > 0 ? `$${exitPx.toFixed(2)}` : '—'}
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
    </TradeListScrollArea>
  )
}

interface BacktestTradesTabProps {
  runId: string
  trades: BacktestTradeEvent[] | undefined
  className?: string
}

export function BacktestTradesTab({ runId, trades, className }: BacktestTradesTabProps) {
  const { language } = useLanguage()
  const allTrades = trades ?? []

  const [modalOpen, setModalOpen] = useState(false)
  const [modalLoading, setModalLoading] = useState(false)
  const [modalError, setModalError] = useState<
    'not_found' | 'system' | 'fetch' | null
  >(null)
  const [modalDecision, setModalDecision] = useState<DecisionRecord | null>(
    null
  )
  const [activeTrade, setActiveTrade] = useState<BacktestTradeEvent | null>(
    null
  )

  const handleActionClick = useCallback(
    async (trade: BacktestTradeEvent) => {
      setActiveTrade(trade)
      setModalOpen(true)
      setModalLoading(true)
      setModalError(null)
      setModalDecision(null)

      const action = tradeActionForBacktest(trade)
      const autoClose = isAutomaticClose(trade)

      try {
        if (trade.cycle > 0) {
          const record = await api.getBacktestTrace(runId, trade.cycle)
          const matchIdx = findMatchingActionIndex(
            record,
            trade.symbol,
            action
          )
          if (matchIdx < 0 && autoClose) {
            setModalDecision(record)
            setModalError('system')
          } else {
            setModalDecision(record)
          }
        } else if (autoClose) {
          setModalError('system')
        } else {
          setModalError('not_found')
        }
      } catch {
        if (autoClose) {
          setModalError('system')
        } else {
          setModalError('not_found')
        }
      } finally {
        setModalLoading(false)
      }
    },
    [runId]
  )

  const closeModal = useCallback(() => {
    setModalOpen(false)
    setActiveTrade(null)
    setModalDecision(null)
    setModalError(null)
  }, [])

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
      className={[
        'flex flex-col h-full min-h-0 w-full gap-4 overflow-hidden',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div
        className="flex flex-wrap items-end gap-3 p-4 rounded-lg shrink-0"
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
          {t('backtestTrades.clickActionHint', language)} · {showingText}
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

      <div className="flex flex-col flex-1 min-h-0 overflow-hidden">
        <TradeTable
          trades={filteredTrades}
          sortKey={sortKey}
          sortDir={sortDir}
          onPnlSort={handlePnlSort}
          onActionClick={handleActionClick}
        />
      </div>

      <DecisionDetailModal
        open={modalOpen}
        onClose={closeModal}
        language={language}
        title={
          activeTrade
            ? t('decisionModal.titleBacktest', language).replace(
                '{cycle}',
                String(activeTrade.cycle)
              )
            : t('decisionModal.loading', language)
        }
        subtitle={
          activeTrade
            ? `${activeTrade.symbol.replace('USDT', '')} · ${formatActionLabel(activeTrade.action, activeTrade.close_reason)}`
            : undefined
        }
        loading={modalLoading}
        errorKind={modalError}
        decision={modalDecision}
        highlightSymbol={activeTrade?.symbol}
        highlightAction={
          activeTrade ? tradeActionForBacktest(activeTrade) : undefined
        }
      />
    </motion.div>
  )
}
