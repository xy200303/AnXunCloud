<template>
  <view class="page bg-page" >
    <!-- 骨架屏 -->
    <view v-if="loading && tasks.length == 0" class="skeleton">
      <view class="sk-block bg-border" ></view>
      <view class="sk-block bg-border" ></view>
      <view class="sk-block sk-short bg-border" ></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded && tasks.length == 0" class="empty">
      <text class="empty-title text-regular" >{{ errorMsg }}</text>
      <text class="empty-retry text-brand"  @click="reload">重试</text>
    </view>

    <view v-else class="content">
      <!-- 统计卡（2x2） -->
      <view class="stats">
        <view class="stat-card bg-card" >
          <text class="stat-num text-brand" >{{ rateText }}</text>
          <text class="stat-label text-secondary" >今日完成率 {{ doneCount }}/{{ totalCount }}</text>
        </view>
        <view class="stat-card bg-card" >
          <text class="stat-num text-brand" >{{ board.doing_tasks }}</text>
          <text class="stat-label text-secondary" >进行中任务</text>
        </view>
        <view class="stat-card bg-card" >
          <text class="stat-num text-danger" >{{ board.overdue_tasks }}</text>
          <text class="stat-label text-secondary" >已逾期任务</text>
        </view>
      </view>

      <AppSegmentTabs :items="tabs" :value="filter" @change="switchFilter" />

      <!-- 空态 -->
      <view v-if="loaded && tasks.length == 0" class="empty">
        <text class="empty-title text-regular" >{{ filter == '' ? '今日暂无巡检任务' : '该筛选下暂无任务' }}</text>
        <text class="empty-sub text-secondary" >下拉可刷新</text>
      </view>

      <!-- 今日任务列表 -->
      <view
        v-for="t in tasks"
        :key="t.id"
        class="card bg-card"
        
        @click="goDetail(t.id)"
      >
        <view class="card-head">
          <text class="card-title text-main" >{{ t.plan_name }}</text>
          <text class="card-status" :style="{ color: t.status_color }">{{ t.status_text }}</text>
        </view>
        <text class="card-sub text-secondary" >
          {{ t.community_name }} · {{ t.inspector_name }}<text v-if="t.time_window != ''"> · {{ t.time_window }}</text>
        </text>
        <view class="progress bg-border" >
          <view class="progress-inner" :style="{ width: t.progress + '%', backgroundColor: t.status_color }"></view>
        </view>
        <view class="card-foot">
          <text class="card-progress-text text-regular" >
            {{ t.done_points }}/{{ t.total_points }} 点位
            <text v-if="t.abnormal_count > 0"  class="text-danger"> · 异常 {{ t.abnormal_count }}</text>
            <text v-if="t.missing_count > 0 && t.status != 'done' && t.status != 'pending'"  class="text-warning"> · 漏 {{ t.missing_count }}</text>
          </text>
          <button
            v-if="t.status != 'done' && t.can_remind !== false"
            plain="true"
            class="btn-remind btn-outline-warning"
            hover-class="hover-dim"
            @click.stop="onRemind(t)"
          >
            <text class="btn-remind-text">催办</text>
          </button>
        </view>
      </view>

      <!-- 加载更多状态 -->
      <uni-load-more
        v-if="tasks.length > 0 && (loadingMore || noMore)"
        :status="loadingMore ? 'loading' : 'noMore'"
        :content-text="{ contentrefresh: '加载中…', contentnomore: '没有更多了' }"
        :color="'#86909C'"
      />
    </view>

    <!-- 催办确认（自绘，替代原生 showModal） -->
    <AppDialog
      :visible="remindDlgShow"
      kind="primary"
      title="任务催办"
      :content="remindTask != null ? '给 ' + remindTask.inspector_name + ' 发送「' + remindTask.plan_name + '」的催办提醒？' : ''"
      confirm-text="催办"
      cancel-text="取消"
      @update:visible="remindDlgShow = $event"
      @confirm="onRemindConfirm"
    />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiAdminDashboard, apiTaskMonitorList, apiTaskRemind, DashboardData, MonitorTask } from '@/services/api'
import AppSegmentTabs from '@/components/AppSegmentTabs.vue'
import AppDialog from '@/components/AppDialog.vue'

const PAGE_SIZE = 20

/** 任务行视图模型：文案/颜色在数据层预计算 */
type TaskView = MonitorTask & {
  status_text: string
  status_color: string
}

type BoardData = {
  board: Pick<DashboardData, 'today_completion' | 'doing_tasks' | 'overdue_tasks'>
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
  /** 催办确认弹窗：待催办任务在打开时暂存 */
  remindDlgShow: boolean
  remindTask: TaskView | null
}

function emptyBoard(): BoardData['board'] {
  return {
    today_completion: { total: 0, done: 0, rate: 0 },
    doing_tasks: 0,
    overdue_tasks: 0
  }
}

function statusTextOf(s: string): string {
  if (s == 'doing') return '进行中'
  if (s == 'done') return '已完成'
  if (s == 'overdue') return '已逾期'
  return '待开始'
}

function statusColorOf(s: string): string {
  if (s == 'doing') return '#2B5AED'
  if (s == 'done') return '#2BA471'
  if (s == 'overdue') return '#D54941'
  return '#ED7B2F'
}

function toTaskView(t: MonitorTask): TaskView {
  return Object.assign({}, t, {
    status_text: statusTextOf(t.status),
    status_color: statusColorOf(t.status)
  })
}

export default {
  components: { AppSegmentTabs, AppDialog },
  data(): BoardData {
    return {
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
      reminding: false,
      remindDlgShow: false,
      remindTask: null
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
            overdue_tasks: d.overdue_tasks
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
      this.remindTask = t
      this.remindDlgShow = true
    },
    onRemindConfirm() {
      const t = this.remindTask
      if (t == null || this.reminding) return
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


</style>
