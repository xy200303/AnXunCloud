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
          <el-button type="primary" :icon="Search" :loading="loading" @click="fetchList">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <template v-if="list.length || loading">
        <!-- ② 指标卡 -->
        <div class="metric-row">
          <div class="metric-card">
            <div class="metric-value">{{ avgCoverage }}%</div>
            <div class="metric-label">人均完成率</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{{ avgAbnormal }}</div>
            <div class="metric-label">人均异常发现</div>
          </div>
          <div class="metric-card">
            <div class="metric-value" :class="{ warn: totalSuspect > 0 }">{{ totalSuspect }}</div>
            <div class="metric-label">疑似作弊总数</div>
          </div>
        </div>

        <!-- ③ 图表：巡检员完成率对比（横向条形，>5 人不用饼图） -->
        <div class="chart-card">
          <h4 class="chart-title">巡检员完成率对比</h4>
          <p class="chart-summary">{{ chartSummary }}</p>
          <echart :option="chartOption" :empty="!list.length" height="280px" />
        </div>

        <!-- ④ 绩效表：服务端排序 -->
        <el-table v-loading="loading" :data="list" stripe class="detail-table" @sort-change="handleSortChange">
          <el-table-column prop="inspector_name" label="巡检员" min-width="110" />
          <el-table-column label="所属小区" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">{{ row.community_names?.join('、') || '--' }}</template>
          </el-table-column>
          <el-table-column prop="total_tasks" label="任务数" width="90" align="right" />
          <el-table-column prop="coverage_rate" label="完成率" width="110" align="right" sortable="custom" :sort-orders="['descending', 'ascending']">
            <template #default="{ row }">{{ row.coverage_rate }}%</template>
          </el-table-column>
          <el-table-column prop="avg_duration_min" label="平均耗时" width="120" align="right" sortable="custom" :sort-orders="['descending', 'ascending']">
            <template #default="{ row }">{{ formatDuration(row.avg_duration_min) }}</template>
          </el-table-column>
          <el-table-column prop="abnormal_found" label="异常发现数" width="110" align="right" sortable="custom" :sort-orders="['descending', 'ascending']" />
          <el-table-column label="疑似作弊" width="100" align="right">
            <template #default="{ row }">
              <!-- 非 0 标橙，可点击跳巡检记录带筛选 -->
              <el-link v-if="row.suspect_count > 0" type="warning" @click="goSuspectRecords(row)">
                {{ row.suspect_count }}
              </el-link>
              <span v-else>0</span>
            </template>
          </el-table-column>
          <el-table-column label="缺卡" width="90" align="right">
            <template #default="{ row }">{{ row.should_points - row.done_points }}</template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrap">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            layout="total, prev, pager, next"
            @change="fetchList"
          />
        </div>

        <!-- ⑤ 导出 -->
        <div class="export-row">
          <el-button v-perms="'stats:export'" :loading="exporting" @click="handleExport">导出 Excel</el-button>
        </div>
      </template>
      <el-empty v-else description="该条件下暂无绩效数据" />
    </div>
  </div>
</template>

<script setup lang="ts">
// 巡检员绩效报表（五段式，表格为主，支持按列排序）
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { getPerformance, exportReport } from '@/api/stats'
import { listCommunities } from '@/api/community'
import Echart from '@/components/Echart.vue'
import { CHART_COLORS } from '@/utils/echarts'
import type { PerformanceItem, CommunityItem } from '@/api/biz-types'

const router = useRouter()
const loading = ref(false)
const exporting = ref(false)
const communities = ref<CommunityItem[]>([])
const communityId = ref<string | undefined>()
const list = ref<PerformanceItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const sortBy = ref('coverage_rate')
const sortOrder = ref('desc')

function defaultRange(): [string, string] {
  const end = new Date()
  const start = new Date(Date.now() - 29 * 86400000)
  const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return [fmt(start), fmt(end)]
}

const dateRange = ref<[string, string]>(defaultRange())

async function fetchList() {
  loading.value = true
  try {
    const data = await getPerformance({
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      community_id: communityId.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      page: page.value,
      page_size: pageSize.value
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleReset() {
  dateRange.value = defaultRange()
  communityId.value = undefined
  page.value = 1
  fetchList()
}

function handleSortChange({ prop, order }: { prop: string; order: string | null }) {
  if (!order) return
  sortBy.value = prop
  sortOrder.value = order === 'ascending' ? 'asc' : 'desc'
  fetchList()
}

onMounted(async () => {
  fetchList()
  const cData = await listCommunities({ page: 1, page_size: 100, status: 1 })
  communities.value = cData.list
})

// ===== 指标卡 =====
const avgCoverage = computed(() => {
  if (!list.value.length) return '--'
  return (list.value.reduce((s, x) => s + x.coverage_rate, 0) / list.value.length).toFixed(1)
})

const avgAbnormal = computed(() => {
  if (!list.value.length) return '--'
  return (list.value.reduce((s, x) => s + x.abnormal_found, 0) / list.value.length).toFixed(1)
})

const totalSuspect = computed(() => list.value.reduce((s, x) => s + x.suspect_count, 0))

// ===== 图表 =====
const chartOption = computed(() => {
  const sorted = [...list.value].sort((a, b) => a.coverage_rate - b.coverage_rate)
  if (!sorted.length) return null
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: number) => `${v}%` },
    legend: { data: ['完成率'], bottom: 0 },
    grid: { left: 8, right: 48, top: 16, bottom: 40, containLabel: true },
    xAxis: { type: 'value', max: 100, axisLabel: { color: CHART_COLORS.textSecondary, formatter: '{value}%' }, splitLine: { lineStyle: { color: CHART_COLORS.grid } } },
    yAxis: { type: 'category', data: sorted.map((x) => x.inspector_name), axisLabel: { color: CHART_COLORS.textRegular }, axisLine: { lineStyle: { color: CHART_COLORS.grid } } },
    series: [{
      name: '完成率', type: 'bar', barWidth: 16,
      data: sorted.map((x) => ({ value: x.coverage_rate, itemStyle: { color: x.coverage_rate >= 95 ? CHART_COLORS.primary : CHART_COLORS.warning } })),
      label: { show: true, position: 'right', formatter: '{c}%', color: CHART_COLORS.textRegular }
    }]
  }
})

const chartSummary = computed(() => {
  if (!list.value.length) return ''
  const best = [...list.value].sort((a, b) => b.coverage_rate - a.coverage_rate)[0]
  return `完成率最高：${best.inspector_name} ${best.coverage_rate}%`
})

// 平均耗时格式：xh xymin
function formatDuration(min: number) {
  if (!min) return '0min'
  const h = Math.floor(min / 60)
  const m = min % 60
  return h ? `${h}h ${m}min` : `${m}min`
}

// 疑似作弊点击 → 巡检记录页带筛选
function goSuspectRecords(row: PerformanceItem) {
  router.push({ path: '/inspection/records', query: { inspector_id: row.inspector_id, is_suspect: '1' } })
}

async function handleExport() {
  exporting.value = true
  try {
    const res = await exportReport({
      report_type: 'performance',
      format: 'excel',
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      community_id: communityId.value
    })
    if (res.status === 'done' && res.download_url) {
      const a = document.createElement('a')
      a.href = res.download_url
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
  margin-bottom: $spacing-md;
}

.export-row {
  margin-top: $spacing-lg;
}
</style>
