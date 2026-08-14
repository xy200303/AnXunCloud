<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 160px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="期间">
          <el-date-picker
            v-model="query.period"
            type="month"
            placeholder="全部期间"
            value-format="YYYY-MM"
            clearable
            style="width: 140px"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 150px">
            <el-option v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="pendingMine" label="只看待我签" @change="handleSearch" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left" />
        <el-button v-perms="'report:generate'" type="primary" :icon="Plus" @click="openGenerate">生成报告</el-button>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="title" label="报告标题" min-width="220" show-overflow-tooltip />
        <el-table-column prop="community_name" label="小区" min-width="110" show-overflow-tooltip />
        <el-table-column prop="period" label="期间" width="90" align="center" />
        <el-table-column label="状态" width="130" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status).type" size="small">{{ statusTag(row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="巡检员确认" width="100" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-secondary': row.inspector_total === 0 }">
              {{ row.inspector_signed_count }}/{{ row.inspector_total }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="主管" width="90" align="center">
          <template #default="{ row }">{{ row.supervisor_name || '--' }}</template>
        </el-table-column>
        <el-table-column label="经理" width="90" align="center">
          <template #default="{ row }">{{ row.manager_name || '--' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="生成时间" width="160" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button v-perms="'report:download'" link type="primary" @click="handleDownload(row)">下载PDF</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该条件下暂无报告，可点击右上角「生成报告」" />
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

    <!-- 报告详情抽屉 -->
    <el-drawer v-model="detailVisible" title="报告详情" size="680px">
      <div v-loading="detailLoading" class="detail-body">
        <template v-if="detail">
          <!-- 概要 -->
          <div class="detail-header">
            <div class="detail-point">
              <span class="point-name">{{ detail.title }}</span>
              <span class="text-secondary">{{ detail.community_name }} · {{ detail.period }}</span>
            </div>
            <el-tag :type="statusTag(detail.status).type">{{ statusTag(detail.status).label }}</el-tag>
          </div>

          <!-- 驳回原因（最近一次） -->
          <el-alert
            v-if="detail.reject_reason"
            type="error"
            :closable="false"
            class="reject-alert"
            :title="`最近驳回原因：${detail.reject_reason}`"
          />

          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="小区">{{ detail.community_name }}</el-descriptions-item>
            <el-descriptions-item label="期间">{{ detail.period }}</el-descriptions-item>
            <el-descriptions-item label="生成时间">{{ detail.created_at }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ detail.updated_at }}</el-descriptions-item>
          </el-descriptions>

          <!-- 汇总统计 -->
          <div class="section-title">汇总统计</div>
          <el-descriptions :column="3" border size="small">
            <el-descriptions-item label="任务总数">{{ stats.task_total }}</el-descriptions-item>
            <el-descriptions-item label="已完成">{{ stats.task_done }}</el-descriptions-item>
            <el-descriptions-item label="逾期">{{ stats.task_overdue }}</el-descriptions-item>
            <el-descriptions-item label="应巡点位">{{ stats.should_points }}</el-descriptions-item>
            <el-descriptions-item label="已巡点位">{{ stats.done_points }}</el-descriptions-item>
            <el-descriptions-item label="覆盖率">{{ stats.coverage_rate }}%</el-descriptions-item>
            <el-descriptions-item label="异常打卡">
              <span :class="{ 'danger-text': stats.abnormal_count > 0 }">{{ stats.abnormal_count }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="疑似作弊">
              <span :class="{ 'danger-text': stats.suspect_count > 0 }">{{ stats.suspect_count }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="新建工单">{{ stats.wo_created }}</el-descriptions-item>
            <el-descriptions-item label="已闭环工单">{{ stats.wo_closed }}</el-descriptions-item>
            <el-descriptions-item label="未闭环工单">{{ stats.wo_unclosed }}</el-descriptions-item>
            <el-descriptions-item label="工单闭环率">{{ stats.wo_close_rate }}%</el-descriptions-item>
          </el-descriptions>

          <!-- 逐日明细 -->
          <template v-if="stats.daily.length">
            <div class="section-title">逐日明细</div>
            <el-table :data="stats.daily" border size="small" max-height="260" style="width: 100%">
              <el-table-column prop="date" label="日期" width="110" />
              <el-table-column prop="task_total" label="任务数" width="90" align="center" />
              <el-table-column prop="task_done" label="已完成" width="90" align="center" />
              <el-table-column label="异常打卡" min-width="90" align="center">
                <template #default="{ row }">
                  <span :class="{ 'danger-text': row.abnormal > 0 }">{{ row.abnormal }}</span>
                </template>
              </el-table-column>
            </el-table>
          </template>

          <!-- 巡检记录明细 -->
          <template v-if="detail.records?.length">
            <div class="section-title">巡检记录明细</div>
            <el-table :data="detail.records" border size="small" max-height="320" style="width: 100%">
              <el-table-column prop="checkin_time" label="打卡时间" width="145" />
              <el-table-column prop="inspector_name" label="巡检员" width="75" />
              <el-table-column prop="point_name" label="点位" min-width="90" show-overflow-tooltip />
              <el-table-column label="方式" width="85" align="center">
                <template #default="{ row }">{{ checkinTypeLabel(row.checkin_type) }}</template>
              </el-table-column>
              <el-table-column label="结果" width="70" align="center">
                <template #default="{ row }">
                  <el-tag :type="resultTag(row).type" size="small">{{ resultTag(row).label }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="审核状态" width="90" align="center">
                <template #default="{ row }">
                  <el-tag :type="auditStatusTag(row.audit_status).type" size="small">
                    {{ auditStatusTag(row.audit_status).label }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="照片" width="110" align="center">
                <template #default="{ row }">
                  <template v-if="row.photos.length">
                    <el-image
                      v-for="(p, i) in row.photos"
                      :key="i"
                      :src="p.url"
                      fit="cover"
                      class="record-photo"
                      :preview-src-list="row.photos.map((x: { url: string }) => x.url)"
                      :initial-index="i"
                      preview-teleported
                    />
                  </template>
                  <span v-else class="text-secondary">无</span>
                </template>
              </el-table-column>
            </el-table>
          </template>

          <!-- 三级签字进度 -->
          <div class="section-title">三级签字</div>
          <el-steps :active="signActive" align-center finish-status="success" class="sign-steps">
            <el-step title="巡检员确认" :description="inspectorStepDesc" />
            <el-step title="安全主管审批" :description="supervisorStepDesc" />
            <el-step title="物业经理终审" :description="managerStepDesc" />
          </el-steps>

          <div class="sign-block">
            <div class="sign-block-title">巡检员电子确认（{{ signedCount }}/{{ detail.inspectors.length }}）</div>
            <div v-if="detail.inspectors.length" class="inspector-list">
              <div v-for="p in detail.inspectors" :key="p.user_id" class="inspector-item">
                <span class="inspector-name">
                  {{ p.name }}
                  <el-image
                    v-if="p.signature_url"
                    :src="withFileToken(p.signature_url)"
                    fit="contain"
                    class="sign-img"
                    :preview-src-list="[withFileToken(p.signature_url)]"
                    preview-teleported
                  />
                </span>
                <el-tag :type="p.signed ? 'success' : 'info'" size="small">
                  {{ p.signed ? `已确认 ${p.signed_at}` : '待确认' }}
                </el-tag>
                <el-tooltip v-if="p.proxy_name" :content="`代签原因：${p.proxy_reason}`" placement="top">
                  <el-tag type="warning" size="small">{{ p.proxy_name }} 代签</el-tag>
                </el-tooltip>
              </div>
            </div>
            <div v-else class="text-secondary">当月无应确认巡检员</div>
          </div>

          <div class="sign-block">
            <div class="sign-block-title">安全主管审批</div>
            <template v-if="detail.supervisor_name">
              <div class="sign-line">
                <el-tag type="success" size="small">已通过</el-tag>
                <span>{{ detail.supervisor_name }}</span>
                <el-image
                  v-if="detail.supervisor_signature_url"
                  :src="withFileToken(detail.supervisor_signature_url)"
                  fit="contain"
                  class="sign-img"
                  :preview-src-list="[withFileToken(detail.supervisor_signature_url)]"
                  preview-teleported
                />
                <span class="text-secondary">{{ detail.supervisor_at }}</span>
              </div>
              <div v-if="detail.supervisor_remark" class="sign-remark">审批意见：{{ detail.supervisor_remark }}</div>
            </template>
            <div v-else-if="!detail.supervisors?.length" class="text-secondary">未指定签字人，该级已跳过（PDF 签字栏留空）</div>
            <div v-else class="sign-line">
              <el-tag type="warning" size="small">待审批</el-tag>
              <span class="text-secondary">签字人：{{ detail.supervisors.map((p) => p.name).join('、') }}（任一签署即可）</span>
            </div>
          </div>

          <div class="sign-block">
            <div class="sign-block-title">物业经理终审</div>
            <template v-if="detail.manager_name">
              <div class="sign-line">
                <el-tag type="success" size="small">已通过</el-tag>
                <span>{{ detail.manager_name }}</span>
                <el-image
                  v-if="detail.manager_signature_url"
                  :src="withFileToken(detail.manager_signature_url)"
                  fit="contain"
                  class="sign-img"
                  :preview-src-list="[withFileToken(detail.manager_signature_url)]"
                  preview-teleported
                />
                <span class="text-secondary">{{ detail.manager_at }}</span>
              </div>
              <div v-if="detail.manager_remark" class="sign-remark">终审意见：{{ detail.manager_remark }}</div>
            </template>
            <div v-else-if="!detail.managers?.length" class="text-secondary">未指定签字人，该级已跳过（PDF 签字栏留空）</div>
            <div v-else class="sign-line">
              <el-tag type="warning" size="small">待终审</el-tag>
              <span class="text-secondary">签字人：{{ detail.managers.map((p) => p.name).join('、') }}（任一签署即可）</span>
            </div>
          </div>

          <!-- 签字操作区 -->
          <div v-if="showInspectorSign || showProxySign || showSupervisorSign || showManagerSign || separationHint" class="sign-actions">
            <el-button
              v-if="showInspectorSign"
              type="primary"
              :icon="CircleCheck"
              :loading="signing"
              @click="handleInspectorSign"
            >
              确认签字
            </el-button>
            <el-button
              v-if="showProxySign"
              type="warning"
              plain
              :icon="EditPen"
              @click="openProxySign"
            >
              代签
            </el-button>
            <template v-if="showSupervisorSign || showManagerSign">
              <el-button type="success" :icon="CircleCheck" :loading="signing" @click="handleApprove">通过</el-button>
            </template>
            <el-button v-if="showRejectBtn" type="danger" :loading="signing" @click="handleReject">驳回</el-button>
            <span v-if="separationHint" class="text-secondary separation-hint">{{ separationHint }}</span>
          </div>

          <!-- 下载 -->
          <div class="drawer-footer">
            <el-button
              v-perms="'report:download'"
              type="primary"
              plain
              :icon="Download"
              :loading="downloading"
              @click="handleDownload(detail)"
            >
              下载 PDF
            </el-button>
          </div>
        </template>
      </div>
    </el-drawer>

    <!-- 生成报告对话框 -->
    <el-dialog v-model="generateVisible" title="生成报告" width="520px" :close-on-click-modal="false">
      <el-alert
        type="info"
        :closable="false"
        title="同一小区同一期间已存在报告时将重新统计并重置签字流程（已通过不可重算）"
        class="generate-tip"
      />
      <el-form ref="generateFormRef" :model="generateForm" :rules="generateRules" label-width="98px">
        <el-form-item label="小区" prop="community_id">
          <el-select v-model="generateForm.community_id" placeholder="请选择小区" style="width: 100%" @change="loadCandidates">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="期间" prop="period">
          <el-date-picker
            v-model="generateForm.period"
            type="month"
            placeholder="请选择月份"
            value-format="YYYY-MM"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="主管签字人">
          <el-select
            v-model="generateForm.supervisor_ids"
            multiple
            collapse-tags
            collapse-tags-tooltip
            placeholder="选择主管签字人（任一签署即可）"
            :loading="candidatesLoading"
            style="width: 100%"
          >
            <el-option v-for="p in candidates.supervisors" :key="p.id" :label="p.name" :value="p.id">
              <span>{{ p.name }}</span>
              <span v-if="!p.has_signature" class="candidate-warn">未配置签名</span>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="经理签字人">
          <el-select
            v-model="generateForm.manager_ids"
            multiple
            collapse-tags
            collapse-tags-tooltip
            placeholder="选择经理签字人（任一签署即可）"
            :loading="candidatesLoading"
            style="width: 100%"
          >
            <el-option v-for="p in candidates.managers" :key="p.id" :label="p.name" :value="p.id">
              <span>{{ p.name }}</span>
              <span v-if="!p.has_signature" class="candidate-warn">未配置签名</span>
            </el-option>
          </el-select>
        </el-form-item>
        <div class="signer-tip text-secondary">
          默认圈选全部候选人（持有对应签字权限且管辖该小区的人员）；清空某级则该级自动跳过，PDF 签字栏留空。签字人须先配置手写签名方可签字。
        </div>
      </el-form>
      <template #footer>
        <el-button @click="generateVisible = false">取消</el-button>
        <el-button type="primary" :loading="generating" @click="submitGenerate">生成</el-button>
      </template>
    </el-dialog>

    <!-- 代签对话框 -->
    <el-dialog v-model="proxyVisible" title="代签确认" width="440px" :close-on-click-modal="false">
      <el-alert
        type="warning"
        :closable="false"
        title="代签将记录你的身份与原因，PDF 签字栏会标注「由你代签」，请谨慎操作"
        class="generate-tip"
      />
      <el-form ref="proxyFormRef" :model="proxyForm" :rules="proxyRules" label-width="88px">
        <el-form-item label="被代签人" prop="user_id">
          <el-select v-model="proxyForm.user_id" placeholder="选择未确认的巡检员" style="width: 100%">
            <el-option v-for="p in unsignedInspectors" :key="p.user_id" :label="p.name" :value="p.user_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="代签原因" prop="reason">
          <el-input v-model="proxyForm.reason" type="textarea" :rows="2" maxlength="100" show-word-limit placeholder="如：巡检员休假/离职，主管代为确认" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="proxyVisible = false">取消</el-button>
        <el-button type="warning" :loading="signing" @click="submitProxySign">确认代签</el-button>
      </template>
    </el-dialog>

    <!-- 签字时未配置手写签名：弹出签名板现场手写（可选保存下次使用） -->
    <SignaturePad ref="padRef" show-save-option @save="handlePadSave" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Search, Refresh, Plus, Download, CircleCheck, EditPen } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  listReports,
  getReport,
  generateReport,
  getSignCandidates,
  signInspector,
  signSupervisor,
  signManager,
  type ReportItem,
  type ReportDetail,
  type ReportStats,
  type ReportStatus,
  type SignCandidate
} from '@/api/report'
import { listCommunities } from '@/api/community'
import { uploadImage, withFileToken } from '@/api/upload'
import { updateProfile } from '@/api/user'
import SignaturePad from '@/components/SignaturePad.vue'
import { downloadFile } from '@/utils/download'
import { useUserStore } from '@/store/user'
import type { CommunityItem } from '@/api/biz-types'

const userStore = useUserStore()
const loading = ref(false)
const list = ref<ReportItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])

