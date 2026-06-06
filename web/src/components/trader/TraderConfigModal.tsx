import { useState, useEffect } from 'react'
import type { AIModel, Exchange, CreateTraderRequest, Strategy, TraderConfigData, RegimeDetectionConfig } from '../../types'
import { DEFAULT_REGIME_DETECTION } from '../../types/trading'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { Pencil, Plus, X as IconX, Sparkles, ExternalLink, UserPlus, GitBranch } from 'lucide-react'
import { httpClient } from '../../lib/httpClient'
import { NofxSelect } from '../ui/select'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

function getStrategyAIConfig(strategy: Strategy) {
  return strategy.config.ai_config || (
    strategy.config.coin_source && strategy.config.risk_control
      ? {
          coin_source: strategy.config.coin_source,
          risk_control: strategy.config.risk_control,
        }
      : null
  )
}

// 交易所注册链接配置
const STRUCTURE_TIMEFRAMES = ['5m', '15m', '30m', '1h']

function mergeRegimeDetection(raw?: Partial<RegimeDetectionConfig>): RegimeDetectionConfig {
  const base = DEFAULT_REGIME_DETECTION
  if (!raw) return { ...base, layer_1h: { ...base.layer_1h }, layer_15m: { ...base.layer_15m }, layer_3m: { ...base.layer_3m } }
  return {
    layer_1h: { ...base.layer_1h, ...raw.layer_1h },
    layer_15m: { ...base.layer_15m, ...raw.layer_15m },
    layer_3m: { ...base.layer_3m, ...raw.layer_3m },
  }
}

function RegimeNumField({
  label,
  hint,
  value,
  onChange,
  min,
  max,
  step = 1,
}: {
  label: string
  hint?: string
  value: number
  onChange: (v: number) => void
  min?: number
  max?: number
  step?: number
}) {
  return (
    <div>
      <label className="text-xs text-[#EAECEF] block mb-1">{label}</label>
      <input
        type="number"
        value={value}
        onChange={(e) => {
          const v = Number(e.target.value)
          if (Number.isFinite(v)) onChange(v)
        }}
        className="w-full px-2 py-1.5 text-sm bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
        min={min}
        max={max}
        step={step}
      />
      {hint && <p className="text-[10px] text-[#848E9C] mt-1">{hint}</p>}
    </div>
  )
}

const EXCHANGE_REGISTRATION_LINKS: Record<string, { url: string; hasReferral?: boolean }> = {
  binance: { url: 'https://www.binance.com/join?ref=NOFXENG', hasReferral: true },
  okx: { url: 'https://www.okx.com/join/1865360', hasReferral: true },
  bybit: { url: 'https://partner.bybit.com/b/83856', hasReferral: true },
  hyperliquid: { url: 'https://app.hyperliquid.xyz/join/AITRADING', hasReferral: true },
  aster: { url: 'https://www.asterdex.com/en/referral/fdfc0e', hasReferral: true },
  lighter: { url: 'https://app.lighter.xyz/?referral=68151432', hasReferral: true },
}
// 表单内部状态类型
interface FormState {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id: string
  regime_switch_enabled: boolean
  trend_strategy_id: string
  oscillation_strategy_id: string
  regime_confirm_cycles: number
  regime_detection: RegimeDetectionConfig
  is_cross_margin: boolean
  show_in_competition: boolean
  scan_interval_minutes: number
}

function findStrategyByNameHint(strategies: Strategy[], hints: string[]): string {
  for (const hint of hints) {
    const match = strategies.find((s) => s.name.includes(hint))
    if (match) return match.id
  }
  return ''
}

interface TraderConfigModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
  isEditMode?: boolean
  availableModels?: AIModel[]
  availableExchanges?: Exchange[]
  onSave?: (data: CreateTraderRequest) => Promise<void>
}

