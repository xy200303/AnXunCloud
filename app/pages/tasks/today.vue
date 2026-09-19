<template>
  <view class="page bg-page" >
    <!-- 离线暂存提示条：队列非空时显示，点击手动触发补传 -->
    <view
      v-if="offlineCount > 0"
      class="offline-bar bg-brand-light"
      
      @click="onOfflineTap"
    >
      <text class="offline-bar-text text-brand" >离线暂存 {{ offlineCount }} 条打卡，点击立即补传</text>
    </view>

    <!-- 维保待办红色卡片：有临期/逾期设备才显示，右侧「去维保」直达维保待办选设备（整卡不再承担跳转） -->
    <view
      v-if="dueCount > 0"
      class="due-bar bg-danger"
      
    >
      <text class="due-bar-text text-white" >
        维保待办 {{ dueCount }} 台{{ dueMaxOverdue > 0 ? '，最早已逾期 ' + dueMaxOverdue + ' 天' : '（临期），请及时处理' }}
      </text>
      <button plain="true" hover-class="hover-dim" class="due-bar-btn bg-white" @click="goEquipmentDue">
        <text class="due-bar-btn-text text-danger">去维保</text>
      </button>
    </view>

    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block bg-border" ></view>
      <view class="sk-block bg-border" ></view>
      <view class="sk-block sk-short bg-border" ></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && tasks.length == 0" class="empty">
      <text class="empty-title text-regular" >今天没有任务，安心休息</text>
      <text class="empty-sub text-secondary" >如需补检请联系主管安排</text>
    </view>

    <!-- 任务列表 -->
    <view v-else-if="loaded" class="content">
      <AppChipScroller :items="typeChips" :value="typeFilter" @change="typeFilter = $event" />

      <view class="summary">
        <text class="summary-date text-secondary" >{{ date }}</text>
        <view class="summary-right">
          <text class="summary-progress text-brand" >{{ donePoints }}/{{ totalPoints }}</text>
        </view>
      </view>

      <view v-if="filteredTasks.length == 0" class="empty-filter">
        <text class="empty-filter-text text-secondary" >这类今天没有任务</text>
      </view>

      <!-- 轮次任务按「窗口开始时刻」分组（带组头）；非轮次任务为平铺列表（展示不变）。
           §18.2 容器官方化：uni-card + uni-tag 徽标；进度条为业务呈现保留 -->
      <view v-for="row in displayRows" :key="row.key">
        <text v-if="row.task == null" class="group-title text-secondary" >{{ row.header }}</text>
        <uni-card
          v-else
          :is-shadow="true"
          :border="false"
          margin="0 0 24rpx 0"
          padding="28rpx 32rpx"
          spacing="0"
          @click="goDetail(row.task.id)"
        >
          <view hover-class="hover-dim" class="card-head">
            <text hover-class="hover-dim" class="card-title text-main" >{{ row.task.community_name }} · {{ row.task.plan_name }}</text>
            <uni-tag :text="row.task.status_text" :inverted="true" size="small" :custom-style="'color:' + row.task.status_color + ';border-color:' + row.task.status_color" />
          </view>
          <view hover-class="hover-dim" class="card-sub-row">
            <uni-tag v-if="row.task.round_name != ''" :text="row.task.round_name" :inverted="true" size="small" :custom-style="'color:#ED7B2F;border-color:#ED7B2F;margin-right:16rpx'" />
            <uni-tag v-if="row.task.patrol_text != ''" :text="row.task.patrol_text" :inverted="true" size="small" :custom-style="'color:#2B5AED;border-color:#2B5AED;margin-right:16rpx'" />
            <uni-tag v-if="row.task.due_text != ''" :text="row.task.due_text" :inverted="true" size="small" :custom-style="row.task.due_urgent ? 'color:#ED7B2F;border-color:#ED7B2F;margin-right:16rpx' : 'color:#909399;border-color:#909399;margin-right:16rpx'" />
            <text hover-class="hover-dim" class="card-sub text-secondary" >{{ row.task.time_window != '' ? row.task.time_window : (row.task.round_name != '' ? '不限时段' : '') }}</text>
          </view>
          <view class="progress bg-border" >
            <view
              class="progress-inner"
              :style="{ width: row.task.progress_width, backgroundColor: row.task.bar_color }"
            ></view>
          </view>
          <text hover-class="hover-dim" class="card-progress-text text-regular" >{{ row.task.done_points }}/{{ row.task.total_points }} 点位</text>
        </uni-card>
      </view>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title text-regular" >{{ errorMsg }}</text>
      <text class="empty-retry text-brand"  @click="load">重试</text>
    </view>

    <!-- 导航栏「+」菜单（微信式下拉；数据驱动，加功能往 plusItems 里加一行） -->
    <view v-if="menuOpen" class="plus-mask" @click="menuOpen = false">
      <view class="plus-menu" @click.stop>
        <block v-for="(m, i) in plusItems" :key="m.key">
          <view v-if="i > 0" class="plus-divider"></view>
          <view  hover-class="hover-dim" class="plus-item" @click="onPlusItem(m.key)">
            <text  hover-class="hover-dim" class="plus-item-icon text-white" >{{ m.icon }}</text>
            <text  hover-class="hover-dim" class="plus-item-text text-white" >{{ m.label }}</text>
          </view>
        </block>
      </view>
    </view>

    <view class="tabbar-space"></view>

    <!-- 维保启动提醒弹窗（每天第一次进本页且有待维保设备时弹出；AppDialog 双按钮） -->
    <AppDialog
      :visible="dueTipVisible"
      kind="warning"
      title="维保提醒"
      :content="dueTipText"
      confirm-text="去处理"
      cancel-text="知道了"
      :mask-closable="true"
      @update:visible="dueTipVisible = $event"
      @confirm="goEquipmentDue"
      @cancel="closeDueTip"
    />

    <!-- 版本更新弹窗（启动自动检查；强制更新不可跳过） -->
    <UpdateDialog ref="updDialog" />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiTasksToday, apiEquipmentDue, TodayTask } from '@/services/api'
