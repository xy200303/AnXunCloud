<template>
  <div class="table-card">
    <div class="table-toolbar">
      <div class="table-toolbar-left"><el-button type="primary" :icon="Plus" @click="openEditor()">新建预置包</el-button></div>
      <el-button :icon="RefreshRight" circle title="刷新" @click="load" />
    </div>
    <el-table v-loading="loading" :data="packs">
      <el-table-column prop="name" label="预置包" min-width="180" />
      <el-table-column prop="code" label="编码" min-width="190" />
      <el-table-column prop="description" label="说明" min-width="300" show-overflow-tooltip />
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.status === 'enabled' ? '已发布' : '已停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="90" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEditor(row)">编辑</el-button></template></el-table-column>
      <template #empty><el-empty description="尚未创建预置包" /></template>
    </el-table>

    <el-drawer v-model="editorVisible" :title="editing ? '编辑预置包' : '新建预置包'" size="720px" :close-on-click-modal="false">
      <el-form label-width="90px">
        <el-form-item label="名称" required><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="编码" required><el-input v-model="form.code" :disabled="editing" maxlength="64" placeholder="小写字母、数字和下划线" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="500" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="enabled">发布</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
        <el-form-item label="包内容" required>
          <el-input v-model="payloadText" type="textarea" :rows="22" class="payload-editor" spellcheck="false" />
        </el-form-item>
      </el-form>
      <el-alert v-if="payloadError" type="error" :closable="false" :title="payloadError" />
      <el-descriptions v-else-if="payloadPreview" :column="5" border class="payload-preview">
        <el-descriptions-item label="点位类型">{{ payloadPreview.point_types.length }}</el-descriptions-item>
        <el-descriptions-item label="检查项">{{ payloadPreview.templates.length }}</el-descriptions-item>
        <el-descriptions-item label="台账方案">{{ payloadPreview.equipment_schemas.length }}</el-descriptions-item>
        <el-descriptions-item label="审批流程">{{ payloadPreview.flows.length }}</el-descriptions-item>
        <el-descriptions-item label="输出模板">{{ payloadPreview.doc_templates.length }}</el-descriptions-item>
      </el-descriptions>
      <template #footer><el-button @click="editorVisible = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!form.name.trim() || !form.code.trim() || !!payloadError" @click="save">保存</el-button></template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { getPlatformPresetPack, listPlatformPresetPacks, savePlatformPresetPack, type PresetPack, type PresetPackSummary } from '@/api/community'

const packs = ref<PresetPackSummary[]>([])
const loading = ref(false)
const saving = ref(false)
const editorVisible = ref(false)
const editing = ref(false)
const payloadText = ref('')
const form = reactive({ code: '', name: '', description: '', status: 'enabled' })
const parsedPayload = computed(() => {
  try {
    const value = JSON.parse(payloadText.value) as PresetPack['payload']
    return { value: Array.isArray(value) ? null : value, error: '' }
  } catch {
    return { value: null, error: '包内容必须是有效的 JSON 对象' }
  }
})
const payloadPreview = computed(() => parsedPayload.value.value)
const payloadError = computed(() => parsedPayload.value.error)

function blankPayload(): PresetPack['payload'] {
  return {
    point_types: [], templates: [], equipment_schemas: [],
    flows: [
      { flow_code: 'checkin_review', steps: [{ role_code: 'quality_supervisor', name: '打卡复核', mode: 'any' }] },
      { flow_code: 'maint_review', steps: [{ role_code: 'quality_supervisor', name: '维保审核', mode: 'any' }] },
      { flow_code: 'workorder_review', steps: [{ role_code: 'project_manager', name: '工单审核', mode: 'any' }] },
      { flow_code: 'report_review', steps: [{ role_code: 'quality_supervisor', name: '品质审核', mode: 'any' }, { role_code: 'project_manager', name: '项目审核', mode: 'any' }, { role_code: 'tenant_admin', name: '租户归档', mode: 'any' }] },
      { flow_code: 'acceptance_review', steps: [{ role_code: 'quality_supervisor', name: '承接查验审核', mode: 'any' }] }
    ],
    doc_templates: []
  }
}
async function load() {
  loading.value = true
  try { packs.value = await listPlatformPresetPacks() } finally { loading.value = false }
}
async function openEditor(row?: PresetPackSummary) {
  editing.value = !!row
  if (row) {
    const pack = await getPlatformPresetPack(row.code)
    Object.assign(form, { code: pack.code, name: pack.name, description: pack.description, status: pack.status })
    payloadText.value = JSON.stringify(pack.payload, null, 2)
  } else {
    Object.assign(form, { code: '', name: '', description: '', status: 'enabled' })
    payloadText.value = JSON.stringify(blankPayload(), null, 2)
  }
  editorVisible.value = true
}
async function save() {
  if (!payloadPreview.value) return
  saving.value = true
  try {
    await savePlatformPresetPack(form.code.trim(), { name: form.name.trim(), description: form.description.trim(), status: form.status, payload: payloadPreview.value })
    editorVisible.value = false
    ElMessage.success('预置包已保存')
    await load()
  } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.payload-editor :deep(textarea) { font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.6; }
.payload-preview { margin-top: 16px; }
</style>