const statusOptions: { label: string; value: ReportStatus }[] = [
  { label: '待巡检员确认', value: 'pending_inspector' },
  { label: '待主管审批', value: 'pending_supervisor' },
  { label: '待经理终审', value: 'pending_manager' },
  { label: '已通过', value: 'approved' }
]

// 状态标签：pending_* 流程中-橙 / approved 已通过-绿
function statusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pending_inspector: { label: '待巡检员确认', type: 'warning' },
      pending_supervisor: { label: '待主管审批', type: 'warning' },
      pending_manager: { label: '待经理终审', type: 'warning' },
      approved: { label: '已通过', type: 'success' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

const query = reactive({
  page: 1,
  page_size: 20,
  community_id: undefined as string | undefined,
  period: undefined as string | undefined,
  status: undefined as string | undefined
})
// 只看待我签（当前用户在报告当前级签字人名单内）
const pendingMine = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const data = await listReports({ ...query, pending_mine: pendingMine.value ? '1' : undefined })
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
  query.community_id = undefined
  query.period = undefined
  query.status = undefined
  handleSearch()
}

onMounted(async () => {
  fetchList()
  const cData = await listCommunities({ page: 1, page_size: 100, status: 1 })
  communities.value = cData.list
})

// ===== 详情抽屉 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<ReportDetail | null>(null)

