<template>
  <div class="app-container" v-loading="loading">
    <!-- 第一行：统计卡 ×4（可点击下钻） -->
    <el-row :gutter="16">
      <el-col :xs="12" :md="6">
        <div class="stat-card" @click="drill('/inspection/tasks')">
          <div class="stat-title">今日任务完成率</div>
          <div class="stat-main">
            <el-progress
              type="circle"
              :percentage="data?.today_completion.rate ?? 0"
              :width="72"
              :stroke-width="8"
              :color="CHART_COLORS.primary"
            />
            <div>
              <div class="stat-value">{{ data?.today_completion.rate ?? '--' }}%</div>
              <div class="stat-desc">
                已检 {{ data?.today_completion.done ?? 0 }} / 应检 {{ data?.today_completion.total ?? 0 }}
              </div>
            </div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :md="6">
        <div class="stat-card" @click="drill('/inspection/tasks')">
          <div class="stat-title">进行中任务</div>
          <div class="stat-value">{{ data?.doing_tasks ?? '--' }}</div>
          <div class="stat-desc">实时执行中的任务</div>
        </div>
      </el-col>
      <el-col :xs="12" :md="6">
        <!-- 待处理异常：有值时红色高亮，为 0 时不高亮 -->
        <div class="stat-card" :class="{ highlight: (data?.pending_workorders ?? 0) > 0 }" @click="drill('/workorders/list')">
          <div class="stat-title">待处理异常</div>
          <div class="stat-value" :class="{ danger: (data?.pending_workorders ?? 0) > 0 }">
            {{ data?.pending_workorders ?? '--' }}
          </div>
          <div class="stat-desc">待派单工单</div>
        </div>
      </el-col>
      <el-col :xs="12" :md="6">
        <div class="stat-card" :class="{ highlight: (data?.overdue_tasks ?? 0) > 0 }" @click="drill('/inspection/tasks')">
          <div class="stat-title">逾期任务</div>
          <div class="stat-value" :class="{ danger: (data?.overdue_tasks ?? 0) > 0 }">
            {{ data?.overdue_tasks ?? '--' }}
          </div>
          <div class="stat-desc">超出执行时段未完成</div>
        </div>
      </el-col>
    </el-row>

    <!-- 第二行：趋势 + 小区排行 -->
    <el-row :gutter="16" class="block-row">
      <el-col :xs="24" :md="12">
        <div class="block-card">
          <h3 class="card-title">近 7 天巡检完成率趋势</h3>
          <p class="chart-summary">{{ trendSummary }}</p>
          <echart :option="trendOption" :empty="!data?.trend_7d?.length" empty-text="近 7 天暂无任务数据" />
        </div>
      </el-col>
      <el-col :xs="24" :md="12">
        <div class="block-card">
          <h3 class="card-title">各小区今日执行情况</h3>
          <p class="chart-summary">{{ rankSummary }}</p>
          <echart :option="rankOption" :empty="!data?.community_rank?.length" empty-text="今日暂无小区任务数据" />
        </div>
      </el-col>
    </el-row>

    <!-- 第三行：最新工单 + 执行动态 -->
    <el-row :gutter="16" class="block-row">
      <el-col :xs="24" :md="12">
        <div class="block-card">
          <div class="block-header">
            <h3 class="card-title">最新异常工单</h3>
            <el-button link type="primary" @click="drill('/workorders/list')">全部工单</el-button>
          </div>
          <template v-if="data?.latest_workorders?.length">
            <div
              v-for="wo in data.latest_workorders"
              :key="wo.id"
              class="wo-item"
              @click="drill(`/workorders/detail/${wo.id}`)"
            >
              <el-tag :type="wo.priority === 'urgent' ? 'danger' : wo.priority === 'low' ? 'info' : 'warning'" size="small">
                {{ priorityLabel(wo.priority) }}
              </el-tag>
              <span class="wo-title">{{ wo.title }}</span>
              <el-tag size="small" :type="woStatusType(wo.status)" effect="plain">{{ woStatusLabel(wo.status) }}</el-tag>
              <span class="wo-time">{{ wo.created_at.slice(5, 16) }}</span>
            </div>
          </template>
          <el-empty v-else description="今日暂无异常工单" :image-size="72" />
        </div>
      </el-col>
      <el-col :xs="24" :md="12">
        <div class="block-card">
          <h3 class="card-title">今日任务执行动态</h3>
          <template v-if="data?.task_timeline?.length">
            <el-timeline class="task-timeline">
              <el-timeline-item
                v-for="(item, i) in data.task_timeline"
                :key="i"
                :timestamp="item.time.slice(5, 16)"
                :type="item.action.includes('异常') ? 'danger' : 'primary'"
              >
                {{ item.inspector_name }} · {{ item.task_name }} · {{ item.action }}
              </el-timeline-item>
            </el-timeline>
          </template>
          <el-empty v-else description="今日暂无执行动态" :image-size="72" />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getDashboard } from '@/api/dashboard'
import type { DashboardData } from '@/api/types'
import Echart from '@/components/Echart.vue'
import { CHART_COLORS } from '@/utils/echarts'

const router = useRouter()
const loading = ref(false)
const data = ref<DashboardData | null>(null)

onMounted(async () => {
  loading.value = true
  try {
    data.value = await getDashboard()
  } finally {
    loading.value = false
  }
})

