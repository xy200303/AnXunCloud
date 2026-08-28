<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && points.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">该任务暂无点位</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">请联系主管检查计划路线</text>
    </view>

    <!-- 任务明细 -->
    <view v-else-if="loaded" class="content">
      <!-- 任务信息卡 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ planName }}</text>
          <text class="card-tag" :style="{ color: statusColor }">{{ statusText }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">{{ communityName }} · {{ inspectorName }}</text>
        <text class="card-sub" :style="{ color: colors.textSecondary }">{{ taskDate }}<text v-if="timeWindow != ''"> · {{ timeWindow }}</text></text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: statusColor }"></view>
        </view>
        <text class="card-progress-text" :style="{ color: colors.textRegular }">{{ donePoints }}/{{ totalPoints }} 点位</text>
      </view>

      <!-- 状态过滤条（数字来自全量统计，不随分页变化） -->
      <view class="filter-bar">
        <view
          v-for="f in filters"
          :key="f.key"
          class="filter-chip"
          hover-class="hover-dim"
          :style="{
            backgroundColor: filterKey == f.key ? colors.primary : colors.bgCard,
            borderColor: filterKey == f.key ? colors.primary : colors.border
          }"
          @click="filterKey = f.key"
        >
          <text
            class="filter-chip-text"
            :style="{ color: filterKey == f.key ? colors.white : (f.key == 'abnormal' && f.count > 0 ? colors.danger : (f.key == 'suspect' && f.count > 0 ? colors.warning : colors.textRegular)) }"
          >{{ f.label }}<text v-if="f.count > 0"> {{ f.count }}</text></text>
        </view>
      </view>

      <!-- 点位状态列表 -->
      <view
        v-for="p in filteredPoints"
        :key="p.point_id"
        class="card point-row"
        hover-class="hover-dim"
        :style="{ backgroundColor: colors.bgCard }"
        @click="onPointTap(p)"
      >
        <view class="point-main">
          <text class="point-sort" :style="{ color: colors.white, backgroundColor: p.status_color }">{{ p.sort }}</text>
          <view class="point-texts">
            <text class="point-name" :style="{ color: colors.textPrimary }">{{ p.point_name }}</text>
            <text class="point-building" :style="{ color: colors.textSecondary }">{{ p.building_name || '未分区' }}</text>
          </view>
        </view>
        <view class="point-side">
          <text class="point-cred" :style="{ color: colors.textSecondary }">{{ p.credential_text }}</text>
          <text class="point-status" :style="{ color: p.status_color }">{{ p.status_text }}</text>
        </view>
      </view>

      <!-- 过滤后为空 / 加载更多 -->
      <view v-if="filteredPoints.length == 0" class="empty">
        <text class="empty-title" :style="{ color: colors.textSecondary }">这类点位一个都没有</text>
      </view>
      <text v-else-if="loadingMore" class="more-text" :style="{ color: colors.textSecondary }">加载中…</text>
      <text v-else-if="points.length < pointsTotal" class="more-text" :style="{ color: colors.textSecondary }">上滑加载更多（{{ points.length }}/{{ pointsTotal }}）</text>
      <text v-else-if="pointsTotal > 0" class="more-text" :style="{ color: colors.textSecondary }">全部 {{ pointsTotal }} 个点位都在这了</text>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiTaskMonitorDetail, MonitorTaskPoint } from '@/services/api'

/** 点位行视图模型：文案/颜色在数据层预计算 */
type PointView = {
  point_id: string
  sort: number
  point_name: string
  building_name: string
  credential_text: string
  status_text: string
  status_color: string
  /** 后端点位状态原值：pending/doing/done */
  raw_status: string
  checkin: MonitorTaskPoint['checkin']
}