// 打卡方式 / 结果 / 审核状态中文映射（与打卡记录页一致）
function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传', nfc: 'NFC' }[t] || t
}

function resultTag(row: { result: string; is_suspect: boolean }): { label: string; type: 'success' | 'warning' | 'danger' } {
  if (row.is_suspect) return { label: '疑似', type: 'warning' }
  if (row.result === 'abnormal') return { label: '异常', type: 'danger' }
  return { label: '正常', type: 'success' }
}

function auditStatusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      auto_pass: { label: '默认通过', type: 'info' },
      pending: { label: '待审核', type: 'warning' },
      pass: { label: '人工通过', type: 'success' },
      rejected: { label: '已打回', type: 'danger' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

// stats 兜底，避免历史数据缺字段
const emptyStats: ReportStats = {
  task_total: 0,
  task_done: 0,
  task_overdue: 0,
  should_points: 0,
  done_points: 0,
  coverage_rate: 0,
  abnormal_count: 0,
  suspect_count: 0,
  wo_created: 0,
  wo_closed: 0,
  wo_unclosed: 0,
  wo_close_rate: 0,
  daily: []
}
const stats = computed<ReportStats>(() => ({ ...emptyStats, ...(detail.value?.stats || {}), daily: detail.value?.stats?.daily || [] }))

async function openDetail(row: ReportItem) {
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await getReport(row.id)
  } finally {
    detailLoading.value = false
  }
}

