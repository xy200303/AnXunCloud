<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="模板名称">
          <el-input v-model="query.name" placeholder="模板名称" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="适用类型">
          <el-select v-model="query.point_type" placeholder="全部类型" clearable style="width: 140px">
            <el-option label="通用（所有类型）" value="" />
            <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部" clearable style="width: 110px">
            <el-option label="启用" :value="1" />
            <el-option label="停用" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'inspection:template:create'" type="primary" :icon="Plus" @click="openForm()">新增模板</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="模板名称" min-width="160" show-overflow-tooltip />
        <el-table-column label="适用类型" width="140" align="center">
          <template #default="{ row }">{{ row.point_type ? pointTypeLabel(row.point_type) : '通用' }}</template>
        </el-table-column>
        <el-table-column label="检查项数" width="90" align="right">
          <template #default="{ row }">{{ row.items?.length || 0 }}</template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="70" align="center" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'inspection:template:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'inspection:template:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无检查项模板">
            <el-button v-perms="'inspection:template:create'" type="primary" @click="openForm()">新增模板</el-button>
          </el-empty>
        </template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.page_size"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑模板' : '新增模板'" width="640px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="模板名称" prop="name">
          <el-input v-model="form.name" placeholder="如：消防设施日检" maxlength="50" show-word-limit />
        </el-form-item>
        <el-form-item label="适用类型">
          <el-select v-model="form.point_type" placeholder="通用（所有类型）" clearable style="width: 100%">
            <el-option label="通用（所有类型）" value="" />
            <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
          </el-select>
        </el-form-item>

        <!-- 检查项动态行编辑器：至少 1 项，名称非空 -->
        <el-form-item label="检查项" prop="items">
          <div class="check-items">
            <div v-for="(item, i) in form.items" :key="i" class="check-item">
              <el-input v-model="item.name" placeholder="检查项名称，如：灭火器压力正常" style="flex: 1" />
              <el-switch v-model="item.required" inline-prompt active-text="必检" inactive-text="选检" />
              <el-button link type="danger" :icon="Delete" :disabled="form.items.length <= 1" @click="form.items.splice(i, 1)" />
            </div>
            <el-button :icon="Plus" size="small" @click="form.items.push({ name: '', required: true })">添加一项</el-button>
            <div class="text-secondary">至少 1 项；打卡时巡检员逐项确认是否合格</div>
          </div>
        </el-form-item>

        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="200" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Delete } from '@element-plus/icons-vue'
import { listTemplates, createTemplate, updateTemplate, deleteTemplate, type TemplateQuery } from '@/api/template'
import { listDictData } from '@/api/dict'
import type { TemplateItem, TemplateCheckItem } from '@/api/biz-types'
import type { DictData } from '@/api/types'

const loading = ref(false)
const list = ref<TemplateItem[]>([])
const total = ref(0)
const query = reactive<TemplateQuery>({ page: 1, page_size: 20, name: '', point_type: '', status: '' })

// 点位类型字典（与点位页同一字典）
const pointTypeOptions = ref<DictData[]>([])

function pointTypeLabel(value: string) {
  return pointTypeOptions.value.find((d) => d.value === value)?.label || value
}

async function fetchList() {
  loading.value = true
  try {
    const data = await listTemplates({
      ...query,
      name: query.name || undefined,
      point_type: query.point_type || undefined,
      status: query.status === '' ? undefined : query.status
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.name = ''
  query.point_type = ''
  query.status = ''
  handleSearch()
}

onMounted(() => {
  fetchList()
  listDictData({ type_code: 'point_type', page: 1, page_size: 100 }).then((d) => {
    pointTypeOptions.value = d.list.filter((x) => x.status === 1)
  })
})

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  name: '',
  point_type: '',
  items: [{ name: '', required: true }] as TemplateCheckItem[],
  sort: 0,
  status: 1,
  remark: ''
})

// 检查项校验：至少 1 项、名称非空（去重）
function validateItems(_rule: unknown, _value: unknown, callback: (e?: Error) => void) {
  const names = form.items.map((x) => x.name.trim())
  if (!names.length) return callback(new Error('至少添加 1 个检查项'))
  if (names.some((n) => !n)) return callback(new Error('检查项名称不能为空'))
  if (new Set(names).size !== names.length) return callback(new Error('检查项名称不能重复'))
  callback()
}

const formRules: FormRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  items: [{ validator: validateItems, trigger: 'change' }]
}

function openForm(row?: TemplateItem) {
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id,
      name: row.name,
      point_type: row.point_type || '',
      items: row.items?.length ? row.items.map((x) => ({ ...x })) : [{ name: '', required: true }],
      sort: row.sort,
      status: row.status,
      remark: row.remark || ''
    })
  } else {
    Object.assign(form, {
      id: '', name: '', point_type: '', items: [{ name: '', required: true }],
      sort: 0, status: 1, remark: ''
    })
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  const payload = {
    name: form.name.trim(),
    point_type: form.point_type,
    items: form.items.map((x) => ({ name: x.name.trim(), required: x.required })),
    sort: form.sort,
    status: form.status,
    remark: form.remark
  }
  submitting.value = true
  try {
    if (form.id) {
      await updateTemplate(form.id, payload)
      ElMessage.success('模板已更新')
    } else {
      await createTemplate(payload)
      ElMessage.success('模板已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// 删除：被点位引用时后端拒删，错误信息由拦截器展示
async function handleDelete(row: TemplateItem) {
  const ok = await ElMessageBox.confirm(
    `确定删除模板「${row.name}」吗？被点位引用的模板无法删除。`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteTemplate(row.id)
  ElMessage.success('已删除')
  fetchList()
}
</script>

<style scoped lang="scss">
.check-items {
  width: 100%;

  .check-item {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    margin-bottom: $spacing-sm;
  }
}
</style>
