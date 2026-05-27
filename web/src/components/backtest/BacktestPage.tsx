import { useEffect, useMemo, useState, useCallback, useRef, useLayoutEffect, type FormEvent } from 'react'
import useSWR from 'swr'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Play,
  Pause,
  Square,
  Download,
  Trash2,
  TrendingUp,
  BarChart3,
  Brain,
  Target,
  AlertTriangle,
} from 'lucide-react'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { api } from '../../lib/api'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { confirmToast } from '../../lib/notify'
import type {
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  DecisionRecord,
  AIModel,
  Strategy,
  BacktestSavedConfig,
} from '../../types'
import {
  BacktestConfigForm,
  type WizardStep,
  type BacktestFormState,
} from './BacktestConfigForm'
import { BacktestRunList, getStateColor, getStateIcon } from './BacktestRunList'
import { StatCard, ProgressRing, PositionsDisplay } from './BacktestOverviewTab'
import { BacktestOverviewTab } from './BacktestOverviewTab'
import { BacktestChartTab } from './BacktestChartTab'
import { BacktestTradesTab } from './BacktestTradesTab'
import { BacktestDecisionsTab } from './BacktestDecisionsTab'

// ============ Types ============
type ViewTab = 'overview' | 'chart' | 'trades' | 'decisions' | 'compare'

