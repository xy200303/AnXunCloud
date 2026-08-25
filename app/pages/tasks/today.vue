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
      <!-- 巡查类型筛选（客户端过滤：全部 / 安全 / 设备专项 / 环境 / 楼栋） -->
      <scroll-view scroll-x class="chips" :show-scrollbar="false">
        <view class="chips-inner">
          <view
            v-for="c in typeChips"
            :key="c.value"
            class="chip"
            :style="typeFilter == c.value
              ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
              : { backgroundColor: colors.bgCard, borderColor: colors.border }"
            @click="typeFilter = c.value"
          >
            <text
              class="chip-text"
              :style="{ color: typeFilter == c.value ? colors.primary : colors.textSecondary }"
            >{{ c.label }}</text>
          </view>
        </view>
      </scroll-view>

      <view class="summary">
        <text class="summary-date" :style="{ color: colors.textSecondary }">{{ date }}</text>
        <text class="summary-progress" :style="{ color: colors.primary }">{{ donePoints }}/{{ totalPoints }}</text>
      </view>

      <view v-if="filteredTasks.length == 0" class="empty-filter">
        <text class="empty-filter-text" :style="{ color: colors.textSecondary }">该类型今日暂无任务</text>
      </view>

      <!-- 轮次任务按「窗口开始时刻」分组（带组头）；非轮次任务为平铺列表（展示不变） -->
      <view v-for="row in displayRows" :key="row.key">
        <text v-if="row.task == null" class="group-title" :style="{ color: colors.textSecondary }">{{ row.header }}</text>
        <view
          v-else
          class="card"
          :style="{ backgroundColor: colors.bgCard }"
          @click="goDetail(row.task.id)"
        >
          <view class="card-head">
            <text class="card-title" :style="{ color: colors.textPrimary }">{{ row.task.community_name }} · {{ row.task.plan_name }}</text>
            <text class="card-tag" :style="{ color: row.task.status_color }">{{ row.task.status_text }}</text>
          </view>
          <view class="card-sub-row">
            <text v-if="row.task.round_name != ''" class="type-tag" :style="{ color: colors.warning, borderColor: colors.warning }">{{ row.task.round_name }}</text>
            <text v-if="row.task.patrol_text != ''" class="type-tag" :style="{ color: colors.primary, borderColor: colors.primary }">{{ row.task.patrol_text }}</text>
            <text class="card-sub" :style="{ color: colors.textSecondary }">{{ row.task.time_window != '' ? row.task.time_window : (row.task.round_name != '' ? '不限时段' : '') }}</text>
          </view>
          <view class="progress" :style="{ backgroundColor: colors.border }">
            <view
              class="progress-inner"
              :style="{ width: row.task.progress_width, backgroundColor: row.task.bar_color }"
            ></view>
          </view>
          <text class="card-progress-text" :style="{ color: colors.textRegular }">{{ row.task.done_points }}/{{ row.task.total_points }} 点位</text>
        </view>
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

/** 巡查类型文案（内置回落：后端未透传 patrol_type_label 时使用；新类型如 fire 以字典 label 为准） */
function patrolTextOf(t: string): string {
  if (t == 'safety') return '安全巡查'
  if (t == 'equipment') return '设备专项'
  if (t == 'environment') return '环境巡查'
  if (t == 'building') return '楼栋巡查'
  return ''
}

/** 巡查类型标签：后端字典 label 优先，空回落内置映射 */
function patrolLabelOf(t: TodayTask): string {
  return t.patrol_type_label != '' ? t.patrol_type_label : patrolTextOf(t.patrol_type)
}

/** 窗口开始时刻（分钟数，用于轮次分组排序；无法解析/空窗口排最后） */
function windowStartOf(w: string): number {
  const m = (w || '').match(/^(\d{1,2}):(\d{2})/)
  if (m == null) return 25 * 60
  return parseInt(m[1], 10) * 60 + parseInt(m[2], 10)
}

/** 列表行：分组组头（task=null）或任务卡 */
type TaskRow = {
  key: string
  /** 组头文案（窗口时段）；task 行为空串 */
  header: string
  task: TaskView | null
}

