<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="菜单名称">
          <el-input v-model="query.title" placeholder="菜单名称" clearable style="width: 180px" @keyup.enter="fetchList" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px">
            <el-option label="启用" :value="1" />
            <el-option label="停用" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="fetchList">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 树形表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'system:menu:create'" type="primary" :icon="Plus" @click="openForm()">新增菜单</el-button>
          <el-button :icon="Sort" @click="toggleExpand">展开/折叠</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table
        v-if="tableVisible"
        ref="tableRef"
        v-loading="loading"
        :data="treeData"
        row-key="id"
        :default-expand-all="expandAll"
        :tree-props="{ children: 'children' }"
        style="width: 100%"
      >
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column prop="icon" label="图标" width="80" align="center">
          <template #default="{ row }">
            <el-icon v-if="row.icon"><MenuIcon :name="row.icon" /></el-icon>
            <span v-else>--</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.type === 'dir'" type="warning" size="small">目录</el-tag>
            <el-tag v-else-if="row.type === 'menu'" type="success" size="small">菜单</el-tag>
            <el-tag v-else type="info" size="small">按钮</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路由路径" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.path || '--' }}</template>
        </el-table-column>
        <el-table-column prop="perms" label="权限标识" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.perms || '--' }}</template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="70" align="center" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180">
          <template #default="{ row }">
            <el-button v-perms="'system:menu:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'system:menu:create'" link type="primary" @click="openForm(undefined, row)">新增下级</el-button>
            <el-button v-perms="'system:menu:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无菜单数据">
            <el-button v-perms="'system:menu:create'" type="primary" @click="openForm()">新增菜单</el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <!-- 编辑对话框：类型单选联动字段显隐 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑菜单' : '新增菜单'" width="560px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="上级菜单" prop="parent_id">
          <el-tree-select
            v-model="form.parent_id"
            :data="parentOptions"
            node-key="id"
            :props="{ label: 'title', children: 'children' }"
            check-strictly
            default-expand-all
            placeholder="顶级菜单"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="form.type">
            <el-radio value="dir">目录</el-radio>
            <el-radio value="menu">菜单</el-radio>
            <el-radio value="button">按钮</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" placeholder="显示名称" />
        </el-form-item>
        <el-form-item v-if="form.type !== 'button'" label="路由路径" prop="path">
          <el-input v-model="form.path" placeholder="如 /system/user，与 views 目录约定对应" />
        </el-form-item>
        <el-form-item v-if="form.type !== 'button'" label="图标">
          <el-select v-model="form.icon" placeholder="选择图标" clearable filterable style="width: 100%">
            <el-option v-for="name in menuIconOptions" :key="name" :label="name" :value="name">
              <span style="display: inline-flex; align-items: center; gap: 8px">
                <el-icon><MenuIcon :name="name" /></el-icon>{{ name }}
              </span>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.type !== 'dir'" label="权限标识" prop="perms">
          <el-input v-model="form.perms" placeholder="如 system:user:list" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="排序">
              <el-input-number v-model="form.sort" :min="0" :max="999" controls-position="right" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item v-if="form.type === 'menu'" label="显隐">
              <el-radio-group v-model="form.visible">
                <el-radio :value="1">显示</el-radio>
                <el-radio :value="0">隐藏</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="0">停用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, Sort, RefreshRight } from '@element-plus/icons-vue'
import { listMenus, createMenu, updateMenu, deleteMenu } from '@/api/menu'
import type { MenuNode } from '@/api/types'
import MenuIcon from '@/components/MenuIcon.vue'
import { menuIconOptions } from '@/utils/menuIcons'

const loading = ref(false)
const treeData = ref<MenuNode[]>([])
const query = reactive({ title: '', status: '' as number | '' })

// 展开/折叠：重建表格以应用 default-expand-all
const tableRef = ref()
const tableVisible = ref(true)
const expandAll = ref(true)

async function toggleExpand() {
  expandAll.value = !expandAll.value
  tableVisible.value = false
  await nextTick()
  tableVisible.value = true
}

async function fetchList() {
  loading.value = true
  try {
    treeData.value = await listMenus({ title: query.title || undefined, status: query.status === '' ? undefined : query.status })
  } finally {
    loading.value = false
  }
}

function handleReset() {
  query.title = ''
  query.status = ''
  fetchList()
}

onMounted(fetchList)

// 上级菜单候选：仅目录/菜单可选为父级，且排除自身与子孙
const parentOptions = computed(() => {
  const onlyDirMenu = (list: MenuNode[]): any[] =>
    list
      .filter((m) => m.type !== 'button' && m.id !== form.id)
      .map((m) => ({ ...m, children: m.children?.length ? onlyDirMenu(m.children) : [] }))
  return [{ id: '', title: '顶级菜单', children: onlyDirMenu(treeData.value) }]
})

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  parent_id: '',
  title: '',
  path: '',
  icon: '',
  type: 'menu' as 'dir' | 'menu' | 'button',
  perms: '',
  sort: 0,
  visible: 1,
  status: 1
})

const formRules: FormRules = {
  parent_id: [{ required: true, message: '请选择上级菜单', trigger: 'change' }],
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  path: [
    {
      validator: (_r: any, value: string, cb: (e?: Error) => void) => {
        if (form.type !== 'button' && !value) cb(new Error('目录/菜单需填写路由路径'))
        else if (value && !value.startsWith('/')) cb(new Error('路由路径需以 / 开头'))
        else cb()
      },
      trigger: 'blur'
    }
  ],
  perms: [
    {
      validator: (_r: any, value: string, cb: (e?: Error) => void) => {
        if (form.type === 'button' && !value) cb(new Error('按钮需填写权限标识'))
        else cb()
      },
      trigger: 'blur'
    }
  ]
}

// row 为空为顶级新增；parent 存在为"新增下级"
function openForm(row?: MenuNode, parent?: MenuNode) {
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id, parent_id: row.parent_id, title: row.title, path: row.path,
      icon: row.icon, type: row.type, perms: row.perms, sort: row.sort,
      visible: row.visible, status: row.status
    })
  } else {
    Object.assign(form, {
      id: '', parent_id: parent?.id ?? '', title: '', path: '', icon: '',
      type: 'menu', perms: '', sort: 0, visible: 1, status: 1
    })
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const payload = { ...form }
    if (form.id) {
      await updateMenu(form.id, payload)
      ElMessage.success('菜单已更新')
    } else {
      await createMenu(payload)
      ElMessage.success('菜单已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: MenuNode) {
  if (row.children?.length) {
    ElMessage.warning(`「${row.title}」下存在子级，请先删除全部子级`)
    return
  }
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除「${row.title}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteMenu(row.id)
  ElMessage.success('已删除')
  fetchList()
}
</script>
