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
