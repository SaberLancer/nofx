import { useEffect } from 'react'
import { createPortal } from 'react-dom'
import { X, Loader2 } from 'lucide-react'
import type { DecisionRecord } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { DecisionCard } from './DecisionCard'
import { actionHighlightKey } from '../../lib/decisionTradeMatch'

export interface DecisionDetailModalProps {
  open: boolean
  onClose: () => void
  language: Language
  title: string
  subtitle?: string
  loading?: boolean
  errorKind?: 'not_found' | 'system' | 'fetch' | null
  decision?: DecisionRecord | null
  highlightSymbol?: string
  highlightAction?: string
}

export function DecisionDetailModal({
  open,
  onClose,
  language,
  title,
  subtitle,
  loading = false,
  errorKind = null,
  decision = null,
  highlightSymbol,
  highlightAction,
}: DecisionDetailModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  const highlightKey =
    highlightSymbol && highlightAction
      ? actionHighlightKey(highlightSymbol, highlightAction)
      : undefined

  const modal = (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="relative w-full max-w-3xl max-h-[90vh] overflow-hidden rounded-xl flex flex-col"
        style={{
          background: 'linear-gradient(180deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
          boxShadow: '0 24px 48px rgba(0,0,0,0.5)',
        }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="decision-detail-title"
      >
        <div
          className="flex items-start justify-between gap-4 px-5 py-4 shrink-0"
          style={{ borderBottom: '1px solid #2B3139' }}
        >
          <div className="min-w-0">
            <h2
              id="decision-detail-title"
              className="text-lg font-bold truncate"
              style={{ color: '#EAECEF' }}
            >
              {title}
            </h2>
            {subtitle ? (
              <p className="text-xs mt-1 truncate" style={{ color: '#848E9C' }}>
                {subtitle}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded-lg transition-colors hover:bg-white/10 shrink-0"
            style={{ color: '#848E9C' }}
            aria-label={t('decisionModal.close', language)}
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="overflow-y-auto flex-1 p-5">
          {loading ? (
            <div
              className="flex flex-col items-center justify-center py-16 gap-3"
              style={{ color: '#848E9C' }}
            >
              <Loader2 className="w-8 h-8 animate-spin" style={{ color: '#F0B90B' }} />
              <span className="text-sm">{t('decisionModal.loading', language)}</span>
            </div>
          ) : !decision && errorKind ? (
            <div
              className="rounded-lg p-6 text-center text-sm"
              style={{
                background: 'rgba(132, 142, 156, 0.08)',
                border: '1px solid #2B3139',
                color: '#848E9C',
              }}
            >
              {errorKind === 'system'
                ? t('decisionModal.systemClose', language)
                : errorKind === 'not_found'
                  ? t('decisionModal.notFound', language)
                  : t('decisionModal.fetchFailed', language)}
            </div>
          ) : decision ? (
            <div className="space-y-4">
              {errorKind === 'system' ? (
                <div
                  className="rounded-lg px-4 py-3 text-xs"
                  style={{
                    background: 'rgba(240, 185, 11, 0.1)',
                    border: '1px solid rgba(240, 185, 11, 0.35)',
                    color: '#F0B90B',
                  }}
                >
                  {t('decisionModal.systemClose', language)}
                </div>
              ) : null}
              <DecisionCard
                decision={decision}
                language={language}
                highlightActionKey={highlightKey}
              />
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )

  return createPortal(modal, document.body)
}
