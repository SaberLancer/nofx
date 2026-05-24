import { ScanLine, Info, ToggleLeft, ToggleRight } from 'lucide-react'
import type { PreDecisionConfig } from '../../types'
import { defaultPreDecisionConfig } from '../../types/strategy'
import { preDecision, ts } from '../../i18n/strategy-translations'

interface PreDecisionEditorProps {
  config?: PreDecisionConfig
  onChange: (config: PreDecisionConfig) => void
  disabled?: boolean
  language: string
}

export function PreDecisionEditor({
  config,
  onChange,
  disabled,
  language,
}: PreDecisionEditorProps) {
  const values: PreDecisionConfig = {
    ...defaultPreDecisionConfig,
    ...config,
  }

  const updateField = <K extends keyof PreDecisionConfig>(
    key: K,
    value: PreDecisionConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...values, [key]: value })
    }
  }

  const pct = (ratio: number) => `${Math.round(ratio * 100)}%`

  return (
    <div className="space-y-6">
      <div className="p-4 rounded-lg" style={{ background: 'rgba(96, 165, 250, 0.08)', border: '1px solid rgba(96, 165, 250, 0.25)' }}>
        <div className="flex items-start gap-2">
          <Info className="w-4 h-4 mt-0.5 shrink-0" style={{ color: '#60a5fa' }} />
          <p className="text-xs leading-relaxed" style={{ color: '#848E9C' }}>
            {ts(preDecision.desc, language)}
          </p>
        </div>
      </div>

      <div className="flex items-center justify-between p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
        <div>
          <div className="flex items-center gap-2 mb-1">
            <ScanLine className="w-4 h-4" style={{ color: '#60a5fa' }} />
            <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              {ts(preDecision.enabled, language)}
            </span>
          </div>
          <p className="text-xs" style={{ color: '#848E9C' }}>
            {ts(preDecision.enabledDesc, language)}
          </p>
        </div>
        <button
          type="button"
          disabled={disabled}
          onClick={() => updateField('enabled', !values.enabled)}
          className="shrink-0"
          aria-pressed={values.enabled}
        >
          {values.enabled ? (
            <ToggleRight className="w-10 h-10" style={{ color: '#0ECB81' }} />
          ) : (
            <ToggleLeft className="w-10 h-10" style={{ color: '#848E9C' }} />
          )}
        </button>
      </div>

      {values.enabled && (
        <>
          <div className="p-3 rounded-lg text-xs" style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#848E9C' }}>
            <span className="font-medium" style={{ color: '#60a5fa' }}>
              {ts(preDecision.signalLogic, language)}：
            </span>{' '}
            {ts(preDecision.signalLogicDesc, language)}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <NumberField
              label={ts(preDecision.pollInterval, language)}
              desc={ts(preDecision.pollIntervalDesc, language)}
              value={values.poll_interval_sec ?? 5}
              onChange={(v) => updateField('poll_interval_sec', v)}
              disabled={disabled}
              min={1}
              max={60}
              suffix="s"
            />
            <NumberField
              label={ts(preDecision.windowSec, language)}
              desc={ts(preDecision.windowSecDesc, language)}
              value={values.window_sec ?? 60}
              onChange={(v) => updateField('window_sec', v)}
              disabled={disabled}
              min={15}
              max={600}
              suffix="s"
            />
            <NumberField
              label={ts(preDecision.minTicks, language)}
              desc={ts(preDecision.minTicksDesc, language)}
              value={values.min_ticks ?? 20}
              onChange={(v) => updateField('min_ticks', v)}
              disabled={disabled}
              min={5}
              max={200}
            />
            <NumberField
              label={ts(preDecision.minMomentumPct, language)}
              desc={ts(preDecision.minMomentumPctDesc, language)}
              value={values.min_momentum_pct ?? 0.03}
              onChange={(v) => updateField('min_momentum_pct', v)}
              disabled={disabled}
              min={0.01}
              max={1}
              step={0.01}
              suffix="%"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <SliderField
              label={ts(preDecision.minBuyPressure, language)}
              desc={ts(preDecision.minBuyPressureDesc, language)}
              value={values.min_buy_pressure ?? 0.55}
              onChange={(v) => updateField('min_buy_pressure', v)}
              disabled={disabled}
              min={0.5}
              max={0.8}
              step={0.01}
              display={pct(values.min_buy_pressure ?? 0.55)}
              accent="#0ECB81"
            />
            <SliderField
              label={ts(preDecision.minSellPressure, language)}
              desc={ts(preDecision.minSellPressureDesc, language)}
              value={values.min_sell_pressure ?? 0.55}
              onChange={(v) => updateField('min_sell_pressure', v)}
              disabled={disabled}
              min={0.5}
              max={0.8}
              step={0.01}
              display={pct(values.min_sell_pressure ?? 0.55)}
              accent="#F6465D"
            />
          </div>

          <div className="flex items-center justify-between p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <div>
              <span className="text-sm font-medium block mb-1" style={{ color: '#EAECEF' }}>
                {ts(preDecision.alwaysWhenPositions, language)}
              </span>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {ts(preDecision.alwaysWhenPositionsDesc, language)}
              </p>
            </div>
            <button
              type="button"
              disabled={disabled}
              onClick={() => updateField('always_when_positions', !values.always_when_positions)}
            >
              {values.always_when_positions ? (
                <ToggleRight className="w-10 h-10" style={{ color: '#0ECB81' }} />
              ) : (
                <ToggleLeft className="w-10 h-10" style={{ color: '#848E9C' }} />
              )}
            </button>
          </div>
        </>
      )}
    </div>
  )
}

function NumberField({
  label,
  desc,
  value,
  onChange,
  disabled,
  min,
  max,
  step = 1,
  suffix,
}: {
  label: string
  desc: string
  value: number
  onChange: (value: number) => void
  disabled?: boolean
  min: number
  max: number
  step?: number
  suffix?: string
}) {
  return (
    <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
      <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
        {label}
      </label>
      <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
        {desc}
      </p>
      <div className="flex items-center gap-2">
        <input
          type="number"
          value={value}
          onChange={(e) => {
            const parsed = parseFloat(e.target.value)
            if (!Number.isNaN(parsed)) onChange(Math.min(max, Math.max(min, parsed)))
          }}
          disabled={disabled}
          min={min}
          max={max}
          step={step}
          className="w-full px-3 py-2 rounded"
          style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
        />
        {suffix && (
          <span className="text-xs shrink-0" style={{ color: '#848E9C' }}>
            {suffix}
          </span>
        )}
      </div>
    </div>
  )
}

function SliderField({
  label,
  desc,
  value,
  onChange,
  disabled,
  min,
  max,
  step,
  display,
  accent,
}: {
  label: string
  desc: string
  value: number
  onChange: (value: number) => void
  disabled?: boolean
  min: number
  max: number
  step: number
  display: string
  accent: string
}) {
  return (
    <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
      <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
        {label}
      </label>
      <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
        {desc}
      </p>
      <div className="flex items-center gap-2">
        <input
          type="range"
          value={value}
          onChange={(e) => onChange(parseFloat(e.target.value))}
          disabled={disabled}
          min={min}
          max={max}
          step={step}
          className="flex-1"
          style={{ accentColor: accent }}
        />
        <span className="w-12 text-center font-mono text-sm" style={{ color: accent }}>
          {display}
        </span>
      </div>
    </div>
  )
}
