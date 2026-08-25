<template>
  <!-- AI 模型配置专用表单：ai 分组下替代通用表格；保存逐项 PUT（缺键时 POST 创建），只提交有变动的项 -->
  <div class="table-card ai-config-panel">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="AI 判定为辅助初筛，误判由管理员复核兜底；最终结果以人工审核为准。"
      class="ai-tip"
    />

    <el-form v-loading="loading" label-width="140px" class="ai-form">
      <el-form-item label="AI 总开关">
        <el-switch v-model="form.enabled" inline-prompt active-text="开" inactive-text="关" />
        <span class="field-tip">关闭后所有 AI 审核/同步判定均不执行</span>
      </el-form-item>

      <el-form-item label="平台预设">
        <el-select v-model="form.platform" placeholder="选择平台预设" style="width: 280px" @change="applyPreset">
          <el-option v-for="p in PRESETS" :key="p.value" :label="p.label" :value="p.value" />
        </el-select>
        <span class="field-tip">选择后自动填充协议类型与接口地址，仍可手动修改</span>
      </el-form-item>

      <el-form-item label="协议类型">
        <el-select v-model="form.protocol" style="width: 280px">
          <el-option label="OpenAI 对话（openai_chat）" value="openai_chat" />
          <el-option label="OpenAI Responses（openai_responses）" value="openai_responses" />
          <el-option label="Google Gemini（gemini）" value="gemini" />
          <el-option label="Anthropic Claude（claude）" value="claude" />
        </el-select>
      </el-form-item>

      <el-form-item label="接口地址">
        <el-input v-model="form.base_url" placeholder="如 https://dashscope.aliyuncs.com/compatible-mode/v1" style="width: 480px" />
      </el-form-item>

      <el-form-item label="API Key">
        <el-input v-model="form.api_key" type="password" show-password placeholder="平台颁发的 API Key" style="width: 480px" />
      </el-form-item>

      <el-form-item label="模型名">
        <el-input v-model="form.model" :placeholder="modelHint || '模型名'" style="width: 480px" />
        <span v-if="modelHint" class="field-tip">当前预设建议模型：{{ modelHint }}</span>
      </el-form-item>

      <el-divider content-position="left">同步判定（打卡时实时判定）</el-divider>

      <el-form-item label="同步判定开关">
        <el-switch v-model="form.sync_enabled" inline-prompt active-text="开" inactive-text="关" />
        <span class="field-tip">开启后打卡时 AI 实时判定照片质量与内容</span>
      </el-form-item>

      <el-form-item label="同步超时秒数">
        <el-input-number v-model="form.sync_timeout_seconds" :min="1" :max="300" controls-position="right" />
        <span class="field-tip">超时后打卡照常提交，转异步审核</span>
      </el-form-item>

      <el-form-item label="重拍放行次数">
        <el-input-number v-model="form.max_photo_attempts" :min="1" :max="10" controls-position="right" />
        <span class="field-tip">同一设施重拍 N 次仍不合格时允许强制提交，转人工复核</span>
      </el-form-item>

      <el-form-item label="自定义判定规则">
        <el-input
          v-model="form.prompt"
          type="textarea"
          :rows="6"
          placeholder="高级选项：留空使用内置判定规则；填写后将整体替换内置规则（输出格式要求不变）"
          style="width: 640px"
        />
      </el-form-item>

      <el-form-item>
        <el-button v-perms="'system:config:update'" type="primary" :loading="saving" @click="handleSave">保存配置</el-button>
        <el-button :icon="RefreshRight" circle @click="fetchItems" />
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { RefreshRight } from '@element-plus/icons-vue'
import { listConfigs, createConfig, updateConfig } from '@/api/config'
import type { ConfigItem } from '@/api/types'

// 平台预设表：选择后回填协议与接口地址；custom 不回填
interface Preset {
  value: string
  label: string
  protocol?: string
  base_url?: string
  modelHint?: string
}

