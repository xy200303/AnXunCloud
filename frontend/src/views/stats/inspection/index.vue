<template>
  <div class="app-container">
    <!-- ① 筛选条 -->
    <div class="filter-card">
      <el-form inline>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            :clearable="false"
            style="width: 260px"
          />
        </el-form-item>
        <el-form-item label="小区">
          <el-select v-model="communityId" placeholder="全部小区" clearable style="width: 160px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" :loading="loading" @click="fetchAll">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <el-tabs v-model="activeTab">
        <!-- ===== 覆盖率 ===== -->
        <el-tab-pane label="巡检覆盖率" name="coverage">
          <template v-if="coverage">
            <!-- ② 指标卡 -->
            <div class="metric-row">
              <div class="metric-card">
                <div class="metric-value">{{ coverage.summary.coverage_rate }}%</div>
                <div class="metric-label">点位覆盖率</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ coverage.summary.done_points }} / {{ coverage.summary.should_points }}</div>
                <div class="metric-label">实检 / 应检点位次</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ coverage.summary.abnormal_count }}</div>
                <div class="metric-label">异常数</div>
              </div>
              <div class="metric-card">
                <div class="metric-value warn">{{ coverage.summary.suspect_count }}</div>
                <div class="metric-label">疑似作弊</div>
              </div>
            </div>

            <!-- ③ 图表 -->
            <el-row :gutter="16">
              <el-col :xs="24" :md="12">
                <div class="chart-card">
                  <h4 class="chart-title">覆盖率趋势</h4>
                  <p class="chart-summary">{{ coverageTrendSummary }}</p>
                  <echart :option="coverageTrendOption" :empty="!coverage.daily.length" height="260px" />
                </div>
              </el-col>
              <el-col :xs="24" :md="12">
                <div class="chart-card">
                  <h4 class="chart-title">各小区覆盖率</h4>
                  <p class="chart-summary">{{ coverageRankSummary }}</p>
                  <echart :option="coverageRankOption" :empty="!coverage.by_community.length" height="260px" />
                </div>
              </el-col>
            </el-row>

            <!-- ④ 明细表 -->
            <el-table :data="coverage.by_community" stripe class="detail-table">
              <el-table-column prop="community_name" label="小区" min-width="150" />
              <el-table-column prop="should_points" label="应检点位次" min-width="110" align="right" sortable />
              <el-table-column prop="done_points" label="实检" min-width="90" align="right" sortable />
              <el-table-column label="缺卡" min-width="90" align="right">
                <template #default="{ row }">{{ row.should_points - row.done_points }}</template>
              </el-table-column>
              <el-table-column prop="coverage_rate" label="覆盖率" min-width="100" align="right" sortable>
                <template #default="{ row }">{{ row.coverage_rate }}%</template>
              </el-table-column>
            </el-table>

            <!-- ⑤ 导出 -->
            <div class="export-row">
              <el-button v-perms="'stats:export'" :loading="exporting" @click="handleExport('coverage', 'excel')">导出 Excel</el-button>
              <el-button v-perms="'stats:export'" :loading="exporting" @click="handleExport('monthly', 'pdf')">导出 PDF 月报</el-button>
            </div>
          </template>
          <el-empty v-else-if="!loading" description="该条件下暂无覆盖率数据" />
        </el-tab-pane>

        <!-- ===== 及时率 ===== -->
        <el-tab-pane label="巡检及时率" name="timeliness">
          <template v-if="timeliness">
            <div class="metric-row">
              <div class="metric-card">
                <div class="metric-value">{{ timeliness.summary.timeliness_rate }}%</div>
                <div class="metric-label">及时完成率</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ timeliness.summary.on_time_tasks }} / {{ timeliness.summary.total_tasks }}</div>
                <div class="metric-label">及时 / 总任务</div>
              </div>
              <div class="metric-card">
                <div class="metric-value" :class="{ danger: timeliness.summary.overdue_tasks > 0 }">
                  {{ timeliness.summary.overdue_tasks }}
                </div>
                <div class="metric-label">逾期任务</div>
              </div>
            </div>

            <el-row :gutter="16">
              <el-col :xs="24" :md="12">
                <div class="chart-card">
                  <h4 class="chart-title">及时率趋势</h4>
                  <p class="chart-summary">{{ timelinessTrendSummary }}</p>
                  <echart :option="timelinessTrendOption" :empty="!timeliness.daily.length" height="260px" />
                </div>
              </el-col>
              <el-col :xs="24" :md="12">
                <div class="chart-card">
                  <h4 class="chart-title">各小区及时率</h4>
                  <p class="chart-summary">{{ timelinessRankSummary }}</p>
                  <echart :option="timelinessRankOption" :empty="!timeliness.by_community.length" height="260px" />
                </div>
              </el-col>
            </el-row>

            <el-table :data="timeliness.by_community" stripe class="detail-table">
              <el-table-column prop="community_name" label="小区" min-width="150" />
              <el-table-column prop="total_tasks" label="任务数" min-width="90" align="right" sortable />
              <el-table-column prop="on_time_tasks" label="及时完成" min-width="100" align="right" sortable />
              <el-table-column label="逾期" min-width="80" align="right">
                <template #default="{ row }">
                  <span :class="{ 'danger-text': row.total_tasks - row.on_time_tasks > 0 }">
                    {{ row.total_tasks - row.on_time_tasks }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="timeliness_rate" label="及时率" min-width="100" align="right" sortable>
                <template #default="{ row }">{{ row.timeliness_rate }}%</template>
              </el-table-column>
            </el-table>

            <div class="export-row">
              <el-button v-perms="'stats:export'" :loading="exporting" @click="handleExport('timeliness', 'excel')">导出 Excel</el-button>
            </div>
          </template>
          <el-empty v-else-if="!loading" description="该条件下暂无及时率数据" />
        </el-tab-pane>
        <!-- ===== 巡更达成率 ===== -->
        <el-tab-pane label="巡更达成率" name="rounds">
          <!-- 巡更口径筛选：小区必选（后端必填），按计划/日期段钻取，默认本月 -->
          <el-form inline class="rounds-filter">
            <el-form-item label="小区">
              <el-select v-model="roundsCommunityId" placeholder="选择小区" style="width: 160px" @change="handleRoundsCommunityChange">
                <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="计划">
              <el-select v-model="roundsPlanId" placeholder="全部计划" clearable style="width: 180px">
                <el-option v-for="p in roundsPlans" :key="p.id" :label="p.name" :value="p.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="日期段">
              <el-date-picker
                v-model="roundsRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                value-format="YYYY-MM-DD"
                :clearable="false"
                style="width: 260px"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :icon="Search" :loading="roundsLoading" @click="fetchRounds">查询</el-button>
            </el-form-item>
          </el-form>

          <template v-if="rounds">
            <div class="metric-row">
              <div class="metric-card">
                <div class="metric-value">{{ rounds.summary.should_rounds }}</div>
                <div class="metric-label">应巡轮次</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ rounds.summary.done_rounds }}</div>
                <div class="metric-label">实巡轮次</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ rounds.summary.achievement_rate }}%</div>
                <div class="metric-label">巡更达成率</div>
              </div>
              <div class="metric-card">
                <div class="metric-value" :class="{ danger: rounds.summary.overdue_rounds > 0 }">
                  {{ rounds.summary.overdue_rounds }}
                </div>
                <div class="metric-label">逾期轮次</div>
              </div>
              <div class="metric-card">
                <div class="metric-value">{{ rounds.summary.avg_point_completion }}%</div>
                <div class="metric-label">单轮点位完成率</div>
              </div>
            </div>
            <p class="chart-summary">
              达标线：{{ rounds.summary.daily_min_rounds != null ? `每日 ≥ ${rounds.summary.daily_min_rounds} 轮` : '未设置' }}；进行中/待开始 {{ rounds.summary.open_rounds }} 轮
            </p>

            <!-- 按天明细：不达标（met=false）或达成率 <100% 标红 -->
            <el-table
              :data="rounds.daily"
              stripe
              class="detail-table"
              :row-class-name="roundsRowClass"
            >
              <el-table-column prop="date" label="日期" min-width="110" />
              <el-table-column prop="should_rounds" label="应巡轮次" min-width="90" align="right" sortable />
              <el-table-column prop="done_rounds" label="实巡" min-width="80" align="right" sortable />
              <el-table-column label="逾期" min-width="80" align="right">
                <template #default="{ row }">
                  <span :class="{ 'danger-text': row.overdue_rounds > 0 }">{{ row.overdue_rounds }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="achievement_rate" label="达成率" min-width="90" align="right" sortable>
                <template #default="{ row }">{{ row.achievement_rate }}%</template>
              </el-table-column>
              <el-table-column label="达标" min-width="90" align="center">
                <template #default="{ row }">
                  <el-tag v-if="row.met === true" type="success" size="small">达标</el-tag>
                  <el-tag v-else-if="row.met === false" type="danger" size="small">不达标</el-tag>
                  <span v-else class="text-secondary">--</span>
                </template>
              </el-table-column>
            </el-table>

            <!-- 逾期轮次清单：overdue 已翻转 / expired_doing 窗口已过未翻转 -->
            <template v-if="rounds.overdue_list.length">
              <h4 class="chart-title">逾期轮次清单</h4>
              <el-table :data="rounds.overdue_list" stripe class="detail-table">
                <el-table-column prop="task_date" label="任务日期" min-width="100" />
                <el-table-column prop="round_name" label="轮次" min-width="100" show-overflow-tooltip />
                <el-table-column prop="time_window" label="时间窗" min-width="110" align="center" />
                <el-table-column prop="inspector_name" label="巡检员" min-width="90" />
                <el-table-column label="点位完成" min-width="90" align="right">
                  <template #default="{ row }">{{ row.done_points }}/{{ row.total_points }}</template>
                </el-table-column>
                <el-table-column label="状态" min-width="130" align="center">
                  <template #default="{ row }">
                    <el-tag v-if="row.state === 'expired_doing'" type="warning" size="small">窗口已过未翻转</el-tag>
                    <el-tag v-else type="danger" size="small">已逾期</el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </template>
          </template>
          <el-empty v-else-if="!roundsLoading" description="请选择小区后查询巡更达成率" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
// 巡检报表：覆盖率 + 及时率（五段式：筛选条 + 指标卡 + 图表 + 明细表 + 导出）
// 说明：后端菜单将两个报表合在「巡检报表」一页，这里用 Tab 承载
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'
import { getCoverage, getTimeliness, exportReport, getPatrolRounds } from '@/api/stats'
import { withFileToken } from '@/api/upload'
import { listCommunities } from '@/api/community'
import { listPlans } from '@/api/plan'
import Echart from '@/components/Echart.vue'
import { CHART_COLORS } from '@/utils/echarts'
import type { CoverageData, TimelinessData, CommunityItem, PlanItem, PatrolRoundsData, PatrolRoundsDaily } from '@/api/biz-types'

const loading = ref(false)
const exporting = ref(false)
const activeTab = ref('coverage')
const communities = ref<CommunityItem[]>([])
const communityId = ref<string | undefined>()

function defaultRange(): [string, string] {
  const end = new Date()
  const start = new Date(Date.now() - 6 * 86400000)
  const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return [fmt(start), fmt(end)]
}

const dateRange = ref<[string, string]>(defaultRange())
const coverage = ref<CoverageData | null>(null)
const timeliness = ref<TimelinessData | null>(null)

async function fetchAll() {
  loading.value = true
  try {
    const params = { start_date: dateRange.value[0], end_date: dateRange.value[1], community_id: communityId.value }
    const [cov, tim] = await Promise.all([getCoverage(params), getTimeliness(params)])
    coverage.value = cov
    timeliness.value = tim
  } finally {
    loading.value = false
  }
}

function handleReset() {
  dateRange.value = defaultRange()
  communityId.value = undefined
  fetchAll()
}

onMounted(async () => {
  fetchAll()
  const cData = await listCommunities({ page: 1, page_size: 100, status: 1 })
  communities.value = cData.list
})

// ===== 巡更达成率（轮次口径，小区必选；计划下拉跟随小区） =====
const roundsLoading = ref(false)
const rounds = ref<PatrolRoundsData | null>(null)
const roundsCommunityId = ref<string | undefined>()
const roundsPlanId = ref<string | undefined>()
const roundsPlans = ref<PlanItem[]>([])

// 默认本月
function monthRange(): [string, string] {
  const now = new Date()
  const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return [fmt(new Date(now.getFullYear(), now.getMonth(), 1)), fmt(now)]
}

const roundsRange = ref<[string, string]>(monthRange())

async function handleRoundsCommunityChange() {
  roundsPlanId.value = undefined
  roundsPlans.value = []
  if (!roundsCommunityId.value) return
  const data = await listPlans({ community_id: roundsCommunityId.value, page: 1, page_size: 100 })
  roundsPlans.value = data.list
  fetchRounds()
}

async function fetchRounds() {
  if (!roundsCommunityId.value) {
    ElMessage.warning('请先选择小区')
    return
  }
  roundsLoading.value = true
  try {
    rounds.value = await getPatrolRounds({
      community_id: roundsCommunityId.value,
      from: roundsRange.value[0],
      to: roundsRange.value[1],
      plan_id: roundsPlanId.value || undefined
    })
  } finally {
    roundsLoading.value = false
  }
}

// 不达标行标红：met=false 或达成率 <100%
function roundsRowClass({ row }: { row: PatrolRoundsDaily }) {
  return row.met === false || (row.should_rounds > 0 && row.achievement_rate < 100) ? 'rounds-row-miss' : ''
}

// ===== 图表组装（折线：主色单系列 + 10% 面积；条图：横向、值标条端） =====
function lineOption(dates: string[], values: number[], name: string) {
  return {
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => `${v}%` },
    legend: { data: [name], bottom: 0 },
    grid: { left: 40, right: 16, top: 24, bottom: 40 },
    xAxis: { type: 'category', data: dates, axisLabel: { color: CHART_COLORS.textSecondary }, axisLine: { lineStyle: { color: CHART_COLORS.grid } } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: CHART_COLORS.textSecondary, formatter: '{value}%' }, splitLine: { lineStyle: { color: CHART_COLORS.grid } } },
    series: [{ name, type: 'line', smooth: true, data: values, itemStyle: { color: CHART_COLORS.primary }, areaStyle: { color: CHART_COLORS.primary, opacity: 0.1 } }]
  }
}

