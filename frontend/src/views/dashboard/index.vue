<template>
  <div class="app-container" v-loading="loading">
    <!-- 问候条 -->
    <div class="hero-card">
      <div class="hero-main">
        <div class="hero-hello">{{ greeting }}，{{ userStore.name || '管理员' }}</div>
        <div class="hero-sub">欢迎使用安巡云物业巡检管理平台</div>
      </div>
      <div class="hero-date">
        <div class="hero-day">{{ dateText }}</div>
        <div class="hero-week">{{ weekText }}</div>
      </div>
    </div>

    <!-- 第一行：统计卡 ×3（可点击下钻） -->
    <el-row :gutter="16" class="block-row">
      <el-col :xs="12" :md="8">
        <div class="stat-card" @click="drill('/inspection/tasks')">
          <div class="stat-top">
            <div class="stat-icon tone-primary"><el-icon :size="22"><CircleCheck /></el-icon></div>
            <div class="stat-title">今日任务完成率</div>
          </div>
          <div class="stat-main">
            <div>
              <div class="stat-value">{{ data?.today_completion.rate ?? '--' }}%</div>
              <div class="stat-desc">
                已检 {{ data?.today_completion.done ?? 0 }} / 应检 {{ data?.today_completion.total ?? 0 }}
              </div>
            </div>
            <el-progress
              type="circle"
              :percentage="data?.today_completion.rate ?? 0"
              :width="64"
              :stroke-width="6"
              :color="CHART_COLORS.primary"
            />
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :md="8">
        <div class="stat-card" @click="drill('/inspection/tasks')">
          <div class="stat-top">
            <div class="stat-icon tone-success"><el-icon :size="22"><Timer /></el-icon></div>
            <div class="stat-title">进行中任务</div>
          </div>
          <div class="stat-value">{{ data?.doing_tasks ?? '--' }}</div>
          <div class="stat-desc">实时执行中的任务</div>
        </div>
      </el-col>
      <el-col :xs="12" :md="8">
        <div class="stat-card" :class="{ highlight: (data?.overdue_tasks ?? 0) > 0 }" @click="drill('/inspection/tasks')">
          <div class="stat-top">
            <div class="stat-icon" :class="(data?.overdue_tasks ?? 0) > 0 ? 'tone-warning' : 'tone-neutral'">
              <el-icon :size="22"><AlarmClock /></el-icon>
            </div>
            <div class="stat-title">逾期任务</div>
          </div>
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

    <!-- 第三行：执行动态 -->
    <el-row :gutter="16" class="block-row">
      <el-col :xs="24" :md="24">
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
import { CircleCheck, Timer, AlarmClock } from '@element-plus/icons-vue'
import { getDashboard } from '@/api/dashboard'
import type { DashboardData } from '@/api/types'
import Echart from '@/components/Echart.vue'
import { CHART_COLORS } from '@/utils/echarts'
import { useUserStore } from '@/store/user'

const router = useRouter()
const userStore = useUserStore()
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

// ===== 问候条 =====
const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '凌晨好'
  if (h < 9) return '早上好'
  if (h < 12) return '上午好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

const now = new Date()
const dateText = `${now.getFullYear()} 年 ${now.getMonth() + 1} 月 ${now.getDate()} 日`
const weekText = '星期' + '日一二三四五六'[now.getDay()]

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
</script>

<style scoped lang="scss">
// ===== 问候条 =====
.hero-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-xl $spacing-xxl;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-left: 4px solid $color-primary;

  .hero-hello {
    font-size: $font-size-page-title;
    font-weight: 600;
    color: $color-text-primary;
  }

  .hero-sub {
    font-size: $font-size-body;
    color: $color-text-secondary;
    margin-top: $spacing-xs;
  }

  .hero-date {
    text-align: right;

    .hero-day {
      font-size: $font-size-card-title;
      font-weight: 600;
      color: $color-text-primary;
    }

    .hero-week {
      font-size: $font-size-aux;
      color: $color-text-secondary;
      margin-top: $spacing-xs;
    }
  }
}

// ===== 统计卡 =====
.stat-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg $spacing-xl;
  cursor: pointer;
  border: 1px solid transparent;
  transition: border-color 0.2s, transform 0.2s;
  min-height: 128px;

  &:hover {
    border-color: $color-primary-hover;
    transform: translateY(-2px);
  }

  &.highlight {
    border-color: $color-danger;
  }

  .stat-top {
    display: flex;
    align-items: center;
    gap: $spacing-md;
  }

  .stat-icon {
    width: 44px;
    height: 44px;
    border-radius: $radius-card;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;

    &.tone-primary {
      background: $color-primary-light;
      color: $color-primary;
    }

    &.tone-success {
      background: rgba($color-success, 0.1);
      color: $color-success;
    }

    &.tone-warning {
      background: rgba($color-warning, 0.1);
      color: $color-warning;
    }

    &.tone-danger {
      background: rgba($color-danger, 0.1);
      color: $color-danger;
    }

    &.tone-neutral {
      background: $color-bg-page;
      color: $color-text-secondary;
    }
  }

  .stat-title {
    font-size: $font-size-body;
    color: $color-text-regular;
  }

  .stat-main {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: $spacing-sm;
  }

  .stat-value {
    font-size: $font-size-data;
    font-weight: 600;
    color: $color-text-primary;
    line-height: 1.3;
    margin: $spacing-md 0 $spacing-xs;

    &.danger {
      color: $color-danger;
    }
  }

  .stat-main .stat-value {
    margin-top: $spacing-sm;
  }

  .stat-desc {
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }
}

.block-row {
  margin-top: $spacing-lg;
}

// ===== 区块卡 =====
.block-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg $spacing-xl;
  margin-bottom: $spacing-lg;

  .card-title {
    font-size: $font-size-card-title;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0;
    padding-left: $spacing-md;
    border-left: 4px solid $color-primary;
    line-height: 1.4;
  }

  .chart-summary {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin: $spacing-xs 0 $spacing-sm;
  }
}

.task-timeline {
  margin-top: $spacing-md;
  max-height: 260px;
  overflow-y: auto;
}
</style>
