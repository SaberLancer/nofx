import type { DecisionRecord } from '../types'

/** True when AI was skipped by the pre-decision gate (live or backtest). */
export function isPreDecisionSkipped(decision: DecisionRecord): boolean {
  if (decision.pre_decision_skipped) {
    return true
  }
  return (decision.execution_log ?? []).some(
    (line) => line.startsWith('pre-decision:') && !line.startsWith('pre-decision signal:')
  )
}