async function refreshDetail() {
  if (!detail.value) return
  detail.value = await getReport(detail.value.id)
}

// ===== 签字进度 =====
const signedCount = computed(() => detail.value?.inspector_signed?.length ?? detail.value?.inspectors.filter((p) => p.signed).length ?? 0)

const signActive = computed(() => {
  switch (detail.value?.status) {
    case 'pending_supervisor':
      return 1
    case 'pending_manager':
      return 2
    case 'approved':
      return 3
    default:
      return 0
  }
})

const supervisorStepDesc = computed(() => {
  const d = detail.value
  if (!d) return ''
  if (d.supervisor_name) return d.supervisor_name
  if (!d.supervisors?.length) return '已跳过'
  return '待审批'
})
const managerStepDesc = computed(() => {
  const d = detail.value
  if (!d) return ''
  if (d.manager_name) return d.manager_name
  if (!d.managers?.length) return '已跳过'
  return '待终审'
})
const inspectorStepDesc = computed(() => {
  const d = detail.value
  if (!d) return ''
  if (!d.inspector_ids.length) return '已跳过'
  return `已确认 ${signedCount.value}/${d.inspector_ids.length} 人`
})

// ===== 签字操作（按当前环节 + 指定签字人名单显隐；未配置签名时点击弹出签名板现场手写）=====
const signing = ref(false)