type DetailData = {
  colors: ColorTokens
  taskId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  planName: string
  communityName: string
  inspectorName: string
  taskDate: string
  timeWindow: string
  statusText: string
  statusColor: string
  progressWidth: string
  totalPoints: number
  donePoints: number
  points: PointView[]
  stats: { total: number; done: number; doing: number; pending: number; normal: number; abnormal: number; suspect: number }
  filterKey: string
  pointsPage: number
  pointsTotal: number
  loadingMore: boolean
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

function credentialTextOf(c: string): string {
  if (c == 'qrcode') return '二维码'
  if (c == 'nfc') return 'NFC'
  if (c == 'any') return '二维码/NFC'
  return '无凭证'
}

function toPointView(p: MonitorTaskPoint): PointView {
  const ck = p.checkin
  let statusText = '待打卡'
  let statusColor = Colors.info
  if (ck != null) {
    if (ck.result == 'abnormal') {
      statusText = '异常'
      statusColor = Colors.danger
    } else if (ck.is_suspect) {
      statusText = '疑似作弊'
      statusColor = Colors.warning
    } else {
      statusText = '已打卡'
      statusColor = Colors.success
    }
  } else if (p.status == 'doing') {
    statusText = '巡检中'
    statusColor = Colors.primary
  }
  return {
    point_id: p.point_id,
    sort: p.sort,
    point_name: p.point_name,
    building_name: p.building_name,
    credential_text: credentialTextOf(p.credential),
    status_text: statusText,
    status_color: statusColor,
    raw_status: p.status,
    checkin: ck
  }
}

export default {
  data(): DetailData {
    return {
      colors: Colors,
      taskId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      planName: '',
      communityName: '',
      inspectorName: '',
      taskDate: '',
      timeWindow: '',
      statusText: '',
      statusColor: '',
      progressWidth: '0%',
      totalPoints: 0,
      donePoints: 0,
      points: [] as PointView[],
      stats: { total: 0, done: 0, doing: 0, pending: 0, normal: 0, abnormal: 0, suspect: 0 },
      filterKey: '',
      pointsPage: 0,
      pointsTotal: 0,
      loadingMore: false
    }
  },
  computed: {
    /** 状态过滤条（计数取自全量聚合 stats，不随分页变化） */
    filters(): Array<{ key: string; label: string; count: number }> {
      return [
        { key: '', label: '全部', count: this.stats.total },
        { key: 'abnormal', label: '异常', count: this.stats.abnormal },
        { key: 'suspect', label: '疑似', count: this.stats.suspect },
        { key: 'doing', label: '进行中', count: this.stats.doing },
        { key: 'pending', label: '未巡', count: this.stats.pending }
      ]
    },
    /** 过滤在已加载点位上做（统计数字是全量的；过滤为空时可上滑继续加载） */
    filteredPoints(): PointView[] {
      if (this.filterKey == '') return this.points
      return this.points.filter((p) => {
        if (this.filterKey == 'abnormal') return p.checkin != null && p.checkin.result == 'abnormal'
        if (this.filterKey == 'suspect') return p.checkin != null && p.checkin.is_suspect
        return p.raw_status == this.filterKey // doing / pending
      })
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.id ? String(options.id) : ''
    this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    load() {
      if (this.taskId == '') {
        this.loading = false
        this.errorMsg = '缺少任务参数'
        return
      }
      this.loading = !this.loaded
      apiTaskMonitorDetail(this.taskId, 1)
        .then((res) => {
          const t = res.task
          this.loading = false
          this.loaded = true
          this.planName = t.plan_name
          this.communityName = t.community_name
          this.inspectorName = t.inspector_name
          this.taskDate = t.task_date
          this.timeWindow = t.time_window
          this.statusText = statusTextOf(t.status)
          this.statusColor = statusColorOf(t.status)
          this.progressWidth = t.progress + '%'
          this.totalPoints = t.total_points
          this.donePoints = t.done_points
          this.stats = res.stats
          this.pointsTotal = res.points_total
          this.pointsPage = res.points_page
          this.points = res.points.map((p: MonitorTaskPoint) => toPointView(p))
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 点位分页加载（单任务可达数百点位，服务端按 points_page 分页） */
    loadMore() {
      if (this.loadingMore || this.loading) return
      if (this.points.length >= this.pointsTotal) return
      this.loadingMore = true
      apiTaskMonitorDetail(this.taskId, this.pointsPage + 1)
        .then((res) => {
          this.pointsPage = res.points_page
          this.points = this.points.concat(res.points.map((p: MonitorTaskPoint) => toPointView(p)))
          this.loadingMore = false
        })
        .catch((e: Error) => {
          this.loadingMore = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 已打卡点位：进打卡明细页（逐项照片 + AI 结论 + 备注，与审核页同数据源） */
    onPointTap(p: PointView) {
      const ck = p.checkin
      if (ck == null) {
        uni.showToast({ title: p.raw_status == 'doing' ? '正在巡检中，还没提交' : '还没巡到这里', icon: 'none' })
        return
      }
      uni.navigateTo({ url: '/pages/admin/checkin-detail?id=' + encodeURIComponent(ck.id) })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
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

.card-progress-text {
  font-size: 26rpx;
  margin-top: 12rpx;
}

/* 状态过滤条 */
.filter-bar {
  flex-direction: row;
  flex-wrap: wrap;
  margin-bottom: 20rpx;
}

.filter-chip {
  border-radius: 999rpx;
  border-width: 1rpx;
  border-style: solid;
  padding: 10rpx 26rpx;
  margin-right: 16rpx;
  margin-bottom: 12rpx;
}

.filter-chip-text {
  font-size: 26rpx;
}

.more-text {
  font-size: 24rpx;
  text-align: center;
  padding: 24rpx 0 32rpx;
}

.point-row {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.point-main {
  flex-direction: row;
  align-items: center;
  flex: 1;
}

.point-sort {
  width: 48rpx;
  height: 48rpx;
  border-radius: 24rpx;
  font-size: 26rpx;
  text-align: center;
  line-height: 48rpx;
  margin-right: 24rpx;
}

.point-texts {
  flex: 1;
}

.point-name {
  font-size: 32rpx;
  font-weight: 600;
}

.point-building {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.point-side {
  align-items: flex-end;
  margin-left: 24rpx;
}

.point-cred {
  font-size: 24rpx;
}

.point-status {
  font-size: 26rpx;
  margin-top: 8rpx;
}
</style>
