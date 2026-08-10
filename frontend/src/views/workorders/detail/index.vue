<template>
  <div class="app-container" v-loading="loading">
    <template v-if="detail">
      <!-- 工单头：标题行 + 元信息行 -->
      <div class="table-card wo-head">
        <div class="wo-head-main">
          <div class="wo-title-row">
            <h2 class="page-title wo-title">{{ detail.order_no }} · {{ detail.title }}</h2>
            <el-tag :type="statusType(detail.status)">{{ statusLabel(detail.status) }}</el-tag>
            <el-tag :type="priorityType(detail.priority)" effect="plain">{{ priorityLabel(detail.priority) }}</el-tag>
          </div>
          <div class="wo-meta">
            <span>{{ detail.community_name }}<template v-if="detail.point_name"> · {{ detail.point_name }}</template></span>
            <span class="wo-meta-sep" />
            <span>上报：{{ detail.reporter_name }} {{ detail.created_at }}</span>
            <span class="wo-meta-sep" />
            <span>处理人：{{ detail.assignee_name || '待派单' }}</span>
            <template v-if="detail.finished_at">
              <span class="wo-meta-sep" />
              <span>完成：{{ detail.finished_at }}</span>
            </template>
          </div>
        </div>
        <el-button class="wo-back" @click="$router.push('/workorders/list')">返回列表</el-button>
      </div>

      <div class="wo-layout">
        <!-- 左：异常信息 -->
        <div class="table-card wo-left">
          <div class="wo-section">
            <h3 class="card-title">异常描述</h3>
            <p class="wo-desc-text">{{ detail.description }}</p>
          </div>

          <div class="wo-section">
            <h3 class="card-title">现场照片</h3>
            <photo-viewer :photos="detail.photos || []" />
          </div>

          <!-- 不合格项快照：整改前后双列对比（旧工单无 items 时不展示） -->
          <div v-if="detail.items?.length" class="wo-section">
            <h3 class="card-title">不合格项整改对比</h3>
            <div v-for="(item, i) in detail.items" :key="i" class="wo-item">
              <div class="wo-item-head">
                <span class="wo-item-name">{{ item.name }}</span>
                <span v-if="item.remark" class="text-secondary">{{ item.remark }}</span>
              </div>
              <div class="compare-grid">
                <div class="compare-col">
                  <div class="compare-col-head">整改前</div>
                  <div class="thumb-row">
                    <template v-if="item.before_photo_urls?.length">
                      <el-image
                        v-for="(url, j) in item.before_photo_urls"
                        :key="j"
                        :src="url"
                        :preview-src-list="item.before_photo_urls"
                        :initial-index="j"
                        fit="cover"
                        preview-teleported
                        class="thumb-img"
                      />
                    </template>
                    <div v-else class="thumb-empty">无照片</div>
                  </div>
                </div>
                <div v-if="hasAfterPhotos(item)" class="compare-col">
                  <div class="compare-col-head">整改后</div>
                  <div class="thumb-row">
                    <template v-if="item.after_photo_urls?.length">
                      <el-image
                        v-for="(url, j) in item.after_photo_urls"
                        :key="j"
                        :src="url"
                        :preview-src-list="item.after_photo_urls"
                        :initial-index="j"
                        fit="cover"
                        preview-teleported
                        class="thumb-img"
                      />
                    </template>
                    <div v-else class="thumb-empty">未回传</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="detail.fix_remark || detail.fix_photos?.length" class="wo-section">
            <h3 class="card-title">处理反馈</h3>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="处理说明">{{ detail.fix_remark }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.finished_at" label="完成时间">{{ detail.finished_at }}</el-descriptions-item>
            </el-descriptions>
            <div class="fix-photos">
              <photo-viewer :photos="detail.fix_photos || []" />
            </div>
          </div>

          <div v-if="detail.review_remark" class="wo-section wo-section-last">
            <h3 class="card-title">复核意见</h3>
            <el-alert :title="detail.review_remark" :type="detail.status === 'closed' ? 'success' : 'error'" :closable="false" show-icon />
          </div>
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
          <div v-if="hasActions" class="wo-actions">
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
    <el-dialog v-model="finishVisible" title="代录处理反馈" width="560px" :close-on-click-modal="false">
      <el-form ref="finishFormRef" :model="finishForm" :rules="finishRules" label-width="88px">
        <el-form-item label="处理说明" prop="fix_remark">
          <el-input v-model="finishForm.fix_remark" type="textarea" :rows="3" placeholder="处理结果说明" />
        </el-form-item>
        <!-- 不合格项逐项补传整改后照片（有 items 快照时展示） -->
        <template v-if="detail?.items?.length">
          <el-form-item v-for="(item, i) in detail.items" :key="i" :label="item.name">
            <div class="after-upload">
              <div v-if="finishForm.after_photos[item.name]?.length" class="thumb-row">
                <span v-for="(p, j) in finishForm.after_photos[item.name]" :key="j" class="thumb-wrap">
                  <el-image :src="p.url" fit="cover" class="thumb-img" />
                  <el-icon class="thumb-del" @click="finishForm.after_photos[item.name].splice(j, 1)"><Close /></el-icon>
                </span>
              </div>
              <el-upload
                :show-file-list="false"
                accept="image/*"
                :http-request="(opt: UploadRequestOptions) => handleAfterUpload(opt, item.name)"
              >
                <el-button size="small" :loading="afterUploading[item.name]">上传整改后照片</el-button>
              </el-upload>
            </div>
          </el-form-item>
        </template>
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
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type UploadRequestOptions } from 'element-plus'
import { Close } from '@element-plus/icons-vue'
import {
  getWorkOrder, assignWorkOrder, finishWorkOrder, reviewWorkOrder
} from '@/api/workorder'
import { listUsers } from '@/api/user'
import { uploadImage, fileUrl } from '@/api/upload'
import { useUserStore } from '@/store/user'
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { WorkOrderDetail, WorkOrderCheckItem } from '@/api/biz-types'
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