function barOption(names: string[], values: number[], name: string) {
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: number) => `${v}%` },
    legend: { data: [name], bottom: 0 },
    grid: { left: 8, right: 48, top: 16, bottom: 40, containLabel: true },
    xAxis: { type: 'value', max: 100, axisLabel: { color: CHART_COLORS.textSecondary, formatter: '{value}%' }, splitLine: { lineStyle: { color: CHART_COLORS.grid } } },
    yAxis: { type: 'category', data: names, axisLabel: { color: CHART_COLORS.textRegular }, axisLine: { lineStyle: { color: CHART_COLORS.grid } } },
    series: [{
      name, type: 'bar', barWidth: 16,
      data: values.map((v) => ({ value: v, itemStyle: { color: v >= 100 ? CHART_COLORS.primary : CHART_COLORS.warning } })),
      label: { show: true, position: 'right', formatter: '{c}%', color: CHART_COLORS.textRegular }
    }]
  }
}

const coverageTrendOption = computed(() =>
  coverage.value?.daily.length
    ? lineOption(coverage.value.daily.map((d) => d.date.slice(5)), coverage.value.daily.map((d) => d.coverage_rate), '覆盖率')
    : null
)

const coverageRankOption = computed(() => {
  const list = [...(coverage.value?.by_community || [])].sort((a, b) => a.coverage_rate - b.coverage_rate)
  return list.length ? barOption(list.map((c) => c.community_name), list.map((c) => c.coverage_rate), '覆盖率') : null
})

