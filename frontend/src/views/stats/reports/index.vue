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
        <el-form-item label="报告类型">
          <el-select v-model="query.patrol_type" placeholder="全部" clearable style="width: 160px">
            <el-option label="综合月报" value="none" />
            <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
              <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
            </el-option-group>
          </el-select>
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
        <el-table-column label="报告类型" width="130" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.patrol_type ? 'warning' : 'info'" effect="plain">
              {{ row.patrol_type_label || '综合月报' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="130" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status).type" size="small">{{ statusTag(row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="审核链" min-width="180" align="center">
          <template #default="{ row }">{{ row.review_steps?.map((x: any) => x.name).join(' → ') || '无需审核' }}</template>
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
            <el-descriptions-item label="报告类型">{{ detail.patrol_type_label || '综合月报' }}</el-descriptions-item>
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
            <el-descriptions-item label="异常问题数">
              <span :class="{ 'danger-text': stats.issue_count > 0 }">{{ stats.issue_count }}</span>
            </el-descriptions-item>
          </el-descriptions>

          <!-- 设备台账（v1.7：状态快照/当期维保/抽查/判定来源；无设备小区不显示） -->
          <template v-if="equipmentStats">
            <div class="section-title">设备台账</div>
            <el-descriptions :column="3" border size="small">
              <el-descriptions-item label="正常">{{ equipmentStats.status_buckets.normal }}</el-descriptions-item>
              <el-descriptions-item label="临期">
                <span :class="{ 'danger-text': equipmentStats.status_buckets.warning > 0 }">{{ equipmentStats.status_buckets.warning }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="已逾期">
                <span :class="{ 'danger-text': equipmentStats.status_buckets.overdue > 0 }">{{ equipmentStats.status_buckets.overdue }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="应报废">
                <span :class="{ 'danger-text': equipmentStats.status_buckets.scrap > 0 }">{{ equipmentStats.status_buckets.scrap }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="标签缺失">
                <span :class="{ 'danger-text': equipmentStats.status_buckets.label_missing > 0 }">{{ equipmentStats.status_buckets.label_missing }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="维保登记/确认">
                {{ equipmentStats.maintenance.registered }} / {{ equipmentStats.maintenance.confirmed }}
              </el-descriptions-item>
              <el-descriptions-item label="抽查触发/不符">
                {{ equipmentStats.spotcheck.triggered }} /
                <span :class="{ 'danger-text': equipmentStats.spotcheck.mismatch > 0 }">{{ equipmentStats.spotcheck.mismatch }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="判定来源（系统/人工·AI）" :span="2">
                {{ equipmentStats.judge_source.system }} / {{ equipmentStats.judge_source.manual_ai }}
                <span class="text-secondary">（台账有效期与日期标签抽查为系统判定）</span>
              </el-descriptions-item>
            </el-descriptions>
          </template>

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

          <!-- 巡检记录明细（后端分页，滚动到底加载下一页，避免几千行一次返回/渲染卡死） -->
          <template v-if="recordRows.length">
            <div class="section-title">巡检记录明细（{{ recordsTotal }} 条）</div>
            <div class="records-scroll" @scroll="onRecordsScroll">
              <el-table :data="recordRows" border size="small" style="width: 100%">
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
              <div v-if="!recordsAllLoaded" class="records-more">
                {{ recordsLoading ? '加载中…' : `下拉加载更多（${recordRows.length}/${recordsTotal}）` }}
              </div>
              <div v-else class="records-more text-secondary">全部 {{ recordsTotal }} 条已加载</div>
            </div>
          </template>

          <!-- 动态审核进度 -->
          <div class="section-title">审核进度</div>
          <el-steps :active="signActive" align-center finish-status="success" class="sign-steps">
            <el-step v-for="step in detail.review_steps" :key="step.slot" :title="step.name" :description="stepDescription(step)" />
          </el-steps>

          <div v-for="step in detail.review_steps" :key="`sign-${step.slot}`" class="sign-block">
            <div class="sign-block-title">{{ step.name }}（{{ step.mode === 'all' ? '全部签署' : '任一签署' }}）</div>
            <div v-if="step.users?.length" class="inspector-list">
              <div v-for="p in step.users" :key="p.user_id" class="inspector-item"><span>{{ p.name }}</span><el-tag :type="p.signed ? 'success' : 'info'" size="small">{{ p.signed ? `已签 ${p.signed_at || ''}` : '待签' }}</el-tag></div>
            </div>
            <div v-else class="text-secondary">该步骤无候选人，已自动跳过</div>
          </div>

          <!-- 签字操作区 -->
          <div v-if="showSignAction || separationHint" class="sign-actions">
            <el-button v-if="showSignAction" type="primary" :icon="CircleCheck" :loading="signing" @click="handleApprove">{{ currentStep?.name || '签字' }}</el-button>
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
          <el-select
            v-model="generateForm.community_id"
            placeholder="请选择小区"
            :loading="communitiesLoading"
            no-data-text="当前租户暂无可用小区"
            style="width: 100%"
            @change="loadCandidates"
          >
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
        <el-form-item label="报告类型">
          <!-- 空值 = 综合月报；EP 将 '' 视为未选，placeholder 直接兜住显示 -->
          <el-select v-model="generateForm.patrol_type" placeholder="综合月报（全部巡查类型）" style="width: 100%" @change="loadCandidates">
            <el-option label="综合月报（全部巡查类型）" value="" />
            <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
              <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
            </el-option-group>
          </el-select>
          <el-alert
            v-if="generateForm.patrol_type"
            type="warning"
            :closable="false"
            title="专项检查报告只统计该类型任务，建议该类型当月任务全部完成后再生成"
            class="patrol-type-tip"
          />
        </el-form-item>
        <el-form-item label="明细范围">
          <el-radio-group v-model="generateForm.detail_mode">
            <el-radio-button value="full">全部点位</el-radio-button>
            <el-radio-button value="abnormal">仅异常点位</el-radio-button>
          </el-radio-group>
          <div class="form-tip">点位量大时选「仅异常点位」可大幅压缩报告页数（汇总统计不受影响）</div>
        </el-form-item>
        <el-form-item v-for="step in candidateSteps" :key="step.slot" :label="step.name">
          <el-select v-model="selectedCandidates[step.slot]" multiple collapse-tags collapse-tags-tooltip :placeholder="step.mode === 'all' ? '选择全部签字人' : '选择任一签字人'" :loading="candidatesLoading" style="width: 100%">
            <el-option v-for="p in step.users" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <div class="signer-tip text-secondary">
          审核步骤和候选人由后端审核链配置返回，可按企业需要配置任意级数；清空步骤候选人表示自动跳过。
        </div>
      </el-form>
      <template #footer>
        <el-button @click="generateVisible = false">取消</el-button>
        <el-button type="primary" :loading="generating" @click="submitGenerate">生成</el-button>
      </template>
    </el-dialog>

    <!-- 签字时未配置手写签名：弹出签名板现场手写（可选保存下次使用） -->
    <SignaturePad ref="padRef" show-save-option @save="handlePadSave" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Search, Refresh, Plus, Download, CircleCheck } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  listReports,
  getReport,
  getReportRecords,
  generateReport,
  getSignCandidates,
  signStep,
  type ReportItem,
  type ReportDetail,
  type ReportRecord,
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
import { usePatrolTypes } from '@/composables/usePatrolTypes'
import type { CommunityItem } from '@/api/biz-types'

const userStore = useUserStore()
// 巡查类型字典（报告类型筛选/生成下拉，按大类分组）
const { patrolTypeGroups } = usePatrolTypes()
const loading = ref(false)
const list = ref<ReportItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const communitiesLoading = ref(false)

const statusOptions: { label: string; value: ReportStatus }[] = [
  { label: '待审核', value: 'pending_review' }, { label: '已通过', value: 'approved' }
]

// 状态标签：pending_* 流程中-橙 / approved 已通过-绿
function statusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pending_review: { label: '待审核', type: 'warning' },
      approved: { label: '已通过', type: 'success' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

const query = reactive({
  page: 1,
  page_size: 20,
  community_id: undefined as string | undefined,
  period: undefined as string | undefined,
  patrol_type: undefined as string | undefined,
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
  query.patrol_type = undefined
  query.status = undefined
  handleSearch()
}

async function loadCommunities() {
  communitiesLoading.value = true
  try {
    const data = await listCommunities({ page: 1, page_size: 100, status: 1 })
    communities.value = data.list
  } finally {
    communitiesLoading.value = false
  }
}

onMounted(async () => {
  fetchList()
  await loadCommunities()
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
  issue_count: 0,
  daily: []
}
const stats = computed<ReportStats>(() => ({ ...emptyStats, ...(detail.value?.stats || {}), daily: detail.value?.stats?.daily || [] }))

// 设备台账章节（v1.7；stats.equipment 缺省=旧报告/无设备，不显示区块）
interface EquipmentStats {
  status_buckets: { normal: number; warning: number; overdue: number; scrap: number; label_missing: number }
  maintenance: { registered: number; confirmed: number; rejected: number }
  spotcheck: { triggered: number; mismatch: number }
  judge_source: { system: number; manual_ai: number }
}
const equipmentStats = computed<EquipmentStats | null>(() => {
  const eq = detail.value?.stats?.equipment as EquipmentStats | undefined
  if (!eq || !eq.status_buckets) return null
  return eq
})

// ===== 记录明细分页加载（后端分页，滚动到底拉下一页，避免几千行一次返回/渲染卡死） =====
const RECORDS_PAGE_SIZE = 100
const recordRows = ref<ReportRecord[]>([])
const recordsTotal = ref(0)
const recordsPage = ref(0)
const recordsLoading = ref(false)

const recordsAllLoaded = computed(() => recordsPage.value > 0 && recordRows.value.length >= recordsTotal.value)

async function loadRecordsPage(id: string, page: number) {
  recordsLoading.value = true
  try {
    const data = await getReportRecords(id, { page, page_size: RECORDS_PAGE_SIZE })
    recordsTotal.value = data.total
    recordRows.value = page === 1 ? data.list : recordRows.value.concat(data.list)
    recordsPage.value = page
  } finally {
    recordsLoading.value = false
  }
}

function onRecordsScroll(e: Event) {
  if (recordsAllLoaded.value || recordsLoading.value || !detail.value) return
  const el = e.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) {
    loadRecordsPage(detail.value.id, recordsPage.value + 1)
  }
}

async function openDetail(row: ReportItem) {
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  recordRows.value = []
  recordsTotal.value = 0
  recordsPage.value = 0
  try {
    const [d] = await Promise.all([getReport(row.id), loadRecordsPage(row.id, 1)])
    detail.value = d
  } finally {
    detailLoading.value = false
  }
}

async function refreshDetail() {
  if (!detail.value) return
  detail.value = await getReport(detail.value.id)
}

// ===== 动态审核签字 =====
const signing = ref(false)
const hasSignature = computed(() => !!userStore.info?.signature_url)
const currentStep = computed(() => detail.value?.review_steps?.[detail.value.review_step])
const signActive = computed(() => detail.value?.status === 'approved' ? (detail.value?.review_steps?.length || 0) : (detail.value?.review_step || 0))
const stepDescription = (step: any) => step.candidate_ids?.length ? `${step.signed?.length || 0}/${step.candidate_ids.length} 已签` : '已跳过'
const showSignAction = computed(() => {
  const d = detail.value, uid = userStore.info?.id, step = currentStep.value
  if (!d || d.status !== 'pending_review' || !uid || !step || !step.candidate_ids.includes(uid)) return false
  return !step.signed?.some((x: any) => x.user_id === uid)
})
const separationHint = computed(() => showSignAction.value ? '' : (detail.value?.status === 'pending_review' ? '当前审核步骤不包含你' : ''))
const showRejectBtn = computed(() => showSignAction.value)
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
    const { file_id } = await uploadImage(file, 'signature')
    let sigKey = file_id
    if (saveForLater) {
      await updateProfile({
        name: userStore.info?.name || '',
        phone: userStore.info?.phone || '',
        signature_file_id: file_id
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

async function afterSign(message: string) {
  ElMessage.success(message)
  await Promise.all([refreshDetail(), fetchList()])
}

async function handleApprove() {
  if (!detail.value || !currentStep.value) return
  const id = detail.value.id, step = detail.value.review_step
  let remark = ''
  try {
    const res = await ElMessageBox.prompt('审批意见（选填）', '审核通过', {
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
      const body = { action: 'approve' as const, remark, signature_file_id: sigKey || undefined }
      await signStep(id, step, body)
      await afterSign('审核已提交')
    } catch {
      // 拦截器已提示
    } finally {
      signing.value = false
    }
  })
}

async function handleReject() {
  if (!detail.value) return
  let reason = ''
  try {
    const res = await ElMessageBox.prompt('请输入驳回原因（驳回后回到审核链首个有效环节）', '驳回报告', {
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
    await signStep(detail.value.id, detail.value.review_step, { action: 'reject', reason })
    await afterSign('已驳回，审核链已重置')
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
  patrol_type: '',
  detail_mode: 'full'
})
const generateRules: FormRules = {
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  period: [{ required: true, message: '请选择月份', trigger: 'change' }]
}

// 签字候选人（选定小区后加载；候选人和默认值由后端动态审核链解析）
const candidatesLoading = ref(false)
const candidateSteps = ref<{ slot: string; name: string; mode: 'any' | 'all'; users: SignCandidate[]; default_candidate_ids: string[] }[]>([])
const selectedCandidates = reactive<Record<string, string[]>>({})

async function loadCandidates() {
  candidateSteps.value = []
  Object.keys(selectedCandidates).forEach((key) => delete selectedCandidates[key])
  if (!generateForm.community_id) return
  candidatesLoading.value = true
  try {
    // 专项报告的主管级默认名单取该类型汇报线槽位（如消防→工程主管）
    const data = await getSignCandidates(generateForm.community_id, generateForm.patrol_type || undefined)
    candidateSteps.value = data.steps
    data.steps.forEach((step) => { selectedCandidates[step.slot] = [...step.default_candidate_ids] })
  } finally {
    candidatesLoading.value = false
  }
}

function openGenerate() {
  generateForm.community_id = query.community_id
  generateForm.period = query.period
  // 列表筛选的综合月报（none）对应生成表单的空值
  generateForm.patrol_type = query.patrol_type && query.patrol_type !== 'none' ? query.patrol_type : ''
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
      patrol_type: generateForm.patrol_type || undefined,
      detail_mode: generateForm.detail_mode,
      sign_steps: candidateSteps.value.filter((step) => step.users.length > 0).map((step) => ({ slot: step.slot, candidate_ids: selectedCandidates[step.slot] || [] }))
    })
    ElMessage.success(data.regenerated ? `「${data.title}」已重新统计并重置签字流程` : `「${data.title}」已生成`)
    generateVisible.value = false
    handleSearch()
  } catch {
    // 拦截器已提示（如「已通过报告不可重算」「签字人须为存在且启用的用户」）
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

// 区块标题：上分隔线对齐打卡详情的区块语言（区块间视觉分隔）
.section-title {
  margin: 0 0 $spacing-md;
  padding-top: $spacing-lg;
  border-top: 1px solid $color-border;
  font-size: $font-size-body;
  font-weight: 600;
  color: $color-text-primary;
}

.records-scroll {
  max-height: 340px;
  overflow-y: auto;
}

.records-more {
  text-align: center;
  font-size: 12px;
  color: #86909c;
  padding: 8px 0;
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

.patrol-type-tip {
  margin-top: $spacing-xs;
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
