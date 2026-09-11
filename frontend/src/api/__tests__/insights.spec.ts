import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import { insightsDashboardAPI, insightsOpsAPI } from '@/api/insights'

describe('shared insights API', () => {
  beforeEach(() => {
    get.mockReset().mockResolvedValue({ data: {} })
  })

  it('uses the authenticated read-only dashboard namespace', async () => {
    await insightsDashboardAPI.getSnapshotV2({ include_stats: true })
    await insightsDashboardAPI.getUserUsageTrend({ limit: 12 })
    await insightsDashboardAPI.getUserSpendingRanking({ limit: 12 })

    expect(get).toHaveBeenNthCalledWith(1, '/insights/dashboard/snapshot-v2', {
      params: { include_stats: true }
    })
    expect(get).toHaveBeenNthCalledWith(2, '/insights/dashboard/users-trend', {
      params: { limit: 12 }
    })
    expect(get).toHaveBeenNthCalledWith(3, '/insights/dashboard/users-ranking', {
      params: { limit: 12 }
    })
  })

  it('uses the read-only ops namespace without exposing mutation methods', async () => {
    await insightsOpsAPI.getDashboardOverview({ time_range: '1h' })
    await insightsOpsAPI.getErrorDistribution({ time_range: '1h' })

    expect(get).toHaveBeenNthCalledWith(1, '/insights/ops/dashboard/overview', {
      params: { time_range: '1h' },
      signal: undefined
    })
    expect(get).toHaveBeenNthCalledWith(2, '/insights/ops/dashboard/error-distribution', {
      params: { time_range: '1h' },
      signal: undefined
    })
    expect(insightsOpsAPI).not.toHaveProperty('updateAdvancedSettings')
    expect(insightsOpsAPI).not.toHaveProperty('cleanupSystemLogs')
    expect(insightsOpsAPI).not.toHaveProperty('updateAlertEventStatus')
  })
})
