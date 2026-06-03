/** Prefer OKX net realized PnL (已实现收益，扣费后) for display. */
export function getDisplayPnL(position: {
  net_realized_pnl?: number
  realized_pnl?: number
}): number {
  if (position.net_realized_pnl != null && position.net_realized_pnl !== 0) {
    return position.net_realized_pnl
  }
  return position.realized_pnl || 0
}

/** OKX「平仓收益率」= 净盈亏 / 保证金 × 100（与 OKX App 一致） */
export function calcCloseROIPct(position: {
  entry_price?: number
  entry_quantity?: number
  quantity?: number
  leverage?: number
  realized_pnl?: number
  net_realized_pnl?: number
  pnl_ratio?: number
}): number {
  const entryPrice = position.entry_price || 0
  const maxQty = position.entry_quantity || position.quantity || 0
  const leverage = position.leverage || 1
  const pnl = getDisplayPnL(position)
  if (entryPrice > 0 && maxQty > 0 && leverage > 0 && pnl !== 0) {
    const margin = (entryPrice * maxQty) / leverage
    if (margin > 0) return (pnl / margin) * 100
  }
  if (position.pnl_ratio != null && position.pnl_ratio !== 0) {
    return position.pnl_ratio * 100
  }
  return 0
}

/** Per close-order ROI = fill PnL / margin × 100 (gross, fee not deducted). */
export function calcOperationCloseROIPct(
  position: {
    entry_price?: number
    leverage?: number
  },
  op: {
    order_action?: string
    exec_quantity?: number
    realized_pnl?: number
  }
): number | null {
  const action = String(op.order_action || '').trim().toLowerCase()
  if (action !== 'close_long' && action !== 'close_short') return null

  const entryPrice = position.entry_price || 0
  const leverage = position.leverage || 1
  const qty = op.exec_quantity || 0
  if (entryPrice <= 0 || qty <= 0 || leverage <= 0) return null

  const margin = (entryPrice * qty) / leverage
  if (margin <= 0) return null

  const pnl = Number(op.realized_pnl ?? 0)
  return (pnl / margin) * 100
}

export function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// 提取下划线后面的名称部分
export function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}
