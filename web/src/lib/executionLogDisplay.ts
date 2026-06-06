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

  const regimeSkipped = s === 'regime-detect: skipped no_symbol'
  if (regimeSkipped) {
    return '[市场状态检测] 已跳过（无检测标的）'
  }

  const formatRegimeMetrics = (
    adx1h: string,
    adx15m: string,
    adx3m: string,
    atr3m: string,
    bbw: string
  ) => `1H ADX=${adx1h} 15m ADX=${adx15m} 3m ADX=${adx3m} 3m ATR=${atr3m} BBW=${bbw}%`

  const regimeInconclusive = s.match(
    /^regime-detect: inconclusive symbol=(\S+) adx_1h=([\d.]+) adx_15m=([\d.]+) adx_3m=([\d.]+) atr_3m=([\d.]+) bbw_15m_pct=([\d.]+) \| (.+)$/
  )
  if (regimeInconclusive) {
    return `[市场状态检测] 未决 ${regimeInconclusive[1]} | ${formatRegimeMetrics(
      regimeInconclusive[2],
      regimeInconclusive[3],
      regimeInconclusive[4],
      regimeInconclusive[5],
      regimeInconclusive[6]
    )} | ${regimeInconclusive[7]}`
  }

  const regimeInconclusiveLegacy = s.match(
    /^regime-detect: inconclusive symbol=(\S+) adx_1h=([\d.]+) adx_15m=([\d.]+) bbw_15m_pct=([\d.]+) \| (.+)$/
  )
  if (regimeInconclusiveLegacy) {
    return `[市场状态检测] 未决 ${regimeInconclusiveLegacy[1]} | 1H ADX=${regimeInconclusiveLegacy[2]} 15m ADX=${regimeInconclusiveLegacy[3]} BBW=${regimeInconclusiveLegacy[4]}% | ${regimeInconclusiveLegacy[5]}`
  }

  const regimeVerdict = s.match(
    /^regime-detect: verdict=(\w+) symbol=(\S+) adx_1h=([\d.]+) adx_15m=([\d.]+) adx_3m=([\d.]+) atr_3m=([\d.]+) bbw_15m_pct=([\d.]+) strategy=(.+) \| (.+)$/
  )
  if (regimeVerdict) {
    const verdictZh =
      regimeVerdict[1] === 'oscillation' ? '震荡' : regimeVerdict[1] === 'trend' ? '趋势' : regimeVerdict[1]
    return `[市场状态检测] 判定=${verdictZh} ${regimeVerdict[2]} | ${formatRegimeMetrics(
      regimeVerdict[3],
      regimeVerdict[4],
      regimeVerdict[5],
      regimeVerdict[6],
      regimeVerdict[7]
    )} | 当前策略=${regimeVerdict[8]} | ${regimeVerdict[9]}`
  }

  const regimeVerdictLegacy = s.match(
    /^regime-detect: verdict=(\w+) symbol=(\S+) adx_1h=([\d.]+) adx_15m=([\d.]+) strategy=(.+) \| (.+)$/
  )
  if (regimeVerdictLegacy) {
    const verdictZh =
      regimeVerdictLegacy[1] === 'oscillation' ? '震荡' : regimeVerdictLegacy[1] === 'trend' ? '趋势' : regimeVerdictLegacy[1]
    return `[市场状态检测] 判定=${verdictZh} ${regimeVerdictLegacy[2]} | 1H ADX=${regimeVerdictLegacy[3]} 15m ADX=${regimeVerdictLegacy[4]} | 当前策略=${regimeVerdictLegacy[5]} | ${regimeVerdictLegacy[6]}`
  }

  const regimePending = s.match(
    /^regime-switch: pending (\w+) symbol=(\S+) confirm=(\d+)\/(\d+) target=(.+) adx_1h=([\d.]+) adx_15m=([\d.]+) adx_3m=([\d.]+) atr_3m=([\d.]+) bbw_15m_pct=([\d.]+) \| (.+)$/
  )
  if (regimePending) {
    const verdictZh =
      regimePending[1] === 'oscillation' ? '震荡' : regimePending[1] === 'trend' ? '趋势' : regimePending[1]
    return `[市场状态切换] 待确认 ${verdictZh} ${regimePending[2]}（${regimePending[3]}/${regimePending[4]}）→ ${regimePending[5]} | ${formatRegimeMetrics(
      regimePending[6],
      regimePending[7],
      regimePending[8],
      regimePending[9],
      regimePending[10]
    )} | ${regimePending[11]}`
  }

  const regimePendingLegacy = s.match(
    /^regime-switch: pending (\w+) symbol=(\S+) confirm=(\d+)\/(\d+) target=(.+) adx_1h=([\d.]+) adx_15m=([\d.]+) \| (.+)$/
  )
  if (regimePendingLegacy) {
    const verdictZh =
      regimePendingLegacy[1] === 'oscillation' ? '震荡' : regimePendingLegacy[1] === 'trend' ? '趋势' : regimePendingLegacy[1]
    return `[市场状态切换] 待确认 ${verdictZh} ${regimePendingLegacy[2]}（${regimePendingLegacy[3]}/${regimePendingLegacy[4]}）→ ${regimePendingLegacy[5]} | 1H ADX=${regimePendingLegacy[6]} 15m ADX=${regimePendingLegacy[7]} | ${regimePendingLegacy[8]}`
  }

  const regimeConfirmed = s.match(
    /^regime-switch: confirmed (\w+) symbol=(\S+) switched_to=(.+) adx_1h=([\d.]+) adx_15m=([\d.]+) adx_3m=([\d.]+) atr_3m=([\d.]+) bbw_15m_pct=([\d.]+) \| (.+)$/
  )
  if (regimeConfirmed) {
    const verdictZh =
      regimeConfirmed[1] === 'oscillation' ? '震荡' : regimeConfirmed[1] === 'trend' ? '趋势' : regimeConfirmed[1]
    return `[市场状态切换] 已确认 ${verdictZh} ${regimeConfirmed[2]} → ${regimeConfirmed[3]} | ${formatRegimeMetrics(
      regimeConfirmed[4],
      regimeConfirmed[5],
      regimeConfirmed[6],
      regimeConfirmed[7],
      regimeConfirmed[8]
    )} | ${regimeConfirmed[9]}`
  }

  const regimeConfirmedLegacy = s.match(
    /^regime-switch: confirmed (\w+) symbol=(\S+) switched_to=(.+) adx_1h=([\d.]+) adx_15m=([\d.]+) \| (.+)$/
  )
  if (regimeConfirmedLegacy) {
    const verdictZh =
      regimeConfirmedLegacy[1] === 'oscillation' ? '震荡' : regimeConfirmedLegacy[1] === 'trend' ? '趋势' : regimeConfirmedLegacy[1]
    return `[市场状态切换] 已确认 ${verdictZh} ${regimeConfirmedLegacy[2]} → ${regimeConfirmedLegacy[3]} | 1H ADX=${regimeConfirmedLegacy[4]} 15m ADX=${regimeConfirmedLegacy[5]} | ${regimeConfirmedLegacy[6]}`
  }

  const regimeFailed = s.match(/^regime-switch: failed (\w+) symbol=(\S+) target=(.+?) error=(.+)$/)
  if (regimeFailed) {
    const verdictZh =
      regimeFailed[1] === 'oscillation' ? '震荡' : regimeFailed[1] === 'trend' ? '趋势' : regimeFailed[1]
    return `[市场状态切换] 失败 ${verdictZh} ${regimeFailed[2]} → ${regimeFailed[3]}：${regimeFailed[4]}`
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

  const pnlAlert = s.match(/^⚠️ ALERT PnL enforce failed (\S+) (\S+):\s*(.+)$/)
  if (pnlAlert) {
    const side = pnlAlert[2] === 'long' ? '多头' : pnlAlert[2] === 'short' ? '空头' : pnlAlert[2]
    return `⚠️ 告警：代码强制平仓失败 ${pnlAlert[1]} ${side}：${pnlAlert[3]}`
  }

  const stillOpen = s.match(/^⚠️ ALERT (\S+) (\S+) still open on exchange after failed enforce close$/)
  if (stillOpen) {
    const side = stillOpen[2] === 'long' ? '多头' : stillOpen[2] === 'short' ? '空头' : stillOpen[2]
    return `⚠️ 告警：${stillOpen[1]} ${side} 强制平仓失败后交易所仍有持仓（界面与 OKX 可能不一致）`
  }

  const tickConflict = s.match(/^⚠️ (.+): tick vs AI conflict → wait \((.+)\)$/)
  if (tickConflict) {
    return `⚠️ ${tickConflict[1]}：Tick 与 AI 方向冲突 → 观望（${tickConflict[2]}）`
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
  const oiLow = trimmed.match(/^open interest too low \(([\d.]+)M USD < ([\d.]+)M\)$/)
  if (oiLow) {
    return `持仓量过低（${oiLow[1]}M 美元 < ${oiLow[2]}M）`
  }
  const emptyTf = trimmed.match(/^Primary timeframe (\S+) K-line data is empty$/)
  if (emptyTf) {
    return `主周期 ${emptyTf[1]} K 线数据为空（请检查行情源或稍后重试）`
  }
  if (trimmed.startsWith('OKX simulated klines')) {
    return trimmed
      .replace(/^OKX simulated klines failed for /, 'OKX 模拟盘 K 线获取失败：')
      .replace(/^OKX simulated klines empty for /, 'OKX 模拟盘 K 线为空：')
      .replace(/ \(live fallback: /, '（实盘回退失败：')
  }
  if (trimmed.startsWith('forced close mismatch:')) {
    return trimmed.replace(/^forced close mismatch: /, '强制平仓与交易所状态不一致：')
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
  if (text.includes('tick conflict')) {
    return text
      .replace(/CODE: downgraded open_long → wait — tick conflict: /g, '代码：开多降级为观望 — Tick 冲突：')
      .replace(/CODE: downgraded open_short → wait — tick conflict: /g, '代码：开空降级为观望 — Tick 冲突：')
      .replace(/tick short sell=/g, 'Tick 偏空 卖压=')
      .replace(/tick long buy=/g, 'Tick 偏多 买压=')
      .replace(/tick sell pressure /g, 'Tick 卖压 ')
      .replace(/tick buy pressure /g, 'Tick 买压 ')
      .replace(/ > buy /g, ' > 买压 ')
      .replace(/ > sell /g, ' > 卖压 ')
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
