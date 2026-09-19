<template>
  <view class="page bg-page" >
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block bg-border" ></view>
      <view class="sk-block bg-border" ></view>
      <view class="sk-block sk-short bg-border" ></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && points.length == 0" class="empty">
      <text class="empty-title text-regular" >该任务暂无点位</text>
      <text class="empty-sub text-secondary" >请联系主管检查计划路线</text>
    </view>

    <!-- 任务明细 -->
    <view v-else-if="loaded" class="content">
      <!-- 任务信息卡 -->
      <view class="card bg-card" >
        <view class="card-head">
          <text class="card-title text-main" >{{ planName }}</text>
          <uni-tag :text="statusText" :inverted="true" size="small" :custom-style="'color:' + statusColor + ';border-color:' + statusColor" />
        </view>
        <text class="card-sub text-secondary" >{{ communityName }} · {{ inspectorName }}</text>
        <text class="card-sub text-secondary" >{{ taskDate }}<text v-if="timeWindow != ''"> · {{ timeWindow }}</text></text>
        <view class="progress bg-border" >
          <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: statusColor }"></view>
        </view>
        <text class="card-progress-text text-regular" >{{ donePoints }}/{{ totalPoints }} 点位</text>
      </view>

      <!-- 状态过滤条（数字来自全量统计，不随分页变化） -->
      <!-- 状态过滤条（数字来自全量统计，不随分页变化；官方 uni-tag，计数告警色语义保留） -->
      <view class="filter-bar">
        <uni-tag
          v-for="f in filters"
          :key="f.key"
          :text="f.count > 0 ? f.label + ' ' + f.count : f.label"
          :inverted="true"
          :custom-style="filterChipStyle(f)"
          @click="filterKey = f.key"
        />
      </view>

      <!-- 点位状态列表 -->
      <view
        v-for="p in filteredPoints"
        :key="p.point_id"
        class="card point-row bg-card"
        hover-class="hover-dim"
        
        @click="onPointTap(p)"
      >
        <view class="point-main">
          <text class="point-sort text-white" :style="{ backgroundColor: p.status_color }">{{ p.sort }}</text>
          <view class="point-texts">
            <text class="point-name text-main" >{{ p.point_name }}</text>
            <text class="point-building text-secondary" >{{ p.building_name || '未分区' }}</text>
          </view>
        </view>
        <view class="point-side">
          <text class="point-cred text-secondary" >{{ p.credential_text }}</text>
          <text class="point-status" :style="{ color: p.status_color }">{{ p.status_text }}</text>
        </view>
      </view>

      <!-- 过滤后为空 / 加载更多 -->
      <view v-if="filteredPoints.length == 0" class="empty">
        <text class="empty-title text-secondary" >这类点位一个都没有</text>
      </view>
      <uni-load-more
        v-else-if="loadingMore || points.length < pointsTotal || pointsTotal > 0"
        :status="loadingMore ? 'loading' : points.length < pointsTotal ? 'more' : 'noMore'"
        :show-icon="false"
        :content-text="{ contentdown: '上滑加载更多（' + points.length + '/' + pointsTotal + '）', contentrefresh: '加载中…', contentnomore: '全部 ' + pointsTotal + ' 个点位都在这了' }"
        :color="'#86909C'"
      />
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
  if (s == 'doing') return '#2B5AED'
  if (s == 'done') return '#2BA471'
  if (s == 'overdue') return '#D54941'
  return '#ED7B2F'
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
  let statusColor = '#909399'
  if (ck != null) {
    if (ck.result == 'abnormal') {
      statusText = '异常'
      statusColor = '#D54941'
    } else if (ck.is_suspect) {
      statusText = '疑似作弊'
      statusColor = '#ED7B2F'
    } else {
      statusText = '已打卡'
      statusColor = '#2BA471'
    }
  } else if (p.status == 'doing') {
    statusText = '巡检中'
    statusColor = '#2B5AED'
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
    /** 筛选 chip 样式字符串（uni-tag customStyle 为 String 类型；选中实心品牌色，未选按异常级别给计数色） */
    filterChipStyle(f: { key: string; label: string; count: number }): string {
      if (this.filterKey == f.key) return 'background-color:#2B5AED;border-color:#2B5AED;color:#FFFFFF;margin-right:20rpx'
      const c = f.key == 'abnormal' && f.count > 0 ? '#D54941' : f.key == 'suspect' && f.count > 0 ? '#ED7B2F' : '#4E5969'
      return 'background-color:#FFFFFF;border-color:#E5E6EB;color:' + c + ';margin-right:20rpx'
    },
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
