<template>
  <div class="app-container">
    <!-- 状态 Tab（带数量角标） -->
    <div class="table-card tab-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane name="pending">
          <template #label>待派单 <el-badge :value="counts.pending || 0" :max="99" type="warning" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="processing">
          <template #label>处理中 <el-badge :value="counts.processing || 0" :max="99" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="review">
          <template #label>待复核 <el-badge :value="counts.review || 0" :max="99" type="primary" class="tab-badge" /></template>
        </el-tab-pane>
        <el-tab-pane name="closed">
          <template #label>已关闭 <el-badge :value="counts.closed || 0" :max="999" type="info" class="tab-badge" /></template>
        </el-tab-pane>
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
          <el-button v-perms="'workorder:create'" type="primary" :icon="Plus" @click="openCreate">手工建单</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%" @row-click="goDetail">
        <el-table-column prop="order_no" label="工单号" width="170" />
        <el-table-column prop="title" label="标题" min-width="170" show-overflow-tooltip />
        <el-table-column label="小区/点位" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.community_name }}<template v-if="row.point_name"> / {{ row.point_name }}</template></template>
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
            <!-- 操作按状态变化 -->
            <el-button
              v-if="(row.status === 'pending' || row.status === 'rejected') && userStore.hasPerm('workorder:assign')"
              link type="primary"
              @click.stop="goDetail(row)"
            >派单</el-button>
            <el-button
              v-else-if="row.status === 'review' && userStore.hasPerm('workorder:review')"
              link type="primary"
              @click.stop="goDetail(row)"
            >复核</el-button>
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

    <!-- 手工建单对话框 -->
    <el-dialog v-model="createVisible" title="手工建单" width="560px" :close-on-click-modal="false">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="88px">
        <el-form-item label="标题" prop="title">
          <el-input v-model="createForm.title" placeholder="如：楼道灯损坏" maxlength="64" />
        </el-form-item>
        <el-form-item label="小区" prop="community_id">
          <el-select v-model="createForm.community_id" placeholder="选择小区" style="width: 100%">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="异常描述" prop="description">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="异常具体情况" />
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
                <el-option v-for="u in assigneeOptions" :key="u.id" :label="u.name" :value="u.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
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
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight } from '@element-plus/icons-vue'
import { listWorkOrders, createWorkOrder, type WorkOrderQuery } from '@/api/workorder'
import { listCommunities } from '@/api/community'
import { listUsers } from '@/api/user'
import { useUserStore } from '@/store/user'
import type { WorkOrderItem, CommunityItem } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const list = ref<WorkOrderItem[]>([])
const total = ref(0)
const counts = reactive<Record<string, number>>({ pending: 0, processing: 0, review: 0, closed: 0 })
const communities = ref<CommunityItem[]>([])
const assigneeOptions = ref<UserItem[]>([])
// 主管登录默认落「待派单」
const activeTab = ref('pending')
const timeRange = ref<[string, string] | null>(null)

const query = reactive<WorkOrderQuery>({ page: 1, page_size: 20, community_id: undefined, order_no: '', priority: '' })

// Tab → status 参数：处理中 = assigned,processing 合并
const tabStatusMap: Record<string, string> = {
  pending: 'pending',
  processing: 'assigned,processing',
  review: 'review',
  closed: 'closed,rejected'
}

const emptyText = computed(() => {
  return {
    pending: '太棒了，没有待派单的工单',
    processing: '暂无处理中的工单',
    review: '暂无待复核的工单',
    closed: '暂无已关闭的工单'
  }[activeTab.value]
})

async function fetchList() {
  loading.value = true
  try {
    const data = await listWorkOrders({
      ...query,
      order_no: query.order_no || undefined,
      priority: query.priority || undefined,
      status: tabStatusMap[activeTab.value],
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
  query.priority = ''
  timeRange.value = null
  handleSearch()
}

onMounted(async () => {
  // 支持从任务明细/巡检记录带工单号进入
  if (route.query.order_no) {
    query.order_no = String(route.query.order_no)
    activeTab.value = 'pending'
  }
  fetchList()
  const [cData, uData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 })
  ])
  communities.value = cData.list
  assigneeOptions.value = uData.list
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
  return { pending: '待派单', assigned: '已派单', processing: '处理中', review: '待复核', closed: '已关闭', rejected: '已驳回' }[s] || s
}

function statusType(s: string) {
  return ({ pending: 'warning', assigned: 'primary', processing: 'primary', review: 'warning', closed: 'info', rejected: 'danger' } as Record<string, any>)[s] || 'info'
}

// ===== 手工建单 =====
const createVisible = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  title: '',
  community_id: null as string | null,
  description: '',
  priority: 'normal',
  assignee_id: null as string | null
})

const createRules: FormRules = {
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  description: [{ required: true, message: '请输入异常描述', trigger: 'blur' }]
}

function openCreate() {
  createFormRef.value?.clearValidate()
  Object.assign(createForm, { title: '', community_id: null, description: '', priority: 'normal', assignee_id: null })
  createVisible.value = true
}

async function handleCreate() {
  await createFormRef.value?.validate()
  creating.value = true
  try {
    const res = await createWorkOrder({
      community_id: createForm.community_id!,
      title: createForm.title,
      description: createForm.description,
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

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