const timelinessTrendOption = computed(() =>
  timeliness.value?.daily.length
    ? lineOption(timeliness.value.daily.map((d) => d.date.slice(5)), timeliness.value.daily.map((d) => d.timeliness_rate), '及时率')
    : null
)

const timelinessRankOption = computed(() => {
  const list = [...(timeliness.value?.by_community || [])].sort((a, b) => a.timeliness_rate - b.timeliness_rate)
  return list.length ? barOption(list.map((c) => c.community_name), list.map((c) => c.timeliness_rate), '及时率') : null
})

// 文字结论（无障碍摘要）
const coverageTrendSummary = computed(() => {
  const daily = coverage.value?.daily || []
  if (!daily.length) return ''
  const avg = daily.reduce((s, d) => s + d.coverage_rate, 0) / daily.length
  return `区间内日均覆盖率 ${avg.toFixed(1)}%`
})

const coverageRankSummary = computed(() => {
  const list = coverage.value?.by_community || []
  if (!list.length) return ''
  const worst = [...list].sort((a, b) => a.coverage_rate - b.coverage_rate)[0]
  return worst.coverage_rate >= 100 ? '全部小区覆盖率达 100%' : `覆盖率最低：${worst.community_name} ${worst.coverage_rate}%`
})

const timelinessTrendSummary = computed(() => {
  const daily = timeliness.value?.daily || []
  if (!daily.length) return ''
  const avg = daily.reduce((s, d) => s + d.timeliness_rate, 0) / daily.length
  return `区间内日均及时率 ${avg.toFixed(1)}%`
})

