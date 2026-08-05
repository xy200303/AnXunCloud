<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="账号/姓名">
          <el-input v-model="query.username" placeholder="账号或姓名" clearable style="width: 160px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="query.phone" placeholder="手机号" clearable style="width: 150px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="query.role_id" placeholder="全部角色" clearable style="width: 140px">
            <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
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

    <!-- 工具栏 + 表格 + 分页 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'system:user:create'" type="primary" :icon="Plus" @click="openForm()">新增用户</el-button>
          <el-button v-perms="'system:user:import'" :icon="Download" @click="handleDownloadTemplate">下载模板</el-button>
          <el-button v-perms="'system:user:import'" :icon="Upload" @click="openImport">导入</el-button>
          <el-button v-perms="'system:user:export'" :icon="Document" :loading="exporting" @click="handleExport">导出</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="username" label="账号" min-width="120" show-overflow-tooltip />
        <el-table-column prop="name" label="姓名" min-width="100" />
        <el-table-column prop="phone" label="手机号" min-width="120" />
        <el-table-column label="角色" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.roles?.map((r: RoleBrief) => r.name).join('、') || '--' }}
          </template>
        </el-table-column>
        <el-table-column label="所属小区" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.community_names?.join('、') || '全部小区' }}
          </template>
        </el-table-column>
        <el-table-column label="小程序绑定" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.openid" type="success" size="small">已绑定</el-tag>
            <el-tag v-else type="info" size="small">未绑定</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最近登录" width="160">
          <template #default="{ row }">{{ row.last_login_at || '--' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'system:user:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'system:user:reset-password'" link type="primary" @click="openResetPwd(row)">重置密码</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleRowCommand(cmd, row)">
              <el-button link type="primary">更多<el-icon><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="userStore.hasPerm('system:user:update')" command="status">
                    {{ row.status === 1 ? '停用' : '启用' }}
                  </el-dropdown-item>
                  <el-dropdown-item v-if="userStore.hasPerm('system:user:delete')" command="delete">
                    <span class="danger-text">删除</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无用户数据">
            <el-button v-perms="'system:user:create'" type="primary" @click="openForm()">新增用户</el-button>
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
    <el-dialog v-model="formVisible" :title="form.id ? '编辑用户' : '新增用户'" width="560px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="账号" prop="username">
              <el-input v-model="form.username" :disabled="!!form.id" placeholder="4-32 位字母数字下划线" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="姓名" prop="name">
              <el-input v-model="form.name" placeholder="真实姓名" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="手机号" prop="phone">
              <el-input v-model="form.phone" placeholder="11 位手机号" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item v-if="!form.id" label="初始密码" prop="password">
              <el-input v-model="form.password" type="password" show-password placeholder="8-32 位，含字母与数字" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="角色" prop="role_ids">
          <el-select v-model="form.role_ids" multiple placeholder="选择角色" style="width: 100%">
            <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属小区">
          <el-select v-model="form.community_ids" multiple placeholder="不选则为全部小区" style="width: 100%">
            <el-option v-for="c in communityOptions" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
          <div class="text-secondary">数据权限依据：角色数据范围为「按小区」时生效</div>
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

    <!-- 重置密码对话框：新密码仅展示一次 -->
    <el-dialog v-model="resetVisible" title="重置密码" width="440px" :close-on-click-modal="false">
      <template v-if="!resetResult">
        <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-width="88px">
          <el-form-item label="账号">
            <el-input :model-value="resetTarget?.username" disabled />
          </el-form-item>
          <el-form-item label="新密码" prop="new_password">
            <el-input v-model="resetForm.new_password" type="password" show-password placeholder="8-32 位，含字母与数字" />
          </el-form-item>
        </el-form>
        <div class="text-secondary">重置后该用户全部会话失效，需使用新密码重新登录</div>
      </template>
      <template v-else>
        <el-result icon="success" title="密码已重置" sub-title="新密码仅展示一次，请立即复制保存">
          <template #default>
            <div class="reset-result">
              <code class="reset-pwd">{{ resetResult }}</code>
              <el-button :icon="CopyDocument" @click="copyPassword">复制</el-button>
            </div>
          </template>
        </el-result>
      </template>
      <template #footer>
        <template v-if="!resetResult">
          <el-button @click="resetVisible = false">取消</el-button>
          <el-button type="primary" :loading="resetting" @click="handleResetPwd">重置密码</el-button>
        </template>
        <el-button v-else type="primary" @click="resetVisible = false">完成</el-button>
      </template>
    </el-dialog>

    <!-- 批量导入：三步向导对话框 -->
    <el-dialog v-model="importVisible" title="批量导入用户" width="640px" :close-on-click-modal="false" @closed="resetImport">
      <el-steps :active="importStep" align-center finish-status="success" class="import-steps">
        <el-step title="模板说明" />
        <el-step title="上传文件" />
        <el-step title="导入结果" />
      </el-steps>

      <!-- 第一步：模板说明 -->
      <div v-show="importStep === 0" class="import-pane">
        <el-button :icon="Download" @click="handleDownloadTemplate">下载导入模板 user_import_template.xlsx</el-button>
        <el-table :data="templateFields" border size="small" class="import-fields">
          <el-table-column prop="field" label="字段" width="100" />
          <el-table-column prop="required" label="必填" width="70" align="center" />
          <el-table-column prop="rule" label="填写规则" />
        </el-table>
      </div>

      <!-- 第二步：上传文件 -->
      <div v-show="importStep === 1" class="import-pane">
        <el-upload
          ref="uploadRef"
          drag
          :auto-upload="false"
          :limit="1"
          accept=".xlsx"
          :on-change="handleFileChange"
          :on-remove="() => (importFile = null)"
          :on-exceed="handleFileExceed"
        >
          <el-icon :size="40" class="upload-icon"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处，或 <em>点击选择文件</em></div>
          <template #tip>
            <div class="text-secondary">仅支持 .xlsx，单次最多 500 行，文件 ≤ 5MB</div>
          </template>
        </el-upload>
        <div v-if="importError" class="import-error">{{ importError }}</div>
      </div>

      <!-- 第三步：结果反馈 -->
      <div v-show="importStep === 2" class="import-pane">
        <el-alert
          v-if="importResult"
          :title="`导入完成：成功 ${importResult.success_count} 条，失败 ${importResult.fail_count} 条`"
          :type="importResult.fail_count > 0 ? 'warning' : 'success'"
          :closable="false"
          show-icon
        />
        <template v-if="importResult && importResult.fail_details.length">
          <div class="fail-header">
            <span class="card-title">失败明细</span>
            <el-button size="small" :icon="Download" @click="downloadFailDetails">下载失败明细</el-button>
          </div>
          <el-table :data="importResult.fail_details" border size="small" max-height="260">
            <el-table-column prop="row" label="行号" width="80" align="center" />
            <el-table-column prop="phone" label="手机号" width="150" />
            <el-table-column prop="reason" label="失败原因" />
          </el-table>
          <div class="text-secondary fail-tip">修正失败行后可重新上传，已成功的数据不会重复创建</div>
        </template>
      </div>

      <template #footer>
        <template v-if="importStep === 0">
          <el-button type="primary" @click="importStep = 1">下一步</el-button>
        </template>
        <template v-else-if="importStep === 1">
          <el-button @click="importStep = 0">上一步</el-button>
          <el-button type="primary" :loading="importing" :disabled="!importFile" @click="handleImport">
            开始导入
          </el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="importVisible = false">完成</el-button>
        </template>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  ElMessage, ElMessageBox,
  type FormInstance, type FormRules, type UploadFile, type UploadInstance, type UploadRawFile
} from 'element-plus'
import {
  Search, Refresh, Plus, Download, Upload, Document, RefreshRight, ArrowDown, CopyDocument, UploadFilled
} from '@element-plus/icons-vue'
import {
  listUsers, createUser, updateUser, deleteUser, resetUserPassword, updateUserStatus,
  importUsers, type UserQuery
} from '@/api/user'
import { listRoles } from '@/api/role'
import { listCommunities } from '@/api/community'
import { downloadFile } from '@/utils/download'
import { useUserStore } from '@/store/user'
import type { UserItem, RoleBrief, RoleItem, Community, ImportResult } from '@/api/types'

