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
      <text class="empty-title" :style="{ color: colors.textRegular }">这个任务还没配点位</text>
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
        <view class="card-sub-row">
          <text v-if="patrolText != ''" class="type-tag" :style="{ color: colors.primary, borderColor: colors.primary }">{{ patrolText }}</text>
          <text class="card-sub" :style="{ color: colors.textSecondary }">{{ communityName }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">{{ taskDate }} · {{ roundName != '' ? roundName + ' ' : '' }}{{ timeWindow }}</text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: colors.primary }"></view>
        </view>
        <text class="card-progress-text" :style="{ color: colors.textPrimary }">已完成 {{ donePoints }} / {{ totalPoints }} 点位</text>
      </view>

      <!-- 连续巡检入口（AI 启用进向导；未启用进手动表单；有本地快照时显示继续巡检） -->
      <view v-if="hasUnchecked"  hover-class="hover-dim" class="quick-btn" :style="{ backgroundColor: colors.success }" @click="startQuick">
        <text  hover-class="hover-dim" class="quick-btn-text" :style="{ color: colors.white }">{{ quickBtnText }}</text>
      </view>

      <!-- 点位列表（未打卡置顶，已打卡沉底；组内保持 sort 顺序；分块渲染，触底加载更多） -->
      <view
        v-for="p in visiblePoints"
        :key="p.point_id"
         hover-class="hover-dim" class="card point-row"
        :style="{ backgroundColor: colors.bgCard }"
        @click="onPointTap(p)"
      >
        <view class="point-main">
          <text class="point-sort" :style="{ color: colors.white, backgroundColor: colors.primary }">{{ p.sort }}</text>
          <view class="point-texts">
            <text class="point-name" :style="{ color: colors.textPrimary }">{{ p.point_name }}</text>
            <text class="point-building" :style="{ color: colors.textSecondary }">{{ p.building_name || '未分区' }}</text>
          </view>
        </view>
        <view class="point-side">
          <text class="point-cred" :style="{ color: colors.textSecondary }">{{ p.credential_text }}</text>
          <view class="point-status-row">
            <view class="point-dot" :style="{ backgroundColor: p.status_color }"></view>
            <text class="point-status" :style="{ color: p.status_color }">{{ p.status_text }}</text>
          </view>
        </view>
      </view>

      <!-- 手动模式入口已移至向导内「手动填写本点位」（按点位进入，不再从首项开始） -->
      <view v-if="visiblePoints.length < sortedPoints.length" class="load-more">
        <text class="load-more-text" :style="{ color: colors.textSecondary }">上拉加载更多（{{ visiblePoints.length }}/{{ sortedPoints.length }}）</text>
      </view>
      <view class="bottom-space"></view>
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
import { apiTaskDetail, apiItemDrafts, ItemDraft, TaskPoint } from '@/services/api'

/** 点位行视图模型：文案/颜色在数据层预计算 */
type PointView = {
  point_id: string
  sort: number
  point_name: string
  building_name: string
  /** 凭证标签文案：二维码/NFC/无凭证 */
  credential_text: string
  status_text: string
  status_color: string
  checked: boolean
  /** 已打卡且已归档锁定（不可修改） */
  locked: boolean
  my_checkin: TaskPoint['my_checkin']
}

type DetailData = {
  colors: ColorTokens
  taskId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  planName: string
  communityName: string
  /** 巡查类型中文标签（空 = 不展示） */
  patrolText: string
  /** 巡更轮次名（任务快照；非轮次任务为空串，头部回落「日期 · 时间窗」原样） */
  roundName: string
  taskDate: string
  timeWindow: string
  statusText: string
  statusColor: string
  progressWidth: string
  totalPoints: number
  donePoints: number
  /** AI 识别是否启用（false = 大按钮与点位打卡均回退手动表单） */
  aiEnabled: boolean
  /** 云端逐项草稿（有 = 显示「继续巡检」并标注 AI 检查中/巡检中点位） */
  drafts: ItemDraft[]
  points: PointView[]
  /** 分块渲染：已挂载到列表的点位数（触底递增，全量数据仍在内存供排序/续巡） */
  visibleCount: number
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

/** 巡查类型文案（内置回落：后端未透传 patrol_type_label 时使用；新类型如 fire 以字典 label 为准） */
function patrolTextOf(t: string): string {
  if (t == 'safety') return '安全巡查'
  if (t == 'equipment') return '设备专项'
  if (t == 'environment') return '环境巡查'
  if (t == 'building') return '楼栋巡查'
  return ''
}

/** 点位分块渲染块大小（触底每次再挂载一块） */
const POINTS_CHUNK = 50

/** 云端草稿点位进度档：recognizing=有项 AI 识别中；doing=已拍/已答未提交；''=未开始（不标注） */
type SnapStage = 'recognizing' | 'doing' | ''

/** 点位项级进度（从云端草稿派生）：stage 进度档 + done 已有结论项数 */
type DraftStat = { stage: SnapStage; done: number }

function toPointView(p: TaskPoint, draftStats: Record<string, DraftStat>): PointView {
  const ck = p.my_checkin
  let statusText = '待打卡'
  let statusColor = Colors.info
  if (ck != null) {
    if (ck.result == 'abnormal') {
      statusText = '异常'
      statusColor = Colors.danger
    } else {
      statusText = '正常'
      statusColor = Colors.success
    }
  } else {
    const stat = draftStats[p.point_id]
    const total = (p.check_items || []).length
    if (stat != null && stat.stage == 'recognizing') {
      // 有照片在 AI 识别队列中
      statusText = total > 0 ? 'AI 检查中 ' + stat.done + '/' + total + ' 项' : 'AI 检查中'
      statusColor = Colors.primary
    } else if (stat != null && stat.stage == 'doing') {
      // 已拍/已答但无识别中项（待收尾提交）
      statusText = total > 0 ? '巡检中 ' + stat.done + '/' + total + ' 项' : '巡检中'
      statusColor = Colors.warning
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
    checked: ck != null,
    locked: ck != null && ck.locked,
    my_checkin: ck
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
      patrolText: '',
      roundName: '',
      taskDate: '',
      timeWindow: '',
      statusText: '',
      statusColor: '',
      progressWidth: '0%',
      totalPoints: 0,
      donePoints: 0,
      aiEnabled: false,
      drafts: [],
      points: [] as PointView[],
      visibleCount: POINTS_CHUNK
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.id ? String(options.id) : ''
    this.load()
  },
  onShow() {
    // 打卡返回后刷新点位状态
    if (this.loaded) this.load()
  },
  onReachBottom() {
    // 分块渲染：触底再挂下一块（3900+ 点位任务避免首屏整屏节点）
    if (this.visibleCount < this.sortedPoints.length) this.visibleCount += POINTS_CHUNK
  },
  onPullDownRefresh() {
    this.load()
  },
  computed: {
    /** 未打卡置顶、已打卡沉底（组内保持 sort 升序；sort 相同按原顺序稳定排列） */
    sortedPoints(): PointView[] {
      return this.points
        .map((p, i) => ({ p: p, i: i }))
        .sort((a, b) => {
          if (a.p.checked != b.p.checked) return a.p.checked ? 1 : -1
          if (a.p.sort != b.p.sort) return a.p.sort - b.p.sort
          return a.i - b.i
        })
        .map((x) => x.p)
    },
    /** 分块渲染：当前已挂载的点位块 */
    visiblePoints(): PointView[] {
      return this.sortedPoints.slice(0, this.visibleCount)
    },
    /** 存在未打卡点位 → 显示大按钮 */
    hasUnchecked(): boolean {
      return this.points.some((p) => !p.checked)
    },
    /** 有进行中的逐项进度（云端草稿或已打卡点位）→ 大按钮显示「继续巡检」 */
    hasProgress(): boolean {
      return this.drafts.length > 0 || this.donePoints > 0
    },
    /** 大按钮文案：继续巡检（带进度）/ 开始连续巡检 / 开始巡检（手动） */
    quickBtnText(): string {
      if (!this.aiEnabled) return '开始巡检'
      if (this.hasProgress) {
        return '继续巡检（已完成 ' + this.donePoints + '/' + this.totalPoints + ' 点位）'
      }
      return '开始连续巡检'
    }
  },
  methods: {
    /** 大按钮入口：AI 启用 → 连续巡检向导（从云端草稿恢复进度）；未启用 → 手动表单 */
    startQuick() {
      if (!this.aiEnabled) {
        this.goManual()
        return
      }
      uni.navigateTo({
        url: '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId)
      })
    },
    /** 手动模式（熟手后路 / AI 未启用）：跳第一个未打卡点位的打卡表单 */
    goManual() {
      const next = this.sortedPoints.find((p) => !p.checked)
      if (next == null) return
      uni.navigateTo({
        url:
          '/pages/checkin/form?task_id=' + encodeURIComponent(this.taskId) +
          '&point_id=' + encodeURIComponent(next.point_id)
      })
    },
    load() {
      if (this.taskId == '') {
        this.loading = false
        this.errorMsg = '缺少任务参数'
        return
      }
      this.loading = !this.loaded
      // 云端逐项草稿：按点位进度分档标注（识别中/巡检中）+ 大按钮续巡进度；草稿失败不阻断任务详情
      Promise.all([apiTaskDetail(this.taskId), apiItemDrafts(this.taskId).catch(() => [] as ItemDraft[])])
        .then(([res, drafts]) => {
          this.loading = false
          this.loaded = true
          this.planName = res.plan_name
          this.communityName = res.community_name
          this.patrolText = res.patrol_type_label != '' ? res.patrol_type_label : patrolTextOf(res.patrol_type)
          this.roundName = res.round_name
          this.taskDate = res.task_date
          this.timeWindow = res.time_window
          this.statusText = statusTextOf(res.status)
          this.statusColor = statusColorOf(res.status)
          this.progressWidth = res.progress + '%'
          this.totalPoints = res.total_points
          this.donePoints = res.done_points
          this.aiEnabled = res.ai_enabled ?? false
          this.drafts = drafts
          const draftStats: Record<string, DraftStat> = {}
          drafts.forEach((d) => {
            let stat = draftStats[d.point_id]
            if (stat == null) {
              stat = { stage: '', done: 0 }
              draftStats[d.point_id] = stat
            }
            if (d.ai_status == 'pending') {
              stat.stage = 'recognizing'
            } else {
              if (stat.stage == '') stat.stage = 'doing'
              if (d.ai_status == 'done') stat.done += 1 // done 含手动项（手动行 ai_status=done）
            }
          })
          this.points = res.points.map((p: TaskPoint) => toPointView(p, draftStats))
          this.visibleCount = POINTS_CHUNK
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    onPointTap(p: PointView) {
      if (!p.checked) {
        if (this.aiEnabled) {
          // 未打卡点位：直接进向导并从该点位开始（之后按顺序继续余下点位）
          uni.navigateTo({
            url:
              '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
              '&point_id=' + encodeURIComponent(p.point_id)
          })
          return
        }
        // AI 未启用：进该点位的手动打卡表单
        uni.navigateTo({
          url:
            '/pages/checkin/form?task_id=' + encodeURIComponent(this.taskId) +
            '&point_id=' + encodeURIComponent(p.point_id)
        })
        return
      }
      // 已打卡点位（含已归档）：进记录卡（先看后改；可改性由记录卡按锁定/任务状态判定）
      uni.navigateTo({
        url:
          '/pages/checkin/record?task_id=' + encodeURIComponent(this.taskId) +
          '&point_id=' + encodeURIComponent(p.point_id)
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

.card-sub-row {
  flex-direction: row;
  align-items: center;
  margin-top: 8rpx;
}

.card-sub-row .card-sub {
  margin-top: 0;
}

.type-tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
  margin-right: 16rpx;
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
  font-size: 32rpx;
  font-weight: 600;
  margin-top: 12rpx;
}

.quick-btn {
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 24rpx;
}

.quick-btn-text {
  font-size: 44rpx;
  font-weight: 700;
}

.load-more {
  text-align: center;
  padding: 20rpx 0;
}

.load-more-text {
  font-size: 24rpx;
}

.bottom-space {
  height: 64rpx;
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

.point-status-row {
  flex-direction: row;
  align-items: center;
  margin-top: 8rpx;
}

.point-dot {
  width: 20rpx;
  height: 20rpx;
  border-radius: 10rpx;
  margin-right: 10rpx;
}

.point-status {
  font-size: 32rpx;
  font-weight: 600;
}
</style>
