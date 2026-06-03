import { t, type Language } from '../i18n/translations'

const ACTION_LABEL_KEYS: Record<string, string> = {
  open_long: 'decisionCard.actionOpenLong',
  open_short: 'decisionCard.actionOpenShort',
  close_long: 'decisionCard.actionCloseLong',
  close_short: 'decisionCard.actionCloseShort',
  hold: 'decisionCard.actionHold',
  wait: 'decisionCard.actionWait',
  unavailable: 'decisionCard.candidateUnavailable',
}

/** Localized label for trading decision actions (open/close/hold/wait). */
export function getDecisionActionLabel(action: string, language: Language): string {
  const key = ACTION_LABEL_KEYS[action] ?? ACTION_LABEL_KEYS.wait
  return t(key, language)
}
