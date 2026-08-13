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

      <!-- 点位状态列表 -->
      <view
        v-for="p in points"
        :key="p.point_id"
        class="card point-row"
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

function checkinTypeTextOf(t: string): string {
  if (t == 'qrcode') return '扫码'
  if (t == 'nfc') return 'NFC'
  if (t == 'offline') return '离线'
  return '围栏'
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
  }
  return {
    point_id: p.point_id,
    sort: p.sort,
    point_name: p.point_name,
    building_name: p.building_name,
    credential_text: credentialTextOf(p.credential),
    status_text: statusText,
    status_color: statusColor,
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
      points: [] as PointView[]
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.id ? String(options.id) : ''
    this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    load() {
      if (this.taskId == '') {
        this.loading = false
        this.errorMsg = '缺少任务参数'
        return
      }
      this.loading = !this.loaded
      apiTaskMonitorDetail(this.taskId)
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
    /** 已打卡点位：弹层展示打卡摘要 */
    onPointTap(p: PointView) {
      const ck = p.checkin
      if (ck == null) return
      const lines = [
        '时间：' + ck.checkin_time,
        '方式：' + checkinTypeTextOf(ck.checkin_type),
        '结果：' + (ck.result == 'abnormal' ? '异常' : '正常')
      ]
      if (ck.distance_to_point != null) {
        lines.push('距点位：' + ck.distance_to_point + ' m')
      }
      if (ck.is_suspect) {
        lines.push('疑似标记：' + (ck.suspect_reason != '' ? ck.suspect_reason : '疑似异常打卡'))
      }
      if (ck.remark != '') {
        lines.push('备注：' + ck.remark)
      }
      uni.showModal({
        title: p.point_name,
        content: lines.join('\n'),
        showCancel: false,
        confirmText: '知道了'
      })
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
