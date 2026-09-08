import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OperatorResources from '../OperatorResources.vue'
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: api }))
const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /></section>' }
}
function render(resource: 'accounts' | 'groups' = 'groups') {
  return mount(OperatorResources, { props: { resource }, global: { stubs } })
}
beforeEach(() => {
  vi.clearAllMocks()
  api.get.mockImplementation(async (url: string) => {
    if (url.endsWith('/compliance')) return { data: { required: false } }
    if (url.endsWith('-options')) return { data: [] }
    return { data: { items: [], total: 0 } }
  })
  api.post.mockResolvedValue({ data: {} })
  api.put.mockResolvedValue({ data: {} })
})
describe('operator resource boundary', () => {
  it('does not load resources or accept compliance until the user explicitly confirms', async () => {
    api.get.mockResolvedValue({ data: { required: true, ack_phrase_zh: '明确同意', document_url_zh: '/legal' } })
    const wrapper = render()
    await flushPromises()
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.post).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('新增分组')
    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
  })
  it('creates subscription groups without exposing administrator fields', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '新增分组')!.trigger('click')
    await flushPromises()
    await wrapper.find('form input').setValue('团队分组')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/operator/groups', { name: '团队分组', description: '', platform: 'openai', subscription_type: 'subscription' })
    expect(api.get.mock.calls.every(([url]) => String(url).startsWith('/operator/'))).toBe(true)
  })
  it('editing a group only sends name and description, preserving subscription limits', async () => {
    api.get.mockImplementation(async (url: string) => ({ data: url.endsWith('/compliance') ? { required: false } : { items: [{ id: 7, name: '原分组', description: '说明', platform: 'openai', status: 'active', daily_limit_usd: 10 }], total: 1 } }))
    const wrapper = render()
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '编辑')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('每日限额')
    await wrapper.find('form input').setValue('新名称')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith('/operator/groups/7', { name: '新名称', description: '说明' })
  })
  it('creates API key accounts without sending arbitrary credentials or ownership fields', async () => {
    const wrapper = render('accounts')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '新增账号')!.trigger('click')
    await flushPromises()
    await wrapper.find('form input').setValue('专用账号')
    await wrapper.find('input[type="password"]').setValue('test-key')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/operator/accounts', { name: '专用账号', platform: 'openai', type: 'apikey', api_key: 'test-key', proxy_id: null, group_ids: [], concurrency: 1, priority: 1 })
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
  })
})

describe('operator API contract and contribution flow', () => {
  it('submits OpenAI OAuth from the actual two-field auth-url response', async () => {
    api.post.mockImplementation(async (url: string) => ({ data: url.endsWith('/auth-url') ? {
      auth_url: 'https://auth.openai.com/oauth/authorize?state=operator-state&client_id=test',
      session_id: 'operator-session'
    } : {} }))
    const wrapper = render('accounts')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '新增账号')!.trigger('click')
    await flushPromises()
    await wrapper.find('form input').setValue('OAuth专用账号')
    await wrapper.findAll('form select')[1].setValue('oauth')
    await wrapper.findAll('button').find(b => b.text() === '生成授权链接')!.trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/operator/openai/auth-url', { proxy_id: null })
    const callback = wrapper.find('form input[autocomplete="off"]')
    await callback.setValue('http://localhost:1455/auth/callback?code=operator-code&state=operator-state')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/operator/openai/create-account', {
      name: 'OAuth专用账号', group_ids: [], concurrency: 1, priority: 1,
      session_id: 'operator-session', code: 'operator-code', state: 'operator-state', proxy_id: null
    })
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('rejects a mismatched OAuth callback before exchanging the code', async () => {
    api.post.mockResolvedValue({ data: { auth_url: 'https://auth.openai.com/oauth/authorize?state=expected', session_id: 'session' } })
    const wrapper = render('accounts')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '新增账号')!.trigger('click')
    await flushPromises()
    await wrapper.find('form input').setValue('OAuth账号')
    await wrapper.findAll('form select')[1].setValue('oauth')
    await wrapper.findAll('button').find(b => b.text() === '生成授权链接')!.trigger('click')
    await flushPromises()
    await wrapper.find('form input[autocomplete="off"]').setValue('http://localhost:1455/auth/callback?code=code&state=other')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.post).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('state 不匹配')
  })

  it('round-trips visible public group contributions while filtering platform options', async () => {
    const own = { id: 11, name: '自有分组', platform: 'openai', manager_user_id: 7, allow_operator_contribution: false }
    const publicGroup = { id: 12, name: '公共分组', platform: 'openai', manager_user_id: null, allow_operator_contribution: true }
    let bindings = [12]
    api.get.mockImplementation(async (url: string) => {
      if (url.endsWith('/compliance')) return { data: { required: false } }
      if (url.endsWith('/group-options')) return { data: [own, publicGroup, { id: 13, name: '其他平台', platform: 'anthropic', allow_operator_contribution: true }] }
      if (url.endsWith('/proxy-options')) return { data: [] }
      return { data: { items: [{ id: 9, name: '我的账号', platform: 'openai', status: 'active', group_ids: bindings, concurrency: 2, priority: 0 }], total: 1 } }
    })
    api.put.mockImplementation(async (_url: string, payload: { group_ids: number[] }) => { bindings = payload.group_ids; return { data: {} } })
    const wrapper = render('accounts')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '编辑')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('form').text()).not.toContain('其他平台')
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    expect((checkboxes[1].element as HTMLInputElement).checked).toBe(true)
    await checkboxes[0].setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith('/operator/accounts/9', { name: '我的账号', group_ids: [12, 11], concurrency: 2, priority: 0 })
    await wrapper.findAll('button').find(b => b.text() === '编辑')!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('input[type="checkbox"]').every(c => (c.element as HTMLInputElement).checked)).toBe(true)
  })
})
