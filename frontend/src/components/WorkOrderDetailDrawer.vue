<template>
  <el-drawer v-model="visible" :title="detail ? `${detail.order_no} · ${statusLabel(detail.status)}` : '工单详情'" size="820px" :close-on-click-modal="false">
    <div v-loading="loading" class="order-detail">
      <template v-if="detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="小区">{{ detail.community_name }}</el-descriptions-item>
          <el-descriptions-item label="执行人">{{ detail.assignee_name || '未指派' }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ detail.work_type === 'inspection_fix' ? '巡检整改' : '计划维保' }}</el-descriptions-item>
          <el-descriptions-item label="优先级">{{ detail.priority === 'high' ? '高' : '普通' }}</el-descriptions-item>
          <el-descriptions-item label="期限">{{ formatTime(detail.deadline) }}</el-descriptions-item>
          <el-descriptions-item label="当前版本">{{ detail.revision }}</el-descriptions-item>
        </el-descriptions>

        <div class="section-title">整改事项（{{ detail.issues.length }}）</div>
        <div v-for="issue in detail.issues" :key="issue.id" class="issue-card">
          <div class="issue-head">
            <div>
              <strong>{{ issue.title }}</strong>
              <div class="issue-location">{{ issue.location }} · {{ issue.problem }}</div>
            </div>
            <el-tag size="small" :type="statusType(issue.status)">{{ statusLabel(issue.status) }}</el-tag>
          </div>
          <div class="evidence-grid">
            <div>
              <div class="evidence-label">问题照片</div>
              <PhotoViewer :photos="issue.photos" />
            </div>
            <div>
              <div class="evidence-label">整改记录</div>
              <div v-for="attempt in attemptsOf(issue.id)" :key="attempt.id" class="attempt-item">
                <span>第 {{ attempt.round }} 轮 · {{ formatTime(attempt.submitted_at) }}</span>
                <div>{{ attempt.note }}</div>
                <PhotoViewer :photos="attempt.photos" />
              </div>
              <span v-if="!attemptsOf(issue.id).length" class="text-secondary">尚未提交整改</span>
            </div>
          </div>
          <div v-if="detail.can_finish && issue.status === 'in_progress'" class="issue-actions">
            <el-button size="small" type="primary" @click="openFinish(issue)">提交整改</el-button>
          </div>
          <div v-if="detail.can_review && issue.status === 'pending_review'" class="issue-actions">
            <el-button size="small" type="danger" plain @click="openReview(issue, 'reject')">驳回整改</el-button>
            <el-button size="small" type="success" :loading="reviewing" @click="openReview(issue, 'approve')">审核通过</el-button>
          </div>
        </div>

        <div class="section-title">操作时间线</div>
        <el-timeline>
          <el-timeline-item v-for="event in detail.events" :key="`${event.created_at}-${event.revision}-${event.event_type}`" :timestamp="formatTime(event.created_at)">
            {{ event.actor_name || '系统' }} · {{ eventLabel(event.event_type) }}<span v-if="event.reason">：{{ event.reason }}</span>
          </el-timeline-item>
        </el-timeline>
      </template>
    </div>
    <template #footer>
      <el-button v-if="detail && !['done', 'closed'].includes(detail.status) && userStore.hasPerm('workorder:close')" type="danger" plain @click="closeVisible = true">关闭工单</el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-drawer>

  <el-dialog v-model="finishVisible" title="提交整改" width="560px" :close-on-click-modal="false">
    <el-form ref="finishFormRef" :model="finishForm" :rules="finishRules" label-width="88px">
      <el-form-item label="整改说明" prop="note">
        <el-input v-model="finishForm.note" type="textarea" :rows="4" maxlength="1000" show-word-limit />
      </el-form-item>
      <el-form-item label="整改照片" prop="photos">
        <el-upload multiple list-type="picture-card" accept="image/*" :auto-upload="false" :on-change="addPhoto" :on-remove="removePhoto">
          <el-icon><Plus /></el-icon>
        </el-upload>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="finishVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitFinish">提交审核</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="reviewVisible" :title="reviewAction === 'approve' ? '审核通过' : '驳回整改'" width="480px" :close-on-click-modal="false">
    <el-form label-width="88px">
      <el-form-item :label="reviewAction === 'approve' ? '审核意见' : '驳回原因'" :required="reviewAction === 'reject'">
        <el-input v-model="reviewReason" type="textarea" :rows="3" maxlength="500" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="reviewVisible = false">取消</el-button>
      <el-button :type="reviewAction === 'approve' ? 'success' : 'danger'" :loading="reviewing" @click="submitReview">确认</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="closeVisible" title="关闭工单" width="480px" :close-on-click-modal="false">
    <el-input v-model="closeReason" type="textarea" :rows="3" maxlength="500" placeholder="请输入无效或重复工单的原因" />
    <template #footer>
      <el-button @click="closeVisible = false">取消</el-button>
      <el-button type="danger" :loading="closing" @click="submitClose">确认关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules, type UploadFile } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getWorkOrder, finishWorkOrder, reviewWorkOrder, closeWorkOrder, type WorkOrderDetail, type WorkOrderIssue, type WorkOrderStatus } from '@/api/workorder'
import { uploadImage, withFileToken } from '@/api/upload'
import PhotoViewer from '@/components/PhotoViewer.vue'
import { useUserStore } from '@/store/user'