// 当前用户是否已配置手写签名（未配置时签字会先弹出签名板）
const hasSignature = computed(() => !!userStore.info?.signature_url)
const inSupervisorList = computed(
  () => !!userStore.info?.id && (detail.value?.supervisor_ids || []).includes(userStore.info.id)
)
const inManagerList = computed(
  () => !!userStore.info?.id && (detail.value?.manager_ids || []).includes(userStore.info.id)
)

const showInspectorSign = computed(() => {
  const d = detail.value
  if (!d || d.status !== 'pending_inspector') return false
  if (!userStore.hasPerm('report:sign:inspector')) return false
  const uid = userStore.info?.id
  if (d.inspector_signed.some((e) => e.user_id === uid)) return false
  return !!uid && d.inspector_ids.includes(uid)
})

// 职责分离：已完成上一级签字（含代签）的用户不再显示本级通过按钮，改提示
const signedLevel1 = computed(() => {
  const uid = userStore.info?.id
  return !!uid && !!detail.value?.inspector_signed.some((e) => e.user_id === uid || e.proxy_by === uid)
})
const isLevel2Signer = computed(
  () => !!userStore.info?.id && detail.value?.supervisor_by === userStore.info.id
)

const showSupervisorSign = computed(
  () =>
    detail.value?.status === 'pending_supervisor' &&
    userStore.hasPerm('report:sign:supervisor') &&
    !signedLevel1.value &&
    inSupervisorList.value
)
const showManagerSign = computed(
  () =>
    detail.value?.status === 'pending_manager' &&
    userStore.hasPerm('report:sign:manager') &&
    !isLevel2Signer.value &&
    inManagerList.value
)

