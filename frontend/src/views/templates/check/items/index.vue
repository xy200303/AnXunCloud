<template>
  <div class="app-container">
    <div class="table-card items-head">
      <el-button :icon="ArrowLeft" @click="router.push('/templates/check')">返回模板列表</el-button>
      <h2 class="page-title items-title">检查项配置<template v-if="tplName"> · {{ tplName }}</template></h2>
    </div>

    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'inspection:template:update'" type="primary" :icon="Plus" @click="openForm()">新增检查项</el-button>
          <el-button v-perms="'inspection:template:update'" :icon="Upload" @click="router.push('/templates/check/import')">Excel 批量录入</el-button>
        </div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="fetchItems" />
      </div>
      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="sort" label="排序" width="70" align="center" />
        <el-table-column prop="code" label="编码" min-width="130" show-overflow-tooltip />
        <el-table-column prop="name" label="检查项" min-width="170" show-overflow-tooltip />
        <el-table-column prop="standard" label="标准" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.standard || '—' }}</template>
        </el-table-column>
        <el-table-column label="值类型" width="100" align="center">
          <template #default="{ row }">{{ valueTypeLabel(row.value_type) }}</template>
        </el-table-column>
        <el-table-column label="判定规则" min-width="170">
          <template #default="{ row }">{{ ruleText(row) }}</template>
        </el-table-column>
        <el-table-column label="拍照" width="90" align="center">
          <template #default="{ row }">{{ photoLabel(row.photo_required) }}</template>
        </el-table-column>
        <el-table-column label="AI" width="75" align="center">
          <template #default="{ row }">
            <el-tag :type="row.ai_enabled ? 'success' : 'info'" size="small">{{ row.ai_enabled ? '启用' : '关闭' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'inspection:template:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'inspection:template:update'" link type="danger" @click="handleDelete(row)">退役</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无检查项" /></template>
      </el-table>
    </div>

    <el-drawer v-model="formVisible" :title="form.id ? '编辑检查项' : '新增检查项'" size="640px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="96px" scroll-to-error>
        <el-form-item label="项编码" prop="code">
          <el-input v-model="form.code" :disabled="Boolean(form.id)" placeholder="如 pressure_gauge" maxlength="64" />
        </el-form-item>
        <el-form-item label="检查项" prop="name">
          <el-input v-model="form.name" placeholder="如：压力表读数" maxlength="128" />
        </el-form-item>
        <el-form-item label="检查标准">
          <el-input v-model="form.standard" type="textarea" :rows="3" maxlength="500" placeholder="该项需要满足的现场标准" />
        </el-form-item>
        <el-form-item label="值类型" prop="value_type">
          <el-radio-group v-model="form.value_type" @change="changeValueType">
            <el-radio-button v-for="(label, value) in valueTypeLabels" :key="value" :value="value">{{ label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="form.value_type === 'number'">
          <el-form-item label="单位"><el-input v-model="form.unit" placeholder="如 MPa、℃" maxlength="16" /></el-form-item>
          <el-form-item label="允许区间">
            <div class="range-fields">
              <el-input-number v-model="form.value_min" controls-position="right" placeholder="下限" />
              <span>至</span>
              <el-input-number v-model="form.value_max" controls-position="right" placeholder="上限" />
            </div>
          </el-form-item>
        </template>

        <template v-if="form.value_type === 'enum'">
          <el-form-item v-for="(option, index) in form.enum_options" :key="index" :label="'选项 ' + (index + 1)">
            <div class="enum-row">
              <el-input v-model="option.value" placeholder="选项值" maxlength="32" />
              <el-input v-model="option.label" placeholder="显示文案" maxlength="32" />
              <el-select v-model="option.result">
                <el-option label="正常" value="normal" />
                <el-option label="异常" value="abnormal" />
              </el-select>
              <el-button :icon="Delete" circle :disabled="form.enum_options!.length <= 2" @click="form.enum_options!.splice(index, 1)" />
            </div>
          </el-form-item>
          <el-form-item><el-button :icon="Plus" @click="addEnumOption">添加选项</el-button></el-form-item>
        </template>

        <el-form-item label="拍照要求">
          <el-select v-model="form.photo_required">
            <el-option label="无需拍照" value="none" />
            <el-option label="选拍" value="optional" />
            <el-option label="必拍" value="required" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注必填"><el-switch v-model="form.note_required" /></el-form-item>
        <el-form-item v-if="form.value_type === 'ok' || form.value_type === 'enum'" label="异常观察点">
          <div class="tags-editor">
            <el-tag v-for="(tag, index) in form.tags" :key="tag" closable class="tag-chip" @close="form.tags!.splice(index, 1)">{{ tag }}</el-tag>
            <div class="tags-input-row">
              <el-input v-model="tagInput" placeholder="如：水带缺失" maxlength="30" @keyup.enter.prevent="addTag" />
              <el-button :disabled="!tagInput.trim()" @click="addTag">添加</el-button>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="拍照引导">
          <el-input v-model="form.guide" type="textarea" :rows="2" maxlength="200" placeholder="如：打开箱门，拍清水带和枪头" />
        </el-form-item>

        <el-collapse class="advanced-fields">
          <el-collapse-item title="高级信息" name="advanced">
            <el-form-item label="标准编号"><el-input v-model="form.standard_no" maxlength="16" /></el-form-item>
            <el-form-item label="排序号"><el-input-number v-model="form.sort" :min="0" :max="9999" controls-position="right" /></el-form-item>
            <el-form-item label="启用 AI"><el-switch v-model="form.ai_enabled" /></el-form-item>
            <template v-if="form.ai_enabled">
              <el-form-item label="AI 提示">
                <el-select v-model="form.ai_prompt_kind">
                  <el-option v-for="option in promptKinds" :key="option.value" :label="option.label" :value="option.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="识别要点"><el-input v-model="form.ai_hint" type="textarea" :rows="2" maxlength="500" /></el-form-item>
            </template>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onActivated, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { ArrowLeft, Delete, Plus, RefreshRight, Upload } from '@element-plus/icons-vue'
import { createTemplateItem, deleteTemplateItem, getTemplate, listTemplateItems, updateTemplateItem, type TemplateItemForm, type TemplateItemRow } from '@/api/template'
import type { CheckItemEnumOption, CheckItemValueType, PhotoRequired } from '@/api/biz-types'

const valueTypeLabels: Record<CheckItemValueType, string> = { ok: '正常/异常', number: '数值', date: '日期', text: '文本', enum: '枚举' }
const photoLabels: Record<PhotoRequired, string> = { none: '无需', optional: '选拍', required: '必拍' }
const promptKinds = [
  { value: 'general', label: '通用检查' }, { value: 'presence', label: '设施有无' },
  { value: 'damage', label: '破损锈蚀' }, { value: 'metric', label: '表计读数' },
  { value: 'state', label: '状态识别' }, { value: 'label', label: '标签日期' },
  { value: 'passage', label: '通道遮挡' }, { value: 'leak', label: '渗漏痕迹' },
  { value: 'indicator', label: '指示灯' }, { value: 'tidiness', label: '环境整洁' },
  { value: 'baseline', label: '基线对比' }
]
const route = useRoute()
const router = useRouter()
const templateId = route.params.id as string
const loading = ref(false)
const list = ref<TemplateItemRow[]>([])
const tplName = ref('')
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const tagInput = ref('')

type EditorForm = TemplateItemForm & { id: string; sort: number }
const defaultEnumOptions = (): CheckItemEnumOption[] => [
  { value: 'normal', label: '正常', result: 'normal' },
  { value: 'abnormal', label: '异常', result: 'abnormal' }
]
const form = reactive<EditorForm>({
  id: '', code: '', name: '', standard: '', standard_no: '', value_type: 'ok', unit: '',
  value_min: null, value_max: null, enum_options: [], photo_required: 'none', note_required: false,
  ai_enabled: true, ai_prompt_kind: 'general', ai_hint: '', tags: [], guide: '', sort: 0
})
const formRules: FormRules = {
  code: [{ required: true, pattern: /^[a-z][a-z0-9_]{2,63}$/, message: '3~64 位小写字母、数字或下划线，以字母开头', trigger: 'blur' }],
  name: [{ required: true, message: '请输入检查项名称', trigger: 'blur' }],
  value_type: [{ required: true, message: '请选择值类型', trigger: 'change' }]
}
function valueTypeLabel(value: CheckItemValueType) { return valueTypeLabels[value] }
function photoLabel(value: PhotoRequired) { return photoLabels[value] }

function ruleText(row: TemplateItemRow) {
  if (row.value_type === 'number') return `${row.value_min ?? '不限'} ~ ${row.value_max ?? '不限'} ${row.unit || ''}`
  if (row.value_type === 'enum') return (row.enum_options || []).map((option) => `${option.label}=${option.result === 'normal' ? '正常' : '异常'}`).join('、')
  if (row.value_type === 'ok') return 'AI 建议 + 人工确认'
  return '仅记录'
}

async function fetchItems() {
  loading.value = true
  try { list.value = await listTemplateItems(templateId) } finally { loading.value = false }
}
onMounted(() => {
  fetchItems()
  getTemplate(templateId).then((template) => { tplName.value = template.name })
})
let activated = false
onActivated(() => {
  if (!activated) { activated = true; return }
  fetchItems()
})

function resetForm(row?: TemplateItemRow) {
  Object.assign(form, {
    id: row?.id || '', code: row?.code || '', name: row?.name || '', standard: row?.standard || '',
    standard_no: row?.standard_no || '', value_type: row?.value_type || 'ok', unit: row?.unit || '',
    value_min: row?.value_min ?? null, value_max: row?.value_max ?? null,
    enum_options: row?.value_type === 'enum' ? (row.enum_options || defaultEnumOptions()).map((option) => ({ ...option })) : [],
    photo_required: row?.photo_required || 'none', note_required: row?.note_required ?? false,
    ai_enabled: row?.ai_enabled ?? true, ai_prompt_kind: row?.ai_prompt_kind || 'general', ai_hint: row?.ai_hint || '',
    tags: [...(row?.tags || [])], guide: row?.guide || '', sort: row?.sort ?? 0
  })
  tagInput.value = ''
}
function openForm(row?: TemplateItemRow) {
  resetForm(row)
  formRef.value?.clearValidate()
  formVisible.value = true
}
function changeValueType() {
  if (form.value_type !== 'number') { form.unit = ''; form.value_min = null; form.value_max = null }
  if (form.value_type === 'enum' && (!form.enum_options || form.enum_options.length < 2)) form.enum_options = defaultEnumOptions()
  if (form.value_type !== 'enum') form.enum_options = []
  if (form.value_type !== 'ok' && form.value_type !== 'enum') form.tags = []
}
function addEnumOption() {
  form.enum_options!.push({ value: `option_${form.enum_options!.length + 1}`, label: '', result: 'abnormal' })
}
function addTag() {
  const tag = tagInput.value.trim()
  if (!tag) return
  if (form.tags!.includes(tag)) { ElMessage.warning('观察点已存在'); return }
  if (form.tags!.length >= 20) { ElMessage.warning('最多 20 个观察点'); return }
  form.tags!.push(tag)
  tagInput.value = ''
}
async function handleSubmit() {
  await formRef.value?.validate()
  if (form.value_min != null && form.value_max != null && form.value_min > form.value_max) {
    ElMessage.warning('数值下限不能大于上限'); return
  }
  if (form.value_type === 'enum') {
    const values = new Set<string>()
    for (const option of form.enum_options || []) {
      if (!option.value.trim() || !option.label.trim()) { ElMessage.warning('请完整填写枚举选项'); return }
      if (values.has(option.value.trim())) { ElMessage.warning('枚举选项值不能重复'); return }
      values.add(option.value.trim())
    }
  }
  const payload: TemplateItemForm = {
    code: form.code.trim(), name: form.name.trim(), standard: form.standard?.trim(), standard_no: form.standard_no?.trim(),
    value_type: form.value_type, unit: form.unit?.trim(), value_min: form.value_min, value_max: form.value_max,
    enum_options: form.enum_options, photo_required: form.photo_required, note_required: form.note_required,
    ai_enabled: form.ai_enabled, ai_prompt_kind: form.ai_prompt_kind, ai_hint: form.ai_hint?.trim(),
    tags: form.tags, guide: form.guide?.trim(), ...(form.sort == null ? {} : { sort: form.sort })
  }
  submitting.value = true
  try {
    if (form.id) await updateTemplateItem(templateId, form.id, payload)
    else await createTemplateItem(templateId, payload)
    ElMessage.success('检查项已保存')
    formVisible.value = false
    await fetchItems()
  } finally { submitting.value = false }
}
async function handleDelete(row: TemplateItemRow) {
  const confirmed = await ElMessageBox.confirm(`确定退役检查项「${row.name}」吗？退役后编码不可复用，历史记录保留。`, '退役确认', { type: 'warning', confirmButtonText: '退役', cancelButtonText: '取消' }).then(() => true).catch(() => false)
  if (!confirmed) return
  await deleteTemplateItem(templateId, row.id)
  ElMessage.success('检查项已退役')
  await fetchItems()
}
</script>

<style scoped lang="scss">
.items-head { display: flex; align-items: center; gap: $spacing-md; }
.items-title { margin: 0; }
.range-fields { display: flex; align-items: center; gap: $spacing-sm; width: 100%; }
.enum-row { display: grid; grid-template-columns: 1fr 1fr 110px 32px; gap: $spacing-sm; width: 100%; }
.tags-editor, .tags-input-row { width: 100%; }
.tags-input-row { display: flex; gap: $spacing-sm; margin-top: $spacing-sm; }
.tag-chip { margin-right: $spacing-sm; margin-bottom: $spacing-xs; }
.advanced-fields { width: 100%; margin-top: $spacing-md; }
</style>
