<template>
  <div class="app-container">
    <div class="community-layout">
      <!-- 左：组织树（小区 → 楼栋/区域） -->
      <div class="tree-card">
        <div class="tree-title">组织架构</div>
        <el-tree
          :data="treeData"
          node-key="treeKey"
          default-expand-all
          highlight-current
          :expand-on-click-node="false"
          :props="{ label: 'label', children: 'children' }"
          @node-click="handleNodeClick"
        >
          <template #default="{ data }">
            <span class="tree-node">
              <span>{{ data.label }}</span>
              <el-tag v-if="data.kind === 'building'" :type="data.buildingType === 'building' ? 'primary' : 'warning'" size="small" class="tree-tag">
                {{ data.buildingType === 'building' ? '楼栋' : '区域' }}
              </el-tag>
            </span>
          </template>
        </el-tree>
      </div>

      <!-- 右：跟随树节点切换 -->
      <div class="community-main">
        <!-- 当前位置 -->
        <div class="context-bar">
          <span class="context-path">{{ contextPath }}</span>
          <span v-if="mode !== 'all'" class="text-secondary">{{ contextHint }}</span>
        </div>

        <!-- 小区列表（选中「全部小区」） -->
        <template v-if="mode === 'all'">
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

          <div class="table-card">
            <div class="table-toolbar">
              <div class="table-toolbar-left">
                <el-button v-perms="'community:create'" type="primary" :icon="Plus" @click="openForm()">新增小区</el-button>
              </div>
              <el-tooltip content="刷新" placement="top">
                <el-button :icon="RefreshRight" circle @click="refreshCurrent" />
              </el-tooltip>
            </div>

            <el-table v-loading="loading" :data="list" stripe style="width: 100%">
              <el-table-column prop="name" label="小区名称" min-width="150" show-overflow-tooltip />
              <el-table-column prop="address" label="地址" min-width="200" show-overflow-tooltip>
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
              <el-table-column label="操作" width="210" fixed="right">
                <template #default="{ row }">
                  <el-button v-perms="'community:staff:list'" link type="primary" @click="openStaffDrawer(row)">编制</el-button>
                  <el-button v-perms="'community:update'" link type="primary" @click="openForm(row)">编辑</el-button>
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
        </template>

        <!-- 楼栋/区域列表（选中某小区） -->
        <div v-else-if="mode === 'community'" class="table-card">
          <div class="table-toolbar">
            <div class="table-toolbar-left">
              <el-button v-perms="'community:building:create'" type="primary" :icon="Plus" @click="openBuildingForm()">新增楼栋/区域</el-button>
            </div>
            <el-tooltip content="刷新" placement="top">
              <el-button :icon="RefreshRight" circle @click="refreshCurrent" />
            </el-tooltip>
          </div>

          <el-table v-loading="loading" :data="buildingList" stripe style="width: 100%">
            <el-table-column prop="name" label="名称" min-width="160" />
            <el-table-column label="类型" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="row.type === 'building' ? 'primary' : 'warning'" size="small">
                  {{ row.type === 'building' ? '楼栋' : '区域' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="point_count" label="点位数" width="90" align="right" />
            <el-table-column prop="created_at" label="创建时间" width="160" />
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button v-perms="'community:building:update'" link type="primary" @click="openBuildingForm(row)">编辑</el-button>
                <el-button v-perms="'community:building:delete'" link type="danger" @click="handleBuildingDelete(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="该小区暂无楼栋/区域">
                <el-button v-perms="'community:building:create'" type="primary" @click="openBuildingForm()">新增楼栋/区域</el-button>
              </el-empty>
            </template>
          </el-table>

          <div class="pagination-wrap">
            <el-pagination
              v-model:current-page="buildingQuery.page"
              v-model:page-size="buildingQuery.page_size"
              :total="buildingTotal"
              layout="total, prev, pager, next"
              @change="fetchBuildings"
            />
          </div>
        </div>

        <!-- 点位列表（选中某楼栋/区域） -->
        <div v-else class="table-card">
          <div class="table-toolbar">
            <div class="table-toolbar-left">
              <el-button type="primary" :icon="MapLocation" @click="goPoints">在点位管理中维护</el-button>
            </div>
            <el-tooltip content="刷新" placement="top">
              <el-button :icon="RefreshRight" circle @click="refreshCurrent" />
            </el-tooltip>
          </div>

          <el-table v-loading="loading" :data="pointList" stripe style="width: 100%">
            <el-table-column prop="name" label="点位名称" min-width="140" show-overflow-tooltip />
            <el-table-column prop="type_label" label="类型" width="110">
              <template #default="{ row }">{{ row.type_label || row.type }}</template>
            </el-table-column>
            <el-table-column prop="qrcode_no" label="二维码编号" width="120" />
            <el-table-column label="打卡方式" width="110" align="center">
              <template #default="{ row }">{{ checkinModeLabel(row) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="该楼栋/区域暂无点位">
                <el-button type="primary" @click="goPoints">前往点位管理新增</el-button>
              </el-empty>
            </template>
          </el-table>

          <div class="pagination-wrap">
            <el-pagination
              v-model:current-page="pointQuery.page"
              v-model:page-size="pointQuery.page_size"
              :total="pointTotal"
              layout="total, prev, pager, next"
              @change="fetchPoints"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- 新增/编辑小区对话框 -->
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
        <el-form-item label="工单分诊">
          <el-switch v-model="form.wo_triage_enabled" />
          <span class="switch-hint">关闭后上报工单直接进待派单，跳过待分诊环节</span>
        </el-form-item>
        <el-form-item label="工单抢单">
          <el-switch v-model="form.wo_grab_enabled" />
          <span class="switch-hint">开启后维修工可从工单池自行抢单</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 新增/编辑楼栋/区域对话框 -->
    <el-dialog v-model="buildingFormVisible" :title="buildingForm.id ? '编辑楼栋/区域' : '新增楼栋/区域'" width="440px" :close-on-click-modal="false">
      <el-form ref="buildingFormRef" :model="buildingForm" :rules="buildingFormRules" label-width="88px">
        <el-form-item label="所属小区">
          <el-input :model-value="currentName" disabled />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="buildingForm.name" placeholder="如：1号楼 / 地下车库A区" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="buildingForm.type">
            <el-radio value="building">楼栋</el-radio>
            <el-radio value="area">区域</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="buildingFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleBuildingSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 岗位编制抽屉：编制名单 + 职责槽位绑定 -->
    <el-drawer v-model="staffVisible" :title="`岗位编制 · ${staffCommunityName}`" size="760px">
      <el-tabs v-model="staffTab">
        <el-tab-pane label="编制名单" name="staff">
          <div class="table-toolbar">
            <div class="table-toolbar-left">
              <el-button v-perms="'community:staff:edit'" type="primary" :icon="Plus" @click="openStaffForm()">新增成员</el-button>
            </div>
            <el-tooltip content="刷新" placement="top">
              <el-button :icon="RefreshRight" circle @click="fetchStaff" />
            </el-tooltip>
          </div>

          <el-table v-loading="staffLoading" :data="staffList" stripe style="width: 100%">
            <el-table-column prop="user_name" label="姓名" min-width="100" />
            <el-table-column prop="phone" label="手机号" width="130">
              <template #default="{ row }">{{ row.phone || '--' }}</template>
            </el-table-column>
            <el-table-column label="岗位" min-width="160">
              <template #default="{ row }">
                <el-tag v-for="n in row.post_names" :key="n" size="small" class="post-tag">{{ n }}</el-tag>
                <span v-if="!row.post_names?.length" class="text-secondary">--</span>
              </template>
            </el-table-column>
            <el-table-column label="责任楼栋" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.posts?.includes(POST_BUILDING_MANAGER) ? (row.building_names?.join('、') || '全部楼栋') : '--' }}
              </template>
            </el-table-column>
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '在职' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button v-perms="'community:staff:edit'" link type="primary" @click="openStaffForm(row)">编辑</el-button>
                <el-button v-perms="'community:staff:edit'" link type="danger" @click="handleStaffDelete(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="该项目暂无编制成员">
                <el-button v-perms="'community:staff:edit'" type="primary" @click="openStaffForm()">新增成员</el-button>
              </el-empty>
            </template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="职责槽位绑定" name="duty">
          <el-alert
            type="info"
            :closable="false"
            title="名单即授权：各环节由「槽位绑定岗位 → 编制在职成员」推导默认人选；修改「平台默认」槽位保存后即形成项目级覆盖；岗位留空 = 本项目该环节跳过（暂不支持恢复平台默认）"
            class="duty-tip"
          />
          <el-table v-loading="dutyLoading" :data="dutyList" stripe style="width: 100%">
            <el-table-column prop="name" label="职责槽位" min-width="130" />
            <el-table-column label="绑定岗位" min-width="280">
              <template #default="{ row }">
                <el-select
                  v-model="dutyEdits[row.slot]"
                  multiple
                  collapse-tags
                  collapse-tags-tooltip
                  placeholder="留空 = 本项目该环节跳过"
                  :disabled="!userStore.hasPerm('community:duty:edit')"
                  style="width: 100%"
                >
                  <el-option-group v-for="g in enabledPostGroups" :key="g.label" :label="g.label">
                    <el-option v-for="p in g.posts" :key="p.code" :label="p.name" :value="p.code" />
                  </el-option-group>
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="来源" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.source === 'project' ? 'warning' : 'info'" size="small">
                  {{ row.source === 'project' ? '项目覆盖' : '平台默认' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <div class="duty-footer">
            <el-button
              v-perms="'community:duty:edit'"
              type="primary"
              :loading="dutySaving"
              @click="handleDutySave"
            >保存绑定</el-button>
          </div>
          <ReviewFlowEditor
            v-if="staffCommunityId"
            :key="staffCommunityId"
            :api="flowApi"
            :slot-options="dutySlotOptions"
            save-perm="community:duty:edit"
          />
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <!-- 新增/编辑编制成员对话框（user_id 不可换绑：换人等价于删除后新增） -->
    <el-dialog v-model="staffFormVisible" :title="staffForm.id ? '编辑编制成员' : '新增编制成员'" width="480px" :close-on-click-modal="false">
      <el-form ref="staffFormRef" :model="staffForm" :rules="staffFormRules" label-width="88px">
        <el-form-item label="所属小区">
          <el-input :model-value="staffCommunityName" disabled />
        </el-form-item>
        <el-form-item label="成员" prop="user_id">
          <el-select
            v-model="staffForm.user_id"
            filterable
            placeholder="选择启用用户（同人同项目仅一条编制）"
            :disabled="!!staffForm.id"
            style="width: 100%"
          >
            <el-option v-for="u in userOptions" :key="u.id" :label="`${u.name}（${u.phone || '无手机号'}）`" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="岗位" prop="posts">
          <el-select v-model="staffForm.posts" multiple collapse-tags collapse-tags-tooltip placeholder="一人可多岗" style="width: 100%">
            <el-option-group v-for="g in enabledPostGroups" :key="g.label" :label="g.label">
              <el-option v-for="p in g.posts" :key="p.code" :label="p.name" :value="p.code" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item v-if="staffForm.posts.includes(POST_BUILDING_MANAGER)" label="责任楼栋">
          <el-select v-model="staffForm.building_ids" multiple collapse-tags collapse-tags-tooltip placeholder="不选则为全部楼栋" style="width: 100%">
            <el-option v-for="b in staffBuildingOptions" :key="b.id" :label="b.name" :value="b.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="staffForm.status">
            <el-radio :value="1">在职</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="staffFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleStaffSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, MapLocation } from '@element-plus/icons-vue'
import {
  listCommunities, createCommunity, updateCommunity, deleteCommunity,
  listCommunityTree, listBuildings, createBuilding, updateBuilding, deleteBuilding,
  listPostDict, listStaff, createStaff, updateStaff, deleteStaff, listDutyBindings, saveDutyBindings,
  getReviewFlow, saveReviewFlow
} from '@/api/community'
import { listPoints } from '@/api/point'
import { POST_LINES } from '@/api/post'
import { listUsers } from '@/api/user'
import { useUserStore } from '@/store/user'
import ReviewFlowEditor from '@/components/ReviewFlowEditor.vue'
import type { CommunityItem, BuildingItem, PointItem, PostDictItem, StaffItem, DutyBindingItem } from '@/api/biz-types'
import { POST_BUILDING_MANAGER } from '@/api/biz-types'

const router = useRouter()
const userStore = useUserStore()

// ===== 左树 =====
interface TreeNode {
  treeKey: string
  label: string
  kind: 'all' | 'community' | 'building'
  communityId?: string
  buildingId?: string
  buildingType?: string
  children?: TreeNode[]
}

const treeData = ref<TreeNode[]>([])

async function fetchTree() {
  const nodes = await listCommunityTree()
  treeData.value = [{
    treeKey: 'all',
    label: '全部小区',
    kind: 'all',
    children: nodes.map((c) => ({
      treeKey: `c-${c.id}`,
      label: c.name,
      kind: 'community' as const,
      communityId: c.id,
      children: c.buildings.map((b) => ({
        treeKey: `b-${b.id}`,
        label: b.name,
        kind: 'building' as const,
        communityId: c.id,
        buildingId: b.id,
        buildingType: b.type
      }))
    }))
  }]
}

// ===== 选中节点与联动 =====
const mode = ref<'all' | 'community' | 'building'>('all')
const currentCommunityId = ref('')
const currentBuildingId = ref('')
const currentName = ref('') // 当前小区名（楼栋模式下为楼栋名）

const contextPath = computed(() => {
  if (mode.value === 'all') return '全部小区'
  const comm = treeData.value[0]?.children?.find((c) => c.communityId === currentCommunityId.value)
  if (mode.value === 'community') return comm?.label || ''
  return `${comm?.label || ''} / ${currentName.value}`
})

const contextHint = computed(() =>
  mode.value === 'community' ? '该小区下的楼栋/区域' : '该楼栋/区域下的点位（只读，维护请前往点位管理）'
)

function handleNodeClick(node: TreeNode) {
  if (node.kind === 'all') {
    mode.value = 'all'
    currentCommunityId.value = ''
    currentBuildingId.value = ''
    fetchList()
  } else if (node.kind === 'community') {
    mode.value = 'community'
    currentCommunityId.value = node.communityId!
    currentBuildingId.value = ''
    currentName.value = node.label
    buildingQuery.page = 1
    fetchBuildings()
  } else {
    mode.value = 'building'
    currentCommunityId.value = node.communityId!
    currentBuildingId.value = node.buildingId!
    currentName.value = node.label
    pointQuery.page = 1
    fetchPoints()
  }
}

function refreshCurrent() {
  if (mode.value === 'all') fetchList()
  else if (mode.value === 'community') fetchBuildings()
  else fetchPoints()
}

// ===== 小区列表 =====
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

onMounted(() => {
  fetchTree()
  fetchList()
})

// ===== 小区新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', name: '', address: '', status: 1, wo_triage_enabled: true, wo_grab_enabled: false })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入小区名称', trigger: 'blur' }]
}

function openForm(row?: CommunityItem) {
  formRef.value?.clearValidate()
  Object.assign(form, row
    ? { id: row.id, name: row.name, address: row.address, status: row.status, wo_triage_enabled: row.wo_triage_enabled, wo_grab_enabled: row.wo_grab_enabled }
    : { id: '', name: '', address: '', status: 1, wo_triage_enabled: true, wo_grab_enabled: false })
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
    fetchTree()
    if (mode.value === 'all') fetchList()
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
  fetchTree()
  fetchList()
}

// ===== 岗位编制抽屉 =====
const staffVisible = ref(false)
const staffTab = ref<'staff' | 'duty'>('staff')
const staffCommunityId = ref('')
const staffCommunityName = ref('')
const staffLoading = ref(false)
const staffList = ref<StaffItem[]>([])

// 岗位字典（按小区所属租户返回，含业务线；仅启用项参与选择；name → code 选项）
const postDict = ref<PostDictItem[]>([])
const enabledPosts = computed(() => postDict.value.filter((p) => p.status === 1))
// 启用岗位按业务线分组（el-option-group 展示）
const enabledPostGroups = computed(() =>
  POST_LINES
    .map((l) => ({ label: l.label, posts: enabledPosts.value.filter((p) => p.line === l.value) }))
    .filter((g) => g.posts.length > 0)
)

// 成员选择：启用用户全量（名单即授权，不限角色/权限点），首次打开抽屉时加载
const userOptions = ref<{ id: string; name: string; phone: string }[]>([])

// 责任楼栋选项（楼管员用）
const staffBuildingOptions = ref<BuildingItem[]>([])

async function loadStaffOptions() {
  // 岗位字典按小区所属租户下发，每次打开编制抽屉随小区重取（不可跨小区缓存）
  postDict.value = await listPostDict(staffCommunityId.value)
  if (!userOptions.value.length) {
    const data = await listUsers({ page: 1, page_size: 200, status: 1 })
    userOptions.value = data.list
  }
  const bData = await listBuildings({ community_id: staffCommunityId.value, page: 1, page_size: 200 })
  staffBuildingOptions.value = bData.list
}

async function fetchStaff() {
  staffLoading.value = true
  try {
    staffList.value = await listStaff(staffCommunityId.value)
  } finally {
    staffLoading.value = false
  }
}

function openStaffDrawer(row: CommunityItem) {
  staffCommunityId.value = row.id
  staffCommunityName.value = row.name
  staffTab.value = 'staff'
  staffVisible.value = true
  loadStaffOptions()
  fetchStaff()
  fetchDutyBindings()
}

// ===== 编制成员新增/编辑 =====
const staffFormVisible = ref(false)
const staffFormRef = ref<FormInstance>()
const staffForm = reactive({ id: '', user_id: '', posts: [] as string[], building_ids: [] as string[], status: 1 })

const staffFormRules: FormRules = {
  user_id: [{ required: true, message: '请选择成员', trigger: 'change' }],
  posts: [{ required: true, type: 'array', min: 1, message: '请选择岗位', trigger: 'change' }]
}

function openStaffForm(row?: StaffItem) {
  staffFormRef.value?.clearValidate()
  Object.assign(staffForm, row
    ? { id: row.id, user_id: row.user_id, posts: [...row.posts], building_ids: [...(row.building_ids || [])], status: row.status }
    : { id: '', user_id: '', posts: [], building_ids: [], status: 1 })
  staffFormVisible.value = true
}

async function handleStaffSubmit() {
  await staffFormRef.value?.validate()
  submitting.value = true
  try {
    const payload = {
      user_id: staffForm.user_id,
      posts: staffForm.posts,
      building_ids: staffForm.posts.includes(POST_BUILDING_MANAGER) ? staffForm.building_ids : [],
      status: staffForm.status
    }
    if (staffForm.id) {
      await updateStaff(staffCommunityId.value, staffForm.id, payload)
      ElMessage.success('编制成员已更新')
    } else {
      await createStaff(staffCommunityId.value, payload)
      ElMessage.success('编制成员已添加')
    }
    staffFormVisible.value = false
    fetchStaff()
  } finally {
    submitting.value = false
  }
}

// 删除：硬删除，恢复即重新添加；项目经理名额/签字默认名单随编制联动，错误信息由拦截器展示
async function handleStaffDelete(row: StaffItem) {
  const ok = await ElMessageBox.confirm(
    `确定将「${row.user_name}」移出该项目编制吗？（槽位默认名单将不再包含该成员）`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteStaff(staffCommunityId.value, row.id)
  ElMessage.success('已删除')
  fetchStaff()
}

// ===== 职责槽位绑定 =====
const dutyLoading = ref(false)
const dutySaving = ref(false)
const dutyList = ref<DutyBindingItem[]>([])

// 打卡审核链（项目级覆盖；环节槽位选项复用职责槽位列表）
const flowApi = computed(() => ({
  listFlow: () => getReviewFlow(staffCommunityId.value),
  saveFlow: (s: { slot: string; name: string }[]) => saveReviewFlow(staffCommunityId.value, s) as Promise<unknown>
}))
const dutySlotOptions = computed(() => dutyList.value.map((d) => ({ slot: d.slot, name: d.name })))
// 逐槽位编辑值（slot → post_codes）；dutyOriginal 记录加载时快照，仅提交有变更的槽位（upsert 语义，避免把平台默认固化成项目覆盖）
const dutyEdits = reactive<Record<string, string[]>>({})
const dutyOriginal = ref<Record<string, string[]>>({})

async function fetchDutyBindings() {
  dutyLoading.value = true
  try {
    const data = await listDutyBindings(staffCommunityId.value)
    dutyList.value = data
    const snapshot: Record<string, string[]> = {}
    for (const b of data) {
      dutyEdits[b.slot] = [...b.post_codes]
      snapshot[b.slot] = [...b.post_codes]
    }
    dutyOriginal.value = snapshot
  } finally {
    dutyLoading.value = false
  }
}

async function handleDutySave() {
  const changed = dutyList.value
    .filter((b) => {
      const cur = dutyEdits[b.slot] || []
      const orig = dutyOriginal.value[b.slot] || []
      return cur.length !== orig.length || cur.some((c) => !orig.includes(c))
    })
    .map((b) => ({ slot: b.slot, post_codes: dutyEdits[b.slot] || [] }))
  if (!changed.length) {
    ElMessage.info('绑定未发生变化')
    return
  }
  dutySaving.value = true
  try {
    await saveDutyBindings(staffCommunityId.value, changed)
    ElMessage.success('职责绑定已保存')
    fetchDutyBindings()
  } finally {
    dutySaving.value = false
  }
}

// ===== 楼栋/区域列表 =====
const buildingList = ref<BuildingItem[]>([])
const buildingTotal = ref(0)
const buildingQuery = reactive({ page: 1, page_size: 20 })

async function fetchBuildings() {
  loading.value = true
  try {
    const data = await listBuildings({ community_id: currentCommunityId.value, ...buildingQuery })
    buildingList.value = data.list
    buildingTotal.value = data.total
  } finally {
    loading.value = false
  }
}

// ===== 楼栋/区域新增/编辑 =====
const buildingFormVisible = ref(false)
const buildingFormRef = ref<FormInstance>()
const buildingForm = reactive({ id: '', name: '', type: 'building' })

const buildingFormRules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }]
}

