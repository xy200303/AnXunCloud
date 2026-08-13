<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading && tasks.length == 0" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded && tasks.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="reload">重试</text>
    </view>

    <view v-else class="content">
      <!-- 统计卡（2x2） -->
      <view class="stats">
        <view class="stat-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="stat-num" :style="{ color: colors.primary }">{{ rateText }}</text>
          <text class="stat-label" :style="{ color: colors.textSecondary }">今日完成率 {{ doneCount }}/{{ totalCount }}</text>
        </view>
        <view class="stat-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="stat-num" :style="{ color: colors.primary }">{{ board.doing_tasks }}</text>
          <text class="stat-label" :style="{ color: colors.textSecondary }">进行中任务</text>
        </view>
        <view class="stat-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="stat-num" :style="{ color: colors.danger }">{{ board.overdue_tasks }}</text>
          <text class="stat-label" :style="{ color: colors.textSecondary }">已逾期任务</text>
        </view>
        <view class="stat-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="stat-num" :style="{ color: colors.warning }">{{ board.pending_workorders }}</text>
          <text class="stat-label" :style="{ color: colors.textSecondary }">待处理工单</text>
        </view>
      </view>

      <!-- 任务筛选 tab：全部 / 有漏点 / 异常 -->
      <view class="tabs" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
        <view v-for="t in tabs" :key="t.value" class="tab" @click="switchFilter(t.value)">
          <text class="tab-text" :style="{ color: filter == t.value ? colors.primary : colors.textRegular }">{{ t.label }}</text>
          <view class="tab-line" :style="{ backgroundColor: filter == t.value ? colors.primary : 'transparent' }"></view>
        </view>
      </view>

      <!-- 空态 -->
      <view v-if="loaded && tasks.length == 0" class="empty">
        <text class="empty-title" :style="{ color: colors.textRegular }">{{ filter == '' ? '今日暂无巡检任务' : '该筛选下暂无任务' }}</text>
        <text class="empty-sub" :style="{ color: colors.textSecondary }">下拉可刷新</text>
      </view>

      <!-- 今日任务列表 -->
      <view
        v-for="t in tasks"
        :key="t.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="goDetail(t.id)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ t.plan_name }}</text>
          <text class="card-status" :style="{ color: t.status_color }">{{ t.status_text }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">
          {{ t.community_name }} · {{ t.inspector_name }}<text v-if="t.time_window != ''"> · {{ t.time_window }}</text>
        </text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view class="progress-inner" :style="{ width: t.progress + '%', backgroundColor: t.status_color }"></view>
        </view>
        <view class="card-foot">
          <text class="card-progress-text" :style="{ color: colors.textRegular }">
            {{ t.done_points }}/{{ t.total_points }} 点位
            <text v-if="t.abnormal_count > 0" :style="{ color: colors.danger }"> · 异常 {{ t.abnormal_count }}</text>
            <text v-if="t.missing_count > 0 && t.status != 'done' && t.status != 'pending'" :style="{ color: colors.warning }"> · 漏 {{ t.missing_count }}</text>
          </text>
          <view
            v-if="t.status != 'done'"
            class="btn-remind"
            :style="{ borderColor: colors.warning }"
            @click.stop="onRemind(t)"
          >
            <text class="btn-remind-text" :style="{ color: colors.warning }">催办</text>
          </view>
        </view>
      </view>

      <!-- 加载更多状态 -->
      <view v-if="tasks.length > 0" class="loadmore">
        <text v-if="loadingMore" class="loadmore-text" :style="{ color: colors.textSecondary }">加载中…</text>
        <text v-else-if="noMore" class="loadmore-text" :style="{ color: colors.textSecondary }">没有更多了</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiAdminDashboard, apiTaskMonitorList, apiTaskRemind, DashboardData, MonitorTask } from '@/services/api'

const PAGE_SIZE = 20

/** 任务行视图模型：文案/颜色在数据层预计算 */
type TaskView = MonitorTask & {
  status_text: string
  status_color: string
}

type BoardData = {
  colors: ColorTokens
  board: Pick<DashboardData, 'today_completion' | 'doing_tasks' | 'overdue_tasks' | 'pending_workorders'>
  filter: string
  tabs: Array<{ label: string; value: string }>
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  tasks: TaskView[]
  reminding: boolean
}

function emptyBoard(): BoardData['board'] {
  return {
    today_completion: { total: 0, done: 0, rate: 0 },
    doing_tasks: 0,
    overdue_tasks: 0,
    pending_workorders: 0
  }
}

function statusTextOf(s: string): string {
  if (s == 'doing') return '进行中'
  if (s == 'done') return '已完成'
  if (s == 'overdue') return '已逾期'
  return '待开始'
}

function statusColorOf(s: string): string {
  if (s == 'doing') return Colors.primary
  if (s == 'done') return Colors.success
  if (s == 'overdue') return Colors.danger
  return Colors.warning
}

