import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const dashboardSource = readFileSync(resolve(testDir, '../OpsDashboard.vue'), 'utf8')
const headerSource = readFileSync(resolve(testDir, '../components/OpsDashboardHeader.vue'), 'utf8')

describe('OpsDashboard viewer access', () => {
  it('mounts settings, alerts and system logs only for admins', () => {
    expect(dashboardSource).toContain('<template v-if="authStore.isAdmin && !isFullscreen">')
    expect(dashboardSource).toContain('v-if="authStore.isAdmin && opsEnabled && showAlertEvents')
    expect(dashboardSource).toContain('v-if="authStore.isAdmin && opsEnabled && !(loading && !hasLoadedOnce)"')
  })

  it('hides settings and alert-management controls from viewers', () => {
    expect(headerSource.match(/v-if="props\.canManage && !props\.fullscreen"/g)).toHaveLength(2)
  })

  it('loads viewer charts through the read-only API', () => {
    expect(dashboardSource).toContain('insightsOpsAPI.getDashboardSnapshotV2')
    expect(dashboardSource).toContain('insightsOpsAPI.getErrorDistribution')
  })
})
