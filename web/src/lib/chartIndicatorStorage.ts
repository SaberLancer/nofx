const CHART_INDICATORS_KEY = 'nofx_chart_indicators'

export interface ChartIndicatorPreset {
  id: string
  enabled: boolean
  params?: Record<string, unknown>
}

export function loadChartIndicatorPresets<T extends ChartIndicatorPreset>(defaults: T[]): T[] {
  if (typeof window === 'undefined') return defaults

  try {
    const raw = localStorage.getItem(CHART_INDICATORS_KEY)
    if (!raw) return defaults

    const saved = JSON.parse(raw) as ChartIndicatorPreset[]
    if (!Array.isArray(saved)) return defaults

    const savedMap = new Map(saved.map(item => [item.id, item]))
    return defaults.map(def => {
      const item = savedMap.get(def.id)
      if (!item) return def

      const merged = {
        ...def,
        enabled: Boolean(item.enabled),
        params: item.params ? { ...def.params, ...item.params } : def.params,
      } as T

      if (def.id === 'adx' && merged.params && typeof merged.params.period === 'number') {
        ;(merged as T & { name?: string }).name = `ADX (${merged.params.period})`
      }

      return merged
    })
  } catch {
    return defaults
  }
}

export function saveChartIndicatorPresets(indicators: ChartIndicatorPreset[]) {
  if (typeof window === 'undefined') return

  try {
    const payload = indicators.map(({ id, enabled, params }) => ({ id, enabled, params }))
    localStorage.setItem(CHART_INDICATORS_KEY, JSON.stringify(payload))
  } catch {
    // ignore quota / private mode errors
  }
}
