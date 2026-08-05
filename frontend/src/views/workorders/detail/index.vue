<template>
  <div class="app-container" v-loading="loading">
    <template v-if="detail">
      <!-- 工单头 -->
      <div class="table-card wo-head">
        <h2 class="page-title wo-title">{{ detail.order_no }} · {{ detail.title }}</h2>
        <el-tag :type="statusType(detail.status)">{{ statusLabel(detail.status) }}</el-tag>
        <el-tag :type="priorityType(detail.priority)" effect="plain">{{ priorityLabel(detail.priority) }}</el-tag>
      </div>

      <div class="wo-layout">
        <!-- 左：异常信息 -->
        <div class="table-card wo-left">
          <h3 class="card-title">异常信息</h3>
          <el-descriptions :column="1" border class="wo-desc">
            <el-descriptions-item label="点位">
              {{ detail.community_name }}<template v-if="detail.point_name"> · {{ detail.point_name }}</template>
            </el-descriptions-item>
            <el-descriptions-item label="上报人">{{ detail.reporter_name }} · {{ detail.created_at.slice(5, 16) }}</el-descriptions-item>
            <el-descriptions-item label="处理人">{{ detail.assignee_name || '—' }}</el-descriptions-item>
            <el-descriptions-item label="描述">{{ detail.description }}</el-descriptions-item>
          </el-descriptions>

          <div class="section-label">异常照片</div>
          <photo-viewer :photos="detail.photos || []" />

          <template v-if="detail.fix_remark || detail.fix_photos?.length">
            <div class="section-label">处理反馈</div>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="处理说明">{{ detail.fix_remark }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.finished_at" label="完成时间">{{ detail.finished_at }}</el-descriptions-item>
            </el-descriptions>
            <div class="fix-photos">
              <photo-viewer :photos="detail.fix_photos || []" />
            </div>
          </template>

          <template v-if="detail.review_remark">
            <div class="section-label">复核意见</div>
            <el-alert :title="detail.review_remark" :type="detail.status === 'closed' ? 'success' : 'error'" :closable="false" show-icon />
          </template>
        </div>

        <!-- 右：流转时间线 + 操作 -->
        <div class="table-card wo-right">
          <h3 class="card-title">流转时间线</h3>
          <el-timeline class="wo-timeline">
            <el-timeline-item
              v-for="(log, i) in detail.logs"
              :key="i"
              :timestamp="log.created_at"
              :type="logType(log.action)"
            >
              <div class="log-line">
                <span class="log-operator">{{ log.operator_name }}</span>
                <el-tag size="small" effect="plain">{{ actionLabel(log.action) }}</el-tag>
              </div>
              <div class="log-detail">{{ log.detail }}</div>
            </el-timeline-item>
            <el-timeline-item v-if="nextStepText" :hollow="true" :timestamp="''">
              <span class="text-secondary">{{ nextStepText }}</span>
            </el-timeline-item>
          </el-timeline>

          <!-- 操作按钮按状态渲染 -->
          <div class="wo-actions">
            <el-button
              v-if="(detail.status === 'pending' || detail.status === 'rejected') && userStore.hasPerm('workorder:assign')"
              type="primary"
              @click="assignVisible = true"
            >派单</el-button>
            <el-button
              v-if="(detail.status === 'assigned' || detail.status === 'processing') && userStore.hasPerm('workorder:finish')"
              type="primary"
              @click="finishVisible = true"
            >代录处理反馈</el-button>
            <template v-if="detail.status === 'review' && userStore.hasPerm('workorder:review')">
              <el-button type="success" :loading="reviewing" @click="handleReviewPass">复核通过</el-button>
              <el-button type="danger" @click="rejectVisible = true">驳回</el-button>
            </template>
            <el-button @click="$router.push('/workorders/list')">返回列表</el-button>
          </div>
        </div>
      </div>
    </template>

    <!-- 异常：工单不存在/无数据权限 -->
    <el-result v-else-if="loadError" icon="error" title="工单不存在或无权限查看" class="error-result">
      <template #extra>
        <el-button type="primary" @click="$router.push('/workorders/list')">返回工单列表</el-button>
      </template>
    </el-result>

    <!-- 派单对话框 -->
    <el-dialog v-model="assignVisible" title="派单" width="440px" :close-on-click-modal="false">
      <el-form ref="assignFormRef" :model="assignForm" :rules="assignRules" label-width="88px">
        <el-form-item label="处理人" prop="assignee_id">
          <el-select v-model="assignForm.assignee_id" filterable placeholder="选择维修人员/巡检员" style="width: 100%">
            <el-option v-for="u in assigneeOptions" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="assignForm.remark" placeholder="如：今天内处理" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="assigning" @click="handleAssign">确认派单</el-button>
      </template>
    </el-dialog>

    <!-- 处理反馈对话框（后台代录） -->
    <el-dialog v-model="finishVisible" title="代录处理反馈" width="440px" :close-on-click-modal="false">
      <el-form ref="finishFormRef" :model="finishForm" :rules="finishRules" label-width="88px">
        <el-form-item label="处理说明" prop="fix_remark">
          <el-input v-model="finishForm.fix_remark" type="textarea" :rows="3" placeholder="处理结果说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="finishVisible = false">取消</el-button>
        <el-button type="primary" :loading="finishing" @click="handleFinish">提交反馈</el-button>
      </template>
    </el-dialog>

    <!-- 复核驳回对话框：原因必填 -->
    <el-dialog v-model="rejectVisible" title="驳回复核" width="440px" :close-on-click-modal="false">
      <el-form ref="rejectFormRef" :model="rejectForm" :rules="rejectRules" label-width="88px">
        <el-form-item label="驳回原因" prop="review_remark">
          <el-input v-model="rejectForm.review_remark" type="textarea" :rows="3" placeholder="必填，说明需整改的内容" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="reviewing" @click="handleReviewReject">确认驳回</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  getWorkOrder, assignWorkOrder, finishWorkOrder, reviewWorkOrder
} from '@/api/workorder'
import { listUsers } from '@/api/user'
import { useUserStore } from '@/store/user'
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { WorkOrderDetail } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const route = useRoute()
const userStore = useUserStore()
const loading = ref(false)
const loadError = ref(false)
const detail = ref<WorkOrderDetail | null>(null)
const assigneeOptions = ref<UserItem[]>([])

