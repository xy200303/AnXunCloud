<template>
  <div class="app-container">
    <!-- 页头：返回 + 类型信息 -->
    <div class="table-card data-head">
      <el-page-header @back="goBack">
        <template #content>
          <span class="page-title">
            {{ typeInfo ? `${typeInfo.name}（${typeInfo.code}）` : typeCode }}
          </span>
        </template>
      </el-page-header>
    </div>

    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="标签">
          <el-input v-model="query.label" placeholder="标签" clearable style="width: 180px" @keyup.enter="handleSearch" />
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
          <el-button v-perms="'system:dict:create'" type="primary" :icon="Plus" @click="openDataForm()">新增数据</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchData" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="dataList" stripe style="width: 100%">
        <el-table-column prop="label" label="标签" min-width="140" show-overflow-tooltip />
        <el-table-column prop="value" label="存储值" min-width="160" show-overflow-tooltip />
        <el-table-column prop="sort" label="排序" width="80" align="center" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'system:dict:update'" link type="primary" @click="openDataForm(row)">编辑</el-button>
            <el-button v-perms="'system:dict:delete'" link type="danger" @click="handleDeleteData(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该类型暂无数据项">
            <el-button v-perms="'system:dict:create'" type="primary" @click="openDataForm()">新增数据</el-button>
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
          @change="fetchData"
        />
      </div>

      <div class="text-secondary dict-tip">两端字典有缓存，修改后将在下次刷新后生效</div>
    </div>

    <!-- 数据编辑对话框 -->
    <el-dialog v-model="dataFormVisible" :title="dataForm.id ? '编辑字典数据' : '新增字典数据'" width="440px" :close-on-click-modal="false">
      <el-form ref="dataFormRef" :model="dataForm" :rules="dataFormRules" label-width="88px">
        <el-form-item label="所属类型">
          <el-input :model-value="typeInfo ? `${typeInfo.name}（${typeInfo.code}）` : typeCode" disabled />
        </el-form-item>
        <el-form-item label="标签" prop="label">
          <el-input v-model="dataForm.label" placeholder="显示名，如：配电房" />
        </el-form-item>
        <el-form-item label="存储值" prop="value">
          <el-input v-model="dataForm.value" placeholder="如：power_room，同类型下唯一" />
        </el-form-item>
        <!-- 巡查类型专属：大类分组（写入 attrs.category，计划表单按此分组展示） -->
        <el-form-item v-if="typeCode === 'patrol_type'" label="巡查大类">
          <el-radio-group v-model="dataForm.category">
            <el-radio value="daily_patrol">日常巡逻</el-radio>
            <el-radio value="special">专项检查</el-radio>
            <el-radio value="">不分组</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="dataForm.sort" :min="0" :max="999" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="dataForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dataFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="dataSubmitting" @click="handleSubmitData">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import { listDictTypes, listDictData, createDictData, updateDictData, deleteDictData } from '@/api/dict'
import { useTagsViewStore } from '@/store/tagsView'
import type { DictType, DictData } from '@/api/types'

const route = useRoute()
const router = useRouter()
const typeCode = route.params.typeCode as string

// 类型信息（直接刷新/深链时按编码反查名称）
const typeInfo = ref<DictType | null>(null)

async function fetchTypeInfo() {
  try {
    const data = await listDictTypes({ code: typeCode, page: 1, page_size: 1 })
    typeInfo.value = data.list[0] || null
    if (typeInfo.value) {
      // TagsView 标签与浏览器标题显示类型名称
      const title = `字典数据-${typeInfo.value.name}`
      useTagsViewStore().updateTagTitle(route.path, title)
      document.title = `${title} · 安巡云`
    }
  } catch {
    // 类型信息拉取失败不阻断数据列表
  }
}

function goBack() {
  router.push('/platform/dicts')
}

// ===== 字典数据列表 =====
const loading = ref(false)
const dataList = ref<DictData[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, label: '', status: '' as number | '' })

async function fetchData() {
  loading.value = true
  try {
    const data = await listDictData({
      type_code: typeCode,
      page: query.page,
      page_size: query.page_size,
      label: query.label || undefined,
      status: query.status === '' ? undefined : query.status
    })
    dataList.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchData()
}

function handleReset() {
  query.label = ''
  query.status = ''
  handleSearch()
}

onMounted(() => {
  fetchTypeInfo()
  fetchData()
})

// ===== 数据表单 =====
const dataFormVisible = ref(false)
const dataSubmitting = ref(false)
const dataFormRef = ref<FormInstance>()
const dataForm = reactive({ id: '', label: '', value: '', sort: 0, status: 1, category: '' })
// 编辑行原有的 attrs（表单只管理 category，保存时合并回写，避免清掉其他扩展属性）
let editingAttrs: Record<string, string> = {}

const dataFormRules: FormRules = {
  label: [{ required: true, message: '请输入标签', trigger: 'blur' }],
  value: [{ required: true, message: '请输入存储值', trigger: 'blur' }]
}

function openDataForm(row?: DictData) {
  dataFormRef.value?.clearValidate()
  editingAttrs = { ...(row?.attrs || {}) }
  Object.assign(dataForm, row
    ? { id: row.id, label: row.label, value: row.value, sort: row.sort, status: row.status, category: row.attrs?.category || '' }
    : { id: '', label: '', value: '', sort: 0, status: 1, category: '' })
  dataFormVisible.value = true
}

// 组装 attrs：表单只管理 category，其余扩展属性原样保留；空对象传 undefined 落 NULL
function buildAttrs(): Record<string, string> | undefined {
  const attrs = { ...editingAttrs }
  if (dataForm.category) attrs.category = dataForm.category
  else delete attrs.category
  return Object.keys(attrs).length ? attrs : undefined
}

async function handleSubmitData() {
  await dataFormRef.value?.validate()
  dataSubmitting.value = true
  try {
    if (dataForm.id) {
      await updateDictData(dataForm.id, {
        label: dataForm.label, value: dataForm.value, sort: dataForm.sort, status: dataForm.status,
        attrs: buildAttrs()
      })
      ElMessage.success('字典数据已更新')
    } else {
      await createDictData({
        type_code: typeCode,
        label: dataForm.label, value: dataForm.value,
        sort: dataForm.sort, status: dataForm.status,
        attrs: buildAttrs()
      })
      ElMessage.success('字典数据已创建')
    }
    dataFormVisible.value = false
    fetchData()
  } finally {
    dataSubmitting.value = false
  }
}

async function handleDeleteData(row: DictData) {
  const ok = await ElMessageBox.confirm(
    `若已有业务数据引用「${row.label}」，删除后历史数据将显示原值，确定删除吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteDictData(row.id)
  ElMessage.success('已删除')
  fetchData()
}
</script>

<style scoped lang="scss">
.data-head {
  margin-bottom: $spacing-lg;

  :deep(.el-page-header__content) {
    font-size: $font-size-page-title;
    font-weight: 600;
    color: $color-text-primary;
  }
}

.dict-tip {
  margin-top: $spacing-md;
}
</style>