const timelinessRankSummary = computed(() => {
  const list = timeliness.value?.by_community || []
  if (!list.length) return ''
  const worst = [...list].sort((a, b) => a.timeliness_rate - b.timeliness_rate)[0]
  return worst.timeliness_rate >= 100 ? '全部小区无逾期' : `及时率最低：${worst.community_name} ${worst.timeliness_rate}%`
})

// ===== 导出：loading 防抖，异步生成后消息中心通知 =====
async function handleExport(reportType: 'coverage' | 'timeliness' | 'monthly', format: 'excel' | 'pdf') {
  exporting.value = true
  try {
    const res = await exportReport({
      report_type: reportType,
      format,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      community_id: communityId.value
    })
    if (res.status === 'done' && res.download_url) {
      const a = document.createElement('a')
      a.href = withFileToken(res.download_url)
      a.target = '_blank'
      a.click()
      ElMessage.success('导出文件已生成，开始下载')
    } else {
      ElMessage.success('导出任务已提交，生成完成后请在消息中心下载')
    }
  } finally {
    exporting.value = false
  }
}
</script>

<style scoped lang="scss">
.metric-row {
  display: flex;
  gap: $spacing-lg;
  margin-bottom: $spacing-xl;

  .metric-card {
    flex: 1;
    background: $color-bg-page;
    border-radius: $radius-card;
    padding: $spacing-lg $spacing-xl;

    .metric-value {
      font-size: $font-size-data;
      font-weight: 600;
      color: $color-text-primary;
      line-height: 1.3;

      &.warn {
        color: $color-warning;
      }

      &.danger {
        color: $color-danger;
      }
    }

    .metric-label {
      font-size: $font-size-aux;
      color: $color-text-secondary;
      margin-top: $spacing-xs;
    }
  }
}

.chart-card {
  margin-bottom: $spacing-lg;

  .chart-title {
    font-size: $font-size-body;
    font-weight: 600;
    margin: 0;
    color: $color-text-primary;
  }

  .chart-summary {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin: $spacing-xs 0 $spacing-sm;
  }
}

.detail-table {
  margin-bottom: $spacing-lg;
}

.rounds-filter {
  margin-bottom: $spacing-md;
}

// 不达标行标红（met=false 或达成率 <100%）
:deep(.rounds-row-miss) {
  --el-table-tr-bg-color: var(--el-color-danger-light-9);
}

.export-row {
  display: flex;
  gap: $spacing-sm;
}

.danger-text {
  color: $color-danger;
}
</style>
