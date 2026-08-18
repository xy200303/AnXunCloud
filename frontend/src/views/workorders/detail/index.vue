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
            <el-tag :type="sourceType(detail.source)" effect="plain">{{ sourceLabel(detail.source) }}</el-tag>
            <el-tag v-if="detail.sla_overdue" type="danger">已超时</el-tag>
          </div>
          <div class="wo-meta">
            <span>{{ detail.community_name }}<template v-if="detail.point_name"> · {{ detail.point_name }}</template></span>
            <span class="wo-meta-sep" />
            <span>上报：{{ detail.reporter_name }} {{ detail.created_at }}</span>
            <span class="wo-meta-sep" />
            <span>处理人：{{ detail.assignee_name || '待派单' }}</span>
            <template v-if="detail.category">
              <span class="wo-meta-sep" />
              <span>分类：{{ detail.category }}</span>
            </template>
            <template v-if="detail.sla_deadline && detail.status !== 'closed' && detail.status !== 'closed_invalid'">
              <span class="wo-meta-sep" />
              <span :class="{ 'danger-text': detail.sla_overdue }">期望完成：{{ detail.sla_deadline }}</span>
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

          <!-- 分诊信息（已分诊工单展示） -->
          <div v-if="detail.triage_at" class="wo-section">
            <h3 class="card-title">分诊信息</h3>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="分诊人">{{ detail.triage_by_name || '—' }}</el-descriptions-item>
              <el-descriptions-item label="分诊时间">{{ detail.triage_at }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.triage_note" label="分诊备注">{{ detail.triage_note }}</el-descriptions-item>
            </el-descriptions>
          </div>

          <!-- 完工信息（维修工提交或后台代录） -->
          <div v-if="detail.finish_note || detail.finish_photos?.length" class="wo-section">
            <h3 class="card-title">完工信息</h3>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="完工说明">{{ detail.finish_note }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.finish_at" label="完工时间">{{ detail.finish_at }}</el-descriptions-item>
            </el-descriptions>
            <div class="fix-photos">
              <photo-viewer :photos="detail.finish_photos || []" />
            </div>
          </div>

          <!-- 验收信息 -->
          <div v-if="detail.confirm_at || detail.confirm_note" class="wo-section">
            <h3 class="card-title">验收信息</h3>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="验收人">{{ detail.confirm_by_name || '—' }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.confirm_at" label="验收时间">{{ detail.confirm_at }}</el-descriptions-item>
              <el-descriptions-item v-if="detail.confirm_note" label="验收意见">{{ detail.confirm_note }}</el-descriptions-item>
            </el-descriptions>
          </div>

          <!-- 最近一次驳回原因（分诊驳回 / 验收退回） -->
          <div v-if="detail.reject_reason" class="wo-section wo-section-last">
            <h3 class="card-title">驳回原因</h3>
            <el-alert :title="detail.reject_reason" type="error" :closable="false" show-icon />
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

          <!-- 操作按钮按状态渲染（权限点控制显隐；名单校验在后端，被拦下按返回错误提示） -->
          <div v-if="hasActions" class="wo-actions">
            <el-button
              v-if="detail.status === 'reported' && userStore.hasPerm('workorder:triage')"
              type="primary"
              @click="openTriage"
            >分诊</el-button>
            <el-button
              v-if="detail.status === 'pending_dispatch' && userStore.hasPerm('workorder:dispatch')"
              type="primary"
              @click="openDispatch"
            >派单</el-button>
            <el-button
              v-if="detail.status === 'processing' && userStore.hasPerm('workorder:finish')"
              type="primary"
              @click="finishVisible = true"
            >代录完工</el-button>
            <template v-if="detail.status === 'pending_confirm' && userStore.hasPerm('workorder:confirm')">
              <el-button type="success" :loading="confirming" @click="handleConfirmPass">验收通过</el-button>
              <el-button type="danger" @click="confirmRejectVisible = true">验收退回</el-button>
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

    <!-- 分诊对话框：通过（可定优先级/分类）或驳回（原因必填 → 作废） -->
    <el-dialog v-model="triageVisible" title="工单分诊" width="480px" :close-on-click-modal="false">
      <el-form ref="triageFormRef" :model="triageForm" :rules="triageRules" label-width="88px">
        <el-form-item label="分诊结果" prop="result">
          <el-radio-group v-model="triageForm.result">
            <el-radio value="pass">通过（进入待派单）</el-radio>
            <el-radio value="reject">驳回（工单作废）</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="triageForm.result === 'pass'">
          <el-form-item label="优先级">
            <el-select v-model="triageForm.priority" style="width: 100%">
              <el-option label="一般" value="normal" />
              <el-option label="紧急" value="urgent" />
              <el-option label="高" value="high" />
              <el-option label="低" value="low" />
            </el-select>
          </el-form-item>
          <el-form-item label="工单分类">
            <el-input v-model="triageForm.category" placeholder="选填，如：水电 / 电梯 / 门禁" maxlength="32" />
          </el-form-item>
          <el-form-item label="分诊备注">
            <el-input v-model="triageForm.note" type="textarea" :rows="2" placeholder="选填" maxlength="512" />
          </el-form-item>
        </template>
        <el-form-item v-else label="驳回原因" prop="note">
          <el-input v-model="triageForm.note" type="textarea" :rows="3" placeholder="必填，说明无效原因" maxlength="512" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="triageVisible = false">取消</el-button>
        <el-button :type="triageForm.result === 'pass' ? 'primary' : 'danger'" :loading="triaging" @click="handleTriage">
          {{ triageForm.result === 'pass' ? '分诊通过' : '确认驳回' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 派单对话框 -->
    <el-dialog v-model="dispatchVisible" title="派单" width="440px" :close-on-click-modal="false">
      <el-form ref="dispatchFormRef" :model="dispatchForm" :rules="dispatchRules" label-width="88px">
        <el-form-item label="维修工" prop="assignee_id">
          <el-select v-model="dispatchForm.assignee_id" filterable placeholder="选择维修工" style="width: 100%" :loading="staffLoading">
            <el-option v-for="s in assigneeOptions" :key="s.user_id" :label="s.user_name" :value="s.user_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="dispatchForm.remark" placeholder="如：今天内处理" />
        </el-form-item>
        <el-alert
          type="info"
          :closable="false"
          title="派单对象须为本项目「工单接单」槽位名单成员（候选人取该项目编制中维修工岗位的在职成员；槽位绑定如被项目覆盖，以编制页为准）"
        />
      </el-form>
      <template #footer>
        <el-button @click="dispatchVisible = false">取消</el-button>
        <el-button type="primary" :loading="dispatching" @click="handleDispatch">确认派单</el-button>
      </template>
    </el-dialog>

    <!-- 代录完工对话框 -->
    <el-dialog v-model="finishVisible" title="代录完工" width="560px" :close-on-click-modal="false">
      <el-form ref="finishFormRef" :model="finishForm" :rules="finishRules" label-width="88px">
        <el-form-item label="完工说明" prop="fix_remark">
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
        <el-button type="primary" :loading="finishing" @click="handleFinish">提交完工</el-button>
      </template>
    </el-dialog>

    <!-- 验收退回对话框：原因必填 -->
    <el-dialog v-model="confirmRejectVisible" title="验收退回" width="440px" :close-on-click-modal="false">
      <el-form ref="confirmRejectFormRef" :model="confirmRejectForm" :rules="confirmRejectRules" label-width="88px">
        <el-form-item label="退回原因" prop="confirm_note">
          <el-input v-model="confirmRejectForm.confirm_note" type="textarea" :rows="3" placeholder="必填，说明需整改的内容" maxlength="512" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="confirmRejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="confirming" @click="handleConfirmReject">确认退回</el-button>
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
  getWorkOrder, triageWorkOrder, dispatchWorkOrder, finishWorkOrder, confirmWorkOrder
} from '@/api/workorder'
import { listStaff } from '@/api/community'
import { uploadImage, fileUrl } from '@/api/upload'
import { useUserStore } from '@/store/user'
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { WorkOrderDetail, WorkOrderCheckItem, StaffItem } from '@/api/biz-types'

const route = useRoute()
const userStore = useUserStore()
const loading = ref(false)
const loadError = ref(false)
const detail = ref<WorkOrderDetail | null>(null)

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

onMounted(fetchDetail)

function statusLabel(s: string) {
  return {
    reported: '待分诊', pending_dispatch: '待派单', processing: '处理中',
    pending_confirm: '待验收', closed: '已闭环', closed_invalid: '已作废'
  }[s] || s
}

function statusType(s: string) {
  return ({
    reported: 'danger', pending_dispatch: 'warning', processing: 'primary',
    pending_confirm: 'warning', closed: 'success', closed_invalid: 'info'
  } as Record<string, any>)[s] || 'info'
}

function priorityLabel(p: string) {
  return { urgent: '紧急', high: '高', normal: '一般', low: '低' }[p] || p
}

function priorityType(p: string) {
  return ({ urgent: 'danger', high: 'warning', normal: 'warning', low: 'info' } as Record<string, any>)[p] || 'info'
}

function sourceLabel(s: string) {
  return { inspection: '巡检转单', active: '主动上报', frontdesk: '前台代录' }[s] || s
}

function sourceType(s: string) {
  return ({ inspection: 'warning', active: 'primary', frontdesk: 'info' } as Record<string, any>)[s] || 'info'
}

// 时间线动作
function actionLabel(a: string) {
  return {
    create: '建单',
    triage_pass: '分诊通过', triage_reject: '分诊驳回',
    dispatch: '派单', grab: '抢单',
    finish: '完工提交',
    confirm_pass: '验收通过', confirm_reject: '验收退回'
  }[a] || a
}

function logType(a: string) {
  return ({
    triage_reject: 'danger', confirm_reject: 'danger',
    triage_pass: 'primary', confirm_pass: 'success',
    finish: 'primary'
  } as Record<string, any>)[a] || 'primary'
}

// 下一待办提示
const nextStepText = computed(() => {
  const s = detail.value?.status
  return ({
    reported: '待分诊', pending_dispatch: '待派单', processing: '等待维修工完工提交',
    pending_confirm: '待验收（报单人或分诊名单成员）'
  } as Record<string, string>)[s || ''] || ''
})

// 当前状态下是否有可执行操作（无则隐藏操作区，避免空框）
const hasActions = computed(() => {
  const s = detail.value?.status
  return (
    (s === 'reported' && userStore.hasPerm('workorder:triage')) ||
    (s === 'pending_dispatch' && userStore.hasPerm('workorder:dispatch')) ||
    (s === 'processing' && userStore.hasPerm('workorder:finish')) ||
    (s === 'pending_confirm' && userStore.hasPerm('workorder:confirm'))
  )
})

// 是否展示"整改后"列：已有整改回传（完工已提交或该项已有整改后照片）
function hasAfterPhotos(item: WorkOrderCheckItem) {
  return !!item.after_photo_urls?.length || !!detail.value?.finish_at || ['pending_confirm', 'closed'].includes(detail.value?.status || '')
}

// ===== 分诊 =====
const triageVisible = ref(false)
const triaging = ref(false)
const triageFormRef = ref<FormInstance>()
const triageForm = reactive({
  result: 'pass' as 'pass' | 'reject',
  priority: 'normal',
  category: '',
  note: ''
})

// 驳回时原因必填（动态校验）
const triageRules: FormRules = {
  note: [{
    validator: (_r, v, cb) => {
      if (triageForm.result === 'reject' && !String(v || '').trim()) cb(new Error('驳回必须填写原因'))
      else cb()
    },
    trigger: 'blur'
  }]
}

function openTriage() {
  triageFormRef.value?.clearValidate()
  Object.assign(triageForm, { result: 'pass', priority: detail.value?.priority || 'normal', category: detail.value?.category || '', note: '' })
  triageVisible.value = true
}

async function handleTriage() {
  await triageFormRef.value?.validate()
  triaging.value = true
  try {
    await triageWorkOrder(detail.value!.id, {
      result: triageForm.result,
      priority: triageForm.result === 'pass' ? triageForm.priority : undefined,
      category: triageForm.result === 'pass' ? triageForm.category || undefined : undefined,
      note: triageForm.note || undefined
    })
    ElMessage.success(triageForm.result === 'pass' ? '分诊通过，工单进入待派单' : '已驳回，工单作废')
    triageVisible.value = false
    fetchDetail()
  } finally {
    triaging.value = false
  }
}

// ===== 派单 =====
const dispatchVisible = ref(false)
const dispatching = ref(false)
const dispatchFormRef = ref<FormInstance>()
const dispatchForm = reactive({ assignee_id: null as string | null, remark: '' })
// 候选人：本项目编制中「维修工」岗位的在职成员（接单槽位名单默认绑定维修工岗位）
const staffList = ref<StaffItem[]>([])
const staffLoading = ref(false)
const assigneeOptions = computed(() =>
  staffList.value.filter((s) => s.status === 1 && s.posts?.includes('repairman'))
)

const dispatchRules: FormRules = {
  assignee_id: [{ required: true, message: '请选择维修工', trigger: 'change' }]
}

async function openDispatch() {
  dispatchFormRef.value?.clearValidate()
  Object.assign(dispatchForm, { assignee_id: null, remark: '' })
  dispatchVisible.value = true
  if (!staffList.value.length && detail.value) {
    staffLoading.value = true
    try {
      staffList.value = await listStaff(detail.value.community_id)
    } finally {
      staffLoading.value = false
    }
  }
}

async function handleDispatch() {
  await dispatchFormRef.value?.validate()
  dispatching.value = true
  try {
    await dispatchWorkOrder(detail.value!.id, { assignee_id: dispatchForm.assignee_id!, remark: dispatchForm.remark || undefined })
    ElMessage.success('已派单，工单进入处理中')
    dispatchVisible.value = false
    fetchDetail()
  } finally {
    dispatching.value = false
  }
}

// ===== 完工（代录） =====
const finishVisible = ref(false)
const finishing = ref(false)
const finishFormRef = ref<FormInstance>()
// after_photos 本地存 {file_key,url} 对象（预览用 url），提交时转为 file_key 数组
const finishForm = reactive({ fix_remark: '', after_photos: {} as Record<string, { file_key: string; url: string }[]> })
const afterUploading = reactive<Record<string, boolean>>({})

const finishRules: FormRules = {
  fix_remark: [{ required: true, message: '请填写完工说明', trigger: 'blur' }]
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
    ElMessage.success('完工已提交，工单进入待验收')
    finishVisible.value = false
    finishForm.fix_remark = ''
    finishForm.after_photos = {}
    fetchDetail()
  } finally {
    finishing.value = false
  }
}

// ===== 验收 =====
const confirming = ref(false)
const confirmRejectVisible = ref(false)
const confirmRejectFormRef = ref<FormInstance>()
const confirmRejectForm = reactive({ confirm_note: '' })

const confirmRejectRules: FormRules = {
  confirm_note: [{ required: true, message: '退回必须填写原因', trigger: 'blur' }]
}

async function handleConfirmPass() {
  const ok = await ElMessageBox.confirm('验收通过后工单将闭环归档，确定通过吗？', '验收通过', {
    confirmButtonText: '验收通过',
    cancelButtonText: '取消',
    type: 'success'
  }).then(() => true).catch(() => false)
  if (!ok) return
  confirming.value = true
  try {
    await confirmWorkOrder(detail.value!.id, { result: 'pass' })
    ElMessage.success('验收通过，工单已闭环')
    fetchDetail()
  } finally {
    confirming.value = false
  }
}

async function handleConfirmReject() {
  await confirmRejectFormRef.value?.validate()
  confirming.value = true
  try {
    await confirmWorkOrder(detail.value!.id, { result: 'reject', confirm_note: confirmRejectForm.confirm_note })
    ElMessage.success('已退回，工单返回处理中')
    confirmRejectVisible.value = false
    confirmRejectForm.confirm_note = ''
    fetchDetail()
  } finally {
    confirming.value = false
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

.danger-text {
  color: $color-danger;
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
