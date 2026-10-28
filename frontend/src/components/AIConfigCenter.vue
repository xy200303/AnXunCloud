<template>
  <div class="table-card">
    <el-tabs v-model="activeTab">
      <el-tab-pane label="模型供应商" name="providers" />
      <el-tab-pane label="能力绑定" name="bindings" />
      <el-tab-pane label="助手与 Agent" name="agents" />
      <el-tab-pane label="用量" name="usage" />
      <el-tab-pane label="配额" name="quotas" />
    </el-tabs>

    <template v-if="activeTab === 'providers'">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'ai:config:update'" type="primary" :icon="Plus" @click="openProvider()">添加供应商</el-button>
        </div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="load" />
      </div>
      <el-table v-loading="loading" :data="providers" row-key="code">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="model-list">
              <div class="model-toolbar">
                <strong>{{ row.name }}的模型</strong>
                <el-button v-perms="'ai:config:update'" link type="primary" @click="openModel(undefined, row.code)">注册模型</el-button>
              </div>
              <el-table :data="modelsFor(row.code)" size="small">
                <el-table-column prop="model_id" label="模型 ID" min-width="150" />
                <el-table-column prop="display_name" label="显示名" min-width="130" />
                <el-table-column label="能力" min-width="150">
                  <template #default="{ row: model }">
                    <el-tag v-for="capability in model.capabilities" :key="capability" size="small" class="capability">{{ capabilityLabels[capability as AICapability] }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="context_window" label="上下文" width="100" align="right" />
                <el-table-column prop="max_output" label="最大输出" width="100" align="right" />
                <el-table-column label="来源" width="90">
                  <template #default="{ row: model }"><el-tag :type="model.source === 'platform' ? 'warning' : 'success'" effect="plain">{{ model.source === 'platform' ? '平台' : '租户' }}</el-tag></template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="{ row: model }"><el-button v-perms="'ai:config:update'" link type="primary" @click="openModel(model)">{{ model.source === 'platform' && scope === 'tenant' ? '覆盖' : '编辑' }}</el-button></template>
                </el-table-column>
              </el-table>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="供应商" min-width="130" />
        <el-table-column prop="protocol" label="协议" min-width="150" />
        <el-table-column prop="base_url" label="接口地址" min-width="230" show-overflow-tooltip />
        <el-table-column prop="credential_ref" label="密钥引用" min-width="150" />
        <el-table-column label="状态" width="85">
          <template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.status === 'enabled' ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="来源" width="85">
          <template #default="{ row }"><el-tag :type="row.source === 'platform' ? 'warning' : 'success'" effect="plain">{{ row.source === 'platform' ? '平台' : '租户' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }"><el-button v-perms="'ai:config:update'" link type="primary" @click="openProvider(row)">{{ row.source === 'platform' && scope === 'tenant' ? '覆盖' : '编辑' }}</el-button></template>
        </el-table-column>
        <template #empty><el-empty description="尚未注册模型供应商" /></template>
      </el-table>
    </template>

    <template v-else-if="activeTab === 'bindings'">
      <div class="table-toolbar">
        <div class="table-toolbar-left"><strong>任务与模型绑定</strong></div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="load" />
      </div>
      <el-table v-loading="loading" :data="tasks">
        <el-table-column prop="name" label="任务类型" min-width="170" />
        <el-table-column label="能力要求" min-width="160">
          <template #default="{ row }"><el-tag v-for="capability in row.required_capabilities" :key="capability" size="small" class="capability">{{ capabilityLabels[capability as AICapability] }}</el-tag></template>
        </el-table-column>
        <el-table-column label="生效模型" min-width="200">
          <template #default="{ row }">
            <template v-if="bindingFor(row.task_type)">{{ bindingFor(row.task_type)?.provider_code }} / {{ bindingFor(row.task_type)?.model_id }}</template>
            <el-tag v-else type="info">未配置</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="超时 / 重试" width="120">
          <template #default="{ row }">{{ bindingFor(row.task_type) ? `${bindingFor(row.task_type)?.timeout_seconds}s / ${bindingFor(row.task_type)?.max_retries}次` : '—' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }"><el-tag :type="bindingFor(row.task_type)?.status === 'enabled' ? 'success' : 'info'">{{ bindingFor(row.task_type)?.status === 'enabled' ? '启用' : '未启用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="来源" width="90">
          <template #default="{ row }"><el-tag v-if="bindingFor(row.task_type)" :type="bindingFor(row.task_type)?.source === 'platform' ? 'warning' : 'success'" effect="plain">{{ bindingFor(row.task_type)?.source === 'platform' ? '平台' : '租户' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="{ row }"><el-button v-perms="'ai:config:update'" link type="primary" @click="openBinding(row)">{{ bindingFor(row.task_type) ? '编辑' : '配置' }}</el-button></template>
        </el-table-column>
      </el-table>
    </template>

    <template v-else-if="activeTab === 'agents'">
      <el-table :data="tasks.filter((task) => ['assistant', 'agent_review'].includes(task.task_type))">
        <el-table-column prop="name" label="能力" min-width="160" />
        <el-table-column label="绑定模型" min-width="200"><template #default="{ row }">{{ bindingFor(row.task_type)?.model_id || '未配置' }}</template></el-table-column>
        <el-table-column label="工具能力" width="130"><template #default="{ row }">{{ row.required_capabilities.map((item: AICapability) => capabilityLabels[item]).join('、') }}</template></el-table-column>
        <el-table-column label="操作" width="100"><template #default="{ row }"><el-button v-perms="'ai:config:update'" link type="primary" @click="openBinding(row)">配置</el-button></template></el-table-column>
      </el-table>
      <template v-if="scope === 'tenant'">
        <div class="table-toolbar agent-tools-title"><div class="table-toolbar-left"><strong>Agent 只读工具</strong></div></div>
        <el-table v-loading="loadingTools" :data="agentTools">
          <el-table-column prop="title" label="工具" min-width="160" />
          <el-table-column prop="name" label="编码" min-width="180" />
          <el-table-column prop="description" label="用途" min-width="300" show-overflow-tooltip />
          <el-table-column label="启用" width="100" align="center">
            <template #default="{ row }"><el-switch v-model="row.enabled" :disabled="savingTool === row.name" @change="setAgentTool(row)" /></template>
          </el-table-column>
        </el-table>
      </template>
    </template>

    <template v-else-if="activeTab === 'usage'">
      <el-descriptions v-if="usage" :column="3" border>
        <el-descriptions-item label="统计日期">{{ usage.date }}</el-descriptions-item>
        <el-descriptions-item label="真实请求数">{{ usage.requests }}</el-descriptions-item>
        <el-descriptions-item label="真实 token">{{ usage.tokens }}</el-descriptions-item>
        <el-descriptions-item label="运行中调用">{{ usage.running }}</el-descriptions-item>
        <el-descriptions-item label="日请求上限">{{ usage.quota.max_requests_daily }}</el-descriptions-item>
        <el-descriptions-item label="日 token 上限">{{ usage.quota.max_tokens_daily }}</el-descriptions-item>
      </el-descriptions>
    </template>

    <template v-else-if="activeTab === 'quotas'">
      <el-form v-if="usage" :model="quotaForm" label-width="150px" class="quota-form">
        <el-form-item label="日请求数上限"><el-input-number v-model="quotaForm.max_requests_daily" :min="1" controls-position="right" /></el-form-item>
        <el-form-item label="日 token 上限"><el-input-number v-model="quotaForm.max_tokens_daily" :min="1" controls-position="right" /></el-form-item>
        <el-form-item label="并发上限"><el-input-number v-model="quotaForm.max_concurrent" :min="1" :max="100" controls-position="right" /></el-form-item>
        <el-form-item>
          <el-button v-perms="'ai:config:update'" type="primary" :loading="saving" @click="submitQuota">保存配额</el-button>
          <el-button v-perms="'ai:config:update'" @click="restoreQuota">恢复默认</el-button>
          <el-tag>{{ usage.source === 'tenant' ? '租户覆盖' : '默认值' }}</el-tag>
        </el-form-item>
      </el-form>
    </template>

    <el-dialog v-model="providerVisible" :title="providerForm.existing ? '编辑供应商' : '添加供应商'" width="560px" :close-on-click-modal="false">
      <el-form :model="providerForm" label-width="110px">
        <el-form-item label="供应商编码" required><el-input v-model="providerForm.code" :disabled="providerForm.existing" placeholder="如 bailian" /></el-form-item>
        <el-form-item label="显示名称" required><el-input v-model="providerForm.name" /></el-form-item>
        <el-form-item label="接口协议" required>
          <el-select v-model="providerForm.protocol" style="width: 100%">
            <el-option label="OpenAI Chat" value="openai_chat" />
            <el-option label="OpenAI Responses" value="openai_responses" />
            <el-option label="Google Gemini" value="gemini" />
            <el-option label="Anthropic Claude" value="claude" />
          </el-select>
        </el-form-item>
        <el-form-item label="接口地址" required><el-input v-model="providerForm.base_url" placeholder="https://..." /></el-form-item>
        <el-form-item label="密钥环境变量" required><el-input v-model="providerForm.credential_ref" placeholder="如 AI_BAILIAN_KEY" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="providerForm.status"><el-radio value="enabled">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="providerVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitProvider">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="modelVisible" :title="modelForm.existing ? '编辑模型' : '注册模型'" width="560px" :close-on-click-modal="false">
      <el-form :model="modelForm" label-width="110px">
        <el-form-item label="所属供应商" required><el-select v-model="modelForm.provider_code" :disabled="modelForm.existing" style="width: 100%"><el-option v-for="provider in providers" :key="provider.code" :label="provider.name" :value="provider.code" /></el-select></el-form-item>
        <el-form-item label="模型 ID" required><el-input v-model="modelForm.model_id" :disabled="modelForm.existing" /></el-form-item>
        <el-form-item label="显示名称" required><el-input v-model="modelForm.display_name" /></el-form-item>
        <el-form-item label="上下文窗口" required><el-input-number v-model="modelForm.context_window" :min="1" controls-position="right" /></el-form-item>
        <el-form-item label="最大输出" required><el-input-number v-model="modelForm.max_output" :min="1" controls-position="right" /></el-form-item>
        <el-form-item label="能力标记" required>
          <el-checkbox-group v-model="modelForm.capabilities"><el-checkbox value="vision">视觉</el-checkbox><el-checkbox value="text">文本</el-checkbox><el-checkbox value="tools">工具调用</el-checkbox></el-checkbox-group>
        </el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="modelForm.status"><el-radio value="enabled">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="modelVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitModel">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="bindingVisible" :title="bindingForm.taskName" width="560px" :close-on-click-modal="false">
      <el-form :model="bindingForm" label-width="110px">
        <el-form-item label="模型" required>
          <el-select v-model="bindingForm.modelKey" filterable style="width: 100%">
            <el-option v-for="model in bindingModels" :key="`${model.provider_code}/${model.model_id}`" :label="`${model.display_name}（${model.provider_code}）`" :value="`${model.provider_code}/${model.model_id}`" />
          </el-select>
        </el-form-item>
        <el-form-item label="超时秒数" required><el-input-number v-model="bindingForm.timeout_seconds" :min="1" :max="300" controls-position="right" /></el-form-item>
        <el-form-item label="重试次数"><el-input-number v-model="bindingForm.max_retries" :min="0" :max="3" controls-position="right" /></el-form-item>
        <el-form-item label="自定义规则"><el-input v-model="bindingForm.custom_prompt" type="textarea" :rows="4" maxlength="8000" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="bindingForm.status"><el-radio value="enabled">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="bindingVisible = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!bindingForm.modelKey" @click="submitBinding">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { listAIProviders, listAIModels, listAIBindings, listAITasks, saveAIProvider, saveAIModel, saveAIBinding, type AIScope, type AIStatus, type AIProvider, type AIModel, type AITaskBinding, type AITask, type AICapability } from '@/api/ai'
import { getAIUsage, saveAIQuota, resetAIQuota, type AIUsage, type AIQuota } from '@/api/usage'
import { listAgentTools, setAgentToolEnabled, type AgentTool } from '@/api/agent'

const props = defineProps<{ scope: AIScope }>()
const activeTab = ref('providers')
const loading = ref(false)
const saving = ref(false)
const loadingTools = ref(false)
const savingTool = ref('')
const agentTools = ref<AgentTool[]>([])
const providers = ref<AIProvider[]>([])
const models = ref<AIModel[]>([])
const bindings = ref<AITaskBinding[]>([])
const tasks = ref<AITask[]>([])
const capabilityLabels: Record<AICapability, string> = { vision: '视觉', text: '文本', tools: '工具调用' }
async function loadAgentTools() {
  if (props.scope !== 'tenant') return
  loadingTools.value = true
  try { agentTools.value = await listAgentTools() } finally { loadingTools.value = false }
}
async function setAgentTool(tool: AgentTool) {
  savingTool.value = tool.name
  try {
    await setAgentToolEnabled(tool.name, tool.enabled)
    ElMessage.success(tool.enabled ? '工具已启用' : '工具已停用')
  } catch {
    tool.enabled = !tool.enabled
  } finally { savingTool.value = '' }
}
const modelsFor = (code: string) => models.value.filter((model) => model.provider_code === code)
const bindingFor = (taskType: string) => bindings.value.find((binding) => binding.task_type === taskType)
async function load() {
  loading.value = true
  try {
    ;[providers.value, models.value, bindings.value, tasks.value] = await Promise.all([listAIProviders(props.scope), listAIModels(props.scope), listAIBindings(props.scope), listAITasks()])
    tasks.value.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
  } finally {
    loading.value = false
  }
}

const providerVisible = ref(false)
const providerForm = ref({ existing: false, code: '', name: '', protocol: 'openai_chat' as AIProvider['protocol'], base_url: '', credential_ref: '', status: 'enabled' as AIStatus })
function openProvider(provider?: AIProvider) {
  providerForm.value = provider ? { ...provider, existing: true } : { existing: false, code: '', name: '', protocol: 'openai_chat', base_url: '', credential_ref: '', status: 'enabled' }
  providerVisible.value = true
}
async function submitProvider() {
  saving.value = true
  try {
    const { code, name, protocol, base_url, credential_ref, status } = providerForm.value
    await saveAIProvider(code, { code, name, protocol, base_url, credential_ref, status }, props.scope)
    providerVisible.value = false
    ElMessage.success('供应商已保存')
    await load()
  } finally { saving.value = false }
}

const modelVisible = ref(false)
const modelForm = ref({ existing: false, provider_code: '', model_id: '', display_name: '', context_window: 8192, max_output: 2048, capabilities: ['text'] as AICapability[], status: 'enabled' as AIStatus })
function openModel(model?: AIModel, providerCode?: string) {
  modelForm.value = model ? { ...model, existing: true } : { existing: false, provider_code: providerCode || providers.value[0]?.code || '', model_id: '', display_name: '', context_window: 8192, max_output: 2048, capabilities: ['text'], status: 'enabled' }
  modelVisible.value = true
}
async function submitModel() {
  saving.value = true
  try {
    const { provider_code, model_id, display_name, context_window, max_output, capabilities, status } = modelForm.value
    await saveAIModel(provider_code, model_id, { display_name, context_window, max_output, capabilities, status }, props.scope)
    modelVisible.value = false
    ElMessage.success('模型已保存')
    await load()
  } finally { saving.value = false }
}

const bindingVisible = ref(false)
const bindingForm = ref({ taskType: '', taskName: '', modelKey: '', timeout_seconds: 60, max_retries: 1, custom_prompt: '', status: 'enabled' as AIStatus })
const activeTask = computed(() => tasks.value.find((task) => task.task_type === bindingForm.value.taskType))
const bindingModels = computed(() => models.value.filter((model) => model.status === 'enabled' && providers.value.some((provider) => provider.code === model.provider_code && provider.status === 'enabled') && activeTask.value?.required_capabilities.every((capability) => model.capabilities.includes(capability))))
function openBinding(task: AITask) {
  const binding = bindingFor(task.task_type)
  bindingForm.value = { taskType: task.task_type, taskName: task.name, modelKey: binding ? `${binding.provider_code}/${binding.model_id}` : '', timeout_seconds: binding?.timeout_seconds || 60, max_retries: binding?.max_retries ?? 1, custom_prompt: binding?.custom_prompt || '', status: binding?.status || 'enabled' }
  bindingVisible.value = true
}
async function submitBinding() {
  saving.value = true
  try {
    const [provider_code, model_id] = bindingForm.value.modelKey.split('/')
    await saveAIBinding(bindingForm.value.taskType, { provider_code, model_id, timeout_seconds: bindingForm.value.timeout_seconds, max_retries: bindingForm.value.max_retries, custom_prompt: bindingForm.value.custom_prompt, status: bindingForm.value.status }, props.scope)
    bindingVisible.value = false
    ElMessage.success('任务绑定已保存')
    await load()
  } finally { saving.value = false }
}
const usage = ref<AIUsage>()
const quotaForm = ref<AIQuota>({ max_requests_daily: 1000, max_tokens_daily: 1000000, max_concurrent: 2 })
async function loadUsage() {
  usage.value = await getAIUsage()
  quotaForm.value = { ...usage.value.quota }
}
async function submitQuota() {
  saving.value = true
  try {
    await saveAIQuota('assistant', quotaForm.value)
    ElMessage.success('配额已保存')
    await loadUsage()
  } finally { saving.value = false }
}
async function restoreQuota() {
  saving.value = true
  try {
    await resetAIQuota('assistant')
    ElMessage.success('已恢复默认配额')
    await loadUsage()
  } finally { saving.value = false }
}
watch(activeTab, (tab) => {
  if (tab === 'usage' || tab === 'quotas') loadUsage()
  if (tab === 'agents') loadAgentTools()
})
onMounted(load)
</script>

<style scoped>
.model-list { padding: 8px 24px 16px; }
.model-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.capability { margin-right: 6px; }
.quota-form { max-width: 560px; }
</style>