// 签字按钮被隐藏时的原因说明（职责分离 / 不在指定名单）
const separationHint = computed(() => {
  const d = detail.value
  if (!d) return ''
  if (d.status === 'pending_supervisor' && userStore.hasPerm('report:sign:supervisor')) {
    if (signedLevel1.value) return '你已完成巡检员确认，主管审批须由其他人员操作'
    if (!inSupervisorList.value) return '你不在本报告主管签字人名单内'
  }
  if (d.status === 'pending_manager' && userStore.hasPerm('report:sign:manager')) {
    if (isLevel2Signer.value) return '你已完成主管审批，终审须由其他人员操作'
    if (!inManagerList.value) return '你不在本报告经理签字人名单内'
  }
  return ''
})

// 驳回不受职责分离限制（负向动作），但须在本级指定签字人名单内（与后端同口径）
const showRejectBtn = computed(() => {
  const d = detail.value
  if (!d) return false
  if (d.status === 'pending_supervisor') return userStore.hasPerm('report:sign:supervisor') && inSupervisorList.value
  if (d.status === 'pending_manager') return userStore.hasPerm('report:sign:manager') && inManagerList.value
  return false
})

// 代签入口：有待确认巡检员且持有 report:sign:proxy 权限（代签用代签人本人签名）
const showProxySign = computed(() => {
  const d = detail.value
  if (!d || d.status !== 'pending_inspector') return false
  if (!userStore.hasPerm('report:sign:proxy')) return false
  return d.inspectors.some((p) => !p.signed)
})

const proxyVisible = ref(false)
const proxyFormRef = ref<FormInstance>()
const proxyForm = reactive({ user_id: '', reason: '' })
const proxyRules: FormRules = {
  user_id: [{ required: true, message: '请选择被代签人', trigger: 'change' }],
  reason: [{ required: true, message: '请填写代签原因', trigger: 'blur' }]
}
const unsignedInspectors = computed(() => detail.value?.inspectors.filter((p) => !p.signed) ?? [])

function openProxySign() {
  proxyForm.user_id = ''
  proxyForm.reason = ''
  proxyFormRef.value?.clearValidate()
  proxyVisible.value = true
}

// ===== 签字时的手写签名补齐：未配置签名则弹出签名板现场手写，可选保存供下次使用 =====
const padRef = ref<InstanceType<typeof SignaturePad>>()
let pendingSign: ((sigKey: string) => Promise<void>) | null = null

// 已配置签名直接执行（sigKey 空串，后端取当前签名资产快照）；否则先弹签名板，签完继续执行
function withSignature(action: (sigKey: string) => Promise<void>) {
  if (hasSignature.value) {
    void action('')
    return
  }
  pendingSign = action
  padRef.value?.open()
}

// 签名板保存：上传 PNG；勾选保存则写入签章资产（下次签字直接用），否则仅本次签字使用
async function handlePadSave(file: File, saveForLater: boolean) {
  try {
    const { file_key } = await uploadImage(file, 'signature')
    let sigKey = file_key
    if (saveForLater) {
      await updateProfile({
        name: userStore.info?.name || '',
        phone: userStore.info?.phone || '',
        signature_file_key: file_key
      })
      await userStore.fetchInfo()
      sigKey = ''
      ElMessage.success('签名已保存，下次签字将直接使用')
    }
    const action = pendingSign
    pendingSign = null
    if (action) await action(sigKey)
    return true
  } catch {
    // 拦截器已提示；返回 false 保持签名板打开，可重试或取消
    return false
  }
}

