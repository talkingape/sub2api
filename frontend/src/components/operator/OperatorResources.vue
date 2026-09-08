<template>
  <AppLayout>
    <div class="space-y-5">
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700">{{ error }}</p>
      <section v-if="compliance?.required" class="card space-y-4 p-6">
        <h2 class="text-lg font-semibold">运营合规确认</h2>
        <a :href="compliance.document_url_zh" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">阅读部署与运营合规承诺</a>
        <p>阅读后，请输入以下完整文字：{{ compliance.ack_phrase_zh }}</p>
        <input v-model="phrase" class="input" aria-label="合规确认文字" autocomplete="off" />
        <button class="btn btn-primary" :disabled="busy || phrase !== compliance.ack_phrase_zh" @click="acceptCompliance">确认并继续</button>
      </section>
      <template v-else-if="ready">
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-semibold">{{ accounts ? '我的上游账号' : '我负责的分组' }}</h2>
          <button class="btn btn-primary" @click="openForm()">新增{{ accounts ? '账号' : '分组' }}</button>
        </div>
        <p class="text-sm text-gray-500">{{ accounts ? '仅显示自己负责的账号。可以向开放贡献的公共分组提供账号。' : '可以创建订阅分组并编辑名称、描述；订阅分配和额度调整由管理员处理。' }}</p>
        <div class="card overflow-x-auto">
          <table class="w-full text-left">
            <thead><tr><th class="p-3">ID</th><th>名称</th><th>平台</th><th>状态</th><th>操作</th></tr></thead>
            <tbody><tr v-for="row in rows" :key="row.id" class="border-t"><td class="p-3">{{ row.id }}</td><td>{{ row.name }}</td><td>{{ row.platform }}</td><td>{{ row.status }}</td><td><button class="btn btn-secondary my-2" @click="openForm(row)">编辑</button></td></tr></tbody>
          </table>
          <p v-if="!rows.length" class="p-6 text-gray-500">暂无资源</p>
        </div>
        <div class="flex items-center gap-3">
          <button class="btn btn-secondary" :disabled="page <= 1 || busy" @click="page--; load()">上一页</button>
          <span>第 {{ page }} 页 · 共 {{ total }} 条</span>
          <button class="btn btn-secondary" :disabled="page * 20 >= total || busy" @click="page++; load()">下一页</button>
        </div>
      </template>
    </div>
    <BaseDialog :show="showForm" :title="(editing ? '编辑' : '新增') + (accounts ? '账号' : '分组')" @close="closeForm">
      <form class="space-y-4" @submit.prevent="save">
        <label class="block">名称<input v-model.trim="form.name" required class="input" maxlength="100" /></label>
        <label v-if="!editing" class="block">平台<select v-model="form.platform" class="input" @change="resetPlatform"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option><option value="gemini">Gemini</option></select></label>
        <template v-if="accounts">
          <template v-if="!editing">
            <label class="block">账号类型<select v-model="form.type" class="input"><option value="apikey">API Key</option><option v-if="form.platform === 'openai'" value="oauth">OpenAI OAuth</option></select></label>
            <label v-if="form.type === 'apikey'" class="block">API Key<input v-model="form.api_key" type="password" required autocomplete="new-password" class="input" /></label>
            <label class="block">代理<select v-model="form.proxy_id" class="input" :disabled="!!oauth"><option :value="null">不使用代理</option><option v-for="proxy in proxies" :key="proxy.id" :value="proxy.id">{{ proxy.name }}</option></select></label>
            <div v-if="form.type === 'oauth'" class="space-y-3">
              <button type="button" class="btn btn-secondary" :disabled="busy" @click="startOAuth">生成授权链接</button>
              <template v-if="oauth">
                <a :href="oauth.auth_url" target="_blank" rel="noopener noreferrer" class="block text-primary-600 underline">打开 OpenAI 授权页面</a>
                <label class="block">授权完成后的完整回调 URL<input v-model.trim="callbackURL" required class="input" autocomplete="off" /></label>
              </template>
            </div>
          </template>
          <label class="block">并发数<input v-model.number="form.concurrency" type="number" min="1" max="100" step="1" required class="input" /></label>
          <label class="block">优先级<input v-model.number="form.priority" type="number" min="0" max="1000" step="1" required class="input" /></label>
          <fieldset><legend>加入分组</legend><label v-for="group in matchingGroups" :key="group.id" class="flex items-center gap-2 py-1"><input v-model="form.group_ids" type="checkbox" :value="group.id" />{{ group.name }}</label><p v-if="!matchingGroups.length" class="text-sm text-gray-500">暂无可用分组，可先创建账号，随后创建分组或联系管理员。</p></fieldset>
        </template>
        <template v-else>
          <label class="block">描述<textarea v-model="form.description" maxlength="2000" class="input" /></label>
          <template v-if="!editing"><label v-for="limit in limits" :key="limit.key" class="block">{{ limit.label }}（美元，留空不设限）<input v-model.number="form[limit.key]" type="number" min="0" step="0.01" class="input" /></label></template>
        </template>
        <p v-if="formError" role="alert" class="text-red-600">{{ formError }}</p>
        <div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" :disabled="busy" @click="closeForm">取消</button><button type="submit" class="btn btn-primary" :disabled="busy || (!editing && accounts && form.type === 'oauth' && !oauth)">保存</button></div>
      </form>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { apiClient } from '@/api/client'