const emit = defineEmits<{ changed: [] }>()
const userStore = useUserStore()
const visible = ref(false)
const loading = ref(false)
const detail = ref<WorkOrderDetail>()
let orderId = ''

async function open(id: string, _action = false) {
  orderId = id
  visible.value = true
  await reload()
}
async function reload() {
  loading.value = true
  try {
    const data = await getWorkOrder(orderId)
    data.issues.forEach((issue) => issue.photos.forEach((photo) => {
      photo.url = withFileToken(photo.url)
      photo.watermarked_url = withFileToken(photo.watermarked_url || photo.url)
    }))
    data.attempts.forEach((attempt) => attempt.photos.forEach((photo) => {
      photo.url = withFileToken(photo.url)
      photo.watermarked_url = withFileToken(photo.watermarked_url || photo.url)
    }))
    detail.value = data
  } finally {
    loading.value = false
  }
}
defineExpose({ open })

const statusMap: Record<WorkOrderStatus, string> = {
  pending: '待接单', in_progress: '整改中', pending_review: '待审核', done: '已完成', closed: '已关闭'
}
function statusLabel(status: WorkOrderStatus) { return statusMap[status] }
function statusType(status: WorkOrderStatus) {
  if (status === 'done') return 'success'
  if (status === 'pending_review') return 'warning'
  if (status === 'closed') return 'info'
  return 'primary'
}
function formatTime(value?: string | null) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }
function eventLabel(type: string) {
  return ({ created: '创建工单', claimed: '抢单', assigned: '分派', fix_submitted: '提交整改', review_approve: '审核通过', review_reject: '驳回整改', closed: '关闭工单' } as Record<string, string>)[type] || type
}
const attemptsOf = computed(() => (issueId: string) => detail.value?.attempts.filter((attempt) => attempt.issue_id === issueId) || [])

const finishVisible = ref(false)
const submitting = ref(false)
const finishFormRef = ref<FormInstance>()
const finishForm = reactive({ issue_id: '', note: '', photos: [] as string[] })
const finishRules: FormRules = {
  note: [{ required: true, message: '请输入整改说明', trigger: 'blur' }],
  photos: [{ required: true, type: 'array', min: 1, message: '请上传整改照片', trigger: 'change' }]
}
function openFinish(issue: WorkOrderIssue) {
  finishForm.issue_id = issue.id
  finishForm.note = ''
  finishForm.photos = []
  finishVisible.value = true
}
async function addPhoto(file: UploadFile) {
  if (!file.raw) return
  const result = await uploadImage(file.raw, 'workorder')
  finishForm.photos.push(result.file_id)
  ElMessage.success('照片已上传')
}
function removePhoto(file: UploadFile) {
  const index = finishForm.photos.findIndex((_, i) => i === file.uid)
  if (index >= 0) finishForm.photos.splice(index, 1)
}
async function submitFinish() {
  await finishFormRef.value?.validate()
  submitting.value = true
  try {
    await finishWorkOrder(orderId, finishForm)
    ElMessage.success('整改已提交，等待主管审核')
    finishVisible.value = false
    await reload()
    emit('changed')
  } finally {
    submitting.value = false
  }
}

const reviewVisible = ref(false)
const reviewing = ref(false)
const reviewAction = ref<'approve' | 'reject'>('approve')
const reviewReason = ref('')
let reviewIssueId = ''
function openReview(issue: WorkOrderIssue, action: 'approve' | 'reject') {
  reviewIssueId = issue.id
  reviewAction.value = action
  reviewReason.value = ''
  reviewVisible.value = true
}
async function submitReview() {
  if (reviewAction.value === 'reject' && !reviewReason.value.trim()) {
    ElMessage.warning('请填写驳回原因')
    return
  }
  reviewing.value = true
  try {
    await reviewWorkOrder(orderId, { issue_id: reviewIssueId, action: reviewAction.value, reason: reviewReason.value })
    ElMessage.success(reviewAction.value === 'approve' ? '审核通过' : '已驳回整改')
    reviewVisible.value = false
    await reload()
    emit('changed')
  } finally {
    reviewing.value = false
  }
}

const closeVisible = ref(false)
const closing = ref(false)
const closeReason = ref('')
async function submitClose() {
  if (!closeReason.value.trim()) {
    ElMessage.warning('请填写关闭原因')
    return
  }
  closing.value = true
  try {
    await closeWorkOrder(orderId, { reason: closeReason.value })
    ElMessage.success('工单已关闭')
    closeVisible.value = false
    visible.value = false
    emit('changed')
  } finally {
    closing.value = false
  }
}
</script>

<style scoped lang="scss">
.order-detail {
  min-height: 240px;
}
.section-title {
  margin: 22px 0 12px;
  font-weight: 600;
  color: $color-text-primary;
}
.issue-card {
  border: 1px solid $color-border;
  border-radius: $radius-card;
  margin-bottom: 12px;
  overflow: hidden;
}
.issue-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: $color-table-header;
}
.issue-location {
  margin-top: 4px;
  color: $color-text-secondary;
}
.evidence-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
  padding: 14px;
}
.evidence-label {
  margin-bottom: 8px;
  color: $color-text-secondary;
}
.attempt-item {
  margin-bottom: 12px;
  display: grid;
  gap: 6px;
}
.issue-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 14px 14px;
}
</style>
