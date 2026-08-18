<template>
  <div class="app-container">

    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form inline @submit.prevent>
        <el-form-item label="角色名称">
          <el-input v-model="roleKeyword" placeholder="名称 / 编码" clearable style="width: 200px" @input="filterRoles" />
        </el-form-item>
      </el-form>
    </div>

    <!-- 角色表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'system:role:create'" type="primary" :icon="Plus" @click="openRoleForm()">新增角色</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchRoles()" />
        </el-tooltip>
      </div>

      <el-table v-loading="roleLoading" :data="filteredRoles" style="width: 100%">
        <el-table-column prop="name" label="角色名称" min-width="140">
          <template #default="{ row }">
            <span style="font-weight: 600">{{ row.name }}</span>
            <el-tag v-if="isBuiltinRole(row)" type="warning" size="small" style="margin-left: 8px">内置</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="code" label="角色编码" min-width="140" show-overflow-tooltip />
        <el-table-column label="数据范围" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="dataScopeMetaOf(row.data_scope).type" size="small">
              {{ dataScopeMetaOf(row.data_scope).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="user_count" label="用户数" width="90" align="center">
          <template #default="{ row }">{{ row.user_count }} 人</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || '--' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button v-perms="'system:role:update'" link type="primary" @click="openPermDrawer(row)">配置权限</el-button>
            <el-tooltip
              :disabled="!isBuiltinRole(row) || userStore.isSuperAdmin"
              content="内置角色全平台共享，仅超管可维护"
              placement="top"
            >
              <span>
                <el-button
                  v-perms="'system:role:update'"
                  link
                  type="primary"
                  :disabled="isBuiltinRole(row) && !userStore.isSuperAdmin"
                  @click="openRoleForm(row)"
                >编辑</el-button>
              </span>
            </el-tooltip>
            <!-- 内置角色后端禁止删除，前端直接不展示删除入口 -->
            <el-button
              v-if="!isBuiltinRole(row)"
              v-perms="'system:role:delete'"
              link
              type="danger"
              @click="handleDeleteRole(row)"
            >删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无角色">
            <el-button v-perms="'system:role:create'" type="primary" @click="openRoleForm()">新增角色</el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <!-- 权限配置抽屉 -->
    <el-drawer
      v-model="permVisible"
      :title="`配置权限 · ${currentRole?.name ?? ''}`"
      size="520px"
      :before-close="handleDrawerClose"
    >
      <div v-loading="detailLoading" class="perm-drawer-body">
        <el-alert
          v-if="isReadonly"
          title="内置角色，权限只读"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 16px"
        />

        <el-form label-width="88px">
          <el-form-item label="数据范围">
            <el-radio-group v-model="permForm.data_scope" :disabled="isReadonly" @change="dirty = true">
              <el-radio value="all">全部数据</el-radio>
              <el-radio value="project">所在项目</el-radio>
              <el-radio value="self">仅本人</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="permForm.data_scope === 'project'" label="项目范围">
            <span class="text-secondary">
              按项目过滤时，以各用户在项目「岗位编制」内的在职名单为准（在小区管理 · 编制中维护）
            </span>
          </el-form-item>
          <el-form-item label="菜单权限">
            <div class="menu-tree-wrap">
              <div class="menu-tree-toolbar">
                <el-checkbox
                  :model-value="allChecked"
                  :indeterminate="halfChecked"
                  :disabled="isReadonly"
                  @change="toggleAll"
                >全选</el-checkbox>
                <span class="text-secondary">父子联动勾选，按钮级为叶子节点</span>
              </div>
              <el-tree
                ref="menuTreeRef"
                :data="menuTree"
                node-key="id"
                show-checkbox
                default-expand-all
                :props="{ label: 'title', children: 'children' }"
                :default-checked-keys="checkedMenuIds"
                @check="syncChecked"
              >
                <template #default="{ data }">
                  <span class="tree-node">
                    {{ data.title }}
                    <el-tag v-if="data.type === 'button'" size="small" type="info" class="node-tag">按钮</el-tag>
                    <el-tag v-else-if="data.type === 'dir'" size="small" type="warning" class="node-tag">目录</el-tag>
                  </span>
                </template>
              </el-tree>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <div class="perm-drawer-footer">
          <span class="text-secondary">保存后，被影响的用户将在下次刷新或重新登录时生效</span>
          <div>
            <el-button @click="handleDrawerClose()">取消</el-button>
            <el-button type="primary" :loading="saving" :disabled="isReadonly" @click="handleSavePerms">保存权限</el-button>
          </div>
        </div>
      </template>
    </el-drawer>

    <!-- 新增/编辑角色对话框 -->
    <el-dialog v-model="roleFormVisible" :title="roleForm.id ? '编辑角色' : '新增角色'" width="480px" :close-on-click-modal="false">
      <el-form ref="roleFormRef" :model="roleForm" :rules="roleFormRules" label-width="88px">
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="如：物业主管" />
        </el-form-item>
        <el-form-item label="角色编码" prop="code">
          <el-input v-model="roleForm.code" :disabled="!!roleForm.id" placeholder="如：manager" />
        </el-form-item>
        <el-form-item label="数据范围" prop="data_scope">
          <el-radio-group v-model="roleForm.data_scope">
            <el-radio value="all">全部数据</el-radio>
            <el-radio value="project">所在项目</el-radio>
            <el-radio value="self">仅本人</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="roleForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="roleForm.remark" type="textarea" :rows="2" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="roleSubmitting" @click="handleSubmitRole">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type TreeInstance } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { listRoles, getRole, createRole, updateRole, deleteRole, assignRoleMenus } from '@/api/role'
import { listMenus } from '@/api/menu'
import { useUserStore } from '@/store/user'
import type { RoleItem, MenuNode, DataScope } from '@/api/types'

const userStore = useUserStore()

// 内置角色编码（tenant_id 为空的平台共享角色，与后端 seed 一致；列表接口不下发 is_builtin，按 code 判定）
const BUILTIN_ROLE_CODES = ['super_admin', 'tenant_admin', 'project_admin', 'field_staff']

function isBuiltinRole(role: RoleItem) {
  return BUILTIN_ROLE_CODES.includes(role.code)
}

// 数据范围三档（all 全部 / project 所在项目 / self 仅本人；迁移后字典同步）
const dataScopeMeta: Record<DataScope, { label: string; type: 'success' | 'primary' | 'info' }> = {
  all: { label: '全部数据', type: 'success' },
  project: { label: '所在项目', type: 'primary' },
  self: { label: '仅本人', type: 'info' }
}

function dataScopeMetaOf(scope: string) {
  return dataScopeMeta[scope as DataScope] || { label: scope, type: 'info' as const }
}

// ===== 角色列表 =====
const roleLoading = ref(false)
const roles = ref<RoleItem[]>([])
const roleKeyword = ref('')
const filteredRoles = ref<RoleItem[]>([])

function filterRoles() {
  const kw = roleKeyword.value.trim()
  filteredRoles.value = kw ? roles.value.filter((r) => r.name.includes(kw) || r.code.includes(kw)) : roles.value
}

async function fetchRoles() {
  roleLoading.value = true
  try {
    const data = await listRoles({ page: 1, page_size: 100 })
    roles.value = data.list
    filterRoles()
  } finally {
    roleLoading.value = false
  }
}

onMounted(() => {
  fetchRoles()
  fetchMenuTree()
})


// ===== 权限配置抽屉 =====
const permVisible = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const currentRole = ref<RoleItem | null>(null)
const menuTree = ref<MenuNode[]>([])
const menuTreeRef = ref<TreeInstance>()
const checkedMenuIds = ref<string[]>([])
const permForm = reactive({ data_scope: 'all' as DataScope })

// 超管角色权限树恒只读（后端禁止改，防误锁死）；其余内置角色仅超管可维护，非超管只读
const isReadonly = computed(() => {
  const role = currentRole.value
  if (!role) return false
  return role.code === 'super_admin' || (isBuiltinRole(role) && !userStore.isSuperAdmin)
})

// 全选状态
const allChecked = computed(() => {
  const allIds = collectLeafIds(menuTree.value)
  return allIds.length > 0 && allIds.every((id) => checkedMenuIds.value.includes(id))
})
const halfChecked = computed(() => !allChecked.value && checkedMenuIds.value.length > 0)

function collectLeafIds(nodes: MenuNode[]): string[] {
  const ids: string[] = []
  const walk = (list: MenuNode[]) => {
    for (const n of list) {
      if (n.children?.length) walk(n.children)
      else ids.push(n.id)
    }
  }
  walk(nodes)
  return ids
}

async function fetchMenuTree() {
  menuTree.value = await listMenus()
}

// 打开抽屉并加载角色权限
async function openPermDrawer(role: RoleItem) {
  currentRole.value = role
  dirty.value = false
  permVisible.value = true
  detailLoading.value = true
  try {
    const detail = await getRole(role.id)
    permForm.data_scope = detail.data_scope
    // el-tree 勾选：仅回显叶子，父级由联动推导
    await nextTick()
    checkedMenuIds.value = detail.menu_ids || []
    menuTreeRef.value?.setCheckedKeys(detail.menu_ids || [])
  } finally {
    detailLoading.value = false
  }
}

// 关闭前确认未保存修改（before-close 与取消按钮共用）
function handleDrawerClose(done?: () => void) {
  const close = () => {
    permVisible.value = false
    dirty.value = false
  }
  if (!dirty.value) {
    close()
    done?.()
    return
  }
  ElMessageBox.confirm('权限修改尚未保存，关闭后将丢失，确定关闭吗？', '未保存的修改', {
    confirmButtonText: '关闭',
    cancelButtonText: '继续编辑',
    type: 'warning'
  }).then(() => {
    close()
    done?.()
  }).catch(() => {})
}

function toggleAll(checked: boolean | string | number) {
  if (checked) {
    menuTreeRef.value?.setCheckedKeys(collectLeafIds(menuTree.value))
  } else {
    menuTreeRef.value?.setCheckedKeys([])
  }
  syncChecked()
}

function syncChecked() {
  const tree = menuTreeRef.value
  if (!tree) return
  // getCheckedKeys 含全选中的父级，再补半选父级，保证目录/菜单级权限不丢
  checkedMenuIds.value = [...tree.getCheckedKeys(), ...tree.getHalfCheckedKeys()] as string[]
  dirty.value = true
}

async function handleSavePerms() {
  if (!currentRole.value) return
  syncChecked()
  saving.value = true
  try {
    await assignRoleMenus(currentRole.value.id, {
      menu_ids: checkedMenuIds.value,
      data_scope: permForm.data_scope
    })
    ElMessage.success('权限已保存')
    dirty.value = false
    permVisible.value = false
    fetchRoles()
  } finally {
    saving.value = false
  }
}

// ===== 角色新增/编辑/删除 =====
const roleFormVisible = ref(false)
const roleSubmitting = ref(false)
const roleFormRef = ref<FormInstance>()
const roleForm = reactive({
  id: '',
  name: '',
  code: '',
  data_scope: 'all' as DataScope,
  status: 1,
  remark: ''
})

const roleFormRules: FormRules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入角色编码', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]{1,31}$/, message: '小写字母开头，仅含小写字母数字下划线', trigger: 'blur' }
  ],
  data_scope: [{ required: true, message: '请选择数据范围', trigger: 'change' }]
}