import type { AdminComplianceStatus } from '@/api/admin/compliance'
interface Resource { id: number; name: string; platform: string; status: string; description?: string; group_ids?: number[]; concurrency?: number; priority?: number }
interface Option { id: number; name: string; platform?: string }
const props = defineProps<{ resource: 'accounts' | 'groups' }>()
const accounts = computed(() => props.resource === 'accounts')
const rows = ref<Resource[]>([])
const total = ref(0), page = ref(1), busy = ref(false), ready = ref(false), error = ref(''), formError = ref('')
const showForm = ref(false), editing = ref<Resource | null>(null)
const groups = ref<Option[]>([]), proxies = ref<Option[]>([])
const compliance = ref<AdminComplianceStatus | null>(null), phrase = ref('')
const oauth = ref<{ auth_url: string; session_id: string; state: string } | null>(null)
const callbackURL = ref('')
const defaults = () => ({ name: '', platform: 'openai', type: 'apikey', api_key: '', proxy_id: null as number | null, group_ids: [] as number[], concurrency: 1, priority: 1, description: '', daily_limit_usd: '' as number | string, weekly_limit_usd: '' as number | string, monthly_limit_usd: '' as number | string })
const form = reactive(defaults())
const limits = [{ key: 'daily_limit_usd', label: '每日限额' }, { key: 'weekly_limit_usd', label: '每周限额' }, { key: 'monthly_limit_usd', label: '每月限额' }] as const
const matchingGroups = computed(() => groups.value.filter(g => g.platform === form.platform))
const message = (e: unknown) => (e as { message?: string })?.message || '请求失败，请重试'
async function load() {
  busy.value = true; error.value = ''
  try { const { data } = await apiClient.get('/operator/' + props.resource, { params: { page: page.value, page_size: 20 } }); rows.value = data.items; total.value = data.total }
  catch (e) { error.value = message(e) }
  finally { busy.value = false }
}
async function initialize() {
  try {
    compliance.value = (await apiClient.get<AdminComplianceStatus>('/operator/compliance')).data
    if (compliance.value.required) return
    ready.value = true
    await load()
  } catch (e) { error.value = message(e) }
}
async function acceptCompliance() {
  busy.value = true
  try { compliance.value = (await apiClient.post<AdminComplianceStatus>('/operator/compliance/accept', { phrase: phrase.value, language: 'zh' })).data; if (!compliance.value.required) { ready.value = true; await load() } }
  catch (e) { error.value = message(e) }
  finally { busy.value = false }
}
async function openForm(row?: Resource) {
  formError.value = ''; editing.value = row ?? null
  Object.assign(form, defaults(), row ? { name: row.name, platform: row.platform, description: row.description || '', group_ids: [...(row.group_ids || [])], concurrency: row.concurrency || 1, priority: row.priority ?? 1 } : {})
  oauth.value = null; callbackURL.value = ''
  try {
    if (accounts.value) {
      const [g, p] = await Promise.all([apiClient.get<Option[]>('/operator/group-options'), apiClient.get<Option[]>('/operator/proxy-options')])
      groups.value = g.data; proxies.value = p.data
    }
    showForm.value = true
  } catch (e) { error.value = message(e) }
}
function resetPlatform() { form.group_ids = []; form.type = 'apikey'; oauth.value = null; callbackURL.value = '' }
function closeForm() { if (busy.value) return; showForm.value = false; Object.assign(form, defaults()); oauth.value = null; callbackURL.value = '' }
async function startOAuth() {
  busy.value = true; formError.value = ''
  try {
    oauth.value = null; callbackURL.value = ''
    const { data } = await apiClient.post<{ auth_url: string; session_id: string }>('/operator/openai/auth-url', { proxy_id: form.proxy_id })
    const state = new URL(data.auth_url).searchParams.get('state')
    if (!state || !data.session_id) throw new Error('授权链接缺少会话信息，请重新生成')
    oauth.value = { ...data, state }
  }
  catch (e) { formError.value = message(e) }
  finally { busy.value = false }
}
async function save() {
  busy.value = true; formError.value = ''
  try {
    let payload: Record<string, unknown> = { name: form.name }
    let endpoint = '/operator/' + props.resource
    if (accounts.value) {
      payload = { ...payload, group_ids: [...form.group_ids], concurrency: form.concurrency, priority: form.priority }
      if (!editing.value) {
        if (form.type === 'oauth') {
          if (!oauth.value) throw new Error('请先生成授权链接')
          const callback = new URL(callbackURL.value)
          const code = callback.searchParams.get('code'), state = callback.searchParams.get('state')
          if (!code || state !== oauth.value.state) throw new Error('回调缺少授权码或 state 不匹配，请重新授权')
          endpoint = '/operator/openai/create-account'
          payload = { ...payload, session_id: oauth.value.session_id, code, state, proxy_id: form.proxy_id }
        } else payload = { ...payload, platform: form.platform, type: 'apikey', api_key: form.api_key, proxy_id: form.proxy_id }
      }
    } else {
      payload.description = form.description
      if (!editing.value) {
        payload.platform = form.platform; payload.subscription_type = 'subscription'
        for (const limit of limits) if (form[limit.key] !== '') payload[limit.key] = Number(form[limit.key])
      }
    }
    if (editing.value) await apiClient.put(endpoint + '/' + editing.value.id, payload)
    else await apiClient.post(endpoint, payload)
    showForm.value = false; Object.assign(form, defaults()); oauth.value = null; callbackURL.value = ''
    await load()
  } catch (e) { formError.value = message(e) }
  finally { busy.value = false }
}
onMounted(initialize)
</script>
