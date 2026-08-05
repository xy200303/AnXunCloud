<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="参数名称">
          <el-input v-model="query.name" placeholder="参数名称" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="键名">
          <el-input v-model="query.key" placeholder="如 inspection.fence_default_radius" clearable style="width: 200px" @keyup.enter="handleSearch" />
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
          <el-button v-perms="'system:config:create'" type="primary" :icon="Plus" @click="openForm()">新增参数</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="参数名称" min-width="180" show-overflow-tooltip />
        <el-table-column prop="key" label="键名" min-width="200" show-overflow-tooltip />
        <el-table-column prop="value" label="参数值" min-width="120" />
        <el-table-column prop="remark" label="说明" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || '--' }}</template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="160" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'system:config:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button
              v-perms="'system:config:delete'"
              link
              type="danger"
              @click="handleDelete(row)"
            >删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无参数配置">
            <el-button v-perms="'system:config:create'" type="primary" @click="openForm()">新增参数</el-button>
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

    <!-- 编辑对话框：内置参数键名只读 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑参数' : '新增参数'" width="480px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="参数名称" prop="name">
          <el-input v-model="form.name" placeholder="如：围栏默认半径（米）" />
        </el-form-item>
        <el-form-item label="键名" prop="key">
          <el-input v-model="form.key" :disabled="!!form.id" placeholder="建议 模块.名称 格式" />
        </el-form-item>
        <el-form-item label="参数值" prop="value">
          <el-input v-model="form.value" placeholder="参数值" />
          <div v-if="rangeHint" class="text-secondary">{{ rangeHint }}</div>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.remark" type="textarea" :rows="2" placeholder="选填" />
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
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import { listConfigs, createConfig, updateConfig, deleteConfig } from '@/api/config'
import type { ConfigItem } from '@/api/types'

const loading = ref(false)
const list = ref<ConfigItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, name: '', key: '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listConfigs({ ...query, name: query.name || undefined, key: query.key || undefined })
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
  query.key = ''
  handleSearch()
}

onMounted(fetchList)

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', name: '', key: '', value: '', remark: '' })

// 数值类参数的范围提示（如围栏半径 50-500）
const rangeHint = computed(() => {
  if (form.key === 'inspection.fence_default_radius') return '取值范围 50-500（米）'
  return ''
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入参数名称', trigger: 'blur' }],
  key: [
    { required: true, message: '请输入键名', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_.]{1,63}$/, message: '小写字母开头，可含小写字母数字 . _', trigger: 'blur' }
  ],
  value: [
    { required: true, message: '请输入参数值', trigger: 'blur' },
    {
      validator: (_r: any, value: string, cb: (e?: Error) => void) => {
        // 围栏默认半径做范围校验
        if (form.key === 'inspection.fence_default_radius') {
          const n = Number(value)
          if (Number.isNaN(n) || n < 50 || n > 500) return cb(new Error('围栏半径需在 50-500 之间'))
        }
        cb()
      },
      trigger: 'blur'
    }
  ]
}

function openForm(row?: ConfigItem) {
  formRef.value?.clearValidate()
  Object.assign(form, row
    ? { id: row.id, name: row.name, key: row.key, value: row.value, remark: row.remark }
    : { id: '', name: '', key: '', value: '', remark: '' })
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (form.id) {
      await updateConfig(form.id, { name: form.name, value: form.value, remark: form.remark })
      ElMessage.success('参数已更新，即时生效')
    } else {
      await createConfig(form)
      ElMessage.success('参数已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: ConfigItem) {
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除参数「${row.name}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteConfig(row.id)
  ElMessage.success('已删除')
  fetchList()
}
</script>
