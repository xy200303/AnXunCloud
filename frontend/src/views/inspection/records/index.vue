<template>
  <div class="app-container">
    <!-- 顶部 Tab：按审核状态分（徽章数量跟随筛选条件联动） -->
    <div class="table-card records-tabs-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="全部记录" name="all" />
        <el-tab-pane v-perms="'inspection:checkin:review'" name="pending">
          <template #label>
            待审核
            <el-badge v-if="counts.pending > 0" :value="counts.pending" :max="99" type="warning" class="tab-badge" />
          </template>
        </el-tab-pane>
        <el-tab-pane v-perms="'inspection:checkin:review'" name="reviewed">
          <template #label>
            已审核
            <el-badge v-if="counts.pass + counts.rejected > 0" :value="counts.pass + counts.rejected" :max="99" type="info" class="tab-badge" />
          </template>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 搜索区：时间筛选默认近 7 天 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 140px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡检员">
          <el-select v-model="query.inspector_id" placeholder="全部" clearable filterable style="width: 120px">
            <el-option v-for="u in inspectors" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="结果">
          <el-select v-model="query.result" placeholder="全部" clearable style="width: 110px">
            <el-option label="正常" value="normal" />
            <el-option label="异常" value="abnormal" />
          </el-select>
        </el-form-item>
        <el-form-item label="异常类型">
          <el-select v-model="query.exception_type" placeholder="全部" clearable style="width: 130px">
            <el-option label="设备缺失" value="device_missing" />
            <el-option label="无法拍摄" value="unable_to_capture" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="activeTab === 'reviewed'" label="审核结果">
          <el-select v-model="reviewedStatus" style="width: 120px">
            <el-option label="全部" value="all" />
            <el-option label="人工通过" value="pass" />
            <el-option label="已打回" value="rejected" />
          </el-select>
        </el-form-item>
        <el-form-item label="疑似作弊">
          <el-checkbox v-model="onlySuspect" label="仅看疑似" />
        </el-form-item>
        <el-form-item label="强制提交">
          <el-checkbox v-model="onlyForceSubmit" label="仅看强制提交" />
        </el-form-item>
        <el-form-item label="AI 存疑">
          <el-checkbox v-model="onlyAiSuspect" label="仅看存疑/失败" />
        </el-form-item>
        <el-form-item label="打卡时间">
          <el-date-picker
            v-model="timeRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 340px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
          <el-button type="success" plain :icon="Document" :loading="exporting" @click="handleExport">导出</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 表格（非待审核 tab 点击行查看详情） -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <template v-if="activeTab === 'pending'">
            <el-button
              v-perms="'inspection:checkin:review'"
              type="success"
              :disabled="selectedRows.length === 0"
              :loading="batchPassing"
              @click="handleBatchPass"
            >
              批量通过{{ selectedRows.length ? `（${selectedRows.length}）` : '' }}
            </el-button>
            <span v-if="selectedRows.length" class="toolbar-tip">已选 {{ selectedRows.length }} 条</span>
          </template>
        </div>
        <el-button v-perms="'inspection:checkin:spotcheck'" type="primary" :icon="Aim" @click="openSpotcheck">
          发起抽查
        </el-button>
      </div>

      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="list"
        stripe
        style="width: 100%"
        :row-class-name="activeTab === 'pending' ? '' : 'clickable-row'"
        @row-click="handleRowClick"
        @selection-change="(rows: CheckinItem[]) => (selectedRows = rows)"
      >
        <el-table-column v-if="activeTab === 'pending'" type="selection" width="44" />
        <el-table-column prop="checkin_time" label="打卡时间" width="160" />
        <el-table-column prop="inspector_name" label="巡检员" width="100" />
        <el-table-column prop="community_name" label="小区" min-width="120" />
        <el-table-column prop="point_name" label="点位" min-width="120" show-overflow-tooltip />
        <el-table-column label="方式" width="100" align="center">
          <template #default="{ row }">{{ checkinTypeLabel(row.checkin_type) }}</template>
        </el-table-column>
        <el-table-column prop="distance_to_point" label="距点位" width="90" align="right">
          <template #default="{ row }">{{ row.distance_to_point }}m</template>
        </el-table-column>
        <el-table-column label="结果" width="110" align="center">
          <template #default="{ row }">
            <div class="result-tags">
              <el-tag v-if="row.is_suspect" type="warning" size="small">疑似作弊</el-tag>
              <el-tag v-else-if="row.result === 'abnormal'" type="danger" size="small">异常</el-tag>
              <el-tag v-else type="success" size="small">正常</el-tag>
              <el-tag v-if="row.force_submit" type="warning" size="small" effect="dark">强制提交</el-tag>
              <el-tag v-for="et in exceptionTypeTags(row.exception_types)" :key="et.value" type="danger" size="small" effect="plain">{{ et.label }}</el-tag>
              <el-tag v-if="row.ai_verdict === 'review' || row.ai_verdict === 'error'" type="warning" size="small" effect="plain">AI 存疑</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="AI 结论" width="110" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.ai_verdict" :type="aiVerdictTag(row.ai_verdict).type" size="small">
              {{ aiVerdictTag(row.ai_verdict).label }}
            </el-tag>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column label="审核" width="130" align="center">
          <template #default="{ row }">
            <el-tag :type="auditStatusTag(row.audit_status).type" size="small">
              {{ row.audit_status === 'pending' && row.current_step_name ? `待${row.current_step_name}` : auditStatusTag(row.audit_status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="photo_count" label="照片数" width="80" align="right" />
        <el-table-column label="操作" :width="activeTab === 'all' ? 90 : 170">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
            <template v-if="activeTab === 'pending'">
              <el-button v-perms="'inspection:checkin:review'" link type="success" @click="handlePass(row)">通过</el-button>
              <el-button v-perms="'inspection:checkin:review'" link type="danger" @click="handleReject(row)">打回</el-button>
            </template>
            <el-button
              v-if="activeTab === 'reviewed'"
              v-perms="'inspection:checkin:review'"
              link
              type="warning"
              @click="handleReopen(row)"
            >
              撤销审核
            </el-button>
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
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 打卡详情抽屉（公共组件，含检查项结果与审核信息区块） -->
    <checkin-detail-drawer
      v-model="detailVisible"
      :row="currentRow"
      :detail="detail"
      :loading="detailLoading"
    />

    <!-- 发起抽查对话框 -->
    <el-dialog v-model="spotcheckVisible" title="发起抽查" width="480px" :close-on-click-modal="false">
      <el-form ref="spotcheckFormRef" :model="spotcheckForm" :rules="spotcheckRules" label-width="90px">
        <el-form-item label="小区">
          <el-select v-model="spotcheckForm.community_id" placeholder="全部小区" clearable style="width: 100%">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡检员">
          <el-select v-model="spotcheckForm.inspector_id" placeholder="全部" clearable filterable style="width: 100%">
            <el-option v-for="u in inspectors" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围" prop="range">
          <el-date-picker
            v-model="spotcheckForm.range"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="抽查方式" prop="mode">
          <el-radio-group v-model="spotcheckForm.mode">
            <el-radio value="random">随机比例</el-radio>
            <el-radio value="full">全量</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="spotcheckForm.mode === 'random'" label="抽查比例">
          <el-input-number v-model="spotcheckForm.ratio" :min="1" :max="100" style="width: 160px" />
          <span class="form-tip">%</span>
        </el-form-item>
        <el-form-item label="处理方式" prop="handler">
          <el-radio-group v-model="spotcheckForm.handler">
            <el-radio value="manual">转人工审核</el-radio>
            <el-radio value="ai">交大模型分析</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="spotcheckVisible = false">取消</el-button>
        <el-button type="primary" :loading="spotcheckLoading" @click="submitSpotcheck">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Search, Refresh, Aim, Document } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type TableInstance } from 'element-plus'
import { listCheckins, getCheckin, getCheckinAuditCounts, type CheckinQuery, type AuditCounts } from '@/api/checkin'
import { downloadFile } from '@/utils/download'
import { passReview, rejectReview, reopenReview, batchPassReview, spotcheck, type SpotcheckBody } from '@/api/review'
import { listCommunities } from '@/api/community'
import { listUsers } from '@/api/user'
import CheckinDetailDrawer from '@/components/CheckinDetailDrawer.vue'
import type { CheckinItem, CheckinDetail, CommunityItem } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const route = useRoute()
const loading = ref(false)
const list = ref<CheckinItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectors = ref<UserItem[]>([])
const onlySuspect = ref(false)
// 强制提交 / AI 存疑筛选：后端列表接口暂未支持对应过滤参数，参数透传备用 + 前端对当页结果兜底过滤
const onlyForceSubmit = ref(false)
const onlyAiSuspect = ref(false)
const activeTab = ref<'all' | 'pending' | 'reviewed'>('all')
const reviewedStatus = ref<'all' | 'pass' | 'rejected'>('all')
const tableRef = ref<TableInstance>()
const selectedRows = ref<CheckinItem[]>([])

function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传', nfc: 'NFC' }[t] || t
}

