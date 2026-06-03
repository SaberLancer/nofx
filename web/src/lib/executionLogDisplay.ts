import type { Language } from '../i18n/translations'
import { getDecisionActionLabel } from './decisionActionLabels'

const PNL_SOURCE_ZH: Record<string, string> = {
  ai_cycle: 'AI 决策周期',
  account_snapshot: '账户快照',
  account_info: '账户信息',
  trading_context: '交易上下文',
}

const SIDE_ZH: Record<string, string> = {
  long: '多头',
  short: '空头',
}

function translateActionToken(action: string, language: Language): string {
  const key = action.trim().toLowerCase().replace(/\s+/g, '_')
  if (language === 'zh' && key in SIDE_ZH) {
    return SIDE_ZH[key]
  }
  if (['open_long', 'open_short', 'close_long', 'close_short', 'hold', 'wait'].includes(key)) {
    return getDecisionActionLabel(key, language)
  }
  return action
}

/** Localize backend execution_log lines for display (stored logs stay English in DB). */
export function translateExecutionLogLine(line: string, language: Language): string {
  if (language !== 'zh' || !line) {
    return line
  }

  let s = line.trim()

  const pnlUpdate = s.match(/^\[PnL on position update\]\s*source=(\w+)$/)
  if (pnlUpdate) {
    const src = PNL_SOURCE_ZH[pnlUpdate[1]] ?? pnlUpdate[1]
    return `[持仓更新·盈利率检查] 来源=${src}`
  }

  if (s === 'pre-decision: no candidate symbols') {
    return '前置决策：无候选币种'
  }

  const waitTrend = s.match(/^pre-decision: waiting tick trend \((.+)\)$/)
  if (waitTrend) {
    return `前置决策：等待 Tick 方向信号（${waitTrend[1]}）`
  }

  const limited = s.match(/^pre-decision: AI limited to position symbols \((.+)\)$/)
  if (limited) {
    return `前置决策：AI 仅分析持仓币种（${limited[1]}）`
  }

  const signal = s.match(
    /^pre-decision signal: (\S+) (\w+) buy=([\d.]+)% sell=([\d.]+)% momentum=([+-][\d.]+)% ticks=(\d+)$/
  )
  if (signal) {
    const dir = signal[2] === 'long' ? '多' : signal[2] === 'short' ? '空' : signal[2]
    return `前置决策信号：${signal[1]} ${dir} 买压=${signal[3]}% 卖压=${signal[4]}% 动量=${signal[5]}% 采样=${signal[6]} 笔`
  }

  const aiDuration = s.match(/^AI call duration: (\d+) ms$/)
  if (aiDuration) {
    return `AI 调用耗时：${aiDuration[1]} 毫秒`
  }

  const cache = s.match(/^Prompt cache: hit=(\d+) miss=(\d+) \(of input (\d+)\)$/)
  if (cache) {
    return `提示词缓存：命中=${cache[1]} 未命中=${cache[2]}（输入 ${cache[3]}）`
  }

  const ok = s.match(/^[✓✅]\s*(\S+)\s+(\S+)\s+succeeded$/)
  if (ok) {
    return `✓ ${ok[1]} ${translateActionToken(ok[2], language)} 执行成功`
  }

  const fail = s.match(/^[❌✗]\s*(\S+)\s+(\S+)\s+failed:\s*(.+)$/)
  if (fail) {
    return `❌ ${fail[1]} ${translateActionToken(fail[2], language)} 执行失败：${fail[3]}`
  }

  if (s === 'No candidate coins available, cycle skipped') {
    return '无候选币种，本周期已跳过'
  }

  const pnlFail = s.match(/^PnL enforce failed (\S+):\s*(.+)$/)
  if (pnlFail) {
    return `盈利率强制平仓失败 ${pnlFail[1]}：${pnlFail[2]}`
  }

  const refresh = s.match(/^Refreshed trading context after auto PnL enforcement \((\d+) position action\(s\)\)$/)
  if (refresh) {
    return `盈利率强制平仓后已刷新交易上下文（${refresh[1]} 个仓位操作）`
  }

  const refreshFail = s.match(/^Context refresh after PnL enforce failed:\s*(.+)$/)
  if (refreshFail) {
    return `盈利率强制平仓后刷新上下文失败：${refreshFail[1]}`
  }

  const unavail = s.match(/^⚠️\s*(\S+)\s+unavailable:\s*(.+)$/)
  if (unavail) {
    return `⚠️ ${unavail[1]} 不可用：${translateUnavailableReason(unavail[2])}`
  }

  if (s.startsWith('[CODE ENFORCED PnL]') || s.includes('[CODE ENFORCED PnL]')) {
    return translateEnforcedPnLText(s)
  }

  return s
}

function translateUnavailableReason(reason: string): string {
  const trimmed = reason.trim()
  if (trimmed === 'AI did not output a decision for this candidate') {
    return 'AI 未对该候选币种输出决策'
  }
  return trimmed
}

/** Localize unavailable-action error text on decision cards. */
export function translateActionError(error: string, language: Language): string {
  if (language !== 'zh' || !error) {
    return error
  }
  return translateUnavailableReason(error)
}

/** Localize code-enforced PnL reasoning on decision action cards. */
export function translateDecisionReasoning(text: string, language: Language): string {
  if (language !== 'zh' || !text) {
    return text
  }
  if (text.includes('[CODE ENFORCED PnL]') || text.includes('stop_loss:') || text.includes('lock_tier')) {
    return translateEnforcedPnLText(text)
  }
  if (text.startsWith('pre-decision:')) {
    return translateExecutionLogLine(text, language)
  }
  return text
}

function translateEnforcedPnLText(text: string): string {
  let s = text
    .replace(/\[CODE ENFORCED PnL\]/g, '[代码强制·盈亏]')
    .replace(/\s+\|\s+/g, ' | ')
    .replace(/\bshort\b/g, '空头')
    .replace(/\blong\b/g, '多头')
    .replace(/\s*->\s*full close/g, ' → 全部平仓')
    .replace(/\s*→\s*full close/g, ' → 全部平仓')
    .replace(/\s*→\s*partial close ratio=([\d.]+)/g, (_, r) => {
      const pct = Math.round(parseFloat(r) * 100)
      return ` → 部分平仓（比例 ${pct}%）`
    })
    .replace(/partial close ratio=([\d.]+)/g, (_, r) => {
      const pct = Math.round(parseFloat(r) * 100)
      return `部分平仓（比例 ${pct}%）`
    })

  s = s.replace(/stop_loss:\s*pnl\s*([+-][\d.]+)%\s*<=\s*([+-][\d.]+)%/g, '亏损止损：盈亏率 $1 ≤ $2')
  s = s.replace(
    /peak_pullback:\s*peak\s*([+-][\d.]+)%\s*→\s*([+-][\d.]+)%\s*\(([\d.]+)\s*pp\s*≥\s*([\d.]+)\)/g,
    '峰值回撤：峰值 $1 → 当前 $2（回撤 $3 个百分点 ≥ $4）'
  )
  s = s.replace(/lock_tier2:\s*pnl\s*([+-][\d.]+)%\s*>=\s*([+-][\d.]+)%/g, '锁盈二档：盈亏率 $1 ≥ $2')
  s = s.replace(/lock_tier1:\s*pnl\s*([+-][\d.]+)%\s*>=\s*([+-][\d.]+)%/g, '锁盈一档：盈亏率 $1 ≥ $2')

  return s
}
