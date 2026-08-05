<template>
  <div class="app-container">
    <div class="role-layout">
      <!-- 左：角色列表 -->
      <div class="role-list-card">
        <div class="role-list-header">
          <el-input v-model="roleKeyword" placeholder="搜索角色" clearable :prefix-icon="Search" @input="filterRoles" />
          <el-button v-perms="'system:role:create'" type="primary" :icon="Plus" @click="openRoleForm()">新增</el-button>
        </div>
        <div v-loading="roleLoading" class="role-list">
          <div
            v-for="role in filteredRoles"
            :key="role.id"
            class="role-item"
            :class="{ active: currentRole?.id === role.id }"
            @click="selectRole(role)"
          >
            <div class="role-item-main">
              <span class="role-name">{{ role.name }}</span>
              <el-tag v-if="role.status === 0" type="info" size="small">停用</el-tag>
            </div>
            <div class="role-item-sub">
              <span class="role-code">{{ role.code }}</span>
              <span class="role-users">{{ role.user_count }} 人</span>
            </div>
            <div class="role-item-actions" @click.stop>
              <el-button v-perms="'system:role:update'" link type="primary" size="small" @click="openRoleForm(role)">编辑</el-button>
              <el-button
                v-if="role.code !== 'super_admin'"
                v-perms="'system:role:delete'"
                link
                type="danger"
                size="small"
                @click="handleDeleteRole(role)"
              >删除</el-button>
            </div>
          </div>
          <el-empty v-if="!roleLoading && !filteredRoles.length" description="暂无角色" :image-size="60" />
        </div>
      </div>

      <!-- 右：权限配置区 -->
      <div class="role-config-card" v-loading="detailLoading">
        <template v-if="currentRole">
          <div class="config-header">
            <h3 class="card-title">权限配置 · {{ currentRole.name }}</h3>
            <el-tag v-if="isReadonly" type="warning" size="small">内置角色，权限只读</el-tag>
          </div>

          <el-form label-width="88px" class="config-form">
            <el-form-item label="角色名称">
              <el-input :model-value="currentRole.name" disabled style="width: 240px" />
            </el-form-item>
            <el-form-item label="角色编码">
              <el-input :model-value="currentRole.code" disabled style="width: 240px" />
            </el-form-item>
            <el-form-item label="数据范围">
              <el-radio-group v-model="permForm.data_scope" :disabled="isReadonly">
                <el-radio value="all">全部数据</el-radio>
                <el-radio value="custom">按小区</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="permForm.data_scope === 'custom'" label="小区范围">
              <span class="text-secondary">
                按小区过滤时，以各用户「所属小区」为准（在用户管理中维护）
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
                  @check="dirty = true"
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

          <div class="config-footer">
            <el-button @click="selectRole(currentRole, true)">取消</el-button>
            <el-button type="primary" :loading="saving" :disabled="isReadonly" @click="handleSavePerms">
              保存权限
            </el-button>
          </div>
          <div class="text-secondary config-tip">保存后，被影响的用户将在下次刷新或重新登录时生效</div>
        </template>

        <el-empty v-else description="请选择左侧角色进行权限配置">
          <el-button v-perms="'system:role:create'" type="primary" @click="openRoleForm()">新增角色</el-button>
        </el-empty>
      </div>
    </div>

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
            <el-radio value="custom">按小区</el-radio>
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
import { Search, Plus } from '@element-plus/icons-vue'
import { listRoles, getRole, createRole, updateRole, deleteRole, assignRoleMenus } from '@/api/role'
import { listMenus } from '@/api/menu'
import type { RoleItem, MenuNode } from '@/api/types'

// ===== 角色列表 =====
const roleLoading = ref(false)
const roles = ref<RoleItem[]>([])
const roleKeyword = ref('')
const filteredRoles = ref<RoleItem[]>([])
const currentRole = ref<RoleItem | null>(null)

// 内置角色（超管）权限树只读，防误锁死；接口无 builtin 字段，按 code 判断
const isReadonly = computed(() => currentRole.value?.code === 'super_admin')

function filterRoles() {
  const kw = roleKeyword.value.trim()
  filteredRoles.value = kw ? roles.value.filter((r) => r.name.includes(kw) || r.code.includes(kw)) : roles.value
}

async function fetchRoles(keepCurrent = false) {
  roleLoading.value = true
  try {
    const data = await listRoles({ page: 1, page_size: 100 })
    roles.value = data.list
    filterRoles()
    if (keepCurrent && currentRole.value) {
      const still = roles.value.find((r) => r.id === currentRole.value!.id)
      if (still) {
        currentRole.value = still
      }
    }
  } finally {
    roleLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([fetchRoles(), fetchMenuTree()])
  if (filteredRoles.value.length) {
    selectRole(filteredRoles.value[0])
  }
})

// ===== 权限配置 =====
const detailLoading = ref(false)
const saving = ref(false)
const menuTree = ref<MenuNode[]>([])
const menuTreeRef = ref<TreeInstance>()
const checkedMenuIds = ref<string[]>([])
const permForm = reactive({ data_scope: 'all' as 'all' | 'custom' })

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

// 选中角色并加载其权限（force 用于"取消"时还原）
async function selectRole(role: RoleItem, force = false) {
  if (!force && currentRole.value && dirty.value) {
    const ok = await ElMessageBox.confirm('当前角色的权限修改尚未保存，切换后将丢失，确定切换吗？', '未保存的修改', {
      confirmButtonText: '切换',
      cancelButtonText: '继续编辑',
      type: 'warning'
    }).then(() => true).catch(() => false)
    if (!ok) return
  }
  currentRole.value = role
  dirty.value = false
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

// 是否有未保存修改
const dirty = ref(false)

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
    fetchRoles(true)
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
  data_scope: 'all' as 'all' | 'custom',
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
    fetchRoles(true)
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
  if (currentRole.value?.id === role.id) {
    currentRole.value = null
  }
  fetchRoles()
}
</script>

<style scoped lang="scss">
.role-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.role-list-card {
  width: 300px;
  flex-shrink: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg;

  .role-list-header {
    display: flex;
    gap: $spacing-sm;
    margin-bottom: $spacing-md;
  }

  .role-list {
    max-height: calc(100vh - 260px);
    overflow-y: auto;
  }

  .role-item {
    position: relative;
    padding: $spacing-md;
    border-radius: $radius-small;
    cursor: pointer;
    margin-bottom: $spacing-xs;
    border: 1px solid transparent;

    &:hover {
      background: $color-bg-page;

      .role-item-actions {
        display: flex;
      }
    }

    &.active {
      background: $color-primary-light;
      border-color: $color-primary;
    }

    .role-item-main {
      display: flex;
      align-items: center;
      gap: $spacing-sm;

      .role-name {
        font-weight: 600;
        color: $color-text-primary;
      }
    }

    .role-item-sub {
      display: flex;
      justify-content: space-between;
      margin-top: $spacing-xs;
      font-size: $font-size-aux;
      color: $color-text-secondary;
    }

    .role-item-actions {
      display: none;
      position: absolute;
      right: $spacing-sm;
      top: $spacing-sm;
      background: inherit;
    }
  }
}

.role-config-card {
  flex: 1;
  min-width: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg $spacing-xl;

  .config-header {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    margin-bottom: $spacing-xl;
  }

  .config-form {
    max-width: 720px;
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

  .config-footer {
    margin-top: $spacing-xl;
  }

  .config-tip {
    margin-top: $spacing-sm;
  }
}
</style>