async function fetchDetail() {
  loading.value = true
  try {
    detail.value = await getWorkOrder(String(route.params.id))
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  fetchDetail()
  const uData = await listUsers({ page: 1, page_size: 100, status: 1 })
  assigneeOptions.value = uData.list
})

function statusLabel(s: string) {
  return { pending: '待派单', assigned: '已派单', processing: '处理中', review: '待复核', closed: '已关闭', rejected: '已驳回' }[s] || s
}

function statusType(s: string) {
  return ({ pending: 'warning', assigned: 'primary', processing: 'primary', review: 'warning', closed: 'success', rejected: 'danger' } as Record<string, any>)[s] || 'info'
}

function priorityLabel(p: string) {
  return { urgent: '紧急', high: '高', normal: '一般', low: '低' }[p] || p
}

function priorityType(p: string) {
  return ({ urgent: 'danger', high: 'warning', normal: 'warning', low: 'info' } as Record<string, any>)[p] || 'info'
}

// 时间线动作
function actionLabel(a: string) {
  return {
    report: '上报', create: '建单', assign: '派单', accept: '接单',
    finish: '处理反馈', review_pass: '复核通过', review_reject: '复核驳回', close: '关闭'
  }[a] || a
}

function logType(a: string) {
  return ({ review_reject: 'danger', review_pass: 'success', close: 'info', finish: 'primary' } as Record<string, any>)[a] || 'primary'
}

// 下一待办提示
const nextStepText = computed(() => {
  const s = detail.value?.status
  return ({
    pending: '待派单', assigned: '等待处理人接单', processing: '等待处理反馈',
    review: '待复核', rejected: '已驳回，可重新派单'
  } as Record<string, string>)[s || ''] || ''
})

