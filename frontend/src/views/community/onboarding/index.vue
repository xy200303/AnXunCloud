<template>
  <div class="app-container onboarding-page">
    <div class="table-card page-head">
      <el-button :icon="ArrowLeft" @click="router.push('/community/list')">返回小区列表</el-button>
      <div>
        <h2 class="page-title">新项目上线 · 不从零配</h2>
        <div class="page-subtitle">导入点位类型、检查项模板、台账字段、审批流程和输出模板，管理员只改差异</div>
      </div>
    </div>
    <el-steps :active="step" align-center finish-status="success" class="onboarding-steps">
      <el-step title="导入预置包" />
      <el-step title="小区基本信息" />
      <el-step title="上线完成" />
    </el-steps>

    <div v-if="step === 0" class="table-card">
      <el-form label-width="96px">
        <el-form-item label="预置包">
          <el-select v-model="selectedCode" placeholder="选择预置包" style="width: 420px" :loading="packsLoading" @change="loadPack">
            <el-option v-for="pack in packs" :key="pack.code" :label="pack.name" :value="pack.code" />
          </el-select>
          <el-button :disabled="!pack" @click="contentVisible = true">查看包内容</el-button>
        </el-form-item>
      </el-form>
      <el-alert v-if="!packsLoading && !packs.length" type="info" :closable="false" title="平台尚未发布预置包，请先到发布管理维护" />
      <template v-if="pack">
        <el-descriptions :column="5" border class="pack-summary">
          <el-descriptions-item label="点位类型">{{ pack.payload.point_types.length }} 项</el-descriptions-item>
          <el-descriptions-item label="检查项">{{ pack.payload.templates.length }} 条</el-descriptions-item>
          <el-descriptions-item label="台账方案">{{ pack.payload.equipment_schemas.length }} 套</el-descriptions-item>
          <el-descriptions-item label="审批流程">{{ pack.payload.flows.length }} 条</el-descriptions-item>
          <el-descriptions-item label="输出模板">{{ pack.payload.doc_templates.length }} 套</el-descriptions-item>
        </el-descriptions>
        <el-table :data="layerResults" border class="layer-table">
          <el-table-column prop="label" label="配置层" width="160" />
          <el-table-column prop="message" label="处理结果" min-width="330" />
          <el-table-column label="状态" width="110" align="center">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template>
          </el-table-column>
        </el-table>
        <el-timeline class="launch-checklist">
          <el-timeline-item timestamp="约 10 分钟">预置包导入</el-timeline-item>
          <el-timeline-item timestamp="按数据量">楼栋、点位、设备 Excel 导入</el-timeline-item>
          <el-timeline-item timestamp="约半天">二维码 / NFC 批量生成打印</el-timeline-item>
          <el-timeline-item timestamp="约 1 天">试点点位验收</el-timeline-item>
        </el-timeline>
        <div class="form-actions">
          <el-button type="primary" :disabled="importing" @click="confirmVisible = true">{{ imported ? '重新检查并导入' : '导入预置包' }}</el-button>
          <el-button :disabled="!canContinue" @click="step = 1">下一步</el-button>
        </div>
      </template>
    </div>

    <div v-if="step === 1" class="table-card basic-panel">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
        <el-form-item label="小区名称" prop="name"><el-input v-model="form.name" maxlength="128" placeholder="请输入小区名称" /></el-form-item>
        <el-form-item label="地址"><el-input v-model="form.address" maxlength="255" placeholder="选填" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status"><el-radio :value="1">启用</el-radio><el-radio :value="0">停用</el-radio></el-radio-group>
        </el-form-item>
      </el-form>
      <div class="form-actions">
        <el-button @click="step = 0">上一步</el-button>
        <el-button type="primary" :loading="creating" @click="createProject">创建小区</el-button>
      </div>
    </div>

    <div v-if="step === 2" class="table-card">
      <el-result icon="success" title="小区已创建" sub-title="默认配置已按导入结果生效，可在小区设置中继续覆盖" />
      <div class="form-actions">
        <el-button type="primary" @click="router.push('/inspection/plans')">去配第一个计划</el-button>
        <el-button @click="router.push('/community/list')">返回小区列表</el-button>
      </div>
    </div>

    <el-drawer v-model="contentVisible" title="预置包内容" size="620px">
      <template v-if="pack">
        <h3>点位类型</h3><el-tag v-for="item in pack.payload.point_types" :key="item.value" class="content-tag">{{ item.label }}</el-tag>
        <h3>检查项</h3>
        <el-table :data="pack.payload.templates" border size="small" max-height="360">
          <el-table-column prop="point_type" label="点位类型" width="150" />
          <el-table-column prop="template_name" label="模板" min-width="160" />
          <el-table-column prop="item_name" label="检查项" min-width="140" />
        </el-table>
        <h3>其他配置</h3>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="台账字段方案">{{ pack.payload.equipment_schemas.map((item) => item.type).join('、') || '无' }}</el-descriptions-item>
          <el-descriptions-item label="审批流程">{{ pack.payload.flows.map((item) => item.flow_code).join('、') }}</el-descriptions-item>
          <el-descriptions-item label="输出模板">{{ pack.payload.doc_templates.map((item) => item.name).join('、') }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-drawer>

    <el-dialog v-model="confirmVisible" title="确认导入预置包" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" title="仅导入差异配置；已有配置不会静默覆盖，平台默认项会复制为租户配置。" />
      <el-table v-if="pack" :data="layerResults" border>
        <el-table-column prop="label" label="将写入的配置层" />
        <el-table-column label="条数" width="100" align="right">
          <template #default="{ row }">{{ layerCount(row.key) }}</template>
        </el-table-column>
      </el-table>
      <el-progress v-if="importing || imported" :percentage="progress" :status="importing ? undefined : 'success'" />
      <template #footer>
        <el-button :disabled="importing" @click="confirmVisible = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="runImport">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { createCommunity, getPresetPack, listPresetPacks, type PresetPack, type PresetPackSummary } from '@/api/community'
import { createDictData, listDictData } from '@/api/dict'
import { applyTemplateImport } from '@/api/template'
import { getTypeSchema } from '@/api/equipment'
import { saveTypeSchema } from '@/api/equipment'
import { getFlow, saveFlow } from '@/api/post'
import { listDocTemplates, saveDocTemplate } from '@/api/docTemplate'

type LayerKey = 'point_types' | 'templates' | 'equipment_schemas' | 'flows' | 'doc_templates'
type LayerStatus = 'pending' | 'running' | 'done' | 'partial' | 'failed'
const layerDefinitions: { key: LayerKey; label: string }[] = [
  { key: 'point_types', label: '点位类型' }, { key: 'templates', label: '检查项模板' },
  { key: 'equipment_schemas', label: '台账字段方案' }, { key: 'flows', label: '审批流程' }, { key: 'doc_templates', label: '输出模板' }
]
const router = useRouter()
const step = ref(0)
const packs = ref<PresetPackSummary[]>([])
const packsLoading = ref(false)
const selectedCode = ref('')
const pack = ref<PresetPack | null>(null)
const contentVisible = ref(false)
const confirmVisible = ref(false)
const importing = ref(false)
const imported = ref(false)
const layerResults = ref(layerDefinitions.map((item) => ({ ...item, status: 'pending' as LayerStatus, message: '待导入' })))
const importedCount = ref(0)
const conflictCount = ref(0)
const failedCount = ref(0)
const progress = computed(() => Math.round(layerResults.value.filter((item) => ['done', 'partial', 'failed'].includes(item.status)).length / layerDefinitions.length * 100))
const canContinue = computed(() => imported.value && (importedCount.value + conflictCount.value > 0) && failedCount.value < layerDefinitions.length)
const formRef = ref<FormInstance>()
const creating = ref(false)
const form = reactive({ name: '', address: '', status: 1 })
const rules: FormRules = { name: [{ required: true, message: '请输入小区名称', trigger: 'blur' }] }

onMounted(async () => {
  packsLoading.value = true
  try {
    packs.value = await listPresetPacks()
    if (packs.value.length) {
      selectedCode.value = packs.value[0].code
      await loadPack()
    }
  } finally { packsLoading.value = false }
})
async function loadPack() {
  pack.value = await getPresetPack(selectedCode.value)
  imported.value = false
  importedCount.value = 0
  conflictCount.value = 0
  failedCount.value = 0
  layerResults.value = layerDefinitions.map((item) => ({ ...item, status: 'pending', message: '待导入' }))
}
function layerCount(key: LayerKey) { return pack.value?.payload[key].length || 0 }
function statusLabel(status: LayerStatus) { return { pending: '待导入', running: '导入中', done: '已完成', partial: '部分完成', failed: '失败' }[status] }
function statusType(status: LayerStatus) { return status === 'done' ? 'success' : status === 'partial' ? 'warning' : status === 'failed' ? 'danger' : 'info' }
function setLayer(key: LayerKey, status: LayerStatus, message: string) {
  const layer = layerResults.value.find((item) => item.key === key)!
  layer.status = status
  layer.message = message
}
function errorText(error: unknown) { return error instanceof Error ? error.message : '请求失败' }

async function importPointTypes() {
  setLayer('point_types', 'running', '正在写入点位类型')
  let inserted = 0, conflicts = 0, failed = 0
  const existing = await listDictData({ type_code: 'point_type', page_size: 500 })
  const existingValues = new Set(existing.list.map((item) => item.value))
  for (const item of pack.value!.payload.point_types) {
    if (existingValues.has(item.value)) { conflicts++; continue }
    try { await createDictData(item); inserted++ } catch { failed++ }
  }
  importedCount.value += inserted
  conflictCount.value += conflicts
  if (failed) failedCount.value++
  setLayer('point_types', failed ? 'partial' : 'done', `新增 ${inserted}，已有未覆盖 ${conflicts}，失败 ${failed}`)
}
async function importTemplates() {
  setLayer('templates', 'running', '正在校验并写入检查项')
  const result = await applyTemplateImport(pack.value!.payload.templates)
  importedCount.value += result.created_items
  conflictCount.value += result.failed.length
  setLayer('templates', result.failed.length ? 'partial' : 'done', `写入 ${result.created_items}，冲突未覆盖 ${result.failed.length}`)
}
async function importEquipmentSchemas() {
  setLayer('equipment_schemas', 'running', '正在写入台账字段方案')
  let copied = 0, conflicts = 0, failed = 0
  for (const item of pack.value!.payload.equipment_schemas) {
    try {
      const current = await getTypeSchema(item.type)
      if (current?.source === 'tenant') { conflicts++; continue }
      await saveTypeSchema(item.type, { config: item.config })
      copied++
    } catch { failed++ }
  }
  importedCount.value += copied
  conflictCount.value += conflicts
  if (failed) failedCount.value++
  setLayer('equipment_schemas', failed ? 'partial' : 'done', `新增或复制 ${copied}，租户已有未覆盖 ${conflicts}，失败 ${failed}`)
}
async function importFlows() {
  setLayer('flows', 'running', '正在写入审批流程')
  let inserted = 0, conflicts = 0, failed = 0
  for (const item of pack.value!.payload.flows) {
    try {
      const current = await getFlow(item.flow_code, 'tenant')
      if (current.source !== 'default') { conflicts++; continue }
      await saveFlow(item.flow_code, item.steps, 'tenant')
      inserted++
    } catch { failed++ }
  }
  importedCount.value += inserted
  conflictCount.value += conflicts
  if (failed) failedCount.value++
  setLayer('flows', failed ? 'partial' : 'done', `新增 ${inserted}，已有未覆盖 ${conflicts}，失败 ${failed}`)
}
async function importDocTemplates() {
  setLayer('doc_templates', 'running', '正在写入输出模板')
  const existing = await listDocTemplates()
  const tenantCodes = new Set(existing.filter((item) => item.source === 'tenant').map((item) => item.code))
  let copied = 0, conflicts = 0, failed = 0
  for (const item of pack.value!.payload.doc_templates) {
    if (tenantCodes.has(item.code)) { conflicts++; continue }
    try {
      const { source: _source, published_at: _publishedAt, ...payload } = item
      await saveDocTemplate(item.code, payload)
      copied++
    } catch { failed++ }
  }
  importedCount.value += copied
  conflictCount.value += conflicts
  if (failed) failedCount.value++
  setLayer('doc_templates', failed ? 'partial' : 'done', `新增或复制 ${copied}，租户已有未覆盖 ${conflicts}，失败 ${failed}`)
}
async function runImport() {
  importing.value = true
  importedCount.value = 0
  conflictCount.value = 0
  failedCount.value = 0
  try {
    for (const action of [importPointTypes, importTemplates, importEquipmentSchemas, importFlows, importDocTemplates]) {
      try { await action() } catch (error) {
        failedCount.value++
        const key = action === importPointTypes ? 'point_types' : action === importTemplates ? 'templates' : action === importEquipmentSchemas ? 'equipment_schemas' : action === importFlows ? 'flows' : 'doc_templates'
        setLayer(key, 'failed', errorText(error))
      }
    }
    imported.value = true
    ElMessage.success('预置包处理完成，请查看各层结果')
  } finally { importing.value = false }
}
async function createProject() {
  await formRef.value?.validate()
  creating.value = true
  try {
    await createCommunity(form)
    step.value = 2
  } finally { creating.value = false }
}
</script>

<style scoped lang="scss">
.onboarding-page { max-width: 1180px; margin: 0 auto; }
.page-head { display: flex; align-items: center; gap: $spacing-lg; }
.page-title { margin: 0; }
.page-subtitle { color: $color-text-secondary; margin-top: $spacing-xs; }
.onboarding-steps { margin: $spacing-xl 12%; }
.pack-summary, .layer-table { margin-bottom: $spacing-lg; }
.launch-checklist { margin: $spacing-lg 0; }
.form-actions { display: flex; justify-content: center; gap: $spacing-md; margin-top: $spacing-xl; }
.basic-panel { max-width: 680px; margin: 0 auto; }
.content-tag { margin-right: $spacing-sm; margin-bottom: $spacing-sm; }
</style>