// 审核状态标签（与详情抽屉同一映射）
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

// 异常类型标签（exception_types 逗号分隔 → 标签数组）
function exceptionTypeTags(v?: string): Array<{ value: string; label: string }> {
  if (!v) return []
  const labels: Record<string, string> = { device_missing: '设备缺失', unable_to_capture: '无法拍摄' }
  return v.split(',').filter(Boolean).map((x) => ({ value: x, label: labels[x] || x }))
}

// AI 结论标签
function aiVerdictTag(v: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pass: { label: '大模型通过', type: 'success' },
      review: { label: '转人工', type: 'warning' },
      abnormal: { label: 'AI 判异常', type: 'danger' },
      error: { label: '审核失败', type: 'info' }
    }[v] || { label: v, type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

const emptyText = computed(() => {
  if (activeTab.value === 'pending') return '该条件下暂无待审核记录'
  if (activeTab.value === 'reviewed') return '该条件下暂无已审核记录'
  return '该条件下暂无打卡记录，试试扩大时间范围'
})

// 时间筛选默认近 7 天
function defaultRange(): [string, string] {
  const end = new Date()
  const start = new Date(Date.now() - 6 * 86400000)
  const fmt = (d: Date, endOfDay: boolean) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${endOfDay ? '23:59:59' : '00:00:00'}`
  return [fmt(start, false), fmt(end, true)]
}

const timeRange = ref<[string, string] | null>(defaultRange())
const query = reactive<CheckinQuery>({ page: 1, page_size: 20, community_id: undefined, inspector_id: undefined, result: '', exception_type: '' })

// tab 徽章计数（与列表过滤条件联动，不含审核状态本身）
const counts = reactive<AuditCounts>({ auto_pass: 0, pending: 0, pass: 0, rejected: 0 })

// 当前 tab 对应的审核状态过滤（已审核-全部 = pass,rejected 多值）
function currentAuditStatus(): string | undefined {
  if (activeTab.value === 'pending') return 'pending'
  if (activeTab.value === 'reviewed') return reviewedStatus.value === 'all' ? 'pass,rejected' : reviewedStatus.value
  return undefined
}

// 列表与计数共用的过滤参数
function filterParams() {
  return {
    community_id: query.community_id,
    inspector_id: query.inspector_id,
    result: query.result || undefined,
    exception_type: query.exception_type || undefined,
    is_suspect: onlySuspect.value || undefined,
    force_submit: onlyForceSubmit.value || undefined,
    ai_verdict: onlyAiSuspect.value ? 'review,error' : undefined,
    start_time: timeRange.value?.[0],
    end_time: timeRange.value?.[1]
  }
}

async function fetchCounts() {
  const data = await getCheckinAuditCounts(filterParams())
  Object.assign(counts, data)
}

async function fetchList() {
  loading.value = true
  try {
    const data = await listCheckins({
      ...query,
      ...filterParams(),
      audit_status: currentAuditStatus()
    })
    // 兜底前端过滤（后端支持 force_submit/ai_verdict 过滤后此处自然为无操作）；分页总数以后端为准
    let rows = data.list
    if (onlyForceSubmit.value) rows = rows.filter((r) => r.force_submit)
    if (onlyAiSuspect.value) rows = rows.filter((r) => r.ai_verdict === 'review' || r.ai_verdict === 'error')
    list.value = rows
    total.value = data.total
  } finally {
    loading.value = false
  }
  fetchCounts()
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.community_id = undefined
  query.inspector_id = undefined
  query.result = ''
  query.exception_type = ''
  reviewedStatus.value = 'all'
  onlySuspect.value = false
  onlyForceSubmit.value = false
  onlyAiSuspect.value = false
  timeRange.value = defaultRange()
  handleSearch()
}

// ===== 导出（按当前筛选条件含审核 tab，文件流下载；上限 5000 条） =====
const exporting = ref(false)

async function handleExport() {
  try {
    await ElMessageBox.confirm('将按当前筛选条件导出巡检记录（最多 5000 条）', '导出确认', {
      confirmButtonText: '导出',
      cancelButtonText: '取消',
      type: 'info'
    })
  } catch {
    return
  }
  exporting.value = true
  try {
    await downloadFile('/inspection/checkins/export', {
      ...filterParams(),
      audit_status: currentAuditStatus()
    }, `巡检记录_${Date.now()}.xlsx`)
    ElMessage.success('导出成功')
  } catch {
    // 拦截器已提示
  } finally {
    exporting.value = false
  }
}

function handleTabChange() {
  selectedRows.value = []
  query.page = 1
  fetchList()
}

// 待审核 tab 下行点击让位给多选框，避免误开详情抽屉
function handleRowClick(row: CheckinItem) {
  if (activeTab.value === 'pending') return
  openDetail(row)
}

onMounted(async () => {
  // 支持从绩效报表带筛选进入（疑似作弊下钻）；reviewed=true 用于抽查后跳转等待审核
  if (route.query.inspector_id) query.inspector_id = String(route.query.inspector_id)
  if (route.query.is_suspect) onlySuspect.value = true
  if (route.query.tab === 'pending') activeTab.value = 'pending'
  fetchList()
  const [cData, uData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 })
  ])
  communities.value = cData.list
  inspectors.value = uData.list
})

// ===== 审核操作 =====
async function handlePass(row: CheckinItem) {
  try {
    await ElMessageBox.confirm(`确认通过「${row.point_name}」的打卡记录？`, '通过确认', { type: 'warning' })
  } catch {
    return
  }
  await passReview(row.id)
  ElMessage.success('已通过')
  fetchList()
}

async function handleReject(row: CheckinItem) {
  let reason = ''
  try {
    const res = await ElMessageBox.prompt('请输入打回原因', '打回记录', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPlaceholder: '如：照片模糊，请重新打卡',
      inputValidator: (v: string) => (v && v.trim() ? true : '打回原因不能为空')
    })
    reason = res.value.trim()
  } catch {
    return
  }
  await rejectReview(row.id, reason)
  ElMessage.success('已打回')
  fetchList()
}

// ===== 撤销审核（已审核 → 退回待审核） =====
async function handleReopen(row: CheckinItem) {
  try {
    await ElMessageBox.confirm(
      `确认撤销「${row.point_name}」打卡记录的审核结果？记录将退回待审核队列重新审核。`,
      '撤销审核',
      { type: 'warning' }
    )
  } catch {
    return
  }
  await reopenReview(row.id)
  ElMessage.success('已退回待审核')
  fetchList()
}

// ===== 批量通过 =====
const batchPassing = ref(false)

async function handleBatchPass() {
  const ids = selectedRows.value.map((r) => r.id)
  try {
    await ElMessageBox.confirm(`确认批量通过选中的 ${ids.length} 条打卡记录？`, '批量通过', { type: 'warning' })
  } catch {
    return
  }
  batchPassing.value = true
  try {
    const data = await batchPassReview(ids)
    ElMessage.success(data.skipped > 0 ? `已通过 ${data.passed} 条，${data.skipped} 条状态已变化被跳过` : `已通过 ${data.passed} 条`)
    tableRef.value?.clearSelection()
    fetchList()
  } finally {
    batchPassing.value = false
  }
}

// ===== 详情抽屉 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<CheckinDetail | null>(null)
const currentRow = ref<CheckinItem | null>(null)

async function openDetail(row: CheckinItem) {
  currentRow.value = row
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await getCheckin(row.id)
  } finally {
    detailLoading.value = false
  }
}

// ===== 发起抽查 =====
const spotcheckVisible = ref(false)
const spotcheckLoading = ref(false)
const spotcheckFormRef = ref<FormInstance>()
const spotcheckForm = reactive({
  community_id: undefined as string | undefined,
  inspector_id: undefined as string | undefined,
  range: null as [string, string] | null,
  mode: 'random' as 'random' | 'full',
  ratio: 10,
  handler: 'manual' as 'manual' | 'ai'
})

const spotcheckRules: FormRules = {
  range: [{ required: true, message: '请选择时间范围', trigger: 'change' }]
}

function openSpotcheck() {
  spotcheckForm.community_id = undefined
  spotcheckForm.inspector_id = undefined
  spotcheckForm.range = defaultRange()
  spotcheckForm.mode = 'random'
  spotcheckForm.ratio = 10
  spotcheckForm.handler = 'manual'
  spotcheckVisible.value = true
}

async function submitSpotcheck() {
  const valid = await spotcheckFormRef.value?.validate().catch(() => false)
  if (!valid) return
  spotcheckLoading.value = true
  try {
    const body: SpotcheckBody = {
      community_id: spotcheckForm.community_id,
      inspector_id: spotcheckForm.inspector_id,
      start_time: spotcheckForm.range?.[0],
      end_time: spotcheckForm.range?.[1],
      mode: spotcheckForm.mode,
      ratio: spotcheckForm.mode === 'random' ? spotcheckForm.ratio : undefined,
      handler: spotcheckForm.handler
    }
    const data = await spotcheck(body)
    ElMessage.success(`已抽取 ${data.picked} 条记录${spotcheckForm.handler === 'manual' ? '转人工审核' : '交大模型分析'}`)
    spotcheckVisible.value = false
    activeTab.value = 'pending'
    handleSearch()
  } catch {
    // 错误提示由请求拦截器统一弹出（如「大模型分析通道未启用」）
  } finally {
    spotcheckLoading.value = false
  }
}
</script>

<style lang="scss">
// 行可点击提示（非 scoped，作用于 el-table 生成的行）
.clickable-row {
  cursor: pointer;
}
</style>

<style lang="scss" scoped>
.records-tabs-card {
  padding-bottom: 0;
  margin-bottom: $spacing-md;

  :deep(.el-tabs) {
    padding: 0 $spacing-md;
  }

  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }
}

.toolbar-tip {
  margin-left: $spacing-sm;
  font-size: 13px;
  color: $color-text-secondary;
}

.result-tags {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.tab-badge {
  margin-left: $spacing-xs;
  transform: translateY(-2px);
}

.form-tip {
  margin-left: $spacing-sm;
  color: $color-text-secondary;
}
</style>