function toTaskView(t: MonitorTask): TaskView {
  return Object.assign({}, t, {
    status_text: statusTextOf(t.status),
    status_color: statusColorOf(t.status)
  })
}

export default {
  data(): BoardData {
    return {
      colors: Colors,
      board: emptyBoard(),
      filter: '',
      tabs: [
        { label: '全部', value: '' },
        { label: '有漏点', value: 'missing' },
        { label: '异常', value: 'abnormal' }
      ],
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      tasks: [] as TaskView[],
      reminding: false
    }
  },
  computed: {
    rateText(): string {
      return this.board.today_completion.rate + '%'
    },
    doneCount(): number {
      return this.board.today_completion.done
    },
    totalCount(): number {
      return this.board.today_completion.total
    },
    noMore(): boolean {
      return this.loaded && this.tasks.length >= this.total
    }
  },
  onLoad() {
    this.reload()
  },
  onShow() {
    // 任务明细返回后刷新（可能在详情页看到最新状态前已操作）
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    switchFilter(v: string) {
      if (this.filter == v) return
      this.filter = v
      this.reload()
    },
    reload() {
      this.page = 1
      this.fetchBoard()
      this.fetchTasks(false)
    },
    /** 顶部统计卡（失败静默，不影响任务列表） */
    fetchBoard() {
      apiAdminDashboard()
        .then((d) => {
          this.board = {
            today_completion: d.today_completion,
            doing_tasks: d.doing_tasks,
            overdue_tasks: d.overdue_tasks,
            pending_workorders: d.pending_workorders
          }
        })
        .catch((_e: any) => {})
    },
    loadMore() {
      if (this.loading || this.loadingMore || this.noMore || !this.loaded) return
      this.page += 1
      this.fetchTasks(true)
    },
    fetchTasks(append: boolean) {
      if (append) {
        this.loadingMore = true
      } else {
        this.loading = true
      }
      apiTaskMonitorList(this.page, PAGE_SIZE, this.filter)
        .then((res) => {
          const views: TaskView[] = []
          res.list.forEach((t: MonitorTask) => {
            views.push(toTaskView(t))
          })
          this.total = res.total
          this.tasks = append ? this.tasks.concat(views) : views
          this.loading = false
          this.loadingMore = false
          this.loaded = true
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          this.loadingMore = false
          if (append) this.page -= 1
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded || append) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: '/pages/admin/task-detail?id=' + encodeURIComponent(id) })
    },
    /** 催办：确认弹窗 → POST remind → toast 结果（已完成任务后端报错文案直接 toast） */
    onRemind(t: TaskView) {
      if (this.reminding) return
      uni.showModal({
        title: '任务催办',
        content: '给 ' + t.inspector_name + ' 发送「' + t.plan_name + '」的催办提醒？',
        confirmText: '催办',
        success: (res) => {
          if (!res.confirm) return
          this.reminding = true
          apiTaskRemind(t.id)
            .then(() => {
              uni.showToast({ title: '已提醒执行人', icon: 'none' })
            })
            .catch((e: Error) => {
              uni.showToast({ title: e.message, icon: 'none' })
            })
            .finally(() => {
              this.reminding = false
            })
        }
      })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.content {
  padding-bottom: 24rpx;
}

.stats {
  flex-direction: row;
  flex-wrap: wrap;
  padding: 24rpx 24rpx 0;
}

.stat-card {
  width: 336rpx;
  border-radius: 24rpx; /* Radius.card */
  padding: 24rpx;
  margin-bottom: 16rpx;
}

/* 2x2 网格：奇数卡右间距 */
.stat-card:nth-child(odd) {
  margin-right: 16rpx;
}

.stat-num {
  font-size: 48rpx; /* FontSize.number */
  font-weight: 600;
}

.stat-label {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.tabs {
  flex-direction: row;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
  margin-bottom: 24rpx;
}

.tab {
  flex: 1;
  align-items: center;
  padding-top: 24rpx;
}

.tab-text {
  font-size: 30rpx;
  font-weight: 600;
}

.tab-line {
  width: 64rpx;
  height: 6rpx;
  border-radius: 3rpx;
  margin-top: 16rpx;
}

.skeleton {
  padding: 24rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.sk-short {
  height: 96rpx;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-sub {
  font-size: 26rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
  margin-left: 24rpx;
  margin-right: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.progress {
  height: 16rpx;
  border-radius: 8rpx;
  overflow: hidden;
  margin-top: 24rpx;
}

.progress-inner {
  height: 16rpx;
  border-radius: 8rpx;
}

.card-foot {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  margin-top: 12rpx;
  min-height: 64rpx;
}

.card-progress-text {
  font-size: 26rpx;
  flex: 1;
}

.btn-remind {
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  padding: 8rpx 32rpx;
}

.btn-remind-text {
  font-size: 26rpx;
}

.loadmore {
  align-items: center;
  padding: 16rpx 0 32rpx;
}

.loadmore-text {
  font-size: 24rpx;
}
</style>
