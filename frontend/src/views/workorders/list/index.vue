<template>
  <div class="app-container">
    <!-- 状态 Tab（带数量角标；P2 六态 + 已作废） -->
    <div class="table-card tab-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane name="reported">
          <template #label>待受理 <el-badge :value="counts.reported || 0" :max="99" type="danger" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="pending_dispatch">
          <template #label>待派单 <el-badge :value="counts.pending_dispatch || 0" :max="99" type="warning" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="processing">
          <template #label>处理中 <el-badge :value="counts.processing || 0" :max="99" type="primary" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="pending_confirm">
          <template #label>待验收 <el-badge :value="counts.pending_confirm || 0" :max="99" type="warning" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="closed">
          <template #label>已闭环 <el-badge :value="counts.closed || 0" :max="999" type="success" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="closed_invalid">
          <template #label>已作废 <el-badge :value="counts.closed_invalid || 0" :max="999" type="info" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="all" label="全部" />
      </el-tabs>

      <!-- 搜索区 -->
      <el-form :model="query" inline class="wo-filter">
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 140px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="工单号">
          <el-input v-model="query.order_no" placeholder="工单号精确查询" clearable style="width: 170px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="来源">
          <el-select v-model="query.source" placeholder="全部" clearable style="width: 130px">
            <el-option label="巡检异常转单" value="inspection" />
            <el-option label="主动上报" value="active" />
            <el-option label="前台代录" value="frontdesk" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-select v-model="query.priority" placeholder="全部" clearable style="width: 110px">
            <el-option label="紧急" value="urgent" />
            <el-option label="高" value="high" />
            <el-option label="一般" value="normal" />
            <el-option label="低" value="low" />
          </el-select>
        </el-form-item>
        <el-form-item label="上报时间">
          <el-date-picker
            v-model="timeRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="YYYY-MM-DD 00:00:00"
            style="width: 260px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格 -->
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'workorder:create'" type="primary" :icon="Plus" @click="openCreate">前台代录</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%" @row-click="goDetail">
        <el-table-column prop="order_no" label="工单号" width="170" />
        <el-table-column label="标题" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.title }}</span>
            <el-tag v-if="row.sla_overdue" type="danger" size="small" class="overdue-tag">超时</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="小区/点位" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.community_name }}<template v-if="row.point_name"> / {{ row.point_name }}</template></template>
        </el-table-column>
        <el-table-column label="来源" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="sourceType(row.source)" size="small" effect="plain">{{ sourceLabel(row.source) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reporter_name" label="上报人" width="90" />
        <el-table-column label="优先级" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="priorityType(row.priority)" size="small">{{ priorityLabel(row.priority) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="处理人" width="90">
          <template #default="{ row }">{{ row.assignee_name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="上报时间" width="160" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small" effect="plain">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <!-- 操作按状态变化（权限点控制显隐；名单校验在后端） -->
            <el-button
              v-if="row.status === 'reported' && userStore.hasPerm('workorder:triage')"
              link type="primary"
              @click.stop="goDetail(row)"
            >受理</el-button>
            <el-button
              v-else-if="row.status === 'pending_dispatch' && userStore.hasPerm('workorder:dispatch')"
              link type="primary"
              @click.stop="goDetail(row)"
            >派单</el-button>
            <el-button
              v-else-if="row.status === 'pending_confirm' && userStore.hasPerm('workorder:confirm')"
              link type="primary"
              @click.stop="goDetail(row)"
            >验收</el-button>
            <el-button v-else link type="primary" @click.stop="goDetail(row)">详情</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty :description="emptyText" />
        </template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.page_size"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 前台代录对话框 -->
    <el-dialog v-model="createVisible" title="前台代录建单" width="560px" :close-on-click-modal="false">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="88px">
        <el-form-item label="标题" prop="title">
          <el-input v-model="createForm.title" placeholder="如：楼道灯损坏" maxlength="64" />
        </el-form-item>
        <el-form-item label="小区" prop="community_id">
          <el-select v-model="createForm.community_id" placeholder="选择小区" style="width: 100%" @change="handleCreateCommunityChange">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联点位">
          <el-select v-model="createForm.point_id" clearable filterable placeholder="选填，先选小区" style="width: 100%">
            <el-option v-for="p in pointOptions" :key="p.id" :label="`${p.building_name ? p.building_name + ' / ' : ''}${p.name}`" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="异常描述" prop="description">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="位置与异常具体情况（如：3号楼2单元电梯异响）" />
        </el-form-item>
        <el-form-item label="现场照片">
          <div class="photo-upload">
            <div v-if="createForm.photos.length" class="thumb-row">
              <span v-for="(p, i) in createForm.photos" :key="i" class="thumb-wrap">
                <el-image :src="p.url" fit="cover" class="thumb-img" />
                <el-icon class="thumb-del" @click="createForm.photos.splice(i, 1)"><Close /></el-icon>
              </span>
            </div>
            <el-upload
              :show-file-list="false"
              accept="image/*"
              :http-request="handlePhotoUpload"
            >
              <el-button size="small" :loading="photoUploading">上传照片</el-button>
            </el-upload>
          </div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="优先级">
              <el-select v-model="createForm.priority" style="width: 100%">
                <el-option label="一般" value="normal" />
                <el-option label="紧急" value="urgent" />
                <el-option label="高" value="high" />
                <el-option label="低" value="low" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="处理人">
              <el-select v-model="createForm.assignee_id" clearable filterable placeholder="选填，填了直接派单" style="width: 100%">
                <el-option v-for="s in assigneeOptions" :key="s.user_id" :label="s.user_name" :value="s.user_id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-alert
          type="info"
          :closable="false"
          title="派单对象须为本项目「工单接单」槽位名单成员（候选人取该项目编制中维修工岗位的在职成员）"
        />
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">创建工单</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules, type UploadRequestOptions } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Close } from '@element-plus/icons-vue'
import { listWorkOrders, createWorkOrder, type WorkOrderQuery } from '@/api/workorder'
import { listCommunities, listStaff } from '@/api/community'
import { listPoints } from '@/api/point'
import { uploadImage, fileUrl } from '@/api/upload'
import { useUserStore } from '@/store/user'
import type { WorkOrderItem, CommunityItem, StaffItem, PointItem } from '@/api/biz-types'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const list = ref<WorkOrderItem[]>([])
const total = ref(0)
// 状态角标（后端按数据权限口径全量统计）
const counts = reactive<Record<string, number>>({
  reported: 0, pending_dispatch: 0, processing: 0, pending_confirm: 0, closed: 0, closed_invalid: 0
})
const communities = ref<CommunityItem[]>([])
// 默认落「待受理」（工单流转入口）
const activeTab = ref('reported')
const timeRange = ref<[string, string] | null>(null)

const query = reactive<WorkOrderQuery>({ page: 1, page_size: 20, community_id: undefined, order_no: '', source: '', priority: '' })

const emptyText = computed(() => {
  return {
    reported: '太棒了，没有待受理的工单',
    pending_dispatch: '暂无待派单的工单',
    processing: '暂无处理中的工单',
    pending_confirm: '暂无待验收的工单',
    closed: '暂无已闭环的工单',
    closed_invalid: '暂无已作废的工单',
    all: '暂无工单'
  }[activeTab.value]
})

async function fetchList() {
  loading.value = true
  try {
    const data = await listWorkOrders({
      ...query,
      order_no: query.order_no || undefined,
      source: query.source || undefined,
      priority: query.priority || undefined,
      status: activeTab.value === 'all' ? undefined : activeTab.value,
      start_time: timeRange.value?.[0],
      end_time: timeRange.value?.[1]
    })
    list.value = data.list
    total.value = data.total
    if (data.status_counts) Object.assign(counts, data.status_counts)
  } finally {
    loading.value = false
  }
}

function handleTabChange() {
  query.page = 1
  fetchList()
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.community_id = undefined
  query.order_no = ''
  query.source = ''
  query.priority = ''
  timeRange.value = null
  handleSearch()
}

onMounted(async () => {
  // 支持从任务明细/巡检记录带工单号进入（工单可能处于任意状态，落「全部」Tab）
  if (route.query.order_no) {
    query.order_no = String(route.query.order_no)
    activeTab.value = 'all'
  }
  fetchList()
  const cData = await listCommunities({ page: 1, page_size: 100, status: 1 })
  communities.value = cData.list
})

function goDetail(row: WorkOrderItem) {
  router.push(`/workorders/detail/${row.id}`)
}

// ===== 标签 =====
function priorityLabel(p: string) {
  return { urgent: '紧急', high: '高', normal: '一般', low: '低' }[p] || p
}

function priorityType(p: string) {
  return ({ urgent: 'danger', high: 'warning', normal: 'warning', low: 'info' } as Record<string, any>)[p] || 'info'
}

function statusLabel(s: string) {
  return {
    reported: '待受理', pending_dispatch: '待派单', processing: '处理中',
    pending_confirm: '待验收', closed: '已闭环', closed_invalid: '已作废'
  }[s] || s
}

function statusType(s: string) {
  return ({
    reported: 'danger', pending_dispatch: 'warning', processing: 'primary',
    pending_confirm: 'warning', closed: 'success', closed_invalid: 'info'
  } as Record<string, any>)[s] || 'info'
}

function sourceLabel(s: string) {
  return { inspection: '巡检转单', active: '主动上报', frontdesk: '前台代录' }[s] || s
}

function sourceType(s: string) {
  return ({ inspection: 'warning', active: 'primary', frontdesk: 'info' } as Record<string, any>)[s] || 'info'
}

// ===== 前台代录 =====
const createVisible = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  title: '',
  community_id: null as string | null,
  point_id: null as string | null,
  description: '',
  photos: [] as { file_key: string; url: string }[],
  priority: 'normal',
  assignee_id: null as string | null
})

const createRules: FormRules = {
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  description: [{ required: true, message: '请输入异常描述', trigger: 'blur' }]
}

// 候选人：所选项目编制中「维修工」岗位的在职成员（派单对象须为工单接单槽位名单成员）
const staffList = ref<StaffItem[]>([])
const assigneeOptions = computed(() =>
  staffList.value.filter((s) => s.status === 1 && s.posts?.includes('repairman'))
)
const pointOptions = ref<PointItem[]>([])
const photoUploading = ref(false)

async function handleCreateCommunityChange() {
  createForm.point_id = null
  createForm.assignee_id = null
  staffList.value = []
  pointOptions.value = []
  if (!createForm.community_id) return
  const [sData, pData] = await Promise.all([
    listStaff(createForm.community_id),
    listPoints({ community_id: createForm.community_id, status: 1, page: 1, page_size: 100 })
  ])
  staffList.value = sData
  pointOptions.value = pData.list
}

// 现场照片上传：scene 用 workorder（与小程序端一致）
async function handlePhotoUpload(opt: UploadRequestOptions) {
  photoUploading.value = true
  try {
    const { file_key, url } = await uploadImage(opt.file as File, 'workorder')
    createForm.photos.push({ file_key, url: url || fileUrl(file_key) })
  } catch {
    ElMessage.warning('照片上传失败，请重试')
  } finally {
    photoUploading.value = false
  }
}

function openCreate() {
  createFormRef.value?.clearValidate()
  Object.assign(createForm, {
    title: '', community_id: null, point_id: null, description: '',
    photos: [], priority: 'normal', assignee_id: null
  })
  staffList.value = []
  pointOptions.value = []
  createVisible.value = true
}

async function handleCreate() {
  await createFormRef.value?.validate()
  creating.value = true
  try {
    const res = await createWorkOrder({
      community_id: createForm.community_id!,
      point_id: createForm.point_id || undefined,
      title: createForm.title,
      description: createForm.description,
      photos: createForm.photos.map((p) => ({ file_key: p.file_key })),
      priority: createForm.priority,
      assignee_id: createForm.assignee_id || undefined
    })
    ElMessage.success(`工单已创建：${res.order_no}`)
    createVisible.value = false
    fetchList()
  } finally {
    creating.value = false
  }
}
</script>

<style scoped lang="scss">
.tab-card {
  .tab-badge {
    margin-left: $spacing-xs;
    vertical-align: 2px;
  }

  .wo-filter {
    margin-bottom: $spacing-md;
    padding-bottom: $spacing-md;
    border-bottom: 1px solid $color-border;
  }
}

.overdue-tag {
  margin-left: $spacing-xs;
}

:deep(.el-table__row) {
  cursor: pointer;
}

.photo-upload {
  display: flex;
  flex-direction: column;
  gap: $spacing-xs;
  align-items: flex-start;
}

.thumb-row {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
}

.thumb-img {
  width: 96px;
  height: 96px;
  border-radius: $radius-small;
  border: 1px solid $color-border;
  display: block;
}

.thumb-wrap {
  position: relative;
  display: inline-block;

  .thumb-del {
    position: absolute;
    top: -6px;
    right: -6px;
    background: $color-danger;
    color: #fff;
    border-radius: 50%;
    padding: 1px;
    cursor: pointer;
    font-size: 10px;
  }
}
</style>