import { offlineCount, syncOfflineCheckins } from '@/utils/offline'
import { useMessageStore } from '@/stores/message'
import { useAuthStore } from '@/stores/auth'
import { doNfc } from '@/utils/scan'
import { fetchLatestRelease } from '@/utils/update'
import AppChipScroller from '@/components/AppChipScroller.vue'
import AppDialog from '@/components/AppDialog.vue'
import UpdateDialog from '@/components/UpdateDialog.vue'

/** 维保提醒「每日一次」存储键 */
const DUE_TIP_KEY = 'equipment_due_tip_date'

function todayKey(): string {
  const d = new Date()
  const pad = (n: number) => (n < 10 ? '0' + n : '' + n)
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate())
}

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
  /** 期限标签（如「期限 09-30」；无 due_date 为空串不展示） */
  due_text: string
  /** 临期（距期限 ≤3 天，含已过期限）：标签橙色 */
  due_urgent: boolean
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
  /** 导航栏「+」菜单是否展开 */
  menuOpen: boolean
  /** 维保待办台数（0 = 不显示卡片；临期+逾期合计） */
  dueCount: number
  /** 最早逾期天数（0 = 无逾期，仅临期） */
  dueMaxOverdue: number
  /** 启动提醒弹窗 */
  dueTipVisible: boolean
  dueTipText: string
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

/** 期限标签：due_date（YYYY-MM-DD，空串=无期限）→ 「期限 MM-DD」；距期限 ≤3 天（含已过）为临期 */
function dueViewOf(dueDate: string): { text: string; urgent: boolean } {
  if (dueDate == '') return { text: '', urgent: false }
  const d = new Date(dueDate.replace(/-/g, '/'))
  if (isNaN(d.getTime())) return { text: '', urgent: false }
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  d.setHours(0, 0, 0, 0)
  const days = Math.round((d.getTime() - now.getTime()) / 86400000)
  return { text: '期限 ' + dueDate.slice(5), urgent: days <= 3 }
}

function toTaskView(t: TodayTask): TaskView {
  const due = dueViewOf(t.due_date ?? '')
  return {
    id: t.id,
    plan_name: t.plan_name,
    community_name: t.community_name,
    patrol_type: t.patrol_type,
    patrol_text: patrolLabelOf(t),
    round_name: t.round_name,
    due_text: due.text,
    due_urgent: due.urgent,
    time_window: t.time_window,
    status_text: statusTextOf(t.status),
    status_color: statusColorOf(t.status),
    progress_width: `${t.progress}%`,
    bar_color: t.status == 'overdue' ? '#D54941' : '#2B5AED',
    total_points: t.total_points,
    done_points: t.done_points
  } as TaskView
}