async function submitProxySign() {
  await proxyFormRef.value?.validate()
  if (!detail.value) return
  const id = detail.value.id
  const { user_id, reason } = proxyForm
  withSignature(async (sigKey) => {
    signing.value = true
    try {
      const res = await signInspector(id, {
        proxy_for: user_id,
        reason,
        signature_file_key: sigKey || undefined
      })
      proxyVisible.value = false
      await afterSign(res.status === 'pending_supervisor' ? '全员已确认，已流转主管审批' : '代签已记录')
    } catch {
      // 错误提示由请求拦截器统一弹出
    } finally {
      signing.value = false
    }
  })
}

async function afterSign(message: string) {
  ElMessage.success(message)
  await Promise.all([refreshDetail(), fetchList()])
}

async function handleInspectorSign() {
  if (!detail.value) return
  try {
    await ElMessageBox.confirm('确认已完成本期全部巡检工作，进行电子确认？', '电子确认', { type: 'warning' })
  } catch {
    return
  }
  const id = detail.value.id
  withSignature(async (sigKey) => {
    signing.value = true
    try {
      const res = await signInspector(id, sigKey ? { signature_file_key: sigKey } : undefined)
      await afterSign(res.status === 'pending_supervisor' ? '全员已确认，已流转主管审批' : '已确认签字')
    } catch {
      // 错误提示由请求拦截器统一弹出（如「不在应签名单」「已确认过」）
    } finally {
      signing.value = false
    }
  })
}

async function handleApprove() {
  if (!detail.value) return
  const id = detail.value.id
  const isManager = detail.value.status === 'pending_manager'
  let remark = ''
  try {
    const res = await ElMessageBox.prompt('审批意见（选填）', isManager ? '终审通过' : '审批通过', {
      confirmButtonText: '通过',
      cancelButtonText: '取消',
      inputPlaceholder: '如：情况属实，同意',
      inputValidator: () => true
    })
    remark = (res.value || '').trim()
  } catch {
    return
  }
  withSignature(async (sigKey) => {
    signing.value = true
    try {
      const body = { action: 'approve' as const, remark, signature_file_key: sigKey || undefined }
      if (isManager) {
        await signManager(id, body)
        await afterSign('终审通过，报告已归档')
      } else {
        await signSupervisor(id, body)
        await afterSign('审批通过，已流转经理终审')
      }
    } catch {
      // 拦截器已提示
    } finally {
      signing.value = false
    }
  })
}

async function handleReject() {
  if (!detail.value) return
  const isManager = detail.value.status === 'pending_manager'
  let reason = ''
  try {
    const res = await ElMessageBox.prompt('请输入驳回原因（驳回后退回巡检员确认环节）', '驳回报告', {
      confirmButtonText: '驳回',
      cancelButtonText: '取消',
      inputPlaceholder: '如：覆盖率不达标，请核实后重新确认',
      inputValidator: (v: string) => (v && v.trim() ? true : '驳回原因不能为空')
    })
    reason = res.value.trim()
  } catch {
    return
  }
  signing.value = true
  try {
    const body = { action: 'reject' as const, reason }
    if (isManager) {
      await signManager(detail.value.id, body)
    } else {
      await signSupervisor(detail.value.id, body)
    }
    await afterSign('已驳回，退回巡检员确认环节')
  } catch {
    // 拦截器已提示
  } finally {
    signing.value = false
  }
}

// ===== PDF 下载 =====
const downloading = ref(false)

async function handleDownload(row: { id: string; title: string }) {
  downloading.value = true
  try {
    await downloadFile(`/reports/${row.id}/pdf`, undefined, `${row.title}.pdf`)
  } catch {
    // 拦截器已提示
  } finally {
    downloading.value = false
  }
}

// ===== 生成报告 =====
const generateVisible = ref(false)
const generating = ref(false)
const generateFormRef = ref<FormInstance>()
const generateForm = reactive({
  community_id: undefined as string | undefined,
  period: undefined as string | undefined,
  supervisor_ids: [] as string[],
  manager_ids: [] as string[]
})
const generateRules: FormRules = {
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  period: [{ required: true, message: '请选择月份', trigger: 'change' }]
}

