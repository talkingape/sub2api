import { apiClient } from './client'
import type {
  DashboardSnapshotV2Params,
  DashboardSnapshotV2Response,
  UserSpendingRankingParams,
  UserTrendParams,
  UserTrendResponse
} from './admin/dashboard'
import type {
  OpsDashboardOverview,
  OpsDashboardSnapshotV2Response,
  OpsErrorDistributionResponse,
  OpsErrorTrendResponse,
  OpsLatencyHistogramResponse,
  OpsQueryMode,
  OpsRealtimeTrafficSummaryResponse,
  OpsRequestOptions,
  OpsThroughputTrendResponse
} from './admin/ops'
import type { UserSpendingRankingResponse } from '@/types'

const DASHBOARD_BASE = '/insights/dashboard'
const OPS_BASE = '/insights/ops'

export const insightsDashboardAPI = {
  async getSnapshotV2(params?: DashboardSnapshotV2Params): Promise<DashboardSnapshotV2Response> {
    const { data } = await apiClient.get<DashboardSnapshotV2Response>(`${DASHBOARD_BASE}/snapshot-v2`, { params })
    return data
  },

  async getUserUsageTrend(params?: UserTrendParams): Promise<UserTrendResponse> {
    const { data } = await apiClient.get<UserTrendResponse>(`${DASHBOARD_BASE}/users-trend`, { params })
    return data
  },

  async getUserSpendingRanking(params?: UserSpendingRankingParams): Promise<UserSpendingRankingResponse> {
    const { data } = await apiClient.get<UserSpendingRankingResponse>(`${DASHBOARD_BASE}/users-ranking`, { params })
    return data
  }
}

export interface OpsDashboardParams {
  time_range?: string
  start_time?: string
  end_time?: string
  platform?: string
  group_id?: number | null
  mode?: OpsQueryMode
}

export const insightsOpsAPI = {
  async getRealtimeTrafficSummary(
    window: '1min' | '5min' | '30min' | '1h',
    platform?: string,
    groupId?: number | null
  ): Promise<OpsRealtimeTrafficSummaryResponse> {
    const params: Record<string, string | number> = { window }
    if (platform) params.platform = platform
    if (typeof groupId === 'number' && groupId > 0) params.group_id = groupId
    const { data } = await apiClient.get<OpsRealtimeTrafficSummaryResponse>(`${OPS_BASE}/realtime-traffic`, { params })
    return data
  },

  async getDashboardSnapshotV2(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsDashboardSnapshotV2Response> {
    const { data } = await apiClient.get<OpsDashboardSnapshotV2Response>(`${OPS_BASE}/dashboard/snapshot-v2`, {
      params,
      signal: options.signal
    })
    return data
  },

  async getDashboardOverview(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsDashboardOverview> {
    const { data } = await apiClient.get<OpsDashboardOverview>(`${OPS_BASE}/dashboard/overview`, {
      params,
      signal: options.signal
    })
    return data
  },

  async getThroughputTrend(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsThroughputTrendResponse> {
    const { data } = await apiClient.get<OpsThroughputTrendResponse>(`${OPS_BASE}/dashboard/throughput-trend`, {
      params,
      signal: options.signal
    })
    return data
  },

  async getLatencyHistogram(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsLatencyHistogramResponse> {
    const { data } = await apiClient.get<OpsLatencyHistogramResponse>(`${OPS_BASE}/dashboard/latency-histogram`, {
      params,
      signal: options.signal
    })
    return data
  },

  async getErrorTrend(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsErrorTrendResponse> {
    const { data } = await apiClient.get<OpsErrorTrendResponse>(`${OPS_BASE}/dashboard/error-trend`, {
      params,
      signal: options.signal
    })
    return data
  },

  async getErrorDistribution(
    params: OpsDashboardParams = {},
    options: OpsRequestOptions = {}
  ): Promise<OpsErrorDistributionResponse> {
    const { data } = await apiClient.get<OpsErrorDistributionResponse>(`${OPS_BASE}/dashboard/error-distribution`, {
      params,
      signal: options.signal
    })
    return data
  }
}
