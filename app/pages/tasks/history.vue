<template>
  <view class="page bg-page" >
    <!-- 日期导航：居中紧凑一组——左右翻日（uni-icons），点日期开系统选择器（最晚到今天） -->
    <view class="date-bar bg-card" >
      <view  hover-class="hover-dim" class="date-arrow bg-page"  @click="shiftDay(-1)">
        <uni-icons type="left" size="20" :color="'#2B5AED'" />
      </view>
      <picker mode="date" :value="date" :end="today" @change="onDatePick">
        <view class="date-mid">
          <text class="date-text text-main" >{{ date }}</text>
          <text class="date-week text-secondary" >{{ weekText }}</text>
        </view>
      </picker>
      <view  hover-class="hover-dim" class="date-arrow"  :class="(canNext ? 'bg-page' : 'bg-border')" @click="shiftDay(1)">
        <uni-icons type="right" size="20" :color="canNext ? '#2B5AED' : '#86909C'" />
      </view>
    </view>

    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block bg-border" ></view>
      <view class="sk-block bg-border" ></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && tasks.length == 0" class="empty">
      <text class="empty-title text-regular" >这天没有任务</text>
      <text class="empty-sub text-secondary" >换个日期看看</text>
    </view>

    <!-- 任务列表 -->
    <view v-else-if="loaded" class="content">
      <view class="summary">
        <text class="summary-progress text-brand" >{{ donePoints }}/{{ totalPoints }} 点位</text>
      </view>
      <!-- 任务卡（§18.2 容器官方化 uni-card + uni-tag 徽标；进度条为业务呈现保留） -->
      <uni-card
        v-for="t in tasks"
        :key="t.id"
        :is-shadow="true"
        :border="false"
        margin="0 0 24rpx 0"
        padding="28rpx 32rpx"
        spacing="0"
        @click="goDetail(t.id)"
      >
        <view class="card-head">
          <text class="card-title text-main" >{{ t.community_name }} · {{ t.plan_name }}</text>
          <uni-tag :text="statusTextOf(t.status)" :inverted="true" size="small" :custom-style="'color:' + statusColorOf(t.status) + ';border-color:' + statusColorOf(t.status)" />
        </view>
        <view class="card-sub-row">
          <uni-tag v-if="t.round_name != ''" :text="t.round_name" :inverted="true" size="small" :custom-style="'color:#ED7B2F;border-color:#ED7B2F;margin-right:16rpx'" />
          <uni-tag v-if="patrolLabelOf(t) != ''" :text="patrolLabelOf(t)" :inverted="true" size="small" :custom-style="'color:#2B5AED;border-color:#2B5AED;margin-right:16rpx'" />
          <text class="card-sub text-secondary" >{{ t.time_window != '' ? t.time_window : (t.round_name != '' ? '不限时段' : '') }}</text>
        </view>
        <view class="progress bg-border" >
          <view class="progress-inner"  :class="(t.status == 'overdue' ? 'bg-danger' : 'bg-brand')":style="{ width: t.progress + '%' }"></view>
        </view>
        <text class="card-progress-text text-regular" >{{ t.done_points }}/{{ t.total_points }} 点位</text>
        <text v-if="t.status == 'overdue' && t.done_points < t.total_points" class="makeup-tip text-warning" >逾期未完成，可进入补拍</text>
      </uni-card>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title text-regular" >{{ errorMsg }}</text>
      <text class="empty-retry text-brand"  @click="load">重试</text>
    </view>
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiTasksHistory, TodayTask } from '@/services/api'

/** 巡查类型文案（内置回落：后端未透传 patrol_type_label 时使用） */
function patrolTextOf(t: string): string {
  if (t == 'safety') return '安全巡查'
  if (t == 'equipment') return '设备专项'
  if (t == 'environment') return '环境巡查'
  if (t == 'building') return '楼栋巡查'
  return ''
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

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n
}

/** Date → YYYY-MM-DD（本地时区） */
function fmtDate(d: Date): string {
  return d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate())
}

const WEEK_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

type HistoryData = {
  /** 当前查看日期 YYYY-MM-DD（默认昨天，最晚今天） */
  date: string
  today: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  totalPoints: number
  donePoints: number
  tasks: TodayTask[]
}

export default {
  data(): HistoryData {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 86400000)
    return {
      date: fmtDate(yesterday),
      today: fmtDate(now),
      loading: true,
      loaded: false,
      errorMsg: '',
      totalPoints: 0,
      donePoints: 0,
      tasks: [] as TodayTask[]
    }
  },
  computed: {
    /** 已到「今天」则不能再往后翻 */
    canNext(): boolean {
      return this.date < this.today
    },
    weekText(): string {
      const d = new Date(this.date.replace(/-/g, '/') + ' 00:00:00')
      if (isNaN(d.getTime())) return ''
      return WEEK_NAMES[d.getDay()]
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 打卡/补拍返回后刷新进度与点位状态（避免「需要手动刷新才显示」）
    if (this.loaded) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    statusTextOf,
    statusColorOf,
    patrolLabelOf(t: TodayTask): string {
      return t.patrol_type_label != '' ? t.patrol_type_label : patrolTextOf(t.patrol_type)
    },
    /** 左右翻日：不超过今天 */
    shiftDay(delta: number) {
      const d = new Date(this.date.replace(/-/g, '/') + ' 00:00:00')
      if (isNaN(d.getTime())) return
      const next = fmtDate(new Date(d.getTime() + delta * 86400000))
      if (next > this.today) return
      this.date = next
      this.load()
    },
    onDatePick(e: any) {
      const v = e != null && e.detail != null ? String(e.detail.value) : ''
      if (v == '' || v == this.date) return
      this.date = v
      this.load()
    },
    load() {
      this.loading = !this.loaded
      apiTasksHistory(this.date)
        .then((res) => {
          this.loading = false
          this.loaded = true
          this.totalPoints = res.total_points
          this.donePoints = res.done_points
          this.tasks = res.tasks
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
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

/* 日期导航条：左右箭头圆钮 + 中间日期，整体居中成组 */
.date-bar {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  border-radius: 24rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
}

.date-arrow {
  width: 64rpx;
  height: 64rpx;
  border-radius: 32rpx;
  align-items: center;
  justify-content: center;
}


.date-mid {
  align-items: center;
  padding: 0 48rpx;
}

.date-text {
  font-size: 34rpx;
  font-weight: 600;
  text-align: center;
}

.date-week {
  font-size: 24rpx;
  margin-top: 2rpx;
  text-align: center;
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

.empty {
  padding-top: 200rpx;
  align-items: center;
}

.empty-title {
  font-size: 32rpx;
}

.empty-sub {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.empty-retry {
  font-size: 28rpx;
  margin-top: 24rpx;
  padding: 8rpx 32rpx;
}

.summary {
  display: flex;
  flex-direction: row;
  justify-content: flex-end;
  margin: 4rpx 8rpx 20rpx;
}

.summary-progress {
  font-size: 26rpx;
  font-weight: 600;
}


.card-head {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 32rpx;
  font-weight: 600;
  flex: 1;
  margin-right: 16rpx;
}


.card-sub-row {
  display: flex;
  flex-direction: row;
  align-items: center;
  margin-top: 12rpx;
}


.card-sub {
  font-size: 26rpx;
}

.progress {
  height: 12rpx;
  border-radius: 6rpx;
  margin-top: 20rpx;
  overflow: hidden;
}

.progress-inner {
  height: 12rpx;
  border-radius: 6rpx;
}

.card-progress-text {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.makeup-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}
</style>
