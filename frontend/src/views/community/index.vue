<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区名称">
          <el-input v-model="query.name" placeholder="小区名称" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px">
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
          <el-button v-perms="'community:create'" type="primary" :icon="Plus" @click="openForm()">新增小区</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="小区名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="address" label="地址" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">{{ row.address || '--' }}</template>
        </el-table-column>
        <el-table-column prop="manager_name" label="负责人" width="110">
          <template #default="{ row }">{{ row.manager_name || '--' }}</template>
        </el-table-column>
        <el-table-column prop="building_count" label="楼栋数" width="90" align="right" />
        <el-table-column prop="point_count" label="点位数" width="90" align="right" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'community:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button link type="primary" @click="goBuildings(row)">楼栋管理</el-button>
            <el-button v-perms="'community:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无小区">
            <el-button v-perms="'community:create'" type="primary" @click="openForm()">新增小区</el-button>
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
    <el-dialog v-model="formVisible" :title="form.id ? '编辑小区' : '新增小区'" width="480px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="小区名称" prop="name">
          <el-input v-model="form.name" placeholder="小区名称，全局唯一" />
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="form.address" placeholder="选填" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
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
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import { listCommunities, createCommunity, updateCommunity, deleteCommunity } from '@/api/community'
import type { CommunityItem } from '@/api/biz-types'

const router = useRouter()
const loading = ref(false)
const list = ref<CommunityItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, name: '', status: '' as number | '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listCommunities({
      ...query,
      name: query.name || undefined,
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
  query.status = ''
  handleSearch()
}

onMounted(fetchList)

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', name: '', address: '', status: 1 })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入小区名称', trigger: 'blur' }]
}

function openForm(row?: CommunityItem) {
  formRef.value?.clearValidate()
  Object.assign(form, row
    ? { id: row.id, name: row.name, address: row.address, status: row.status }
    : { id: '', name: '', address: '', status: 1 })
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (form.id) {
      await updateCommunity(form.id, form)
      ElMessage.success('小区已更新')
    } else {
      await createCommunity(form)
      ElMessage.success('小区已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// 删除：存在楼栋/点位时后端拒删（42002），错误信息由拦截器展示
async function handleDelete(row: CommunityItem) {
  const ok = await ElMessageBox.confirm(
    row.building_count > 0 || row.point_count > 0
      ? `「${row.name}」下存在 ${row.building_count} 个楼栋、${row.point_count} 个点位，删除会被拒绝，仍需尝试吗？`
      : `删除后不可恢复，确定删除小区「${row.name}」吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteCommunity(row.id)
  ElMessage.success('已删除')
  fetchList()
}

// 跳转楼栋管理并带小区过滤
function goBuildings(row: CommunityItem) {
  router.push({ path: '/community/building', query: { community_id: row.id } })
}
</script>