const userStore = useUserStore()

// ===== 列表 =====
const loading = ref(false)
const list = ref<UserItem[]>([])
const total = ref(0)
const query = reactive<UserQuery>({ page: 1, page_size: 20, username: '', phone: '', role_id: undefined, status: '' })

const roleOptions = ref<RoleItem[]>([])
const communityOptions = ref<Community[]>([])

async function fetchList() {
  loading.value = true
  try {
    const data = await listUsers(query)
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
  query.username = ''
  query.phone = ''
  query.role_id = undefined
  query.status = ''
  handleSearch()
}

onMounted(async () => {
  fetchList()
  // 角色与小区下拉候选
  const [rolesData, communitiesData] = await Promise.all([
    listRoles({ page: 1, page_size: 100 }),
    listCommunities({ page: 1, page_size: 100, status: 1 })
  ])
  roleOptions.value = rolesData.list
  communityOptions.value = communitiesData.list
})

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  username: '',
  password: '',
  name: '',
  phone: '',
  role_ids: [] as string[],
  community_ids: [] as string[],
  status: 1
})

const formRules: FormRules = {
  username: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { pattern: /^\w{4,32}$/, message: '4-32 位字母、数字或下划线', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1\d{10}$/, message: '手机号格式不正确（1 开头 11 位）', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入初始密码', trigger: 'blur' },
    { pattern: /^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/, message: '8-32 位，须含字母与数字', trigger: 'blur' }
  ],
  role_ids: [{ required: true, type: 'array', min: 1, message: '请选择角色', trigger: 'change' }]
}

