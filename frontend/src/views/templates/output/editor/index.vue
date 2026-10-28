<template>
  <div v-loading="loading" class="app-container">
    <template v-if="template">
      <div class="table-card editor-header">
        <div><h2>{{ template.name }}</h2><div>{{ template.code }} · {{ template.system_filter || '按批次' }}</div></div>
        <div><el-button @click="router.push('/templates/output')">返回输出模板</el-button><el-button type="primary" :loading="saving" @click="save">保存配置</el-button></div>
      </div>

      <div class="table-card">
        <div class="table-toolbar"><div class="table-toolbar-left"><strong>甲方母版</strong></div></div>
        <el-form :model="template" label-width="110px" class="mother-form">
          <el-form-item label="模板名称" required><el-input v-model="template.name" maxlength="128" /></el-form-item>
          <el-form-item label="母版 profile" required>
            <el-select v-model="template.layout.mother.profile" style="width: 360px">
              <el-option v-for="profile in profiles" :key="profile.value" :label="profile.label" :value="profile.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="输出格式"><el-radio-group v-model="template.format"><el-radio-button value="docx">DOCX</el-radio-button><el-radio-button value="pdf">PDF</el-radio-button></el-radio-group></el-form-item>
          <el-form-item label="状态"><el-radio-group v-model="template.status"><el-radio value="enabled">启用</el-radio><el-radio value="draft">草稿</el-radio></el-radio-group></el-form-item>
          <el-form-item label="母版文件" required>
            <div class="mother-file"><el-input v-model="template.skeleton_file_id" readonly placeholder="选择内置母版或上传 DOCX" />
            <el-upload accept=".docx" :auto-upload="false" :limit="1" :show-file-list="false" :on-change="uploadMother"><el-button :loading="uploading">上传 DOCX 母版</el-button></el-upload></div>
          </el-form-item>
        </el-form>
      </div>

      <div class="table-card">
        <div class="table-toolbar"><div class="table-toolbar-left"><strong>签字位与审批环节</strong></div><el-button link type="primary" :icon="Plus" @click="addSlot">添加签字位</el-button></div>
        <el-table :data="signSlots">
          <el-table-column label="签字位编码" min-width="150"><template #default="{ row }"><el-input v-model="row.code" /></template></el-table-column>
          <el-table-column label="显示名称" min-width="160"><template #default="{ row }"><el-input v-model="row.label" /></template></el-table-column>
          <el-table-column label="审批环节序号" width="170"><template #default="{ row }"><el-input-number v-model="row.step_index" :min="0" :max="4" controls-position="right" /></template></el-table-column>
          <el-table-column label="必需" width="100"><template #default="{ row }"><el-switch v-model="row.required" /></template></el-table-column>
          <el-table-column label="操作" width="90"><template #default="{ $index }"><el-button link type="danger" @click="signSlots.splice($index, 1)">删除</el-button></template></el-table-column>
        </el-table>
      </div>

      <div class="table-card">
        <div class="table-toolbar"><div class="table-toolbar-left"><strong>检查模板绑定</strong></div><el-button type="primary" :icon="Plus" @click="addBinding">绑定检查模板</el-button></div>
        <div v-for="(binding, index) in bindings" :key="index" class="binding-editor">
          <div class="binding-head">
            <el-select v-model="binding.template_id" placeholder="选择检查模板" style="width: 320px"><el-option v-for="item in l1Templates" :key="item.id" :label="item.name" :value="item.id" /></el-select>
            <el-select v-model="binding.row_mode" style="width: 170px"><el-option label="按点位" value="point" /><el-option label="按日期" value="date" /><el-option label="按记录" value="grid" /><el-option label="按设备" value="equipment" /></el-select>
            <el-button link type="danger" @click="bindings.splice(index, 1)">删除绑定</el-button>
          </div>
          <el-table :data="bindingItems(binding)" size="small" border>
            <el-table-column prop="name" label="检查项" min-width="220" />
            <el-table-column prop="code" label="项编码（用于母版槽位映射）" min-width="220" />
          </el-table>
        </div>
        <el-empty v-if="!bindings.length" description="未绑定时按点位现有模板生成，绑定后按此处配置生成" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type UploadFile } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listDocTemplates, saveDocTemplate, listTemplateOutputBindings, saveTemplateOutputBindings, type DocTemplate, type SignSlot, type TemplateOutputBinding } from '@/api/docTemplate'