// 签字候选人（选定小区后加载，默认全选；清空某级 = 该级跳过）
const candidatesLoading = ref(false)
const candidates = reactive<{ supervisors: SignCandidate[]; managers: SignCandidate[] }>({
  supervisors: [],
  managers: []
})

async function loadCandidates() {
  candidates.supervisors = []
  candidates.managers = []
  generateForm.supervisor_ids = []
  generateForm.manager_ids = []
  if (!generateForm.community_id) return
  candidatesLoading.value = true
  try {
    const data = await getSignCandidates(generateForm.community_id)
    candidates.supervisors = data.supervisors
    candidates.managers = data.managers
    generateForm.supervisor_ids = data.supervisors.map((p) => p.id)
    generateForm.manager_ids = data.managers.map((p) => p.id)
  } finally {
    candidatesLoading.value = false
  }
}

function openGenerate() {
  generateForm.community_id = query.community_id
  generateForm.period = query.period
  generateVisible.value = true
  loadCandidates()
}

async function submitGenerate() {
  const valid = await generateFormRef.value?.validate().catch(() => false)
  if (!valid) return
  generating.value = true
  try {
    const data = await generateReport({
      community_id: generateForm.community_id!,
      period: generateForm.period!,
      supervisor_ids: generateForm.supervisor_ids,
      manager_ids: generateForm.manager_ids
    })
    ElMessage.success(data.regenerated ? `「${data.title}」已重新统计并重置签字流程` : `「${data.title}」已生成`)
    generateVisible.value = false
    handleSearch()
  } catch {
    // 拦截器已提示（如「已通过报告不可重算」「签字人须在候选池内」）
  } finally {
    generating.value = false
  }
}
</script>

<style lang="scss" scoped>
.detail-body {
  padding-bottom: $spacing-md;
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: $spacing-md;
  margin-bottom: $spacing-md;

  .detail-point {
    display: flex;
    flex-direction: column;
    gap: 4px;

    .point-name {
      font-size: 16px;
      font-weight: 600;
    }
  }
}

.reject-alert {
  margin-bottom: $spacing-md;
}

// 区块标题：上分隔线对齐工单/打卡详情的区块语言（区块间视觉分隔）
.section-title {
  margin: 0 0 $spacing-md;
  padding-top: $spacing-lg;
  border-top: 1px solid $color-border;
  font-size: $font-size-body;
  font-weight: 600;
  color: $color-text-primary;
}

.record-photo {
  width: 40px;
  height: 40px;
  border-radius: 4px;
  margin-right: 4px;
  vertical-align: middle;
  cursor: pointer;
}

.sign-steps {
  margin-bottom: $spacing-md;
}

.sign-block {
  padding: $spacing-sm $spacing-md;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  margin-bottom: $spacing-sm;

  .sign-block-title {
    font-weight: 600;
    margin-bottom: $spacing-sm;
  }

  .inspector-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .inspector-item {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .inspector-name {
      display: flex;
      align-items: center;
      gap: $spacing-sm;
    }
  }

  .sign-line {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
  }

  // 手写签名图（小尺寸，点击放大预览）
  .sign-img {
    width: 72px;
    height: 28px;
    background: $color-white;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 4px;
    cursor: pointer;
    vertical-align: middle;
  }

  .sign-remark {
    margin-top: 6px;
    color: $color-text-secondary;
  }
}

.sign-actions {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  margin: $spacing-md 0;

  .separation-hint {
    font-size: $font-size-aux;
  }
}

.drawer-footer {
  margin-top: $spacing-md;
  display: flex;
  justify-content: flex-end;
}

.generate-tip {
  margin-bottom: $spacing-md;
}

.signer-tip {
  margin: -8px 0 $spacing-md 98px;
  font-size: $font-size-aux;
  line-height: 1.6;
}

// 候选人选项内的「未配置签名」警示
.candidate-warn {
  float: right;
  color: var(--el-color-warning);
  font-size: $font-size-aux;
}
</style>
