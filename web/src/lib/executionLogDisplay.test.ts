import { describe, expect, it } from 'vitest'
import {
  translateDecisionReasoning,
  translateExecutionLogLine,
} from './executionLogDisplay'

describe('executionLogDisplay zh', () => {
  const zh = 'zh' as const

  it('translates PnL source line', () => {
    expect(translateExecutionLogLine('[PnL on position update] source=ai_cycle', zh)).toBe(
      '[持仓更新·盈利率检查] 来源=AI 决策周期'
    )
  })

  it('translates code-enforced reason', () => {
    const line =
      '[CODE ENFORCED PnL] BTCUSDT short | stop_loss: pnl -10.24% <= -10.0% → full close'
    expect(translateExecutionLogLine(line, zh)).toContain('代码强制·盈亏')
    expect(translateExecutionLogLine(line, zh)).toContain('亏损止损')
    expect(translateExecutionLogLine(line, zh)).toContain('全部平仓')
    expect(translateExecutionLogLine(line, zh)).not.toContain('stop_loss')
  })

  it('translates pre-decision wait', () => {
    expect(
      translateExecutionLogLine('pre-decision: waiting tick trend (BTCUSDT, ETHUSDT)', zh)
    ).toContain('等待 Tick 方向信号')
  })

  it('translates regime detection inconclusive', () => {
    const line =
      'regime-detect: inconclusive symbol=ETHUSDT adx_1h=22.3 adx_15m=21.0 bbw_15m_pct=1.45 | 1H ADX=22.3 灰区(20-25)'
    expect(translateExecutionLogLine(line, zh)).toContain('[市场状态检测] 未决 ETHUSDT')
    expect(translateExecutionLogLine(line, zh)).toContain('1H ADX=22.3')
  })

  it('translates regime switch pending', () => {
    const line =
      'regime-switch: pending oscillation symbol=SOLUSDT confirm=1/2 target=震荡高抛低吸 adx_1h=18.2 adx_15m=19.5 | 15m ADX=19.5 偏震荡'
    expect(translateExecutionLogLine(line, zh)).toContain('[市场状态切换] 待确认 震荡 SOLUSDT（1/2）')
  })

  it('translates success log', () => {
    expect(translateExecutionLogLine('✓ BTCUSDT close_short succeeded', zh)).toBe(
      '✓ BTCUSDT 平空 执行成功'
    )
  })

  it('leaves English when language is en', () => {
    const line = '[CODE ENFORCED PnL] BTCUSDT short | stop_loss: pnl -10.24% <= -10.0%'
    expect(translateDecisionReasoning(line, 'en')).toBe(line)
  })
})
