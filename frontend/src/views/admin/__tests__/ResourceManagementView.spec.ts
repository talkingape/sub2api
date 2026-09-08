import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ResourceManagementView from '../ResourceManagementView.vue'
const auth = vi.hoisted(() => ({ isAdmin: false, isOperator: true }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/components/operator/OperatorResources.vue', () => ({ default: { props: ['resource'], template: '<div>operator {{ resource }}</div>' } }))
vi.mock('../AccountsView.vue', () => ({ default: { template: '<div>administrator accounts</div>' } }))
vi.mock('../GroupsView.vue', () => ({ default: { template: '<div>administrator groups</div>' } }))
describe('resource page role separation', () => {
  it('operator mounts only the restricted account component', async () => {
    auth.isAdmin = false; auth.isOperator = true
    const wrapper = mount(ResourceManagementView, { props: { resource: 'accounts' } })
    await flushPromises()
    expect(wrapper.text()).toBe('operator accounts')
  })
  it('administrator retains the original group page', async () => {
    auth.isAdmin = true; auth.isOperator = false
    const wrapper = mount(ResourceManagementView, { props: { resource: 'groups' } })
    await flushPromises()
    expect(wrapper.text()).toBe('administrator groups')
  })
  it('ordinary users do not mount either management page', async () => {
    auth.isAdmin = false; auth.isOperator = false
    const wrapper = mount(ResourceManagementView, { props: { resource: 'accounts' } })
    await flushPromises()
    expect(wrapper.text()).toBe('')
  })
})