function openForm(row?: UserItem) {
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id,
      username: row.username,
      password: '',
      name: row.name,
      phone: row.phone,
      role_ids: row.roles?.map((r) => r.id) || [],
      community_ids: [...row.community_ids],
      status: row.status
    })
  } else {
    Object.assign(form, {
      id: '', username: '', password: '', name: '', phone: '',
      role_ids: [], community_ids: [], status: 1
    })
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (form.id) {
      await updateUser(form.id, {
        name: form.name, phone: form.phone,
        role_ids: form.role_ids, community_ids: form.community_ids, status: form.status
      })
      ElMessage.success('用户已更新')
    } else {
      await createUser({
        username: form.username, password: form.password, name: form.name,
        phone: form.phone, role_ids: form.role_ids, community_ids: form.community_ids, status: form.status
      })
      ElMessage.success('用户已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// ===== 行内操作：停用/启用、删除 =====
async function handleRowCommand(cmd: string, row: UserItem) {
  if (cmd === 'status') {
    const target = row.status === 1 ? 0 : 1
    const action = target === 0 ? '停用' : '启用'
    const ok = await ElMessageBox.confirm(
      target === 0
        ? `停用后「${row.name}」将立即无法登录后台与小程序，确定停用吗？`
        : `确定启用「${row.name}」吗？`,
      `${action}确认`,
      { confirmButtonText: action, cancelButtonText: '取消', type: 'warning' }
    ).then(() => true).catch(() => false)
    if (!ok) return
    await updateUserStatus(row.id, target)
    ElMessage.success(`已${action}`)
    fetchList()
  } else if (cmd === 'delete') {
    const ok = await ElMessageBox.confirm(
      `删除后不可恢复，确定删除用户「${row.name}」吗？`,
      '删除确认',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
    ).then(() => true).catch(() => false)
    if (!ok) return
    await deleteUser(row.id)
    ElMessage.success('已删除')
    fetchList()
  }
}

// ===== 重置密码 =====
const resetVisible = ref(false)
const resetting = ref(false)
const resetTarget = ref<UserItem | null>(null)
const resetResult = ref('')
const resetFormRef = ref<FormInstance>()
const resetForm = reactive({ new_password: '' })

const resetRules: FormRules = {
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { pattern: /^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/, message: '8-32 位，须含字母与数字', trigger: 'blur' }
  ]
}

function openResetPwd(row: UserItem) {
  resetTarget.value = row
  resetForm.new_password = ''
  resetResult.value = ''
  resetVisible.value = true
}

async function handleResetPwd() {
  await resetFormRef.value?.validate()
  resetting.value = true
  try {
    await resetUserPassword(resetTarget.value!.id, resetForm.new_password)
    // 新密码仅展示一次
    resetResult.value = resetForm.new_password
  } finally {
    resetting.value = false
  }
}

async function copyPassword() {
  try {
    await navigator.clipboard.writeText(resetResult.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

// ===== 导入导出 =====
const exporting = ref(false)

function handleDownloadTemplate() {
  downloadFile('/system/users/import-template', undefined, 'user_import_template.xlsx')
}

async function handleExport() {
  exporting.value = true
  try {
    await ElMessageBox.confirm('将按当前筛选条件导出用户列表（不含密码等敏感字段）', '导出确认', {
      confirmButtonText: '导出',
      cancelButtonText: '取消',
      type: 'info'
    })
    await downloadFile('/system/users/export', {
      username: query.username || undefined,
      phone: query.phone || undefined,
      role_id: query.role_id,
      status: query.status === '' ? undefined : query.status
    }, `users_export.xlsx`)
    ElMessage.success('导出成功')
  } catch {
    // 取消或失败：拦截器已提示
  } finally {
    exporting.value = false
  }
}

// 三步导入向导
const importVisible = ref(false)
const importStep = ref(0)
const importFile = ref<File | null>(null)
const importError = ref('')
const importing = ref(false)
const importResult = ref<ImportResult | null>(null)
const uploadRef = ref<UploadInstance>()

const templateFields = [
  { field: '姓名', required: '*', rule: '真实姓名' },
  { field: '手机号', required: '*', rule: '11 位，作为登录账号，全表不可重复' },
  { field: '角色', required: '*', rule: '多个用英文逗号分隔，如：巡检员,维修人员' },
  { field: '所属小区', required: '*', rule: '多个用英文逗号分隔，须为已存在的小区名' },
  { field: '初始密码', required: '—', rule: '选填，默认为手机号后 6 位' },
  { field: '状态', required: '—', rule: '启用 / 停用，默认启用' }
]

function openImport() {
  importStep.value = 0
  importFile.value = null
  importError.value = ''
  importResult.value = null
  importVisible.value = true
}

function resetImport() {
  importFile.value = null
  importError.value = ''
  importResult.value = null
  uploadRef.value?.clearFiles()
}

// 前置校验：非 .xlsx 或超限直接红字拒绝，不发起请求
function validateFile(file: File): boolean {
  if (!file.name.endsWith('.xlsx')) {
    importError.value = '文件格式错误：仅支持 .xlsx 文件'
    return false
  }
  if (file.size > 5 * 1024 * 1024) {
    importError.value = '文件大小超限：请控制在 5MB 以内（约 500 行）'
    return false
  }
  importError.value = ''
  return true
}

function handleFileChange(uploadFile: UploadFile) {
  const raw = uploadFile.raw
  if (!raw) return
  if (!validateFile(raw)) {
    uploadRef.value?.clearFiles()
    importFile.value = null
    return
  }
  importFile.value = raw
}

function handleFileExceed(files: File[]) {
  uploadRef.value?.clearFiles()
  const raw = files[0] as UploadRawFile
  if (validateFile(raw)) {
    uploadRef.value?.handleStart(raw)
    importFile.value = raw
  }
}

async function handleImport() {
  if (!importFile.value) return
  importing.value = true
  try {
    importResult.value = await importUsers(importFile.value)
    importStep.value = 2
    if (importResult.value.success_count > 0) fetchList()
  } catch {
    // 拦截器已提示；文件级错误（格式/空文件/超 500 行）停留在当前步可重新选择
  } finally {
    importing.value = false
  }
}

// 失败明细本地导出为 CSV，供修正后重新导入
function downloadFailDetails() {
  if (!importResult.value) return
  const rows = importResult.value.fail_details
    .map((d) => `${d.row},${d.phone},${d.reason}`)
    .join('\n')
  const blob = new Blob([`﻿行号,手机号,失败原因\n${rows}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = '导入失败明细.csv'
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<style scoped lang="scss">
.danger-text {
  color: $color-danger;
}

.import-steps {
  margin-bottom: $spacing-xl;
}

.import-pane {
  min-height: 280px;
}

.import-fields {
  margin-top: $spacing-lg;
}

.upload-icon {
  color: $color-text-secondary;
}

.import-error {
  margin-top: $spacing-sm;
  color: $color-danger;
  font-size: $font-size-aux;
}

.fail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: $spacing-lg 0 $spacing-sm;
}

.fail-tip {
  margin-top: $spacing-sm;
}

.reset-result {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: $spacing-md;
  margin-top: $spacing-md;

  .reset-pwd {
    font-size: $font-size-card-title;
    background: $color-primary-light;
    color: $color-primary;
    padding: $spacing-sm $spacing-lg;
    border-radius: $radius-small;
  }
}
</style>
