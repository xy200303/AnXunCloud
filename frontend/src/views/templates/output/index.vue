<template>
  <div class="app-container">
    <div class="filter-card">
      <el-form inline>
        <el-form-item label="文档类别"><el-select v-model="filters.kind" clearable style="width: 160px"><el-option label="巡检月报" value="monthly_report" /><el-option label="承接查验" value="acceptance" /><el-option label="换粉台账" value="maintenance_ledger" /></el-select></el-form-item>
        <el-form-item label="引擎"><el-select v-model="filters.engine" clearable style="width: 150px"><el-option label="OOXML" value="ooxml" /><el-option label="Excel 模板" value="xlsx_template" /></el-select></el-form-item>
        <el-form-item label="状态"><el-select v-model="filters.status" clearable style="width: 120px"><el-option label="草稿" value="draft" /><el-option label="启用" value="enabled" /></el-select></el-form-item>
      </el-form>
    </div>
    <div class="table-card">
      <div class="table-toolbar"><div class="table-toolbar-left"><el-button v-perms="'report:template:update'" type="primary" :icon="Plus" @click="openForm()">新建输出模板</el-button></div><el-button :icon="RefreshRight" circle @click="load" /></div>
      <el-table v-loading="loading" :data="filtered">
        <el-table-column prop="name" label="名称" min-width="230" show-overflow-tooltip />
        <el-table-column label="文档类别" width="120"><template #default="{ row }">{{ kindLabels[row.doc_kind] }}</template></el-table-column>
        <el-table-column prop="engine" label="引擎" width="130" />
        <el-table-column prop="system_filter" label="装订范围" min-width="150"><template #default="{ row }">{{ row.system_filter || '按批次' }}</template></el-table-column>
        <el-table-column prop="format" label="格式" width="85" />
        <el-table-column label="来源" width="90"><template #default="{ row }">{{ row.source === 'platform' ? '平台' : '租户' }}</template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.status === 'enabled' ? '启用' : '草稿' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="170" fixed="right"><template #default="{ row }"><el-button v-perms="'report:template:update'" link type="primary" @click="openForm(row)">编辑</el-button><el-button v-perms="'report:template:update'" link type="primary" @click="router.push(`/templates/output/${row.code}/editor`)">版式</el-button></template></el-table-column>
        <template #empty><el-empty description="还没有输出模板" /></template>
      </el-table>
    </div>
    <el-dialog v-model="formVisible" :title="form.existing ? '编辑输出模板' : '新建输出模板'" width="580px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="模板编码" required><el-input v-model="form.code" :disabled="form.existing" /></el-form-item>
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="文档类别" required><el-select v-model="form.doc_kind" style="width: 100%"><el-option label="巡检月报" value="monthly_report" /><el-option label="承接查验" value="acceptance" /><el-option label="换粉台账" value="maintenance_ledger" /></el-select></el-form-item>
        <el-form-item label="引擎" required><el-select v-model="form.engine" style="width: 100%"><el-option label="OOXML" value="ooxml" /><el-option label="Excel 模板" value="xlsx_template" /></el-select></el-form-item>
        <el-form-item label="输出格式" required><el-select v-model="form.format" style="width: 100%"><el-option label="DOCX" value="docx" /><el-option label="PDF" value="pdf" /><el-option label="XLSX" value="xlsx" /></el-select></el-form-item>
        <el-form-item label="报告样式"><el-select v-model="form.style_pack_id" clearable placeholder="默认样式" style="width: 100%"><el-option v-for="pack in stylePacks" :key="pack.id" :label="pack.name" :value="pack.id" /></el-select></el-form-item>
        <el-form-item label="册别编码" required><el-input v-model="form.volume_code" /></el-form-item>
        <el-form-item label="设备系统"><el-input v-model="form.system_filter" placeholder="月报分册填写，其他可留空" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="draft">草稿</el-radio><el-radio value="enabled">启用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="formVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { listDocTemplates, saveDocTemplate, listStylePacks, type DocTemplate, type StylePack } from '@/api/docTemplate'

const router = useRouter()
const kindLabels: Record<string, string> = { monthly_report: '巡检月报', acceptance: '承接查验', maintenance_ledger: '换粉台账' }
const loading = ref(false)
const saving = ref(false)
const templates = ref<DocTemplate[]>([])
const stylePacks = ref<StylePack[]>([])
const filters = reactive({ kind: '', engine: '', status: '' })
const filtered = computed(() => templates.value.filter((item) => (!filters.kind || item.doc_kind === filters.kind) && (!filters.engine || item.engine === filters.engine) && (!filters.status || item.status === filters.status)))
async function load() {
  loading.value = true
  try { templates.value = await listDocTemplates() } finally { loading.value = false }
}
const formVisible = ref(false)
const form = ref({ existing: false, code: '', name: '', doc_kind: 'monthly_report' as DocTemplate['doc_kind'], engine: 'ooxml' as DocTemplate['engine'], format: 'docx' as DocTemplate['format'], volume_code: '', system_filter: '', status: 'draft' as DocTemplate['status'], layout: { mother: { profile: 'fire_equipment' } }, style_pack_id: null as string | null, skeleton_file_id: null as string | null })
function openForm(template?: DocTemplate) {
  form.value = template ? { ...template, existing: true, layout: JSON.parse(JSON.stringify(template.layout)) } : { existing: false, code: '', name: '', doc_kind: 'monthly_report', engine: 'ooxml', format: 'docx', volume_code: '', system_filter: '', status: 'draft', layout: { mother: { profile: 'fire_equipment' } }, style_pack_id: null, skeleton_file_id: null }
  formVisible.value = true
}
async function save() {
  saving.value = true
  try {
    const { code, name, doc_kind, engine, format, volume_code, system_filter, status, layout, style_pack_id, skeleton_file_id } = form.value
    await saveDocTemplate(code, { code, name, doc_kind, engine, format, volume_code, system_filter, status, layout, style_pack_id, skeleton_file_id })
    formVisible.value = false
    ElMessage.success('输出模板已保存')
    await load()
  } finally { saving.value = false }
}
onMounted(() => {
  load()
  listStylePacks().then((packs) => { stylePacks.value = packs })
})
</script>
