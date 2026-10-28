<template>
  <div class="app-container">
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="模板名称">
          <el-input v-model="query.name" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="点位类型">
          <el-select v-model="query.point_type" placeholder="全部类型" clearable style="width: 150px">
            <el-option v-for="item in pointTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="用途">
          <el-select v-model="query.purpose" placeholder="全部用途" clearable style="width: 130px">
            <el-option label="日常巡检" value="patrol" />
            <el-option label="专项抽查" value="spot" />
            <el-option label="承接查验" value="verify" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px">
            <el-option label="草稿" value="draft" />
            <el-option label="启用" value="enabled" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'inspection:template:create'" type="primary" :icon="Plus" @click="openForm()">新建检查项模板</el-button>
          <el-button v-perms="'inspection:template:update'" :icon="Upload" @click="router.push('/templates/check/import')">批量录入</el-button>
        </div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="fetchList" />
      </div>
      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="模板名称" min-width="180" show-overflow-tooltip />
        <el-table-column label="点位类型" min-width="130">
          <template #default="{ row }">{{ pointTypeLabel(row.point_type) }}</template>
        </el-table-column>
        <el-table-column label="用途" width="110">
          <template #default="{ row }">{{ purposeLabels[row.purpose as TemplatePurpose] }}</template>
        </el-table-column>
        <el-table-column label="检查项数" width="100" align="right">
          <template #default="{ row }">{{ row.items?.length || 0 }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.status === 'enabled' ? '启用' : '草稿' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="165" />
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'inspection:template:update'" link type="primary" @click="goItems(row)">检查项</el-button>
            <el-button v-perms="'inspection:template:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-if="row.status === 'enabled'" v-perms="'inspection:template:update'" link type="warning" @click="disableTemplate(row)">停用</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="还没有检查项模板" /></template>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.page_size" :total="total" :page-sizes="[10, 20, 50, 100]" layout="total, sizes, prev, pager, next" @change="fetchList" />
      </div>
    </div>

    <el-dialog v-model="formVisible" :title="form.id ? '编辑模板' : '新建检查项模板'" width="560px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
        <el-form-item label="模板名称" prop="name"><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="点位类型" prop="point_type">
          <el-select v-model="form.point_type" placeholder="请选择点位类型" style="width: 100%">
            <el-option v-for="item in pointTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="用途" prop="purpose">
          <el-radio-group v-model="form.purpose">
            <el-radio-button value="patrol">日常巡检</el-radio-button>
            <el-radio-button value="spot">专项抽查</el-radio-button>
            <el-radio-button value="verify">承接查验</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" :max="9999" controls-position="right" /></el-form-item>
        <el-form-item v-if="form.id" label="状态">
          <el-radio-group v-model="form.status">
            <el-radio value="draft">草稿</el-radio>
            <el-radio value="enabled" :disabled="!form.itemCount">发布启用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onActivated, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Upload } from '@element-plus/icons-vue'
import { listTemplates, createTemplate, updateTemplate } from '@/api/template'
import { useDictOptions } from '@/composables/useDictOptions'
import { usePagedList } from '@/composables/usePagedList'
import type { TemplateItem, TemplatePurpose, TemplateStatus } from '@/api/biz-types'

const router = useRouter()
const { loading, list, total, query, fetchList, handleSearch, handleReset } = usePagedList(
  (q) => listTemplates({ ...q, name: q.name || undefined, point_type: q.point_type || undefined, purpose: q.purpose || undefined, status: q.status || undefined }),
  () => ({ page: 1, page_size: 20, name: '', point_type: '', purpose: '' as TemplatePurpose | '', status: '' as TemplateStatus | '' })
)
const { options: pointTypeOptions } = useDictOptions('point_type')
const purposeLabels: Record<TemplatePurpose, string> = { patrol: '日常巡检', spot: '专项抽查', verify: '承接查验' }
const pointTypeLabel = (value: string) => pointTypeOptions.value.find((item) => item.value === value)?.label || value
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', name: '', point_type: '', purpose: 'patrol' as TemplatePurpose, sort: 0, status: 'draft' as TemplateStatus, remark: '', itemCount: 0 })
const rules: FormRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  point_type: [{ required: true, message: '请选择点位类型', trigger: 'change' }],
  purpose: [{ required: true, message: '请选择用途', trigger: 'change' }]
}
function openForm(row?: TemplateItem) {
  Object.assign(form, row ? { id: row.id, name: row.name, point_type: row.point_type, purpose: row.purpose, sort: row.sort, status: row.status, remark: row.remark, itemCount: row.items.length } : { id: '', name: '', point_type: '', purpose: 'patrol', sort: 0, status: 'draft', remark: '', itemCount: 0 })
  formRef.value?.clearValidate()
  formVisible.value = true
}
const goItems = (row: TemplateItem) => router.push(`/templates/check/${row.id}/items`)
async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const payload = { name: form.name.trim(), point_type: form.point_type, purpose: form.purpose, sort: form.sort, status: form.id ? form.status : 'draft' as TemplateStatus, remark: form.remark }
    if (form.id) {
      await updateTemplate(form.id, payload)
      ElMessage.success('模板已保存')
    } else {
      const created = await createTemplate(payload)
      formVisible.value = false
      router.push(`/templates/check/${created.id}/items`)
      return
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}
async function disableTemplate(row: TemplateItem) {
  await ElMessageBox.confirm(`停用「${row.name}」后，新点位不能再选择该模板，历史记录不变。`, '停用模板', { type: 'warning' })
  await updateTemplate(row.id, { name: row.name, point_type: row.point_type, purpose: row.purpose, sort: row.sort, status: 'draft', remark: row.remark })
  ElMessage.success('模板已停用')
  fetchList()
}
onMounted(fetchList)
let activated = false
onActivated(() => {
  if (!activated) { activated = true; return }
  fetchList()
})
</script>