// ===== 派单 =====
const assignVisible = ref(false)
const assigning = ref(false)
const assignFormRef = ref<FormInstance>()
const assignForm = reactive({ assignee_id: null as string | null, remark: '' })

const assignRules: FormRules = {
  assignee_id: [{ required: true, message: '请选择处理人', trigger: 'change' }]
}

async function handleAssign() {
  await assignFormRef.value?.validate()
  assigning.value = true
  try {
    await assignWorkOrder(detail.value!.id, { assignee_id: assignForm.assignee_id!, remark: assignForm.remark || undefined })
    ElMessage.success('已派单')
    assignVisible.value = false
    fetchDetail()
  } finally {
    assigning.value = false
  }
}

// ===== 处理反馈（代录） =====
const finishVisible = ref(false)
const finishing = ref(false)
const finishFormRef = ref<FormInstance>()
const finishForm = reactive({ fix_remark: '' })

const finishRules: FormRules = {
  fix_remark: [{ required: true, message: '请填写处理说明', trigger: 'blur' }]
}

async function handleFinish() {
  await finishFormRef.value?.validate()
  finishing.value = true
  try {
    await finishWorkOrder(detail.value!.id, { fix_remark: finishForm.fix_remark })
    ElMessage.success('处理反馈已提交，工单进入待复核')
    finishVisible.value = false
    finishForm.fix_remark = ''
    fetchDetail()
  } finally {
    finishing.value = false
  }
}

// ===== 复核 =====
const reviewing = ref(false)
const rejectVisible = ref(false)
const rejectFormRef = ref<FormInstance>()
const rejectForm = reactive({ review_remark: '' })

const rejectRules: FormRules = {
  review_remark: [{ required: true, message: '驳回必须填写原因', trigger: 'blur' }]
}

async function handleReviewPass() {
  const ok = await ElMessageBox.confirm('复核通过后工单将关闭，并通知巡检员与处理人，确定通过吗？', '复核通过', {
    confirmButtonText: '复核通过',
    cancelButtonText: '取消',
    type: 'success'
  }).then(() => true).catch(() => false)
  if (!ok) return
  reviewing.value = true
  try {
    await reviewWorkOrder(detail.value!.id, { result: 'pass' })
    ElMessage.success('复核通过，工单已关闭')
    fetchDetail()
  } finally {
    reviewing.value = false
  }
}

async function handleReviewReject() {
  await rejectFormRef.value?.validate()
  reviewing.value = true
  try {
    await reviewWorkOrder(detail.value!.id, { result: 'reject', review_remark: rejectForm.review_remark })
    ElMessage.success('已驳回，可重新派单')
    rejectVisible.value = false
    rejectForm.review_remark = ''
    fetchDetail()
  } finally {
    reviewing.value = false
  }
}
</script>

<style scoped lang="scss">
.wo-head {
  display: flex;
  align-items: center;
  gap: $spacing-md;
  margin-bottom: $spacing-lg;

  .wo-title {
    margin: 0;
  }
}

.wo-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.wo-left {
  flex: 3;
  min-width: 0;

  .wo-desc {
    margin-top: $spacing-md;
  }

  .section-label {
    font-weight: 600;
    margin: $spacing-lg 0 $spacing-sm;
    color: $color-text-primary;
  }

  .fix-photos {
    margin-top: $spacing-sm;
  }
}

.wo-right {
  flex: 2;
  min-width: 0;

  .wo-timeline {
    margin-top: $spacing-lg;
  }

  .log-line {
    display: flex;
    align-items: center;
    gap: $spacing-sm;

    .log-operator {
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  .log-detail {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin-top: $spacing-xs;
  }

  .wo-actions {
    margin-top: $spacing-xl;
    padding-top: $spacing-lg;
    border-top: 1px solid $color-border;
    display: flex;
    gap: $spacing-sm;
    flex-wrap: wrap;
  }
}

.error-result {
  background: $color-bg-card;
  border-radius: $radius-card;
}
</style>