/** 列表项视图模型：模板只做简单属性读取，颜色/文案/宽度在数据层预计算 */
type TaskView = {
  id: string
  plan_name: string
  community_name: string
  /** 巡查类型原始值（safety/equipment/environment/building/fire…），筛选用 */
  patrol_type: string
  /** 巡查类型中文标签（空 = 不展示标签） */
  patrol_text: string
  /** 巡更轮次名（非轮次任务为空串） */
  round_name: string
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
  /** 巡查类型筛选（'' = 全部） */
  typeFilter: string
  typeChips: Array<{ label: string; value: string }>
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
    patrol_type: t.patrol_type,
    patrol_text: patrolLabelOf(t),
    round_name: t.round_name,
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
      typeFilter: '',
      // 类型筛选 chip：「全部」+ 按当日任务实际类型动态生成（字典新类型如 fire 自动出现）
      typeChips: [{ label: '全部', value: '' }] as Array<{ label: string; value: string }>,
      offlineCount: 0
    }
  },
  computed: {
    /** 按巡查类型过滤后的任务列表 */
    filteredTasks(): TaskView[] {
      if (this.typeFilter == '') return this.tasks
      return this.tasks.filter((t) => t.patrol_type == this.typeFilter)
    },
    /** 是否含轮次任务（含则列表按窗口分组显示组头） */
    hasRounds(): boolean {
      return this.tasks.some((t) => t.round_name != '')
    },
    /** 渲染行：轮次场景按「窗口开始时刻」分组排序并插组头；非轮次平铺（展示不变） */
    displayRows(): TaskRow[] {
      const list = this.filteredTasks
      if (!this.hasRounds) {
        return list.map((t) => ({ key: t.id, header: '', task: t }))
      }
      // 同窗口归一组，组内保持后端顺序（进行中优先）；组间按窗口开始时刻升序
      const groups: Array<{ start: number; window: string; tasks: TaskView[] }> = []
      list.forEach((t) => {
        let g = groups.find((x) => x.window == t.time_window)
        if (g == null) {
          g = { start: windowStartOf(t.time_window), window: t.time_window, tasks: [] }
          groups.push(g)
        }
        g.tasks.push(t)
      })
      groups.sort((a, b) => a.start - b.start)
      const rows: TaskRow[] = []
      groups.forEach((g) => {
        rows.push({ key: 'h-' + g.window, header: g.window != '' ? g.window : '全天', task: null })
        g.tasks.forEach((t) => rows.push({ key: t.id, header: '', task: t }))
      })
      return rows
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
          this.buildTypeChips()
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
    },
    /** 类型筛选 chip：按当日任务实际类型动态生成（label 走后端字典，新类型零改动生效） */
    buildTypeChips() {
      const chips: Array<{ label: string; value: string }> = [{ label: '全部', value: '' }]
      const seen: Record<string, boolean> = {}
      this.tasks.forEach((t) => {
        if (t.patrol_type == '' || seen[t.patrol_type]) return
        seen[t.patrol_type] = true
        chips.push({ label: t.patrol_text != '' ? t.patrol_text : t.patrol_type, value: t.patrol_type })
      })
      this.typeChips = chips
      // 当前选中类型已不在列表时回落「全部」
      if (this.typeFilter != '' && !seen[this.typeFilter]) this.typeFilter = ''
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

/* 巡查类型筛选 chips */
.chips {
  margin-bottom: 16rpx;
}

.chips-inner {
  display: inline-flex;
  flex-direction: row;
  flex-wrap: nowrap;
}

.chip {
  flex-shrink: 0;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  padding: 8rpx 24rpx;
  margin-right: 16rpx;
}

.chip-text {
  font-size: 26rpx;
  white-space: nowrap;
}

.empty-filter {
  align-items: center;
  padding: 64rpx 0;
}

.empty-filter-text {
  font-size: 26rpx;
}

.card-sub-row {
  flex-direction: row;
  align-items: center;
  margin-top: 8rpx;
  margin-bottom: 24rpx;
}

.type-tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
  margin-right: 16rpx;
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
}

/* 轮次分组组头 */
.group-title {
  font-size: 26rpx;
  margin: 8rpx 8rpx 16rpx;
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