function openBuildingForm(row?: BuildingItem) {
  buildingFormRef.value?.clearValidate()
  Object.assign(buildingForm, row
    ? { id: row.id, name: row.name, type: row.type }
    : { id: '', name: '', type: 'building' })
  buildingFormVisible.value = true
}

async function handleBuildingSubmit() {
  await buildingFormRef.value?.validate()
  submitting.value = true
  try {
    if (buildingForm.id) {
      await updateBuilding(buildingForm.id, { name: buildingForm.name, type: buildingForm.type })
      ElMessage.success('已更新')
    } else {
      await createBuilding({ community_id: currentCommunityId.value, name: buildingForm.name, type: buildingForm.type })
      ElMessage.success('已创建')
    }
    buildingFormVisible.value = false
    fetchTree()
    fetchBuildings()
  } finally {
    submitting.value = false
  }
}

// 删除：存在点位时后端拒删（42003），错误信息由拦截器展示
async function handleBuildingDelete(row: BuildingItem) {
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除「${row.name}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteBuilding(row.id)
  ElMessage.success('已删除')
  fetchTree()
  fetchBuildings()
}

// ===== 楼栋下点位列表（只读预览） =====
const pointList = ref<PointItem[]>([])
const pointTotal = ref(0)
const pointQuery = reactive({ page: 1, page_size: 20 })