const PRESETS: Preset[] = [
  { value: 'bailian', label: '阿里云百炼', protocol: 'openai_chat', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1', modelHint: 'qwen-vl-max-latest' },
  { value: 'hunyuan', label: '腾讯混元', protocol: 'openai_chat', base_url: 'https://api.hunyuan.cloud.tencent.com/v1', modelHint: 'hunyuan-vision' },
  { value: 'kimi', label: 'Kimi（Moonshot）', protocol: 'openai_chat', base_url: 'https://api.moonshot.cn/v1', modelHint: 'moonshot-v1-8k-vision-preview' },
  { value: 'zhipu', label: '智谱 GLM', protocol: 'openai_chat', base_url: 'https://open.bigmodel.cn/api/paas/v4', modelHint: 'glm-4v-flash' },
  { value: 'openai', label: 'OpenAI', protocol: 'openai_chat', base_url: 'https://api.openai.com/v1', modelHint: 'gpt-4o-mini' },
  { value: 'gemini', label: 'Google Gemini', protocol: 'gemini', base_url: 'https://generativelanguage.googleapis.com/v1beta', modelHint: 'gemini-2.0-flash' },
  { value: 'claude', label: 'Anthropic Claude', protocol: 'claude', base_url: 'https://api.anthropic.com/v1', modelHint: 'claude-sonnet-4-5' },
  { value: 'custom', label: '自定义' }
]

// 键 → 表单字段（布尔/数值在序列化时转字符串）
const form = reactive({
  enabled: false,
  platform: 'custom',
  protocol: 'openai_chat',
  base_url: '',
  api_key: '',
  model: '',
  sync_enabled: false,
  sync_timeout_seconds: 30,
  max_photo_attempts: 3,
  prompt: ''
})

const modelHint = computed(() => PRESETS.find((p) => p.value === form.platform)?.modelHint || '')

// 新建缺失键时的默认名称/说明
const KEY_META: Record<string, { name: string; remark?: string }> = {
  'ai.enabled': { name: 'AI 总开关' },
  'ai.platform': { name: 'AI 平台预设' },
  'ai.protocol': { name: 'AI 协议类型' },
  'ai.base_url': { name: 'AI 接口地址' },
  'ai.api_key': { name: 'AI API Key' },
  'ai.model': { name: 'AI 模型名' },
  'ai.sync_enabled': { name: 'AI 同步判定开关' },
  'ai.sync_timeout_seconds': { name: 'AI 同步超时秒数' },
  'ai.max_photo_attempts': { name: 'AI 重拍放行次数' },
  'ai.prompt': { name: 'AI 自定义判定规则' }
}

function serialize(): Record<string, string> {
  return {
    'ai.enabled': String(form.enabled),
    'ai.platform': form.platform,
    'ai.protocol': form.protocol,
    'ai.base_url': form.base_url.trim(),
    'ai.api_key': form.api_key.trim(),
    'ai.model': form.model.trim(),
    'ai.sync_enabled': String(form.sync_enabled),
    'ai.sync_timeout_seconds': String(form.sync_timeout_seconds),
    'ai.max_photo_attempts': String(form.max_photo_attempts),
    'ai.prompt': form.prompt
  }
}

function applyPreset(value: string) {
  const p = PRESETS.find((x) => x.value === value)
  if (!p || !p.protocol) return // 自定义不回填
  form.protocol = p.protocol
  form.base_url = p.base_url || ''
}

// ===== 加载与保存 =====
const loading = ref(false)
const saving = ref(false)
const items = ref<Record<string, ConfigItem>>({})

async function fetchItems() {
  loading.value = true
  try {
    const data = await listConfigs({ page: 1, page_size: 100, group: 'ai' })
    const map: Record<string, ConfigItem> = {}
    for (const it of data.list) map[it.key] = it
    items.value = map
    // 回填表单（缺键保持默认值）
    if (map['ai.enabled']) form.enabled = map['ai.enabled'].value === 'true'
    if (map['ai.platform']) form.platform = map['ai.platform'].value || 'custom'
    if (map['ai.protocol']) form.protocol = map['ai.protocol'].value || 'openai_chat'
    if (map['ai.base_url']) form.base_url = map['ai.base_url'].value
    if (map['ai.api_key']) form.api_key = map['ai.api_key'].value
    if (map['ai.model']) form.model = map['ai.model'].value
    if (map['ai.sync_enabled']) form.sync_enabled = map['ai.sync_enabled'].value === 'true'
    if (map['ai.sync_timeout_seconds']) form.sync_timeout_seconds = Number(map['ai.sync_timeout_seconds'].value) || 30
    if (map['ai.max_photo_attempts']) form.max_photo_attempts = Number(map['ai.max_photo_attempts'].value) || 3
    if (map['ai.prompt']) form.prompt = map['ai.prompt'].value
  } finally {
    loading.value = false
  }
}

onMounted(fetchItems)

async function handleSave() {
  const values = serialize()
  const jobs: Promise<unknown>[] = []
  for (const [key, value] of Object.entries(values)) {
    const existing = items.value[key]
    if (existing) {
      // 只提交有变动的项
      if (existing.value !== value) jobs.push(updateConfig(existing.id, { value }))
    } else {
      const meta = KEY_META[key] || { name: key }
      jobs.push(createConfig({ key, name: meta.name, value, config_group: 'ai', remark: meta.remark }))
    }
  }
  if (!jobs.length) {
    ElMessage.info('配置无变动')
    return
  }
  saving.value = true
  try {
    await Promise.all(jobs)
    ElMessage.success('AI 配置已保存，即时生效')
    fetchItems()
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="scss">
.ai-tip {
  margin-bottom: $spacing-lg;
}

.ai-form {
  max-width: 860px;
}

.field-tip {
  margin-left: $spacing-sm;
  font-size: 12px;
  color: $color-text-secondary;
}
</style>