// 当前状态下是否有可执行操作（无则隐藏操作区，避免空框）
const hasActions = computed(() => {
  const s = detail.value?.status
  return (
    ((s === 'pending' || s === 'rejected') && userStore.hasPerm('workorder:assign')) ||
    ((s === 'assigned' || s === 'processing') && userStore.hasPerm('workorder:finish')) ||
    (s === 'review' && userStore.hasPerm('workorder:review'))
  )
})

// 是否展示"整改后"列：已有整改回传（处理反馈已提交或该项已有整改后照片）
function hasAfterPhotos(item: WorkOrderCheckItem) {
  return !!item.after_photo_urls?.length || !!detail.value?.finished_at || ['review', 'closed'].includes(detail.value?.status || '')
}

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
// after_photos 本地存 {file_key,url} 对象（预览用 url），提交时转为 file_key 数组
const finishForm = reactive({ fix_remark: '', after_photos: {} as Record<string, { file_key: string; url: string }[]> })
const afterUploading = reactive<Record<string, boolean>>({})

const finishRules: FormRules = {
  fix_remark: [{ required: true, message: '请填写处理说明', trigger: 'blur' }]
}

// 整改后照片上传：scene 用 workorder（与小程序端一致）
async function handleAfterUpload(opt: UploadRequestOptions, itemName: string) {
  afterUploading[itemName] = true
  try {
    const { file_key, url } = await uploadImage(opt.file as File, 'workorder')
    if (!finishForm.after_photos[itemName]) finishForm.after_photos[itemName] = []
    finishForm.after_photos[itemName].push({ file_key, url: url || fileUrl(file_key) })
  } catch {
    ElMessage.warning('照片上传失败，请重试')
  } finally {
    afterUploading[itemName] = false
  }
}

async function handleFinish() {
  await finishFormRef.value?.validate()
  finishing.value = true
  try {
    // after_photos 按检查项 name 提交 file_key 数组，仅提交有照片的项
    const after_photos = Object.fromEntries(
      Object.entries(finishForm.after_photos)
        .map(([name, photos]) => [name, photos.map(p => p.file_key)])
        .filter(([, keys]) => (keys as string[]).length)
    )
    await finishWorkOrder(detail.value!.id, {
      fix_remark: finishForm.fix_remark,
      ...(Object.keys(after_photos).length ? { after_photos } : {})
    })
    ElMessage.success('处理反馈已提交，工单进入待复核')
    finishVisible.value = false
    finishForm.fix_remark = ''
    finishForm.after_photos = {}
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
  align-items: flex-start;
  justify-content: space-between;
  gap: $spacing-md;
  margin-bottom: $spacing-lg;

  .wo-head-main {
    min-width: 0;
  }

  .wo-title-row {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    flex-wrap: wrap;

    .wo-title {
      margin: 0;
    }
  }

  .wo-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: $spacing-sm;
    margin-top: $spacing-sm;
    font-size: $font-size-aux;
    color: $color-text-secondary;

    .wo-meta-sep {
      width: 1px;
      height: 12px;
      background: $color-border;
    }
  }

  .wo-back {
    flex-shrink: 0;
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

  // 统一区块：标题 + 内容，区块间分隔线
  .wo-section {
    padding-bottom: $spacing-lg;
    margin-bottom: $spacing-lg;
    border-bottom: 1px solid $color-border;

    &:last-child,
    &.wo-section-last {
      padding-bottom: 0;
      margin-bottom: 0;
      border-bottom: none;
    }

    .card-title {
      margin-bottom: $spacing-md;
    }
  }

  .wo-desc-text {
    margin: 0;
    line-height: 1.7;
    color: $color-text-primary;
  }

  .fix-photos {
    margin-top: $spacing-md;
  }

  .wo-item {
    border: 1px solid $color-border;
    border-radius: $radius-small;
    padding: $spacing-md;
    margin-bottom: $spacing-md;

    &:last-child {
      margin-bottom: 0;
    }

    .wo-item-head {
      display: flex;
      align-items: baseline;
      gap: $spacing-md;
      margin-bottom: $spacing-sm;

      .wo-item-name {
        font-weight: 600;
        color: $color-text-primary;
      }
    }
  }

  // 整改前后等宽双列对比
  .compare-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: $spacing-md;

    .compare-col-head {
      font-size: $font-size-aux;
      color: $color-text-secondary;
      padding-bottom: $spacing-xs;
      margin-bottom: $spacing-sm;
      border-bottom: 1px dashed $color-border;
    }
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

// 整改前/后照片缩略图（区块与代录弹窗共用）
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
  cursor: pointer;
  display: block;
}

// 无照片/未回传占位（与缩略图同尺寸，保证双列对齐）
.thumb-empty {
  width: 96px;
  height: 96px;
  border: 1px dashed $color-border;
  border-radius: $radius-small;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: $font-size-aux;
  color: $color-text-secondary;
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

.after-upload {
  display: flex;
  flex-direction: column;
  gap: $spacing-xs;
  align-items: flex-start;
}
</style>