async function fetchPoints() {
  loading.value = true
  try {
    const data = await listPoints({ building_id: currentBuildingId.value, ...pointQuery })
    pointList.value = data.list
    pointTotal.value = data.total
  } finally {
    loading.value = false
  }
}

// 打卡方式展示：凭证 + 围栏两个维度组合成一句话
function checkinModeLabel(p: { credential: string; require_fence: boolean }) {
  const cred = { qrcode: '扫码', nfc: 'NFC', none: '' }[p.credential] ?? p.credential
  if (p.credential === 'none') return p.require_fence ? '围栏' : '--'
  return p.require_fence ? `${cred}+围栏` : cred
}

function goPoints() {
  router.push('/inspection/points')
}
</script>

<style scoped lang="scss">
.community-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.tree-card {
  width: 240px;
  flex-shrink: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg;

  .tree-title {
    font-weight: 600;
    margin-bottom: $spacing-md;
    color: $color-text-primary;
  }

  .tree-node {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
  }

  .tree-tag {
    transform: scale(0.85);
  }
}

.community-main {
  flex: 1;
  min-width: 0;
}

.post-tag {
  margin-right: $spacing-xs;
}

.switch-hint {
  margin-left: $spacing-sm;
  font-size: $font-size-aux;
  color: $color-text-secondary;
}

.duty-tip {
  margin-bottom: $spacing-md;
}

.duty-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: $spacing-md;
}

.context-bar {
  display: flex;
  align-items: baseline;
  gap: $spacing-md;
  margin-bottom: $spacing-md;
  padding: 0 $spacing-xs;

  .context-path {
    font-weight: 600;
    color: $color-text-primary;
  }
}
</style>
