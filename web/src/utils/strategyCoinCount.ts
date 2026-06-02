import type { CoinSourceConfig } from '../types'

export const MAX_CANDIDATE_COINS = 10

/** Mirrors backend StrategyConfig.getEffectiveCoinCount */
export function getEffectiveCoinCount(coinSource?: CoinSourceConfig): number {
  if (!coinSource) {
    return 3
  }

  let count = 0
  switch (coinSource.source_type) {
    case 'static':
      count = coinSource.static_coins?.length ?? 0
      break
    case 'oi_top':
      count = coinSource.oi_top_limit ?? 0
      break
    case 'oi_low':
      count = coinSource.oi_low_limit ?? 0
      break
    case 'ai500':
    default:
      count = coinSource.ai500_limit ?? 0
      break
  }

  if (count <= 0) {
    count = 3
  }
  return Math.min(count, MAX_CANDIDATE_COINS)
}

/** Mirrors backend StrategyConfig.EffectiveMaxPositions */
export function effectiveMaxPositions(
  maxPositions: number | undefined,
  candidateCoinCount: number
): number {
  if (!maxPositions || maxPositions <= 0) {
    return candidateCoinCount
  }
  return Math.min(maxPositions, candidateCoinCount)
}