function drill(path: string) {
  router.push(path)
}

// ===== 近 7 天趋势：单系列主色折线 + 10% 面积填充（图表规范 §五） =====
const trendOption = computed(() => {
  const trend = data.value?.trend_7d || []
  if (!trend.length) return null
  return {
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => `${v}%` },
    legend: { data: ['完成率'], bottom: 0 },
    grid: { left: 40, right: 16, top: 24, bottom: 40, borderColor: CHART_COLORS.grid },
    xAxis: {
      type: 'category',
      data: trend.map((t) => t.date.slice(5)),
      axisLabel: { color: CHART_COLORS.textSecondary },
      axisLine: { lineStyle: { color: CHART_COLORS.grid } }
    },
    yAxis: {
      type: 'value',
      max: 100,
      axisLabel: { color: CHART_COLORS.textSecondary, formatter: '{value}%' },
      splitLine: { lineStyle: { color: CHART_COLORS.grid } }
    },
    series: [
      {
        name: '完成率',
        type: 'line',
        smooth: true,
        data: trend.map((t) => t.rate),
        itemStyle: { color: CHART_COLORS.primary },
        areaStyle: { color: CHART_COLORS.primary, opacity: 0.1 }
      }
    ]
  }
})

const trendSummary = computed(() => {
  const trend = data.value?.trend_7d || []
  if (!trend.length) return ''
  const avg = trend.reduce((s, t) => s + t.rate, 0) / trend.length
  return `近 7 天平均完成率 ${avg.toFixed(1)}%`
})

// ===== 小区排行：横向条形图，按完成率排序，未完成标橙 =====
const rankOption = computed(() => {
  const rank = [...(data.value?.community_rank || [])].sort((a, b) => a.rate - b.rate)
  if (!rank.length) return null
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: number) => `${v}%` },
    legend: { data: ['完成率'], bottom: 0 },
    grid: { left: 8, right: 48, top: 16, bottom: 40, containLabel: true },
    xAxis: {
      type: 'value',
      max: 100,
      axisLabel: { color: CHART_COLORS.textSecondary, formatter: '{value}%' },
      splitLine: { lineStyle: { color: CHART_COLORS.grid } }
    },
    yAxis: {
      type: 'category',
      data: rank.map((r) => r.community_name),
      axisLabel: { color: CHART_COLORS.textRegular },
      axisLine: { lineStyle: { color: CHART_COLORS.grid } }
    },
    series: [
      {
        name: '完成率',
        type: 'bar',
        barWidth: 18,
        data: rank.map((r) => ({
          value: r.rate,
          // 未完成（<100%）标橙
          itemStyle: { color: r.rate >= 100 ? CHART_COLORS.primary : CHART_COLORS.warning }
        })),
        label: { show: true, position: 'right', formatter: '{c}%', color: CHART_COLORS.textRegular }
      }
    ]
  }
})

const rankSummary = computed(() => {
  const rank = data.value?.community_rank || []
  if (!rank.length) return ''
  const incomplete = rank.filter((r) => r.rate < 100).length
  return incomplete ? `${incomplete} 个小区今日未全部完成` : '全部小区今日已完成'
})

// ===== 工单标签 =====
function priorityLabel(p: string) {
  return { urgent: '紧急', high: '高', normal: '一般', low: '低' }[p] || p
}

function woStatusLabel(s: string) {
  return { pending: '待派单', assigned: '已派单', processing: '处理中', review: '待复核', closed: '已关闭', rejected: '已驳回' }[s] || s
}

function woStatusType(s: string) {
  return ({ pending: 'warning', assigned: 'primary', processing: 'primary', review: 'warning', closed: 'info', rejected: 'danger' } as Record<string, any>)[s] || 'info'
}
</script>

<style scoped lang="scss">
.stat-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg $spacing-xl;
  cursor: pointer;
  border: 1px solid transparent;
  transition: border-color 0.2s;
  min-height: 108px;

  &:hover {
    border-color: $color-primary-hover;
  }

  &.highlight {
    border-color: $color-danger;
  }

  .stat-title {
    font-size: $font-size-body;
    color: $color-text-regular;
  }

  .stat-main {
    display: flex;
    align-items: center;
    gap: $spacing-lg;
    margin-top: $spacing-sm;
  }

  .stat-value {
    font-size: $font-size-data;
    font-weight: 600;
    color: $color-text-primary;
    line-height: 1.3;
    margin: $spacing-sm 0 $spacing-xs;

    &.danger {
      color: $color-danger;
    }
  }

  .stat-desc {
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }
}

.block-row {
  margin-top: $spacing-lg;
}

.block-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg $spacing-xl;
  margin-bottom: $spacing-lg;

  .block-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .chart-summary {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin: $spacing-xs 0 $spacing-sm;
  }
}

.wo-item {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  padding: $spacing-sm 0;
  border-bottom: 1px solid $color-border;
  cursor: pointer;

  &:last-child {
    border-bottom: none;
  }

  &:hover .wo-title {
    color: $color-primary;
  }

  .wo-title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: $color-text-primary;
  }

  .wo-time {
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }
}

.task-timeline {
  margin-top: $spacing-md;
  max-height: 260px;
  overflow-y: auto;
}
</style>