export function TraderConfigModal({
  isOpen,
  onClose,
  traderData,
  isEditMode = false,
  availableModels = [],
  availableExchanges = [],
  onSave,
}: TraderConfigModalProps) {
  const { language } = useLanguage()
  const [formData, setFormData] = useState<FormState>({
    trader_name: '',
    ai_model: '',
    exchange_id: '',
    strategy_id: '',
    regime_switch_enabled: false,
    trend_strategy_id: '',
    oscillation_strategy_id: '',
    regime_confirm_cycles: 2,
    regime_detection: { ...DEFAULT_REGIME_DETECTION },
    is_cross_margin: true,
    show_in_competition: true,
    scan_interval_minutes: 3,
  })
  const [isSaving, setIsSaving] = useState(false)

  const updateLayer1H = <K extends keyof RegimeDetectionConfig['layer_1h']>(
    key: K,
    value: RegimeDetectionConfig['layer_1h'][K]
  ) => {
    setFormData((prev) => ({
      ...prev,
      regime_detection: {
        ...prev.regime_detection,
        layer_1h: { ...prev.regime_detection.layer_1h, [key]: value },
      },
    }))
  }
  const updateLayer15m = <K extends keyof RegimeDetectionConfig['layer_15m']>(
    key: K,
    value: RegimeDetectionConfig['layer_15m'][K]
  ) => {
    setFormData((prev) => ({
      ...prev,
      regime_detection: {
        ...prev.regime_detection,
        layer_15m: { ...prev.regime_detection.layer_15m, [key]: value },
      },
    }))
  }
  const updateLayer3m = <K extends keyof RegimeDetectionConfig['layer_3m']>(
    key: K,
    value: RegimeDetectionConfig['layer_3m'][K]
  ) => {
    setFormData((prev) => ({
      ...prev,
      regime_detection: {
        ...prev.regime_detection,
        layer_3m: { ...prev.regime_detection.layer_3m, [key]: value },
      },
    }))
  }
  const [strategies, setStrategies] = useState<Strategy[]>([])

  // 获取用户的策略列表
  useEffect(() => {
    const fetchStrategies = async () => {
      try {
        const result = await httpClient.get<{ strategies: Strategy[] }>('/api/strategies')
        if (result.success && result.data?.strategies) {
          const strategyList = result.data.strategies
          setStrategies(strategyList)
          setFormData((prev) => {
            const next = { ...prev }
            if (!next.strategy_id && !isEditMode) {
              const activeStrategy = strategyList.find((s) => s.is_active)
              next.strategy_id = activeStrategy?.id || strategyList[0]?.id || ''
            }
            if (next.regime_switch_enabled) {
              if (!next.trend_strategy_id) {
                next.trend_strategy_id = findStrategyByNameHint(strategyList, ['短线', 'Short-Term', '波段'])
              }
              if (!next.oscillation_strategy_id) {
                next.oscillation_strategy_id = findStrategyByNameHint(strategyList, ['震荡', 'Oscillation', '高抛低吸'])
              }
              if (next.trend_strategy_id && !next.strategy_id) {
                next.strategy_id = next.trend_strategy_id
              }
            }
            return next
          })
        }
      } catch (error) {
        console.error('Failed to fetch strategies:', error)
      }
    }
    if (isOpen) {
      fetchStrategies()
    }
  }, [isOpen])

  useEffect(() => {
    if (traderData) {
      setFormData({
        ...traderData,
        strategy_id: traderData.strategy_id || '',
        regime_switch_enabled: traderData.regime_switch_enabled ?? false,
        trend_strategy_id: traderData.trend_strategy_id || '',
        oscillation_strategy_id: traderData.oscillation_strategy_id || '',
        regime_confirm_cycles: traderData.regime_confirm_cycles || 2,
        regime_detection: mergeRegimeDetection(traderData.regime_detection),
      })
    } else if (!isEditMode) {
      setFormData({
        trader_name: '',
        ai_model: availableModels[0]?.id || '',
        exchange_id: availableExchanges[0]?.id || '',
        strategy_id: '',
        regime_switch_enabled: false,
        trend_strategy_id: '',
        oscillation_strategy_id: '',
        regime_confirm_cycles: 2,
        regime_detection: { ...DEFAULT_REGIME_DETECTION },
        is_cross_margin: true,
        show_in_competition: true,
        scan_interval_minutes: 3,
      })
    }
  }, [traderData, isEditMode, availableModels, availableExchanges])

  if (!isOpen) return null

  const handleInputChange = (field: keyof FormState, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleExchangeChange = (exchangeId: string) => {
    setFormData((prev) => ({ ...prev, exchange_id: exchangeId }))
  }

  const handleSave = async () => {
    if (!onSave) return

    setIsSaving(true)
    try {
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        strategy_id: formData.regime_switch_enabled
          ? (formData.trend_strategy_id || formData.strategy_id)
          : formData.strategy_id,
        regime_switch_enabled: formData.regime_switch_enabled,
        trend_strategy_id: formData.trend_strategy_id,
        oscillation_strategy_id: formData.oscillation_strategy_id,
        regime_confirm_cycles: formData.regime_confirm_cycles,
        regime_detection: formData.regime_detection,
        is_cross_margin: formData.is_cross_margin,
        show_in_competition: formData.show_in_competition,
        scan_interval_minutes: formData.scan_interval_minutes,
      }

      await onSave(saveData)
    } catch (error) {
       console.error(t('saveFailed', language) + ':', error)
    } finally {
      setIsSaving(false)
    }
  }

  const selectedStrategy = strategies.find(s => s.id === formData.strategy_id)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 backdrop-blur-sm p-4 overflow-y-auto">
      <div
        className="bg-[#1E2329] border border-[#2B3139] rounded-xl shadow-2xl max-w-2xl w-full my-8"
        style={{ maxHeight: 'calc(100vh - 4rem)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky top-0 z-10 rounded-t-xl">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#F0B90B] to-[#E1A706] flex items-center justify-center text-black">
              {isEditMode ? (
                <Pencil className="w-5 h-5" />
              ) : (
                <Plus className="w-5 h-5" />
              )}
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {isEditMode ? t('editTrader', language) : t('createTrader', language)}
              </h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {isEditMode ? t('editTraderConfig', language) : t('selectStrategyAndConfigParams', language)}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center"
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div
          className="p-6 space-y-6 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Basic Info */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              <span className="text-[#F0B90B]">1</span> {t('basicConfig', language)}
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('traderNameRequired', language)}
                </label>
                <input
                  type="text"
                  value={formData.trader_name}
                  onChange={(e) =>
                    handleInputChange('trader_name', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                   placeholder={t('enterTraderNamePlaceholder', language)}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('aiModelRequired', language)}
                  </label>
                  <NofxSelect
                    value={formData.ai_model}
                    onChange={(val) =>
                      handleInputChange('ai_model', val)
                    }
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                    options={availableModels.map((model) => ({
                      value: model.id,
                      label: getShortName(model.name || model.id).toUpperCase(),
                    }))}
                  />
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('exchangeRequired', language)}
                  </label>
                  <NofxSelect
                    value={formData.exchange_id}
                    onChange={handleExchangeChange}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                    options={availableExchanges.map((exchange) => ({
                      value: exchange.id,
                      label: getShortName(exchange.name || exchange.exchange_type || exchange.id).toUpperCase()
                        + (exchange.account_name ? ` - ${exchange.account_name}` : ''),
                    }))}
                  />
                  {/* Exchange Registration Link */}
                  {formData.exchange_id && (() => {
                    // Find the selected exchange to get its type
                    const selectedExchange = availableExchanges.find(e => e.id === formData.exchange_id)
                    const exchangeType = selectedExchange?.exchange_type?.toLowerCase() || ''
                    const regLink = EXCHANGE_REGISTRATION_LINKS[exchangeType]
                    if (!regLink) return null
                    return (
                      <a
                        href={regLink.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="mt-2 inline-flex items-center gap-1.5 text-xs text-[#848E9C] hover:text-[#F0B90B] transition-colors"
                      >
                        <UserPlus className="w-3.5 h-3.5" />
                        <span>{t('noExchangeAccount', language)}</span>
                        {regLink.hasReferral && (
                          <span className="px-1.5 py-0.5 bg-[#F0B90B]/10 text-[#F0B90B] rounded text-[10px]">
                            {t('discount', language)}
                          </span>
                        )}
                        <ExternalLink className="w-3 h-3" />
                      </a>
                    )
                  })()}
                </div>
              </div>
            </div>
          </div>

          {/* Strategy Selection */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              <span className="text-[#F0B90B]">2</span> {t('selectTradingStrategy', language)}
              <Sparkles className="w-4 h-4 text-[#F0B90B]" />
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('useStrategy', language)}
                </label>
                <NofxSelect
                  value={formData.strategy_id}
                  onChange={(val) =>
                    handleInputChange('strategy_id', val)
                  }
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                  options={[
                    { value: '', label: t('noStrategyManual', language) },
                    ...strategies.map((strategy) => ({
                      value: strategy.id,
                      label: strategy.name + (strategy.is_active ? t('strategyActive', language) : '') + (strategy.is_default ? t('strategyDefault', language) : ''),
                    })),
                  ]}
                />
                {strategies.length === 0 && (
                    <p className="text-xs text-[#848E9C] mt-2">
                      {t('noStrategyHint', language)}
                  </p>
                )}
              </div>

              {/* Strategy Preview */}
              {selectedStrategy && (
                <div className="mt-3 p-4 bg-[#1E2329] border border-[#2B3139] rounded-lg">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-[#F0B90B] text-sm font-medium">
                      {t('strategyDetails', language)}
                    </span>
                    {selectedStrategy.is_active && (
                      <span className="px-2 py-0.5 bg-green-500/20 text-green-400 text-xs rounded">
                        {t('activating', language)}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-[#848E9C] mb-2">
                    {selectedStrategy.description || (language === 'zh' ? '无描述' : 'No description')}
                  </p>
                  {selectedStrategy.config.strategy_type === 'grid_trading' && selectedStrategy.config.grid_config ? (
                    <div className="grid grid-cols-2 gap-2 text-xs text-[#848E9C]">
                      <div>{language === 'zh' ? '交易对' : 'Symbol'}: {selectedStrategy.config.grid_config.symbol || '-'}</div>
                      <div>{language === 'zh' ? '网格数' : 'Grids'}: {selectedStrategy.config.grid_config.grid_count}</div>
                    </div>
                  ) : (() => {
                    const aiConfig = getStrategyAIConfig(selectedStrategy)
                    if (!aiConfig) return null
                    return (
                      <div className="grid grid-cols-2 gap-2 text-xs text-[#848E9C]">
                        <div>
                          {t('coinSource', language)}: {aiConfig.coin_source.source_type === 'static' ? '固定币种' :
                            aiConfig.coin_source.source_type === 'ai500' ? 'AI500' :
                            aiConfig.coin_source.source_type === 'oi_top' ? 'OI Top' :
                            aiConfig.coin_source.source_type === 'oi_low' ? 'OI Low' : '-'}
                        </div>
                        <div>
                          {t('marginLimit', language)}: {((aiConfig.risk_control?.max_margin_usage || 0.9) * 100).toFixed(0)}%
                        </div>
                      </div>
                    )
                  })()}
                </div>
              )}
            </div>
          </div>

          {/* Trading Parameters */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              <span className="text-[#F0B90B]">3</span> {t('tradingParams', language)}
            </h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('marginMode', language)}
                  </label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', true)}
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        formData.is_cross_margin
                          ? 'bg-[#F0B90B] text-black'
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
                    >
                      {t('crossMargin', language)}
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        handleInputChange('is_cross_margin', false)
                      }
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        !formData.is_cross_margin
                          ? 'bg-[#F0B90B] text-black'
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
                    >
                      {t('isolatedMargin', language)}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('aiScanInterval', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.scan_interval_minutes}
                    onChange={(e) => {
                      const parsedValue = Number(e.target.value)
                      const safeValue = Number.isFinite(parsedValue)
                        ? Math.max(3, parsedValue)
                        : 3
                      handleInputChange('scan_interval_minutes', safeValue)
                    }}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                    min="3"
                    max="60"
                    step="1"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    {t('scanIntervalRecommend', language)}
                  </p>
                </div>
              </div>

              {/* Competition visibility */}
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('competitionDisplay', language)}
                </label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => handleInputChange('show_in_competition', true)}
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      formData.show_in_competition
                        ? 'bg-[#F0B90B] text-black'
                        : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                    }`}
                  >
                    {t('show', language)}
                  </button>
                  <button
                    type="button"
                    onClick={() => handleInputChange('show_in_competition', false)}
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      !formData.show_in_competition
                        ? 'bg-[#F0B90B] text-black'
                        : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                    }`}
                  >
                    {t('hide', language)}
                  </button>
                </div>
                  <p className="text-xs text-[#848E9C] mt-1">
                    {t('hiddenInCompetition', language)}
                </p>
              </div>

              <div className="p-3 bg-[#1E2329] border border-[#2B3139] rounded flex items-center gap-2">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="w-4 h-4 text-[#F0B90B]"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" x2="12" y1="8" y2="12" />
                  <line x1="12" x2="12.01" y1="16" y2="16" />
                </svg>
                <span className="text-sm text-[#848E9C]">
                  {t('autoFetchBalanceInfo', language)}
                </span>
              </div>

            </div>
          </div>

          {/* Step 4: Regime-based auto strategy switching */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              <span className="text-[#F0B90B]">4</span>
              {language === 'zh' ? '市场状态自动切换策略' : 'Auto Strategy by Market Regime'}
              <GitBranch className="w-4 h-4 text-[#F0B90B]" />
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {language === 'zh' ? '启用自动切换' : 'Enable auto switch'}
                </label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setFormData((prev) => {
                        const next = { ...prev, regime_switch_enabled: true }
                        if (!next.trend_strategy_id) {
                          next.trend_strategy_id = findStrategyByNameHint(strategies, ['短线', 'Short-Term', '波段'])
                        }
                        if (!next.oscillation_strategy_id) {
                          next.oscillation_strategy_id = findStrategyByNameHint(strategies, ['震荡', 'Oscillation', '高抛低吸'])
                        }
                        if (next.trend_strategy_id) {
                          next.strategy_id = next.trend_strategy_id
                        }
                        return next
                      })
                    }}
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      formData.regime_switch_enabled
                        ? 'bg-[#F0B90B] text-black'
                        : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                    }`}
                  >
                    {language === 'zh' ? '启用' : 'On'}
                  </button>
                  <button
                    type="button"
                    onClick={() => handleInputChange('regime_switch_enabled', false)}
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      !formData.regime_switch_enabled
                        ? 'bg-[#F0B90B] text-black'
                        : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                    }`}
                  >
                    {language === 'zh' ? '关闭' : 'Off'}
                  </button>
                </div>
              </div>

              {formData.regime_switch_enabled && (
                <>
                  <p className="text-xs text-[#848E9C] leading-relaxed">
                    {language === 'zh'
                      ? '按 1H 大局 → 15m 决策台 → 3m 扳机 三层配置。切换判定以 1H+15m 为主，3m 扳机同向确认或 1H/15m 未决时参考；ATR 倍数供止损参考。'
                      : '1H bias → 15m desk → 3m trigger. Switching uses 1H+15m with 3m confirmation; ATR multiple for SL.'}
                  </p>
                  <div className="grid grid-cols-1 gap-4">
                    <div>
                      <label className="text-sm text-[#EAECEF] block mb-2">
                        {language === 'zh' ? '趋势市场策略（日内波段）' : 'Trend strategy (intraday swing)'}
                      </label>
                      <NofxSelect
                        value={formData.trend_strategy_id}
                        onChange={(val) => {
                          setFormData((prev) => ({
                            ...prev,
                            trend_strategy_id: val,
                            strategy_id: val || prev.strategy_id,
                          }))
                        }}
                        className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                        options={strategies.map((strategy) => ({
                          value: strategy.id,
                          label: strategy.name,
                        }))}
                      />
                    </div>
                    <div>
                      <label className="text-sm text-[#EAECEF] block mb-2">
                        {language === 'zh' ? '震荡市场策略（高抛低吸）' : 'Oscillation strategy (range trading)'}
                      </label>
                      <NofxSelect
                        value={formData.oscillation_strategy_id}
                        onChange={(val) => handleInputChange('oscillation_strategy_id', val)}
                        className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                        options={strategies.map((strategy) => ({
                          value: strategy.id,
                          label: strategy.name,
                        }))}
                      />
                    </div>
                    <div>
                      <label className="text-sm text-[#EAECEF] block mb-2">
                        {language === 'zh' ? '确认周期数' : 'Confirm cycles'}
                      </label>
                      <input
                        type="number"
                        value={formData.regime_confirm_cycles}
                        onChange={(e) => {
                          const parsed = Number(e.target.value)
                          const safe = Number.isFinite(parsed) ? Math.min(10, Math.max(1, parsed)) : 2
                          handleInputChange('regime_confirm_cycles', safe)
                        }}
                        className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                        min={1}
                        max={10}
                        step={1}
                      />
                      <p className="text-xs text-[#848E9C] mt-1">
                        {language === 'zh'
                          ? '连续 N 个决策周期检测到同一市场状态后才切换，避免频繁抖动。'
                          : 'Switch only after N consecutive cycles detect the same regime.'}
                      </p>
                    </div>

                    {/* 1H 层 */}
                    <div className="border-t border-[#2B3139] pt-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium text-[#F0B90B]">
                          {language === 'zh' ? '【1H 层】大局 Bias' : '[1H] Bias'}
                        </h4>
                        <label className="flex items-center gap-2 text-xs text-[#848E9C] cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formData.regime_detection.layer_1h.enabled}
                            onChange={(e) => updateLayer1H('enabled', e.target.checked)}
                            className="accent-yellow-500"
                          />
                          {language === 'zh' ? '启用' : 'On'}
                        </label>
                      </div>
                      <p className="text-[10px] text-[#848E9C]">
                        {language === 'zh'
                          ? 'EMA 定主方向；ADX<震荡阈值=震荡日，ADX>趋势阈值=趋势日；中间灰区交给 15m。'
                          : 'EMA direction; ADX thresholds define ranging vs trending day.'}
                      </p>
                      {formData.regime_detection.layer_1h.enabled && (
                        <div className="grid grid-cols-2 gap-3">
                          <RegimeNumField label="EMA Fast" hint={language === 'zh' ? '快线周期' : 'Fast EMA'} value={formData.regime_detection.layer_1h.ema_fast} onChange={(v) => updateLayer1H('ema_fast', Math.round(v))} min={5} max={100} />
                          <RegimeNumField label="EMA Slow" hint={language === 'zh' ? '慢线周期' : 'Slow EMA'} value={formData.regime_detection.layer_1h.ema_slow} onChange={(v) => updateLayer1H('ema_slow', Math.round(v))} min={10} max={200} />
                          <RegimeNumField label="ADX Period" value={formData.regime_detection.layer_1h.adx_period} onChange={(v) => updateLayer1H('adx_period', Math.round(v))} min={7} max={28} />
                          <RegimeNumField label={language === 'zh' ? '1H K线根数' : '1H bars'} value={formData.regime_detection.layer_1h.kline_count} onChange={(v) => updateLayer1H('kline_count', Math.round(v))} min={30} max={200} />
                          <RegimeNumField label={language === 'zh' ? '震荡日 ADX<' : 'Ranging ADX <'} hint={language === 'zh' ? '低于此值→震荡日' : 'Below → range day'} value={formData.regime_detection.layer_1h.adx_ranging_below} onChange={(v) => updateLayer1H('adx_ranging_below', v)} min={10} max={40} step={0.5} />
                          <RegimeNumField label={language === 'zh' ? '趋势日 ADX≥' : 'Trend ADX ≥'} hint={language === 'zh' ? '高于此值→趋势日' : 'Above → trend day'} value={formData.regime_detection.layer_1h.adx_trend_above} onChange={(v) => updateLayer1H('adx_trend_above', v)} min={15} max={50} step={0.5} />
                        </div>
                      )}
                    </div>

                    {/* 15m 层 */}
                    <div className="border-t border-[#2B3139] pt-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium text-[#F0B90B]">
                          {language === 'zh' ? '【15m 层】决策台' : '[15m] Decision desk'}
                        </h4>
                        <label className="flex items-center gap-2 text-xs text-[#848E9C] cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formData.regime_detection.layer_15m.enabled}
                            onChange={(e) => updateLayer15m('enabled', e.target.checked)}
                            className="accent-yellow-500"
                          />
                          {language === 'zh' ? '灰区确认' : 'Gray-zone confirm'}
                        </label>
                      </div>
                      <p className="text-[10px] text-[#848E9C]">
                        {language === 'zh'
                          ? 'EMA 结构、ADX+ATR/BBW 厚薄、RSI、成交量均线；1H 灰区或未决时由此裁定。'
                          : 'EMA, ADX, BBW, RSI, volume MA for gray-zone resolution.'}
                      </p>
                      {formData.regime_detection.layer_15m.enabled && (
                        <div className="grid grid-cols-2 gap-3">
                          <RegimeNumField label="EMA 20" value={formData.regime_detection.layer_15m.ema_fast} onChange={(v) => updateLayer15m('ema_fast', Math.round(v))} min={5} max={50} />
                          <RegimeNumField label="EMA 50" value={formData.regime_detection.layer_15m.ema_slow} onChange={(v) => updateLayer15m('ema_slow', Math.round(v))} min={10} max={100} />
                          <RegimeNumField label="ADX(14) 震荡<" value={formData.regime_detection.layer_15m.adx_ranging_below} onChange={(v) => updateLayer15m('adx_ranging_below', v)} min={10} max={40} step={0.5} />
                          <RegimeNumField label="ADX(14) 趋势≥" value={formData.regime_detection.layer_15m.adx_trend_above} onChange={(v) => updateLayer15m('adx_trend_above', v)} min={15} max={50} step={0.5} />
                          <RegimeNumField label="BBW 薄< (%)" hint={language === 'zh' ? '布林带宽低于此→偏震荡' : 'Thin BB → range'} value={formData.regime_detection.layer_15m.bbw_thin_below_pct} onChange={(v) => updateLayer15m('bbw_thin_below_pct', v)} min={1} max={10} step={0.1} />
                          <RegimeNumField label="RSI Period" value={formData.regime_detection.layer_15m.rsi_period} onChange={(v) => updateLayer15m('rsi_period', Math.round(v))} min={7} max={21} />
                          <RegimeNumField label="ATR Period" value={formData.regime_detection.layer_15m.atr_period} onChange={(v) => updateLayer15m('atr_period', Math.round(v))} min={7} max={28} />
                          <RegimeNumField label={language === 'zh' ? '成交量 MA' : 'Volume MA'} value={formData.regime_detection.layer_15m.volume_ma_period} onChange={(v) => updateLayer15m('volume_ma_period', Math.round(v))} min={5} max={50} />
                          <RegimeNumField label={language === 'zh' ? '窄幅区间 %' : 'Max range %'} value={formData.regime_detection.layer_15m.max_range_pct} onChange={(v) => updateLayer15m('max_range_pct', v)} min={1} max={20} step={0.5} />
                          <RegimeNumField label={language === 'zh' ? '15m K线根数' : '15m bars'} value={formData.regime_detection.layer_15m.kline_count} onChange={(v) => updateLayer15m('kline_count', Math.round(v))} min={28} max={200} />
                        </div>
                      )}
                    </div>

                    {/* 3m 层 */}
                    <div className="border-t border-[#2B3139] pt-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium text-[#F0B90B]">
                          {language === 'zh' ? '【3m 层】扳机 / 止损参考' : '[3m] Trigger / SL ref'}
                        </h4>
                        <label className="flex items-center gap-2 text-xs text-[#848E9C] cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formData.regime_detection.layer_3m.enabled}
                            onChange={(e) => updateLayer3m('enabled', e.target.checked)}
                            className="accent-yellow-500"
                          />
                          {language === 'zh' ? '记录配置' : 'Save ref'}
                        </label>
                      </div>
                      <p className="text-[10px] text-[#848E9C]">
                        {language === 'zh'
                          ? '3m ADX/ATR 参与行情检测：与 1H/15m 同向则确认切换，冲突或未决则暂缓；止损=ATR倍数或结构外。'
                          : '3m ADX/ATR joins regime check: confirms 1H/15m or blocks on conflict; SL = ATR × or structure.'}
                      </p>
                      {formData.regime_detection.layer_3m.enabled && (
                        <div className="grid grid-cols-2 gap-3">
                          <RegimeNumField label={language === 'zh' ? '止损 ATR 倍数' : 'SL ATR ×'} hint="默认 1.5" value={formData.regime_detection.layer_3m.atr_sl_multiplier} onChange={(v) => updateLayer3m('atr_sl_multiplier', v)} min={0.5} max={5} step={0.1} />
                          <RegimeNumField label="ATR Period" value={formData.regime_detection.layer_3m.atr_period} onChange={(v) => updateLayer3m('atr_period', Math.round(v))} min={7} max={28} />
                          <div>
                            <label className="text-xs text-[#EAECEF] block mb-1">
                              {language === 'zh' ? '结构止损周期' : 'Structure TF'}
                            </label>
                            <NofxSelect
                              value={formData.regime_detection.layer_3m.structure_timeframe}
                              onChange={(val) => updateLayer3m('structure_timeframe', val)}
                              className="w-full px-2 py-1.5 text-sm bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF]"
                              options={STRUCTURE_TIMEFRAMES.map((tf) => ({ value: tf, label: tf }))}
                            />
                          </div>
                          <div className="flex items-end pb-1">
                            <label className="flex items-center gap-2 text-xs text-[#EAECEF] cursor-pointer">
                              <input
                                type="checkbox"
                                checked={formData.regime_detection.layer_3m.use_structure_sl}
                                onChange={(e) => updateLayer3m('use_structure_sl', e.target.checked)}
                                className="accent-yellow-500"
                              />
                              {language === 'zh' ? '允许 15m 结构外+buffer 止损' : '15m structure SL'}
                            </label>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>

        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky bottom-0 z-10 rounded-b-xl">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-all duration-200 border border-[#404750]"
          >
            {t('cancel', language)}
          </button>
          {onSave && (
            <button
              onClick={handleSave}
              disabled={
                isSaving ||
                !formData.trader_name ||
                !formData.ai_model ||
                !formData.exchange_id ||
                (formData.regime_switch_enabled &&
                  (!formData.trend_strategy_id ||
                    !formData.oscillation_strategy_id ||
                    formData.trend_strategy_id === formData.oscillation_strategy_id)) ||
                (!formData.regime_switch_enabled && !formData.strategy_id)
              }
              className="px-8 py-3 bg-gradient-to-r from-[#F0B90B] to-[#E1A706] text-black rounded-lg hover:from-[#E1A706] hover:to-[#D4951E] transition-all duration-200 disabled:bg-[#848E9C] disabled:cursor-not-allowed font-medium shadow-lg"
            >
              {isSaving ? t('saving', language) : isEditMode ? t('editTrader', language) : t('createTraderButton', language)}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