const toLocalInput = (date: Date) => {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

// ============ Main Component ============
export function BacktestPage() {
  const { language } = useLanguage()
  const tr = useCallback(
    (key: string, params?: Record<string, string | number>) => t(`backtestPage.${key}`, language, params),
    [language]
  )

  // State
  const now = new Date()
  const [wizardStep, setWizardStep] = useState<WizardStep>(1)
  const [viewTab, setViewTab] = useState<ViewTab>('overview')
  const [selectedRunId, setSelectedRunId] = useState<string>()
  const [compareRunIds, setCompareRunIds] = useState<string[]>([])
  const [isStarting, setIsStarting] = useState(false)
  const [toast, setToast] = useState<{ text: string; tone: 'info' | 'error' | 'success' } | null>(null)
  const leftColumnRef = useRef<HTMLDivElement>(null)
  const [leftColumnHeight, setLeftColumnHeight] = useState<number>()

  useLayoutEffect(() => {
    const el = leftColumnRef.current
    if (!el) return
    const sync = () => setLeftColumnHeight(el.offsetHeight)
    sync()
    const ro = new ResizeObserver(sync)
    ro.observe(el)
    return () => ro.disconnect()
  }, [selectedRunId, wizardStep])

  // Form state
  const [formState, setFormState] = useState<BacktestFormState>({
    runId: '',
    symbols: 'BTCUSDT,ETHUSDT,SOLUSDT',
    timeframes: ['3m', '15m', '4h'],
    decisionTf: '3m',
    cadence: 20,
    start: toLocalInput(new Date(now.getTime() - 3 * 24 * 3600 * 1000)),
    end: toLocalInput(now),
    balance: 1000,
    fee: 5,
    slippage: 2,
    btcEthLeverage: 5,
    altcoinLeverage: 5,
    fill: 'next_open',
    prompt: 'baseline',
    promptTemplate: 'default',
    customPrompt: '',
    overridePrompt: false,
    cacheAI: true,
    replayOnly: false,
    aiModelId: '',
    strategyId: '',
  })

  // Data fetching
  const { data: runsResp, mutate: refreshRuns } = useSWR(['backtest-runs'], () =>
    api.getBacktestRuns({ limit: 100, offset: 0 })
    , { refreshInterval: 5000 })
  const runs = runsResp?.items ?? []

  const { data: aiModels } = useSWR<AIModel[]>('ai-models', api.getModelConfigs, { refreshInterval: 30000 })
  const { data: strategies } = useSWR<Strategy[]>('strategies', api.getStrategies, { refreshInterval: 30000 })

  const { data: status } = useSWR<BacktestStatusPayload>(
    selectedRunId ? ['bt-status', selectedRunId] : null,
    () => api.getBacktestStatus(selectedRunId!),
    { refreshInterval: 2000 }
  )

  const { data: equity } = useSWR<BacktestEquityPoint[]>(
    selectedRunId ? ['bt-equity', selectedRunId] : null,
    () => api.getBacktestEquity(selectedRunId!, '1m', 2000),
    { refreshInterval: 5000 }
  )

  const { data: trades } = useSWR<BacktestTradeEvent[]>(
    selectedRunId ? ['bt-trades', selectedRunId] : null,
    () => api.getBacktestTrades(selectedRunId!, 0),
    { refreshInterval: 5000 }
  )

  const { data: metrics } = useSWR<BacktestMetrics>(
    selectedRunId ? ['bt-metrics', selectedRunId] : null,
    () => api.getBacktestMetrics(selectedRunId!),
    { refreshInterval: 10000 }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    selectedRunId ? ['bt-decisions', selectedRunId] : null,
    () => api.getBacktestDecisions(selectedRunId!, 30),
    { refreshInterval: 2000 }
  )

  const selectedRun = runs.find((r) => r.run_id === selectedRunId)
  const selectedModel = aiModels?.find((m) => m.id === formState.aiModelId)
  const selectedStrategy = strategies?.find((s) => s.id === formState.strategyId)

  // Check if selected strategy has dynamic coin source (needed for handleStart)
  const strategyHasDynamicCoins = useMemo(() => {
    if (!selectedStrategy) return false
    const coinSource = selectedStrategy.config?.coin_source
    if (!coinSource) return false

    if (coinSource.source_type === 'ai500' || coinSource.source_type === 'oi_top') {
      return true
    }
    if ((coinSource.source_type as string) === 'mixed' && (coinSource.use_ai500 || coinSource.use_oi_top)) {
      return true
    }

    const srcType = coinSource.source_type as string
    if (!srcType && (coinSource.use_ai500 || coinSource.use_oi_top)) {
      return true
    }

    return false
  }, [selectedStrategy])

  // Auto-select first model
  useEffect(() => {
    if (!formState.aiModelId && aiModels?.length) {
      const enabled = aiModels.find((m) => m.enabled)
      if (enabled) setFormState((s) => ({ ...s, aiModelId: enabled.id }))
    }
  }, [aiModels, formState.aiModelId])

  // Auto-select first run
  useEffect(() => {
    if (!selectedRunId && runs.length > 0) {
      setSelectedRunId(runs[0].run_id)
    }
  }, [runs, selectedRunId])

  // Handlers
  const handleFormChange = (key: string, value: string | number | boolean | string[]) => {
    setFormState((prev) => {
      // Keep decision timeframe consistent with selected timeframes.
      // If user removes the current decisionTf from timeframes, fallback to the first timeframe.
      if (key === 'timeframes' && Array.isArray(value)) {
        const nextTfs = value.filter(Boolean)
        const nextDecisionTf = nextTfs.includes(prev.decisionTf) ? prev.decisionTf : (nextTfs[0] ?? prev.decisionTf)
        return { ...prev, timeframes: nextTfs, decisionTf: nextDecisionTf }
      }
      // If user changes decisionTf, ensure it is one of selected timeframes; otherwise fallback.
      if (key === 'decisionTf' && typeof value === 'string') {
        const next = value.trim()
        if (next && prev.timeframes.includes(next)) {
          return { ...prev, decisionTf: next }
        }
        return { ...prev, decisionTf: prev.timeframes[0] ?? prev.decisionTf }
      }
      return { ...prev, [key]: value }
    })
  }

  const handleStart = async (event: FormEvent) => {
    event.preventDefault()
    if (!selectedModel?.enabled) {
      setToast({ text: tr('toasts.selectModel'), tone: 'error' })
      return
    }

    try {
      setIsStarting(true)
      const start = new Date(formState.start).getTime()
      const end = new Date(formState.end).getTime()
      if (end <= start) throw new Error(tr('toasts.invalidRange'))

      const userSymbols = formState.symbols.split(',').map((s) => s.trim()).filter(Boolean)
      const symbolsToSend = (userSymbols.length === 0 && strategyHasDynamicCoins) ? [] : userSymbols

      const payload = await api.startBacktest({
        run_id: formState.runId.trim() || undefined,
        strategy_id: formState.strategyId || undefined,
        symbols: symbolsToSend,
        timeframes: formState.timeframes,
        decision_timeframe: formState.decisionTf,
        decision_cadence_nbars: formState.cadence,
        start_ts: Math.floor(start / 1000),
        end_ts: Math.floor(end / 1000),
        initial_balance: formState.balance,
        fee_bps: formState.fee,
        slippage_bps: formState.slippage,
        fill_policy: formState.fill,
        prompt_variant: formState.prompt,
        prompt_template: formState.promptTemplate,
        custom_prompt: formState.customPrompt.trim() || undefined,
        override_prompt: formState.overridePrompt,
        cache_ai: formState.cacheAI,
        replay_only: formState.replayOnly,
        ai_model_id: formState.aiModelId,
        leverage: {
          btc_eth_leverage: formState.btcEthLeverage,
          altcoin_leverage: formState.altcoinLeverage,
        },
      })

      setToast({ text: tr('toasts.startSuccess', { id: payload.run_id }), tone: 'success' })
      setSelectedRunId(payload.run_id)
      setWizardStep(1)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.startFailed')
      setToast({ text: errMsg, tone: 'error' })
    } finally {
      setIsStarting(false)
    }
  }

  const handleControl = async (action: 'pause' | 'resume' | 'stop') => {
    if (!selectedRunId) return
    try {
      if (action === 'pause') await api.pauseBacktest(selectedRunId)
      if (action === 'resume') await api.resumeBacktest(selectedRunId)
      if (action === 'stop') await api.stopBacktest(selectedRunId)
      setToast({ text: tr('toasts.actionSuccess', { action, id: selectedRunId }), tone: 'success' })
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.actionFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleDelete = async () => {
    if (!selectedRunId) return
    const confirmed = await confirmToast(tr('toasts.confirmDelete', { id: selectedRunId }), {
      title: t('backtestPageExtra.confirmDelete', language),
      okText: t('backtestPageExtra.delete', language),
      cancelText: t('backtestPageExtra.cancel', language),
    })
    if (!confirmed) return
    try {
      await api.deleteBacktestRun(selectedRunId)
      setToast({ text: tr('toasts.deleteSuccess'), tone: 'success' })
      setSelectedRunId(undefined)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.deleteFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleDeleteRun = async (runId: string) => {
    if (!runId) return
    const confirmed = await confirmToast(tr('toasts.confirmDelete', { id: runId }), {
      title: t('backtestPageExtra.confirmDelete', language),
      okText: t('backtestPageExtra.delete', language),
      cancelText: t('backtestPageExtra.cancel', language),
    })
    if (!confirmed) return
    try {
      await api.deleteBacktestRun(runId)
      if (selectedRunId === runId) {
        setSelectedRunId(undefined)
      }
      setToast({ text: tr('toasts.deleteSuccess'), tone: 'success' })
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.deleteFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleReuseRun = async (runId: string) => {
    try {
      const first = await confirmToast(
        language === 'zh'
          ? `将复用 ${runId} 的回测参数创建新回测，是否继续？`
          : `Reuse parameters from ${runId} to create a new backtest?`,
        {
          title: language === 'zh' ? '复用回测参数' : 'Reuse Backtest Config',
          okText: language === 'zh' ? '继续' : 'Continue',
          cancelText: language === 'zh' ? '取消' : 'Cancel',
        }
      )
      if (!first) return

      const second = await confirmToast(
        language === 'zh'
          ? '请再次确认：将立即启动一个新的回测任务。'
          : 'Please confirm again: a new backtest run will start immediately.',
        {
          title: language === 'zh' ? '二次确认' : 'Final Confirmation',
          okText: language === 'zh' ? '确认启动' : 'Start New Run',
          cancelText: language === 'zh' ? '取消' : 'Cancel',
        }
      )
      if (!second) return

      const saved = await api.getBacktestConfig(runId)
      const next: BacktestSavedConfig = {
        ...saved,
      }
      // Always create a new run id.
      delete next.run_id
      delete next.user_id

      setIsStarting(true)
      const payload = await api.startBacktest(next)
      setToast({ text: tr('toasts.startSuccess', { id: payload.run_id }), tone: 'success' })
      setSelectedRunId(payload.run_id)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.startFailed')
      setToast({ text: errMsg, tone: 'error' })
    } finally {
      setIsStarting(false)
    }
  }

  const handleExport = async () => {
    if (!selectedRunId) return
    try {
      const blob = await api.exportBacktest(selectedRunId)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${selectedRunId}_export.zip`
      link.click()
      URL.revokeObjectURL(url)
      setToast({ text: tr('toasts.exportSuccess', { id: selectedRunId }), tone: 'success' })
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.exportFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const toggleCompare = (runId: string) => {
    setCompareRunIds((prev) =>
      prev.includes(runId) ? prev.filter((id) => id !== runId) : [...prev, runId].slice(-3)
    )
  }

  // Render
  return (
    <DeepVoidBackground className="py-8" disableAnimation>
      <div className="w-full px-4 md:px-8 space-y-6">
        {/* Toast */}
        <AnimatePresence>
          {toast && (
            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              className="p-3 rounded-lg text-sm"
              style={{
                background:
                  toast.tone === 'error'
                    ? 'rgba(246,70,93,0.15)'
                    : toast.tone === 'success'
                      ? 'rgba(14,203,129,0.15)'
                      : 'rgba(240,185,11,0.15)',
                color: toast.tone === 'error' ? '#F6465D' : toast.tone === 'success' ? '#0ECB81' : '#F0B90B',
                border: `1px solid ${toast.tone === 'error' ? 'rgba(246,70,93,0.3)' : toast.tone === 'success' ? 'rgba(14,203,129,0.3)' : 'rgba(240,185,11,0.3)'}`,
              }}
            >
              {toast.text}
            </motion.div>
          )}
        </AnimatePresence>

        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold flex items-center gap-3" style={{ color: '#EAECEF' }}>
              <Brain className="w-7 h-7" style={{ color: '#F0B90B' }} />
              {tr('title')}
            </h1>
            <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
              {tr('subtitle')}
            </p>
          </div>
          <button
            onClick={() => setWizardStep(1)}
            className="px-4 py-2 rounded-lg font-medium flex items-center gap-2 transition-all hover:opacity-90"
            style={{ background: '#F0B90B', color: '#0B0E11' }}
          >
            <Play className="w-4 h-4" />
            {t('backtestPageExtra.newBacktest', language)}
          </button>
        </div>

        <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 xl:items-stretch">
          {/* Left: config + run list (run list keeps max-h-[300px]) */}
          <div ref={leftColumnRef} className="flex flex-col gap-4 min-h-0">
            <BacktestConfigForm
              formState={formState}
              wizardStep={wizardStep}
              isStarting={isStarting}
              aiModels={aiModels}
              strategies={strategies}
              language={language}
              tr={tr}
              onFormChange={handleFormChange}
              onWizardStepChange={setWizardStep}
              onStart={handleStart}
            />

            <BacktestRunList
              runs={runs}
              selectedRunId={selectedRunId}
              compareRunIds={compareRunIds}
              language={language}
              tr={tr}
              onSelectRun={setSelectedRunId}
              onToggleCompare={toggleCompare}
              onReuseRun={handleReuseRun}
              onDeleteRun={handleDeleteRun}
            />
          </div>

          {/* Right: status + stats + tab card — column height matches left; tab body scrolls */}
          <div
            className="xl:col-span-2 flex flex-col gap-4 min-h-0 overflow-hidden"
            style={
              leftColumnHeight != null && leftColumnHeight > 0
                ? { height: leftColumnHeight, maxHeight: leftColumnHeight }
                : undefined
            }
          >
            {!selectedRunId ? (
              <div
                className="binance-card flex-1 min-h-0 flex items-center justify-center p-12"
                style={{ color: '#5E6673' }}
              >
                <div className="text-center">
                  <Brain className="w-12 h-12 mx-auto mb-4 opacity-30" />
                  <p>{tr('emptyStates.selectRun')}</p>
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-4 flex-1 min-h-0 h-full">
                <div className="binance-card p-4 shrink-0">
                  <div className="flex flex-wrap items-center justify-between gap-4">
                    <div className="flex items-center gap-4">
                      <ProgressRing progress={status?.progress_pct ?? selectedRun?.summary.progress_pct ?? 0} size={80} />
                      <div>
                        <h2 className="font-mono font-bold" style={{ color: '#EAECEF' }}>
                          {selectedRunId}
                        </h2>
                        <div className="flex items-center gap-2 mt-1">
                          <span
                            className="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium"
                            style={{
                              background: `${getStateColor(status?.state ?? selectedRun?.state ?? '')}20`,
                              color: getStateColor(status?.state ?? selectedRun?.state ?? ''),
                            }}
                          >
                            {getStateIcon(status?.state ?? selectedRun?.state ?? '')}
                            {tr(`states.${status?.state ?? selectedRun?.state}`)}
                          </span>
                          {selectedRun?.summary.decision_tf && (
                            <span className="text-xs" style={{ color: '#848E9C' }}>
                              {selectedRun.summary.decision_tf} · {selectedRun.summary.symbol_count} symbols
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      {(status?.state === 'running' || selectedRun?.state === 'running') && (
                        <>
                          <button
                            onClick={() => handleControl('pause')}
                            className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                            style={{ border: '1px solid #2B3139' }}
                            title={tr('actions.pause')}
                          >
                            <Pause className="w-4 h-4" style={{ color: '#F0B90B' }} />
                          </button>
                          <button
                            onClick={() => handleControl('stop')}
                            className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                            style={{ border: '1px solid #2B3139' }}
                            title={tr('actions.stop')}
                          >
                            <Square className="w-4 h-4" style={{ color: '#F6465D' }} />
                          </button>
                        </>
                      )}
                      {status?.state === 'paused' && (
                        <button
                          onClick={() => handleControl('resume')}
                          className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                          style={{ border: '1px solid #2B3139' }}
                          title={tr('actions.resume')}
                        >
                          <Play className="w-4 h-4" style={{ color: '#0ECB81' }} />
                        </button>
                      )}
                      <button
                        onClick={handleExport}
                        className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                        style={{ border: '1px solid #2B3139' }}
                        title={tr('detail.exportLabel')}
                      >
                        <Download className="w-4 h-4" style={{ color: '#EAECEF' }} />
                      </button>
                      <button
                        onClick={handleDelete}
                        className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                        style={{ border: '1px solid #2B3139' }}
                        title={tr('detail.deleteLabel')}
                      >
                        <Trash2 className="w-4 h-4" style={{ color: '#F6465D' }} />
                      </button>
                    </div>
                  </div>

                  {(status?.note || status?.last_error) && (
                    <div
                      className="mt-3 p-2 rounded-lg text-xs flex items-center gap-2"
                      style={{
                        background: 'rgba(246,70,93,0.1)',
                        border: '1px solid rgba(246,70,93,0.3)',
                        color: '#F6465D',
                      }}
                    >
                      <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                      {status?.note || status?.last_error}
                    </div>
                  )}

                  {status?.positions && status.positions.length > 0 && (
                    <PositionsDisplay
                      positions={status.positions}
                      language={language}
                      currentTimeMs={status.current_time}
                      decisionTf={status.decision_tf}
                    />
                  )}
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 shrink-0">
                  <StatCard
                    icon={Target}
                    label={t('backtestPageExtra.equity', language)}
                    value={(status?.equity ?? 0).toFixed(2)}
                    suffix="USDT"
                    language={language}
                  />
                  <StatCard
                    icon={TrendingUp}
                    label={t('backtestPageExtra.totalReturn', language)}
                    value={`${(metrics?.total_return_pct ?? 0).toFixed(2)}%`}
                    trend={(metrics?.total_return_pct ?? 0) >= 0 ? 'up' : 'down'}
                    color={(metrics?.total_return_pct ?? 0) >= 0 ? '#0ECB81' : '#F6465D'}
                    metricKey="total_return"
                    language={language}
                  />
                  <StatCard
                    icon={AlertTriangle}
                    label={t('backtestPageExtra.maxDD', language)}
                    value={`${(metrics?.max_drawdown_pct ?? 0).toFixed(2)}%`}
                    color="#F6465D"
                    metricKey="max_drawdown"
                    language={language}
                  />
                  <StatCard
                    icon={BarChart3}
                    label={t('backtestPageExtra.sharpe', language)}
                    value={(metrics?.sharpe_ratio ?? 0).toFixed(2)}
                    metricKey="sharpe_ratio"
                    language={language}
                  />
                </div>

                <div className="binance-card flex flex-col flex-1 min-h-0 overflow-hidden">
                  <div className="flex border-b shrink-0" style={{ borderColor: '#2B3139' }}>
                    {(['overview', 'chart', 'trades', 'decisions'] as ViewTab[]).map((tab) => (
                      <button
                        key={tab}
                        onClick={() => setViewTab(tab)}
                        className="px-4 py-3 text-sm font-medium transition-all relative"
                        style={{ color: viewTab === tab ? '#F0B90B' : '#848E9C' }}
                      >
                        {tab === 'overview'
                          ? t('backtestPageExtra.tabOverview', language)
                          : tab === 'chart'
                            ? t('backtestPageExtra.tabChart', language)
                            : tab === 'trades'
                              ? t('backtestPageExtra.tabTrades', language)
                              : t('backtestPageExtra.tabDecisions', language)}
                        {viewTab === tab && (
                          <motion.div
                            layoutId="tab-indicator"
                            className="absolute bottom-0 left-0 right-0 h-0.5"
                            style={{ background: '#F0B90B' }}
                          />
                        )}
                      </button>
                    ))}
                  </div>

                  <div
                    className={`flex-1 min-h-0 p-4 flex flex-col ${
                      viewTab === 'trades' ? 'overflow-hidden' : 'overflow-y-auto overscroll-contain'
                    }`}
                  >
                    <div
                      className={`flex-1 min-h-0 flex flex-col ${
                        viewTab === 'trades' ? 'relative overflow-hidden' : ''
                      }`}
                    >
                    <AnimatePresence mode="wait">
                      {viewTab === 'overview' && (
                        <BacktestOverviewTab
                          equity={equity}
                          trades={trades}
                          metrics={metrics}
                          language={language}
                          tr={tr}
                        />
                      )}

                      {viewTab === 'chart' && (
                        <BacktestChartTab
                          equity={equity}
                          trades={trades}
                          selectedRunId={selectedRunId}
                          language={language}
                          tr={tr}
                        />
                      )}

                      {viewTab === 'trades' && (
                        <BacktestTradesTab
                          key={selectedRunId}
                          runId={selectedRunId}
                          trades={trades}
                          className="absolute inset-0"
                        />
                      )}

                      {viewTab === 'decisions' && (
                        <BacktestDecisionsTab
                          decisions={decisions}
                          livePositions={status?.positions}
                          language={language}
                          tr={tr}
                        />
                      )}
                    </AnimatePresence>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
