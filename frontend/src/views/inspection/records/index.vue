<template>
  <div class="app-container">
    <!-- 顶部 Tab：全部记录 / 待审核 / 已审核 -->
    <div class="table-card records-tabs-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="全部记录" name="all" />
        <el-tab-pane v-perms="'inspection:checkin:review'" label="待审核" name="pending" />
        <el-tab-pane v-perms="'inspection:checkin:review'" label="已审核" name="reviewed" />
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
        <el-form-item v-if="activeTab === 'all'" label="审核">
          <el-select v-model="query.audit_status" placeholder="全部" clearable style="width: 110px">
            <el-option label="待审核" value="pending" />
            <el-option label="默认通过" value="auto_pass" />
            <el-option label="人工通过" value="pass" />
            <el-option label="已打回" value="rejected" />
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
            <el-tag v-if="row.is_suspect" type="warning" size="small">疑似作弊</el-tag>
            <el-tag v-else-if="row.result === 'abnormal'" type="danger" size="small">异常</el-tag>
            <el-tag v-else type="success" size="small">正常</el-tag>
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
        <el-table-column label="审核" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="auditStatusTag(row.audit_status).type" size="small">
              {{ auditStatusTag(row.audit_status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="photo_count" label="照片数" width="80" align="right" />
        <el-table-column label="操作" :width="activeTab === 'pending' ? 170 : 90">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
            <template v-if="activeTab === 'pending'">
              <el-button v-perms="'inspection:checkin:review'" link type="success" @click="handlePass(row)">通过</el-button>
              <el-button v-perms="'inspection:checkin:review'" link type="danger" @click="handleReject(row)">打回</el-button>
            </template>
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
      @go-work-order="goWorkOrder"
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
import { useRouter, useRoute } from 'vue-router'
import { Search, Refresh, Aim } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type TableInstance } from 'element-plus'
import { listCheckins, getCheckin, type CheckinQuery } from '@/api/checkin'
import { passReview, rejectReview, batchPassReview, spotcheck, type SpotcheckBody } from '@/api/review'
import { listCommunities } from '@/api/community'
import { listUsers } from '@/api/user'
import CheckinDetailDrawer from '@/components/CheckinDetailDrawer.vue'
import type { CheckinItem, CheckinDetail, CommunityItem } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const list = ref<CheckinItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectors = ref<UserItem[]>([])
const onlySuspect = ref(false)
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

// AI 结论标签
function aiVerdictTag(v: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pass: { label: '大模型通过', type: 'success' },
      review: { label: '转人工', type: 'warning' },
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
const query = reactive<CheckinQuery>({ page: 1, page_size: 20, community_id: undefined, inspector_id: undefined, result: '', audit_status: '' })

// 当前 tab 对应的审核状态过滤（已审核-全部 = pass,rejected 多值）
function currentAuditStatus(): string | undefined {
  if (activeTab.value === 'pending') return 'pending'
  if (activeTab.value === 'reviewed') return reviewedStatus.value === 'all' ? 'pass,rejected' : reviewedStatus.value
  return query.audit_status || undefined
}

async function fetchList() {
  loading.value = true
  try {
    const data = await listCheckins({
      ...query,
      result: query.result || undefined,
      audit_status: currentAuditStatus(),
      is_suspect: onlySuspect.value || undefined,
      start_time: timeRange.value?.[0],
      end_time: timeRange.value?.[1]
    })
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
  query.inspector_id = undefined
  query.result = ''
  query.audit_status = ''
  reviewedStatus.value = 'all'
  onlySuspect.value = false
  timeRange.value = defaultRange()
  handleSearch()
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

function goWorkOrder(orderNo: string) {
  detailVisible.value = false
  router.push({ path: '/workorders/list', query: { order_no: orderNo } })
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

.form-tip {
  margin-left: $spacing-sm;
  color: $color-text-secondary;
}
</style>
