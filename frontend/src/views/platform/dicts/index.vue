<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="类型名称">
          <el-input v-model="query.name" placeholder="类型名称" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="类型编码">
          <el-input v-model="query.code" placeholder="如 point_type" clearable style="width: 180px" @keyup.enter="handleSearch" />
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
          <el-button v-perms="'system:dict:create'" type="primary" :icon="Plus" @click="openTypeForm()">新增类型</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchTypes" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="typeList" stripe style="width: 100%">
        <el-table-column prop="name" label="类型名称" min-width="160" show-overflow-tooltip />
        <el-table-column prop="code" label="编码" min-width="180" show-overflow-tooltip />
        <el-table-column label="字典数据" width="100" align="center">
          <template #default="{ row }">{{ row.data_count ?? '--' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160" />
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="goData(row)">字典数据</el-button>
            <el-button v-perms="'system:dict:update'" link type="primary" @click="openTypeForm(row)">编辑</el-button>
            <el-button
              v-perms="'system:dict:delete'"
              link
              type="danger"
              @click="handleDeleteType(row)"
            >删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无字典类型">
            <el-button v-perms="'system:dict:create'" type="primary" @click="openTypeForm()">新增类型</el-button>
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
          @change="fetchTypes"
        />
      </div>
    </div>

    <!-- 类型编辑对话框 -->
    <el-dialog v-model="typeFormVisible" :title="typeForm.id ? '编辑字典类型' : '新增字典类型'" width="440px" :close-on-click-modal="false">
      <el-form ref="typeFormRef" :model="typeForm" :rules="typeFormRules" label-width="88px">
        <el-form-item label="类型名称" prop="name">
          <el-input v-model="typeForm.name" placeholder="如：点位类型" />
        </el-form-item>
        <el-form-item label="类型编码" prop="code">
          <el-input v-model="typeForm.code" :disabled="!!typeForm.id" placeholder="如：point_type" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="typeFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="typeSubmitting" @click="handleSubmitType">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import { listDictTypes, createDictType, updateDictType, deleteDictType } from '@/api/dict'
import type { DictType } from '@/api/types'

const router = useRouter()

const loading = ref(false)
const typeList = ref<DictType[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, name: '', code: '' })

async function fetchTypes() {
  loading.value = true
  try {
    const data = await listDictTypes({ ...query, name: query.name || undefined, code: query.code || undefined })
    typeList.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchTypes()
}

function handleReset() {
  query.name = ''
  query.code = ''
  handleSearch()
}

onMounted(fetchTypes)

// 进入第二级：字典数据页
function goData(row: DictType) {
  router.push(`/platform/dicts/data/${row.code}`)
}

// ===== 类型表单 =====
const typeFormVisible = ref(false)
const typeSubmitting = ref(false)
const typeFormRef = ref<FormInstance>()
const typeForm = reactive({ id: '', name: '', code: '' })

const typeFormRules: FormRules = {
  name: [{ required: true, message: '请输入类型名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入类型编码', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]{1,63}$/, message: '小写字母开头，仅含小写字母数字下划线', trigger: 'blur' }
  ]
}

function openTypeForm(row?: DictType) {
  typeFormRef.value?.clearValidate()
  Object.assign(typeForm, row ? { id: row.id, name: row.name, code: row.code } : { id: '', name: '', code: '' })
  typeFormVisible.value = true
}

async function handleSubmitType() {
  await typeFormRef.value?.validate()
  typeSubmitting.value = true
  try {
    if (typeForm.id) {
      await updateDictType(typeForm.id, { name: typeForm.name })
      ElMessage.success('字典类型已更新')
    } else {
      await createDictType({ name: typeForm.name, code: typeForm.code })
      ElMessage.success('字典类型已创建')
    }
    typeFormVisible.value = false
    fetchTypes()
  } finally {
    typeSubmitting.value = false
  }
}

async function handleDeleteType(row: DictType) {
  const ok = await ElMessageBox.confirm(
    `删除类型「${row.name}」将级联删除其下全部字典数据，删除后不可恢复，确定删除吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteDictType(row.id)
  ElMessage.success('已删除')
  fetchTypes()
}
</script>
