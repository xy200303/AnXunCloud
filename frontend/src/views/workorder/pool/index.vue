<template>
  <div class="app-container">
    <div class="table-card">
      <el-tabs v-model="activeTab" @tab-change="handleSearch">
        <el-tab-pane label="待接单" name="pending" />
        <el-tab-pane label="整改中" name="in_progress" />
        <el-tab-pane label="待审核" name="pending_review" />
        <el-tab-pane label="已完成" name="done" />
        <el-tab-pane label="已关闭" name="closed" />
      </el-tabs>
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" clearable placeholder="全部小区" style="width: 180px">
            <el-option v-for="community in communities" :key="community.id" :label="community.name" :value="community.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="query.work_type" clearable placeholder="全部类型" style="width: 150px">
            <el-option label="巡检整改" value="inspection_fix" />
            <el-option label="计划维保" value="maintenance" />
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
          <el-checkbox v-model="mineOnly" @change="handleSearch">只看我负责的</el-checkbox>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>
      <el-table v-loading="loading" :data="list" stripe style="width: 100%" @row-click="openDetail">
        <el-table-column prop="order_no" label="工单号" min-width="180" show-overflow-tooltip />
        <el-table-column prop="title" label="整改事项" min-width="180" show-overflow-tooltip />
        <el-table-column prop="community_name" label="小区" min-width="120" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ row.work_type === 'inspection_fix' ? '巡检整改' : '计划维保' }}</template>
        </el-table-column>
        <el-table-column prop="issue_count" label="事项数" width="80" align="right" />
        <el-table-column prop="assignee_name" label="执行人" width="110">
          <template #default="{ row }">{{ row.assignee_name || '未指派' }}</template>
        </el-table-column>
        <el-table-column label="期限" width="170">
          <template #default="{ row }">{{ row.deadline ? new Date(row.deadline).toLocaleString('zh-CN', { hour12: false }) : '--' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusMeta(row.status).type" size="small">{{ statusMeta(row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
            <el-button v-if="row.can_claim" link type="primary" @click.stop="claim(row)">抢单</el-button>
            <el-button v-if="['pending', 'in_progress'].includes(row.status) && userStore.hasPerm('workorder:assign')" link type="primary" @click.stop="openAssign(row)">分派</el-button>
            <el-button v-if="row.can_finish" link type="primary" @click.stop="openDetail(row, true)">整改</el-button>
            <el-button v-if="row.status === 'pending_review'" link type="success" @click.stop="openDetail(row, true)">审核</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="当前没有工单" /></template>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.page_size" :total="total" :page-sizes="[10, 20, 50, 100]" layout="total, sizes, prev, pager, next" @change="fetchList" />
      </div>
    </div>

    <WorkOrderDetailDrawer ref="detailRef" @changed="fetchList" />

    <el-dialog v-model="assignVisible" title="分派工单" width="480px" :close-on-click-modal="false">
      <el-form ref="assignFormRef" :model="assignForm" :rules="assignRules" label-width="88px">
        <el-form-item label="执行人" prop="assignee_id">
          <el-select v-model="assignForm.assignee_id" filterable style="width: 100%" :loading="candidatesLoading">
            <el-option v-for="candidate in candidates" :key="candidate.id" :label="`${candidate.name} · ${candidate.role_name}`" :value="candidate.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="改派原因">
          <el-input v-model="assignForm.reason" type="textarea" :rows="3" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="assigning" :disabled="!candidates.length" @click="submitAssign">确认分派</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, RefreshRight } from '@element-plus/icons-vue'
import { listWorkOrders, claimWorkOrder, assignWorkOrder, listAssignCandidates, type WorkOrderItem, type WorkOrderStatus } from '@/api/workorder'
import { listCommunities } from '@/api/community'
import { usePagedList } from '@/composables/usePagedList'
import { useUserStore } from '@/store/user'
import WorkOrderDetailDrawer from '@/components/WorkOrderDetailDrawer.vue'

const userStore = useUserStore()
const activeTab = ref<WorkOrderStatus>('pending')
const mineOnly = ref(false)
const communities = ref<{ id: string; name: string }[]>([])
const { loading, list, total, query, fetchList, handleSearch: search, handleReset: reset } = usePagedList(
  (q) => listWorkOrders({ ...q, status: activeTab.value, mine: mineOnly.value ? '1' : undefined }),
  () => ({ page: 1, page_size: 20, community_id: '', work_type: '' })
)
function handleSearch() { query.page = 1; search() }
function handleReset() { query.community_id = ''; query.work_type = ''; mineOnly.value = false; query.page = 1; reset() }

const statusMap: Record<WorkOrderStatus, { label: string; type: 'info' | 'warning' | 'primary' | 'success' | 'danger' }> = {
  pending: { label: '待接单', type: 'warning' },
  in_progress: { label: '整改中', type: 'primary' },
  pending_review: { label: '待审核', type: 'warning' },
  done: { label: '已完成', type: 'success' },
  closed: { label: '已关闭', type: 'info' }
}
function statusMeta(status: WorkOrderStatus) { return statusMap[status] }

const detailRef = ref<InstanceType<typeof WorkOrderDetailDrawer>>()
function openDetail(row: WorkOrderItem, action = false) { detailRef.value?.open(row.id, action) }
async function claim(row: WorkOrderItem) {
  await ElMessageBox.confirm(`确认接单「${row.order_no}」吗？`, '抢单确认', { type: 'warning' })
  await claimWorkOrder(row.id)
  ElMessage.success('已接单')
  fetchList()
}

const assignVisible = ref(false)
const assigning = ref(false)
const candidatesLoading = ref(false)
const assignFormRef = ref<FormInstance>()
const current = ref<WorkOrderItem>()
const candidates = ref<{ id: string; name: string; role_name: string }[]>([])
const assignForm = reactive({ assignee_id: '', reason: '' })
const assignRules: FormRules = { assignee_id: [{ required: true, message: '请选择执行人', trigger: 'change' }] }
async function openAssign(row: WorkOrderItem) {
  current.value = row
  assignForm.assignee_id = row.assignee_id || ''
  assignForm.reason = ''
  assignVisible.value = true
  candidatesLoading.value = true
  try {
    candidates.value = await listAssignCandidates(row.community_id, row.work_type)
  } finally {
    candidatesLoading.value = false
  }
}
async function submitAssign() {
  if (!current.value) return
  await assignFormRef.value?.validate()
  assigning.value = true
  try {
    await assignWorkOrder(current.value.id, assignForm)
    ElMessage.success('分派成功')
    assignVisible.value = false
    fetchList()
  } finally {
    assigning.value = false
  }
}

onMounted(async () => {
  fetchList()
  communities.value = (await listCommunities({ page: 1, page_size: 200 })).list
})
</script>