export default {
  components: { AppChipScroller, AppDialog, UpdateDialog },
  data(): TodayData {
    return {
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
      offlineCount: 0,
      menuOpen: false,
      dueCount: 0,
      dueMaxOverdue: 0,
      dueTipVisible: false,
      dueTipText: '',
      // 「+」菜单项（数据驱动，加功能加一行；维保登记是主动上报入口，巡楼发现该维保的设备随时登记）
      plusItems: [
        { key: 'maintain', label: '维保登记', icon: '⚒' },
        { key: 'nearby', label: '附近点位', icon: '◎' },
        { key: 'nfc', label: 'NFC 识别', icon: '≋' },
        { key: 'history', label: '历史任务', icon: '◷' }
      ] as Array<{ key: string; label: string; icon: string }>
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
    // 启动自动检查更新（静默：有更新才弹窗；页面实例常驻，onLoad 只触发一次）
    fetchLatestRelease().then((rel) => {
      if (rel == null) return
      const dlg: any = this.$refs.updDialog
      if (dlg != null) dlg.open(rel)
    })
  },
  onShow() {
    // 打卡返回后刷新进度 + 离线队列计数；有网时自动补传（sync 内部判网/单飞）
    this.offlineCount = offlineCount()
    if (this.loaded) this.load()
    if (this.offlineCount > 0) {
      this.trySync()
    }
    // 维保待办卡片 + 每日一次启动提醒（失败静默：无权限/未上线不打扰）
    this.loadDue(true)
    // 刷新消息 tab 未读角标（轻量请求只取 unread_count，失败静默）
    useMessageStore().refresh()
  },
  onPullDownRefresh() {
    this.load()
  },
  /** 原生导航栏「+」按钮（pages.json titleNView.buttons）：展开/收起功能菜单 */
  onNavigationBarButtonTap() {
    this.menuOpen = !this.menuOpen
  },
  methods: {
    /** 「+」菜单项分发 */
    onPlusItem(key: string) {
      this.menuOpen = false
      if (key == 'maintain') {
        // 维保登记需先选设备（裸进 maintain 是空表单）：跳台账选择模式，点设备行带参直达登记页
        uni.navigateTo({ url: '/pages/equipment/index?mode=pick' })
        return
      }
      if (key == 'nearby') {
        this.goNearby()
        return
      }
      if (key == 'nfc') {
        // iOS 手动触发入口（CoreNFC 必须用户明确触发）；Android/鸿蒙同样可用
        doNfc()
        return
      }
      if (key == 'history') {
        // 历史任务回看（逾期任务可进详情补拍）
        uni.navigateTo({ url: '/pages/tasks/history' })
        return
      }
    },
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
    goNearby() {
      uni.navigateTo({ url: '/pages/tasks/nearby' })
    },
    /** 维保待办：拉取本租户临期+逾期设备；withTip 时按「每天一次」弹启动提醒 */
    loadDue(withTip: boolean) {
      // 能看台账或能登记维保的才查（卡片与弹窗都不显示给无关角色）
      if (!useAuthStore().hasPerm(['equipment:list', 'equipment:maintenance'])) return
      apiEquipmentDue()
        .then((list) => {
          this.dueCount = list.length
          let maxOverdue = 0
          list.forEach((e) => {
            if ((e.overdue_days ?? 0) > maxOverdue) maxOverdue = e.overdue_days ?? 0
          })
          this.dueMaxOverdue = maxOverdue
          if (!withTip || list.length == 0) return
          // 启动弹窗每天最多一次（本地记日期）
          if (uni.getStorageSync(DUE_TIP_KEY) == todayKey()) return
          uni.setStorageSync(DUE_TIP_KEY, todayKey())
          this.dueTipText = maxOverdue > 0
            ? list.length + ' 台设备待维保，最早已逾期 ' + maxOverdue + ' 天，请尽快处理'
            : list.length + ' 台设备临近维保期，请安排维保'
          this.dueTipVisible = true
        })
        .catch((_e: any) => {})
    },
    closeDueTip() {
      this.dueTipVisible = false
    },
    /** 待办卡片/弹窗「去处理」：跳维保待办页选设备模式（点卡片直达维保拍照页；有逾期按已逾期筛选，否则按临期） */
    goEquipmentDue() {
      this.dueTipVisible = false
      uni.navigateTo({
        url: '/pages/equipment/due?mode=pick&due_state=' + (this.dueMaxOverdue > 0 ? 'overdue' : 'warning')
      })
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

/* 维保待办红色卡片 */
.due-bar {
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.due-bar-text {
  font-size: 26rpx;
  flex: 1;
}

.due-bar-btn {
  border-radius: 32rpx;
  padding: 12rpx 32rpx;
  margin-left: 16rpx;
  align-items: center;
  justify-content: center;
}

.due-bar-btn-text {
  font-size: 26rpx;
  font-weight: 600;
}

/* 巡查类型筛选 chips */
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

.summary-right {
  flex-direction: row;
  align-items: center;
}

.summary-date {
  font-size: 26rpx;
}

.summary-progress {
  font-size: 48rpx; /* FontSize.number */
  font-weight: 600;
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

/* 导航栏「+」菜单：微信式深色下拉，锚定右上角（原生导航栏下方） */
.plus-mask {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 998;
}

.plus-menu {
  position: absolute;
  top: 8rpx;
  right: 24rpx;
  min-width: 280rpx;
  background-color: #4c4c4c;
  border-radius: 16rpx;
  overflow: hidden;
}

.plus-item {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 28rpx 36rpx;
}

.plus-item-icon {
  font-size: 34rpx;
  margin-right: 20rpx;
  width: 40rpx;
  text-align: center;
}

.plus-item-text {
  font-size: 30rpx;
}

.plus-divider {
  height: 1rpx;
  background-color: rgba(255, 255, 255, 0.15);
  margin: 0 36rpx;
}
</style>
