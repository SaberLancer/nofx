import { useState } from 'react'
import { ChevronDown, ChevronRight, RotateCcw, FileText } from 'lucide-react'
import type { PromptSectionsConfig } from '../../types'
import { promptSections as promptSectionsI18n, ts } from '../../i18n/strategy-translations'

interface PromptSectionsEditorProps {
  config: PromptSectionsConfig | undefined
  onChange: (config: PromptSectionsConfig) => void
  disabled?: boolean
  language: string
}

// Default prompt sections (same as backend defaults)
const defaultSections: PromptSectionsConfig = {
  role_definition: `# 你是专业的加密货币交易AI

你专注于技术分析和风险管理，基于市场数据做出理性的交易决策。
你的目标是在控制风险的前提下，捕捉高概率的交易机会。`,

  trading_frequency: `# ⏱️ 交易频率认知

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时>2笔 = 过度交易
- 单笔持仓时间≥30-60分钟
如果你发现自己每个周期都在交易 → 标准过低；若持仓<30分钟就平仓 → 过于急躁。`,

  entry_standards: `# 🎯 开仓标准（严格）

只在多重信号共振时开仓：
- 趋势方向明确（EMA排列、价格位置）
- 动量确认（MACD、RSI协同）
- 波动率适中（ATR合理范围）
- 量价配合（成交量支持方向）

避免：单一指标、信号矛盾、横盘震荡、刚平仓即重启。`,

  reduce_standards: `# 🪓 减仓标准（锁盈 / 降风险）

当持仓出现明显浮盈或波动加剧时，优先锁定利润，避免回吐。你可以通过 close_ratio 做部分减仓（例如 0.3=减仓 30%）。

## 盈亏阈值口径（强制）

本段落中所有「+x%」「-x%」阈值，均指系统「当前持仓」里的 **PnL%（未实现盈亏百分比，相对保证金、已含杠杆）**；峰值规则使用 **Peak PnL%**。**禁止**用标的涨跌幅 (Current-Entry)/Entry 替代 PnL% 判断是否触发减仓。

- PnL% 达到 +8%：开始锁盈，建议减仓 30% 或上调止损到保本/小幅盈利
- PnL% 达到 +12%：建议再减仓 30%-50%，剩余仓位用更紧止损保护
- 峰值回撤：Peak PnL%（H）≥+10% 且当前 PnL% 较峰值回撤≥4 个百分点 → 立即减仓/平仓；H≥+15% 回撤≥6 个百分点 → 立即平仓

（阈值可由用户自行调整）`,

  exit_standards: `# 🧯 平仓标准（纪律性退出）

当持仓逻辑失效或回撤风险超过收益空间时，必须果断退出，避免从大幅浮盈回撤到亏损。

## 盈亏阈值口径（强制）

本段落触发条件的百分比均指系统 **PnL%（未实现盈亏百分比，相对保证金、已含杠杆）**，不是标的涨跌幅。

- 触发无效/反转：趋势反转、关键位破坏、动量反向确认 → 立即平仓
- 保护利润：PnL% ≥ +10% 且出现反转信号 → 立即平仓
- 亏损控制：PnL% 达到允许亏损上限（例如 -3%~-6%）且无快速修复迹象 → 平仓止损

（阈值与规则可由用户自行调整）`,

  decision_process: `# 📋 决策流程

1. 检查持仓 → 是否该止盈/止损
2. 扫描候选币 + 多时间框 → 是否存在强信号
3. 评估风险回报比 → 是否满足最小要求
4. 先写思维链，再输出结构化JSON`,
}

export function PromptSectionsEditor({
  config,
  onChange,
  disabled,
  language,
}: PromptSectionsEditorProps) {
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    role_definition: false,
    trading_frequency: false,
    entry_standards: false,
    reduce_standards: false,
    exit_standards: false,
    decision_process: false,
  })

  const sections = [
    { key: 'role_definition', label: ts(promptSectionsI18n.roleDefinition, language), desc: ts(promptSectionsI18n.roleDefinitionDesc, language) },
    { key: 'trading_frequency', label: ts(promptSectionsI18n.tradingFrequency, language), desc: ts(promptSectionsI18n.tradingFrequencyDesc, language) },
    { key: 'entry_standards', label: ts(promptSectionsI18n.entryStandards, language), desc: ts(promptSectionsI18n.entryStandardsDesc, language) },
    { key: 'reduce_standards', label: ts(promptSectionsI18n.reduceStandards, language), desc: ts(promptSectionsI18n.reduceStandardsDesc, language) },
    { key: 'exit_standards', label: ts(promptSectionsI18n.exitStandards, language), desc: ts(promptSectionsI18n.exitStandardsDesc, language) },
    { key: 'decision_process', label: ts(promptSectionsI18n.decisionProcess, language), desc: ts(promptSectionsI18n.decisionProcessDesc, language) },
  ]

  const currentConfig = config || {}

  const updateSection = (key: keyof PromptSectionsConfig, value: string) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: value })
    }
  }

  const resetSection = (key: keyof PromptSectionsConfig) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: defaultSections[key] })
    }
  }

  const toggleSection = (key: string) => {
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const getValue = (key: keyof PromptSectionsConfig): string => {
    return currentConfig[key] || defaultSections[key] || ''
  }

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-2 mb-4">
        <FileText className="w-5 h-5 mt-0.5" style={{ color: '#a855f7' }} />
        <div>
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(promptSectionsI18n.promptSections, language)}
          </h3>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {ts(promptSectionsI18n.promptSectionsDesc, language)}
          </p>
        </div>
      </div>

      <div className="space-y-2">
        {sections.map(({ key, label, desc }) => {
          const sectionKey = key as keyof PromptSectionsConfig
          const isExpanded = expandedSections[key]
          const value = getValue(sectionKey)
          const isModified = currentConfig[sectionKey] !== undefined && currentConfig[sectionKey] !== defaultSections[sectionKey]

          return (
            <div
              key={key}
              className="rounded-lg overflow-hidden"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <button
                onClick={() => toggleSection(key)}
                className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-white/5 transition-colors text-left"
              >
                <div className="flex items-center gap-2">
                  {isExpanded ? (
                    <ChevronDown className="w-4 h-4" style={{ color: '#848E9C' }} />
                  ) : (
                    <ChevronRight className="w-4 h-4" style={{ color: '#848E9C' }} />
                  )}
                  <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                    {label}
                  </span>
                  {isModified && (
                    <span
                      className="px-1.5 py-0.5 text-[10px] rounded"
                      style={{ background: 'rgba(168, 85, 247, 0.15)', color: '#a855f7' }}
                    >
                      {ts(promptSectionsI18n.modified, language)}
                    </span>
                  )}
                </div>
                <span className="text-[10px]" style={{ color: '#848E9C' }}>
                  {value.length} {ts(promptSectionsI18n.chars, language)}
                </span>
              </button>

              {isExpanded && (
                <div className="px-3 pb-3">
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {desc}
                  </p>
                  <textarea
                    value={value}
                    onChange={(e) => updateSection(sectionKey, e.target.value)}
                    disabled={disabled}
                    rows={6}
                    className="w-full px-3 py-2 rounded-lg resize-y font-mono text-xs"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                      minHeight: '120px',
                    }}
                  />
                  <div className="flex justify-end mt-2">
                    <button
                      onClick={() => resetSection(sectionKey)}
                      disabled={disabled || !isModified}
                      className="flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/5 disabled:opacity-30"
                      style={{ color: '#848E9C' }}
                    >
                      <RotateCcw className="w-3 h-3" />
                      {ts(promptSectionsI18n.resetToDefault, language)}
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