function openRoleForm(role?: RoleItem) {
  roleFormRef.value?.clearValidate()
  Object.assign(roleForm, role
    ? { id: role.id, name: role.name, code: role.code, data_scope: role.data_scope, status: role.status, remark: role.remark }
    : { id: '', name: '', code: '', data_scope: 'all', status: 1, remark: '' })
  roleFormVisible.value = true
}

async function handleSubmitRole() {
  await roleFormRef.value?.validate()
  roleSubmitting.value = true
  try {
    if (roleForm.id) {
      await updateRole(roleForm.id, roleForm)
      ElMessage.success('角色已更新')
    } else {
      await createRole(roleForm)
      ElMessage.success('角色已创建')
    }
    roleFormVisible.value = false
    fetchRoles()
  } finally {
    roleSubmitting.value = false
  }
}

async function handleDeleteRole(role: RoleItem) {
  if (role.user_count > 0) {
    ElMessage.warning(`角色「${role.name}」下存在 ${role.user_count} 个用户，请先调整用户角色后再删除`)
    return
  }
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除角色「${role.name}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteRole(role.id)
  ElMessage.success('已删除')
  fetchRoles()
}
</script>

<style scoped lang="scss">
.perm-drawer-body {
  min-height: 200px;
}

.menu-tree-wrap {
  width: 100%;
  border: 1px solid $color-border;
  border-radius: $radius-small;
  padding: $spacing-md;

  .menu-tree-toolbar {
    display: flex;
    align-items: center;
    gap: $spacing-lg;
    padding-bottom: $spacing-sm;
    border-bottom: 1px solid $color-border;
    margin-bottom: $spacing-sm;
  }

  .tree-node {
    display: inline-flex;
    align-items: center;
    gap: $spacing-sm;

    .node-tag {
      transform: scale(0.9);
    }
  }
}

.perm-drawer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: $spacing-md;
}
</style>
