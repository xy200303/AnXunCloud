<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 离线暂存提示条：队列非空时显示，点击手动触发补传 -->
    <view
      v-if="offlineCount > 0"
      class="offline-bar"
      :style="{ backgroundColor: colors.primaryLight }"
      @click="onOfflineTap"
    >
      <text class="offline-bar-text" :style="{ color: colors.primary }">离线暂存 {{ offlineCount }} 条打卡，点击立即补传</text>
    </view>

    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && tasks.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">今日无巡检任务</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">如需补检请联系主管安排</text>
    </view>

    <!-- 任务列表 -->
    <view v-else-if="loaded" class="content">
      <view class="summary">
        <text class="summary-date" :style="{ color: colors.textSecondary }">{{ date }}</text>
        <text class="summary-progress" :style="{ color: colors.primary }">{{ donePoints }}/{{ totalPoints }}</text>
      </view>

      <view
        v-for="t in tasks"
        :key="t.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="goDetail(t.id)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ t.community_name }} · {{ t.plan_name }}</text>
          <text class="card-tag" :style="{ color: t.status_color }">{{ t.status_text }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">{{ t.time_window }}</text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view
            class="progress-inner"
            :style="{ width: t.progress_width, backgroundColor: t.bar_color }"
          ></view>
        </view>
        <text class="card-progress-text" :style="{ color: colors.textRegular }">{{ t.done_points }}/{{ t.total_points }} 点位</text>
      </view>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <view class="tabbar-space"></view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiTasksToday, TodayTask } from '@/services/api'
import { offlineCount, syncOfflineCheckins } from '@/utils/offline'
import { useMessageStore } from '@/stores/message'

/** 列表项视图模型：模板只做简单属性读取，颜色/文案/宽度在数据层预计算 */
type TaskView = {
  id: string
  plan_name: string
  community_name: string
  time_window: string
  status_text: string
  status_color: string
  /** 进度条宽度，如 '50%' */
  progress_width: string
  /** 进度条颜色（逾期 danger，其余 primary） */
  bar_color: string
  total_points: number
  done_points: number
}

type TodayData = {
  colors: ColorTokens
  loading: boolean
  loaded: boolean
  errorMsg: string
  date: string
  totalPoints: number
  donePoints: number
  tasks: TaskView[]
  /** 离线暂存待补传条数（>0 显示提示条） */
  offlineCount: number
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

function toTaskView(t: TodayTask): TaskView {
  return {
    id: t.id,
    plan_name: t.plan_name,
    community_name: t.community_name,
    time_window: t.time_window,
    status_text: statusTextOf(t.status),
    status_color: statusColorOf(t.status),
    progress_width: `${t.progress}%`,
    bar_color: t.status == 'overdue' ? Colors.danger : Colors.primary,
    total_points: t.total_points,
    done_points: t.done_points
  } as TaskView
}

export default {
  data(): TodayData {
    return {
      colors: Colors,
      loading: true,
      loaded: false,
      errorMsg: '',
      date: '',
      totalPoints: 0,
      donePoints: 0,
      tasks: [] as TaskView[],
      offlineCount: 0
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 打卡返回后刷新进度 + 离线队列计数；有网时自动补传（sync 内部判网/单飞）
    this.offlineCount = offlineCount()
    if (this.loaded) this.load()
    if (this.offlineCount > 0) {
      this.trySync()
    }
    // 刷新消息 tab 未读角标（轻量请求只取 unread_count，失败静默）
    useMessageStore().refresh()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    /** 触发一轮补传并回显结果/剩余数 */
    trySync() {
      syncOfflineCheckins().then((r) => {
        this.offlineCount = r.left
        if (r.done > 0) {
          uni.showToast({ title: '已补传 ' + r.done + ' 条离线打卡', icon: 'none' })
          // 补传成功后刷新任务进度
          this.load()
        }
      })
    },
    /** 点击离线提示条手动补传 */
    onOfflineTap() {
      this.trySync()
    },
    load() {
      this.loading = !this.loaded
      apiTasksToday()
        .then((res) => {
          this.loading = false
          this.loaded = true
          this.date = res.date
          this.totalPoints = res.total_points
          this.donePoints = res.done_points
          const views: TaskView[] = []
          res.tasks.forEach((t: TodayTask) => {
            views.push(toTaskView(t))
          })
          this.tasks = views
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          this.loaded = this.tasks.length > 0 ? this.loaded : false
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: '/pages/tasks/detail?id=' + encodeURIComponent(id) })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.offline-bar {
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
  align-items: center;
}

.offline-bar-text {
  font-size: 26rpx;
}

.skeleton {
  padding-top: 8rpx;
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

.summary {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24rpx;
  padding-left: 8rpx;
  padding-right: 8rpx;
}

.summary-date {
  font-size: 26rpx;
}

.summary-progress {
  font-size: 48rpx; /* FontSize.number */
  font-weight: 600;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
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

.card-tag {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
  margin-bottom: 24rpx;
}

.progress {
  height: 16rpx;
  border-radius: 8rpx;
  overflow: hidden;
}

.progress-inner {
  height: 16rpx;
  border-radius: 8rpx;
}

.card-progress-text {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.tabbar-space {
  height: 160rpx;
}
</style>
