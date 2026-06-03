import { t, type Language } from '../../i18n/translations'
import { formatBeijingDateTime, formatPrice } from '../../utils/format'
import type { Position } from '../../types'
import type { PositionPnlHistoryEntry } from '../../hooks/usePositionPnlHistory'

interface PositionPnlHistoryModalProps {
  open: boolean
  language: Language
  position: Position | null
  entries: PositionPnlHistoryEntry[]
  onClose: () => void
}

export function PositionPnlHistoryModal({
  open,
  language,
  position,
  entries,
  onClose,
}: PositionPnlHistoryModalProps) {
  if (!open || !position) return null

  const sideLabel =
    position.side?.toLowerCase() === 'long'
      ? t('long', language)
      : t('short', language)

  return (
    <div
      className="fixed inset-0 z-[90] flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="w-full max-w-lg max-h-[85vh] flex flex-col rounded-xl border overflow-hidden nofx-glass"
        style={{ borderColor: 'rgba(255,255,255,0.08)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="pnl-history-title"
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/10 shrink-0">
          <div>
            <div className="text-xs text-nofx-text-muted">
              {t('traderDashboard.pnlHistoryTitle', language)}
            </div>
            <div id="pnl-history-title" className="mt-1 font-bold text-nofx-text-main">
              {position.symbol}{' '}
              <span className="text-nofx-gold uppercase text-sm">{sideLabel}</span>
            </div>
            <div className="text-[10px] text-nofx-text-muted mt-1">
              {t('traderDashboard.marginPnlPctHint', language)}
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 rounded text-sm bg-white/5 text-nofx-text-main border border-white/10 hover:bg-white/10 transition-colors"
          >
            {t('traderDashboard.cancel', language)}
          </button>
        </div>

        <div className="flex-1 min-h-0 overflow-auto">
          {entries.length === 0 ? (
            <div className="p-8 text-center text-nofx-text-muted text-sm">
              {t('traderDashboard.pnlHistoryEmpty', language)}
            </div>
          ) : (
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-[#12161C]/95 backdrop-blur border-b border-white/10">
                <tr className="text-left text-nofx-text-muted">
                  <th className="px-4 py-2.5 font-semibold whitespace-nowrap">
                    {t('traderDashboard.pnlHistoryTime', language)}
                  </th>
                  <th className="px-4 py-2.5 font-semibold text-right whitespace-nowrap">
                    {t('traderDashboard.marginPnlPct', language)}
                  </th>
                  <th className="px-4 py-2.5 font-semibold text-right whitespace-nowrap hidden sm:table-cell">
                    {t('traderDashboard.uPnL', language)}
                  </th>
                  <th className="px-4 py-2.5 font-semibold text-right whitespace-nowrap hidden md:table-cell">
                    {t('traderDashboard.mark', language)}
                  </th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry, idx) => {
                  const pct = entry.unrealized_pnl_pct
                  const pnl = entry.unrealized_pnl
                  const timeStr =
                    formatBeijingDateTime(entry.recorded_at) ??
                    String(entry.recorded_at)
                  return (
                    <tr
                      key={`${entry.recorded_at}-${idx}`}
                      className="border-b border-white/5 last:border-0 hover:bg-white/5"
                    >
                      <td className="px-4 py-2.5 font-mono text-nofx-text-main whitespace-nowrap">
                        {timeStr}
                      </td>
                      <td className="px-4 py-2.5 font-mono text-right whitespace-nowrap">
                        <span
                          className={`font-bold ${pct >= 0 ? 'text-nofx-green' : 'text-nofx-red'}`}
                        >
                          {pct >= 0 ? '+' : ''}
                          {pct.toFixed(2)}%
                        </span>
                      </td>
                      <td className="px-4 py-2.5 font-mono text-right whitespace-nowrap hidden sm:table-cell">
                        <span className={pnl >= 0 ? 'text-nofx-green' : 'text-nofx-red'}>
                          {pnl >= 0 ? '+' : ''}
                          {pnl.toFixed(2)}
                        </span>
                      </td>
                      <td className="px-4 py-2.5 font-mono text-right text-nofx-text-muted whitespace-nowrap hidden md:table-cell">
                        {formatPrice(entry.mark_price)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>

        {entries.length > 0 && (
          <div className="px-5 py-3 border-t border-white/10 text-[10px] text-nofx-text-muted shrink-0">
            {t('traderDashboard.pnlHistoryCount', language, { count: entries.length })}
          </div>
        )}
      </div>
    </div>
  )
}
