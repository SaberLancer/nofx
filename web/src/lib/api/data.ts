import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  CompetitionData,
  PositionHistoryResponse,
  PositionCloseOperation,
} from '../../types'
import { API_BASE, httpClient } from './helpers'

export const dataApi = {
  async getStatus(traderId?: string, silent?: boolean): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.request<SystemStatus>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch system status')
    return result.data!
  },

  async getAccount(traderId?: string, silent?: boolean): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.request<AccountInfo>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch account info')
    return result.data!
  },

  async getPositions(
    traderId?: string,
    silent?: boolean
  ): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.request<Position[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch positions')
    return result.data!
  },

  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('Failed to fetch decision logs')
    return result.data!
  },

  async getDecisionTrace(
    traderId: string,
    cycle: number
  ): Promise<DecisionRecord> {
    const query = new URLSearchParams({
      trader_id: traderId,
      cycle: String(cycle),
    })
    const result = await httpClient.get<DecisionRecord>(
      `${API_BASE}/decisions/trace?${query}`
    )
    if (!result.success) throw new Error('Failed to fetch decision trace')
    return result.data!
  },

  async getLatestDecisions(
    traderId?: string,
    limit: number = 5,
    silent?: boolean
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.request<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch latest decisions')
    return result.data!
  },

  async getStatistics(
    traderId?: string,
    silent?: boolean
  ): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.request<Statistics>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch statistics')
    return result.data!
  },

  async getEquityHistory(
    traderId?: string,
    silent?: boolean
  ): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.request<any[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch equity history')
    return result.data!
  },

  async getEquityHistoryBatch(traderIds: string[], hours?: number): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds, hours: hours || 0 }
    )
    if (!result.success) throw new Error('Failed to fetch batch equity history')
    return result.data!
  },

  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('Failed to fetch top traders')
    return result.data!
  },

  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/traders/${traderId}/public-config`
    )
    if (!result.success) throw new Error('Failed to fetch public trader config')
    return result.data!
  },

  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('Failed to fetch competition data')
    return result.data!
  },

  async getPositionHistory(
    traderId: string,
    limit: number = 100,
    silent?: boolean,
    days: number = 90
  ): Promise<PositionHistoryResponse> {
    const result = await httpClient.request<PositionHistoryResponse>(
      `${API_BASE}/positions/history?trader_id=${traderId}&limit=${limit}&days=${days}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch position history')
    return result.data!
  },

  async getPositionCloseOperations(
    traderId: string,
    params: {
      symbol: string
      side: string
      entryTime?: string | number
      exitTime?: string | number
      limit?: number
    }
  ): Promise<{ operations: PositionCloseOperation[]; count: number }> {
    const toMs = (v?: string | number): number => {
      if (v === undefined || v === null || v === '') return 0
      if (typeof v === 'number') return v
      const n = Number(v)
      if (!Number.isNaN(n)) return n
      const ts = Date.parse(v)
      return Number.isNaN(ts) ? 0 : ts
    }
    const query = new URLSearchParams({
      trader_id: traderId,
      symbol: params.symbol,
      side: params.side,
      limit: String(params.limit ?? 100),
    })
    if (params.entryTime !== undefined && params.entryTime !== null) {
      query.set('entry_time', String(params.entryTime))
    }
    if (params.exitTime !== undefined && params.exitTime !== null) {
      query.set('exit_time', String(params.exitTime))
    }
    try {
      const result = await httpClient.get<{ operations: PositionCloseOperation[]; count: number }>(
        `${API_BASE}/positions/history/operations?${query}`
      )
      if (!result.success) throw new Error('Failed to fetch close operation details')
      return result.data!
    } catch (err) {
      // Backward-compatible fallback for servers that haven't been restarted/upgraded yet.
      const msg = err instanceof Error ? err.message : ''
      if (!msg.toLowerCase().includes('not found')) {
        throw err
      }

      const ordersResp = await httpClient.request<any[]>(
        `${API_BASE}/orders?trader_id=${encodeURIComponent(traderId)}&symbol=${encodeURIComponent(params.symbol)}&limit=200`,
        { silent: true }
      )
      if (!ordersResp.success) {
        throw new Error('Failed to fetch close operation details')
      }

      const closeAction = String(params.side || '').toUpperCase() === 'SHORT' ? 'close_short' : 'close_long'
      const startMs = Math.max(0, toMs(params.entryTime) - 5 * 60 * 1000)
      const endMsRaw = toMs(params.exitTime)
      const endMs = (endMsRaw > 0 ? endMsRaw : Date.now()) + 10 * 60 * 1000

      const operations: PositionCloseOperation[] = (ordersResp.data || [])
        .filter((o) => String(o.order_action || '') === closeAction)
        .filter((o) => {
          const ps = String(o.position_side || '').toUpperCase()
          const expected = String(params.side || '').toUpperCase()
          return ps === '' || ps === expected
        })
        .filter((o) => {
          const created = toMs(o.created_at)
          return created >= startMs && created <= endMs
        })
        .map((o) => {
          const execQty = Number(o.filled_quantity || 0) > 0 ? Number(o.filled_quantity || 0) : Number(o.quantity || 0)
          const execPrice = Number(o.avg_fill_price || 0) > 0 ? Number(o.avg_fill_price || 0) : Number(o.price || 0)
          return {
            id: Number(o.id || 0),
            exchange_order_id: String(o.exchange_order_id || ''),
            order_action: String(o.order_action || ''),
            position_side: String(o.position_side || ''),
            side: String(o.side || ''),
            status: String(o.status || ''),
            quantity: Number(o.quantity || 0),
            filled_quantity: Number(o.filled_quantity || 0),
            exec_quantity: execQty,
            price: Number(o.price || 0),
            avg_fill_price: Number(o.avg_fill_price || 0),
            exec_price: execPrice,
            created_at: String(o.created_at || ''),
            filled_at: String(o.filled_at || ''),
          }
        })
        .sort((a, b) => toMs(a.created_at) - toMs(b.created_at))

      return { operations, count: operations.length }
    }
  },
}