import { listTemplates } from '@/api/template'
import { uploadImage } from '@/api/upload'
import type { TemplateItem } from '@/api/biz-types'

const route = useRoute()
const router = useRouter()
const code = route.params.code as string
const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const template = ref<DocTemplate>()
const signSlots = ref<SignSlot[]>([])
const bindings = ref<TemplateOutputBinding[]>([])
const l1Templates = ref<TemplateItem[]>([])
const profiles = [
  { value: 'fire_equipment', label: '消防设施（器材类）' }, { value: 'fire_system', label: '消防设施（系统类）' },
  { value: 'supply', label: '供水设备' }, { value: 'power', label: '供电设备' }, { value: 'special', label: '特种设备' }
]
function newBinding(): TemplateOutputBinding {
  return { template_id: '', sort: bindings.value.length, columns: [], row_mode: 'point' }
}
async function load() {
  loading.value = true
  try {
    const templates = await listDocTemplates()
    const found = templates.find((item) => item.code === code)
    if (!found || !found.id) {
      ElMessage.error('输出模板不存在或缺少 ID')
      router.push('/templates/output')
      return
    }
    const [output, l1] = await Promise.all([listTemplateOutputBindings(found.id), listTemplates({ purpose: 'patrol', status: 'enabled', page: 1, page_size: 100 })])
    l1Templates.value = l1.list
    bindings.value = output
    const current = JSON.parse(JSON.stringify(found)) as DocTemplate
    if (!current.layout.mother) current.layout.mother = { profile: 'fire_equipment' }
    template.value = current
    signSlots.value = current.layout.sign_slots || []
  } finally { loading.value = false }
}
function addSlot() {
  signSlots.value.push({ code: `sign_${signSlots.value.length + 1}`, label: '签署人', step_index: signSlots.value.length, required: true })
}
function addBinding() { bindings.value.push(newBinding()) }
function bindingItems(binding: TemplateOutputBinding) {
  return l1Templates.value.find((item) => item.id === binding.template_id)?.items || []
}
async function uploadMother(file: UploadFile) {
  if (!file.raw || !template.value) return
  uploading.value = true
  try {
    const result = await uploadImage(file.raw, 'doc_template')
    template.value.skeleton_file_id = result.file_id
    ElMessage.success('母版已上传')
  } finally { uploading.value = false }
}
async function save() {
  if (!template.value?.id) return
  if (!template.value.skeleton_file_id || !template.value.layout.mother.profile) {
    ElMessage.warning('请选择母版 profile 并上传或绑定母版文件')
    return
  }
  if (bindings.value.some((binding) => !binding.template_id)) {
    ElMessage.warning('请为每个绑定选择检查模板')
    return
  }
  saving.value = true
  try {
    const current = template.value
    current.layout.sign_slots = signSlots.value
    await saveDocTemplate(current.code, { code: current.code, name: current.name, doc_kind: current.doc_kind, engine: current.engine, format: current.format, layout: current.layout, style_pack_id: current.style_pack_id, skeleton_file_id: current.skeleton_file_id, volume_code: current.volume_code, system_filter: current.system_filter, status: current.status })
    await saveTemplateOutputBindings(current.id!, bindings.value.map((binding, index) => ({ ...binding, sort: index, columns: bindingItems(binding).map((item) => ({ item_code: item.code })) })))
    ElMessage.success('母版配置已保存')
    await load()
  } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.editor-header { display: flex; justify-content: space-between; align-items: center; }
.editor-header h2 { margin: 0 0 8px; font-size: 20px; }
.editor-header > div:last-child { display: flex; gap: 8px; }
.mother-form { max-width: 760px; }
.mother-file { display: flex; gap: 12px; width: 100%; }
.mother-file .el-input { flex: 1; }
.binding-editor { margin-top: 20px; }
.binding-head { display: flex; gap: 12px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
</style>
