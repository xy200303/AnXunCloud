<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="租户">
          <el-input v-model="query.name" placeholder="名称 / 公司代码" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px">
            <el-option label="启用" value="enabled" />
            <el-option label="停用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 工具栏 + 表格 + 分页 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'tenant:create'" type="primary" :icon="Plus" @click="openCreate">开通租户</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="租户名称" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span style="font-weight: 600">{{ row.name }}</span>
            <el-tag v-if="row.code === DEFAULT_TENANT_CODE" type="warning" size="small" style="margin-left: 8px">默认</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="code" label="公司代码" min-width="110" show-overflow-tooltip />
        <el-table-column prop="contact_name" label="联系人" min-width="100">
          <template #default="{ row }">{{ row.contact_name || '--' }}</template>
        </el-table-column>
        <el-table-column prop="contact_phone" label="联系电话" min-width="120">
          <template #default="{ row }">{{ row.contact_phone || '--' }}</template>
        </el-table-column>
        <el-table-column label="用户数" width="90" align="center">
          <template #default="{ row }">{{ row.user_count }} 人</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" />
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'tenant:update'" link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button
              v-perms="'tenant:update'"
              link
              :type="row.status === 1 ? 'danger' : 'primary'"
              :disabled="row.code === DEFAULT_TENANT_CODE && row.status === 1"
              @click="handleToggleStatus(row)"
            >{{ row.status === 1 ? '停用' : '启用' }}</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无租户">
            <el-button v-perms="'tenant:create'" type="primary" @click="openCreate">开通租户</el-button>
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

    <!-- 开通租户对话框：租户信息 + 初始管理员账号 -->
    <el-dialog v-model="createVisible" title="开通租户" width="560px" :close-on-click-modal="false">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="96px">
        <div class="form-section-title">租户信息</div>
        <el-form-item label="租户名称" prop="name">
          <el-input v-model="createForm.name" placeholder="物业公司名称" />
        </el-form-item>
        <el-form-item label="公司代码" prop="code">
          <el-input v-model="createForm.code" placeholder="2-32 位小写字母/数字/中划线，创建后不可改" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="联系人">
              <el-input v-model="createForm.contact_name" placeholder="选填" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="联系电话">
              <el-input v-model="createForm.contact_phone" placeholder="选填" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注">
          <el-input v-model="createForm.remark" type="textarea" :rows="2" placeholder="选填" />
        </el-form-item>

        <div class="form-section-title">初始管理员账号</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="登录账号" prop="admin_username">
              <el-input v-model="createForm.admin_username" placeholder="4-32 位字母数字下划线" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="姓名">
              <el-input v-model="createForm.admin_name" placeholder="选填，默认「租户名+管理员」" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="初始密码" prop="admin_password">
          <el-input v-model="createForm.admin_password" type="password" show-password placeholder="8-32 位，含字母与数字" />
        </el-form-item>
        <div class="text-secondary">管理员首次登录将被强制修改初始密码</div>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreate">开通</el-button>
      </template>
    </el-dialog>

    <!-- 编辑租户对话框（公司代码不可改） -->
    <el-dialog v-model="editVisible" title="编辑租户" width="480px" :close-on-click-modal="false">
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-width="88px">
        <el-form-item label="公司代码">
          <el-input :model-value="editForm.code" disabled />
        </el-form-item>
        <el-form-item label="租户名称" prop="name">
          <el-input v-model="editForm.name" placeholder="物业公司名称" />
        </el-form-item>
        <el-form-item label="联系人">
          <el-input v-model="editForm.contact_name" placeholder="选填" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="editForm.contact_phone" placeholder="选填" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editForm.remark" type="textarea" :rows="2" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import {
  listTenants, createTenant, updateTenant, updateTenantStatus,
  type TenantItem, type TenantQuery
} from '@/api/tenant'

// 默认租户（私有化部署的唯一租户）不可停用，与后端 model.DefaultTenantCode 一致
const DEFAULT_TENANT_CODE = 'default'

// ===== 列表 =====
const loading = ref(false)
const list = ref<TenantItem[]>([])
const total = ref(0)
const query = reactive<TenantQuery>({ page: 1, page_size: 20, name: '', status: '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listTenants(query)
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

// ===== 开通租户 =====
const createVisible = ref(false)
const submitting = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  name: '',
  code: '',
  contact_name: '',
  contact_phone: '',
  remark: '',
  admin_username: '',
  admin_password: '',
  admin_name: ''
})

const createRules: FormRules = {
  name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入公司代码', trigger: 'blur' },
    { pattern: /^[a-z0-9][a-z0-9-]{1,31}$/, message: '2-32 位小写字母/数字/中划线', trigger: 'blur' }
  ],
  admin_username: [
    { required: true, message: '请输入管理员登录账号', trigger: 'blur' },
    { pattern: /^\w{4,32}$/, message: '4-32 位字母、数字或下划线', trigger: 'blur' }
  ],
  admin_password: [
    { required: true, message: '请输入初始密码', trigger: 'blur' },
    { pattern: /^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/, message: '8-32 位，须含字母与数字', trigger: 'blur' }
  ]
}

function openCreate() {
  createFormRef.value?.clearValidate()
  Object.assign(createForm, {
    name: '', code: '', contact_name: '', contact_phone: '', remark: '',
    admin_username: '', admin_password: '', admin_name: ''
  })
  createVisible.value = true
}

async function handleCreate() {
  await createFormRef.value?.validate()
  submitting.value = true
  try {
    await createTenant({ ...createForm })
    ElMessage.success('租户已开通，初始管理员账号已创建')
    createVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// ===== 编辑租户 =====
const editVisible = ref(false)
const editFormRef = ref<FormInstance>()
const editForm = reactive({
  id: '',
  code: '',
  name: '',
  contact_name: '',
  contact_phone: '',
  remark: ''
})

const editRules: FormRules = {
  name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }]
}

function openEdit(row: TenantItem) {
  editFormRef.value?.clearValidate()
  Object.assign(editForm, {
    id: row.id, code: row.code, name: row.name,
    contact_name: row.contact_name, contact_phone: row.contact_phone, remark: row.remark
  })
  editVisible.value = true
}

async function handleEdit() {
  await editFormRef.value?.validate()
  submitting.value = true
  try {
    await updateTenant(editForm.id, {
      name: editForm.name,
      contact_name: editForm.contact_name,
      contact_phone: editForm.contact_phone,
      remark: editForm.remark
    })
    ElMessage.success('租户已更新')
    editVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// ===== 停用 / 启用 =====
async function handleToggleStatus(row: TenantItem) {
  const target = row.status === 1 ? 0 : 1
  const ok = await ElMessageBox.confirm(
    target === 0
      ? `停用后租户「${row.name}」全部账号将立即无法登录，并踢下线该租户全部在线会话，确定停用吗？`
      : `确定启用租户「${row.name}」吗？`,
    `${target === 0 ? '停用' : '启用'}确认`,
    { confirmButtonText: target === 0 ? '停用' : '启用', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await updateTenantStatus(row.id, target)
  ElMessage.success(target === 0 ? '已停用' : '已启用')
  fetchList()
}
</script>

<style scoped lang="scss">
.form-section-title {
  font-size: $font-size-card-title;
  font-weight: 600;
  color: $color-text-primary;
  margin: 0 0 $spacing-lg;
  padding-left: $spacing-md;
  border-left: 3px solid $color-primary;

  &:not(:first-child) {
    margin-top: $spacing-md;
  }
}
</style>
