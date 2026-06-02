/** OKX「平仓收益率」= 毛盈亏 / 保证金 × 100（与订单详情一致，非 API pnlRatio 净收益率） */
export function calcCloseROIPct(position: {
  entry_price?: number
  entry_quantity?: number
  quantity?: number
  leverage?: number
  realized_pnl?: number
  pnl_ratio?: number
}): number {
  const entryPrice = position.entry_price || 0
  const qty = position.entry_quantity || position.quantity || 0
  const leverage = position.leverage || 1
  const pnl = position.realized_pnl || 0
  if (entryPrice > 0 && qty > 0 && leverage > 0 && pnl !== 0) {
    const margin = (entryPrice * qty) / leverage
    if (margin > 0) return (pnl / margin) * 100
  }
  if (position.pnl_ratio != null && position.pnl_ratio !== 0) {
    return position.pnl_ratio * 100
  }
  return 0
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
