<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 自定义导航栏：左返回（退出巡检）、中标题 -->
    <view class="navbar" :style="{ backgroundColor: colors.primary, paddingTop: statusBarHeight + 'px' }">
      <view class="navbar-row">
        <view  hover-class="hover-dim" class="navbar-side" @click="onBackTap">
          <text class="navbar-back" :style="{ color: colors.white }">‹</text>
        </view>
        <text class="navbar-title" :style="{ color: colors.white }">连续巡检</text>
        <view  hover-class="hover-dim" class="navbar-side navbar-right"></view>
      </view>
    </view>
    <view class="navbar-space" :style="{ height: 'calc(88rpx + ' + statusBarHeight + 'px)' }"></view>

    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <!-- 连续巡检向导 -->
    <view v-else class="wizard">
      <!-- 顶部进度区（任务完成页不显示） -->
      <view v-if="phase != 'taskDone'" class="head" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <view class="head-row">
          <text class="head-progress" :style="{ color: colors.primary }">点位 {{ pointOrdinal }}/{{ totalPoints }}</text>
          <view v-if="phase == 'items'" class="head-item-pill" :style="{ backgroundColor: colors.primaryLight }">
            <text class="head-item-pill-text" :style="{ color: colors.primary }">{{ curItem != null && curItem.status == 'recognizing' ? 'AI 检查中' : '第 ' + (itemIdx + 1) + '/' + curItemCount + ' 项' }}</text>
          </view>
        </view>
        <text class="head-point" :style="{ color: colors.textPrimary }">{{ curPoint != null ? curPoint.point_name : '' }}</text>
        <text class="head-building" :style="{ color: colors.textSecondary }">{{ curPoint != null && curPoint.building_name != '' ? curPoint.building_name : '未分区' }}</text>
        <view class="head-bar-row">
          <view class="progress head-bar" :style="{ backgroundColor: colors.border }">
            <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: colors.primary }"></view>
          </view>
          <text class="head-bar-text" :style="{ color: colors.textSecondary }">{{ progressWidth }}</text>
        </view>
      </view>

      <!-- 凭证核验步 -->
      <block v-if="phase == 'cred'">
        <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
          <text class="cred-title" :style="{ color: colors.textPrimary }">到场打卡</text>
          <text v-if="!needsCred" class="cred-none" :style="{ color: colors.success }">✓ 本点位直接拍照就行</text>
          <!-- 扫码行 -->
          <view
            v-if="curPoint != null && (curPoint.credential == 'qrcode' || curPoint.credential == 'any')"
             hover-class="hover-dim" class="cred-row"
            @click="onScanRowTap"
          >
            <text  hover-class="hover-dim" class="cred-row-name" :style="{ color: colors.textPrimary }">扫点位二维码</text>
            <text v-if="curWizPoint != null && curWizPoint.scannedNo != ''" class="cred-status" :style="{ color: colors.success }">✓ 已完成</text>
            <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
          </view>
          <!-- 读卡行 -->
          <view
            v-if="curPoint != null && (curPoint.credential == 'nfc' || curPoint.credential == 'any')"
             hover-class="hover-dim" class="cred-row"
            @click="onNfcRowTap"
          >
            <text  hover-class="hover-dim" class="cred-row-name" :style="{ color: colors.textPrimary }">刷 NFC 卡</text>
            <text v-if="curWizPoint != null && curWizPoint.nfcCardId != ''" class="cred-status" :style="{ color: colors.success }">✓ 已完成</text>
            <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
          </view>
          <!-- 定位行 -->
          <view v-if="curPoint != null && curPoint.require_fence"  hover-class="hover-dim" class="cred-row" @click="onLocTap">
            <text  hover-class="hover-dim" class="cred-row-name" :style="{ color: colors.textPrimary }">位置确认</text>
            <text v-if="locating" class="cred-status" :style="{ color: colors.textSecondary }">定位中…</text>
            <text v-else-if="locFailed" class="cred-status" :style="{ color: colors.danger }">失败，点我重试</text>
            <text v-else-if="distance >= 0 && curPoint != null && distance <= curPoint.fence_radius" class="cred-status" :style="{ color: colors.success }">✓ 在范围内（{{ distance }}米）</text>
            <text v-else-if="distance >= 0" class="cred-status" :style="{ color: colors.danger }">超出范围（{{ distance }}米）</text>
            <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
          </view>
        </view>
        <view
           hover-class="hover-dim" class="btn-big"
          :style="{ backgroundColor: credOk && fenceOk ? colors.success : colors.border }"
          @click="startItems"
        >
          <text  hover-class="hover-dim" class="btn-big-text" :style="{ color: credOk && fenceOk ? colors.white : colors.textSecondary }">开始检查</text>
        </view>
        <text v-if="!(credOk && fenceOk)" class="start-hint" :style="{ color: colors.textSecondary }">先完成上面的确认，再开始检查</text>
      </block>

      <!-- 逐项卡片步 -->
      <block v-if="phase == 'items' && curItem != null">
        <QuickItemCard
          :item="curItem"
          :is-photo="curItemIsPhoto"
          :manual-abnormal-open="manualAbnormalOpen"
          :manual-note="manualNote"
          :exception-label="curItemExceptionText"
          :colors="colors"
          :shadow="shadow"
          @take-photo="takePhoto"
          @preview-photo="previewCurPhoto"
          @image-error="curItem.img_error = true"
          @next="nextStep"
          @report-missing="reportPhotoItemMissing"
          @manual-ok="tapManualOk"
          @manual-abnormal="tapManualAbnormal"
          @update:manual-note="manualNote = $event"
          @confirm-manual-abnormal="confirmManualAbnormal"
        />
      </block>

      <!-- 点位收尾步 -->
      <block v-if="phase == 'gate'">
        <view class="card gate-card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
          <text class="gate-title" :style="{ color: colors.textPrimary }">本点位 {{ curItemCount }} 项已过完</text>
          <view class="gate-stats">
            <view class="gate-stat">
              <text class="gate-stat-num" :style="{ color: colors.success }">{{ gateStats.done }}</text>
              <text class="gate-stat-label" :style="{ color: colors.textSecondary }">已完成</text>
            </view>
            <view class="gate-stat">
              <text class="gate-stat-num" :style="{ color: colors.primary }">{{ gateStats.recognizing }}</text>
              <text class="gate-stat-label" :style="{ color: colors.textSecondary }">AI 检查中</text>
            </view>
            <view class="gate-stat">
              <text class="gate-stat-num" :style="{ color: colors.danger }">{{ gateStats.abnormal }}</text>
              <text class="gate-stat-label" :style="{ color: colors.textSecondary }">异常</text>
            </view>
          </view>
          <text class="gate-sub" :style="{ color: colors.textSecondary }">{{ gateStats.recognizing > 0 ? '提交后将等待 AI 检查完成' : '提交后 AI 统一检查' }}</text>
        </view>
        <view  hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.success }" @click="submitPoint">
          <text  hover-class="hover-dim" class="btn-big-text" :style="{ color: colors.white }">提交本点位</text>
        </view>
      </block>

      <!-- 补拍步（质量不合格 / 识别失败） -->
      <block v-if="phase == 'retake'">
        <QuickIssuePanel
          mode="retake"
          :items="retakeItems"
          :colors="colors"
          :shadow="shadow"
          @preview="previewPhoto"
          @image-error="$event.img_error = true"
          @retake="retakePhoto"
          @confirm="submitPoint"
        />
      </block>

      <!-- 异常确认步（红屏） -->
      <block v-if="phase == 'abnormal'">
        <QuickIssuePanel
          mode="abnormal"
          :items="abnormalItems"
          :ai-editable="aiEditable"
          :colors="colors"
          :shadow="shadow"
          @update-note="onAbnNoteChange"
          @confirm="confirmAbnormalSubmit"
        />
      </block>

      <!-- 点位完成（绿勾，自动下一点位） -->
      <view v-if="phase == 'pointDone'" class="done-pane" :style="{ backgroundColor: colors.success }">
        <text class="done-icon" :style="{ color: colors.white }">✓</text>
        <text class="done-title" :style="{ color: colors.white }">本点位完成</text>
      </view>

      <!-- 任务完成 -->
      <view v-if="phase == 'taskDone'" class="done-pane" :style="{ backgroundColor: colors.success }">
        <text class="done-icon" :style="{ color: colors.white }">✓</text>
        <text class="done-title" :style="{ color: colors.white }">任务完成</text>
        <view  hover-class="hover-dim" class="done-btn" :style="{ backgroundColor: colors.white }" @click="exitWizard">
          <text  hover-class="hover-dim" class="done-btn-text" :style="{ color: colors.success }">返回</text>
        </view>
      </view>

      <!-- 上一项：主推进阶段且不在起点时显示（返回键=退出巡检，回退只走此按钮） -->
      <view v-if="showPrev && !atWizardStart"  hover-class="hover-dim" class="prev-link" @click="onPrevTap">
        <text  hover-class="hover-dim" class="prev-link-text" :style="{ color: colors.textSecondary }">‹ 上一项</text>
      </view>

      <!-- 手动模式入口：当前点位改用逐项填写表单（向导进度在云端草稿，切走重进可续） -->
      <view v-if="showManualEntry"  hover-class="hover-dim" class="prev-link" @click="goManualPoint">
        <text  hover-class="hover-dim" class="prev-link-text" :style="{ color: colors.textSecondary }">手动填写本点位</text>
      </view>

      <view class="bottom-space"></view>
    </view>

    <!-- 上传 / AI 检查 / 提交中弹窗 -->
    <view v-if="overlayMsg != ''" class="overlay" :style="{ backgroundColor: colors.mask }">
      <view class="overlay-dialog" :style="{ backgroundColor: colors.bgCard }">
        <view class="spinner" :style="{ borderTopColor: colors.primary }"></view>
        <text class="overlay-text" :style="{ color: colors.textPrimary }">{{ overlayMsg }}</text>
        <text class="overlay-sub" :style="{ color: colors.textSecondary }">{{ overlaySub }}</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import {
  apiTaskDetail,
  apiCheckin,
  apiCheckinItems,
  apiUploadLocal,
  apiAiItemJobCreate,
  apiAiItemJobs,
  apiItemDrafts,
  apiItemDraftManual,
  apiItemDraftPhotoAbnormal,
  CODE_AI_DISABLED,
  CODE_CHECKIN_LOCKED,
  ItemDraft,
  TaskPoint
} from '@/services/api'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { extractPointCode, resolvePointCode } from '@/utils/scan'
import { getLocationGcj02 } from '@/utils/geo'
import { playVoice } from '@/utils/voice'
import { compressForUpload } from '@/utils/image'
import { WizardPointSnap, WizardItemSnap } from '@/utils/checkinWizard'
import QuickItemCard from '@/components/QuickItemCard.vue'
import QuickIssuePanel from '@/components/QuickIssuePanel.vue'

/** 向导阶段：cred 凭证 / items 逐项 / gate 提交本点位 / retake 补拍 / abnormal 异常确认 / pointDone 点位完成 / taskDone 任务完成 */
type Phase = 'cred' | 'items' | 'gate' | 'retake' | 'abnormal' | 'pointDone' | 'taskDone'

/** AI 轮询节奏：pending 时 1.5s 间隔批量查，30s 超时按 failed 处理（契约 §2） */
const POLL_INTERVAL = 1500
const POLL_TIMEOUT = 30000

type QuickData = {
  colors: ColorTokens
  shadow: string
  /** 状态栏高度（px），自定义导航栏占位用 */
  statusBarHeight: number
  taskId: string
  pointIdParam: string
  /** true = 单点位修改模式（覆盖提交） */
  modify: boolean
  /** 扫码进入带来的已核验二维码编号 */
  preVerifiedNo: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  /** 任务全部点位（sort 升序，展示与序号用） */
  taskPoints: TaskPoint[]
  totalPoints: number
  /** 服务端已打卡点位数（进度条基数） */
  doneBase: number
  /** 本次会话内向导提交成功的点位数 */
  doneLocal: number
  /** AI 异常描述是否允许巡检员编辑 */
  aiEditable: boolean
  /** 向导范围内点位（仅未提交；快照持久化对象） */
  wizPoints: WizardPointSnap[]
  pointIdx: number
  itemIdx: number
  phase: Phase
  /** 感官项「有异常」展开输入 */
  manualAbnormalOpen: boolean
  manualNote: string
  /** 补拍 / 异常确认列表（当前点位 items 下标） */
  retakeIdxs: number[]
  abnormalIdxs: number[]
  locating: boolean
  locFailed: boolean
  hasLoc: boolean
  myLng: number
  myLat: number
  /** 海拔/定位精度（米，0=未取得；仅随打卡上送作参考展示） */
  myAlt: number
  myAcc: number
  /** 与点位距离（米），-1 表示未知 */
  distance: number
  /** 遮盖层文案（空 = 不显示） */
  overlayMsg: string
  submitting: boolean
  /** 轮询定时器 */
  pollTimer: any
  /** 页面已卸载（停止轮询回调写状态） */
  destroyed: boolean
  /** 遮罩看门狗定时器 */
  overlayWatchdog: any
  /** 手动退出放行标记（exitWizard 时 onBackPress 不拦截） */
  forceExit: boolean
}

/** haversine 距离（米） */
function haversine(lng1: number, lat1: number, lng2: number, lat2: number): number {
  const R = 6371000
  const rad = (d: number) => (d * Math.PI) / 180
  const dLat = rad(lat2 - lat1)
  const dLng = rad(lng2 - lng1)
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(rad(lat1)) * Math.cos(rad(lat2)) * Math.sin(dLng / 2) * Math.sin(dLng / 2)
  return 2 * R * Math.asin(Math.sqrt(a))
}

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n
}

/** YYYY-MM-DD HH:mm:ss（与后端 timefmt.Layout 一致，本地时区） */
function fmtDateTime(d: Date): string {
  return (
    d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate()) +
    ' ' + pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + ':' + pad2(d.getSeconds())
  )
}

/** 由点位模板生成向导项初始状态 */
function freshItem(name: string, requirement: string, judgeType: string): WizardItemSnap {
  return {
    name: name,
    requirement: requirement,
    judge_type: judgeType,
    photos: [],
    file_ids: [],
    exception_type: '',
    file_id: '',
    job_id: '',
    status: 'todo',
    verdict: '',
    reason: '',
    reading: '',
    quality_pass: true,
    quality_issue: '',
    pass: true,
    note: ''
  }
}

/** 由点位模板生成向导点位初始状态 */
function freshPoint(p: TaskPoint): WizardPointSnap {
  return {
    point_id: p.point_id,
    status: 'doing',
    scannedNo: '',
    nfcCardId: '',
    items: (p.check_items || []).map((c) => freshItem(c.name, c.requirement, c.judge_type))
  }
}

export default {
  components: { QuickItemCard, QuickIssuePanel },
  data(): QuickData {
    return {
      colors: Colors,
      shadow: ShadowCard,
      statusBarHeight: 0,
      taskId: '',
      pointIdParam: '',
      modify: false,
      preVerifiedNo: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      taskPoints: [] as TaskPoint[],
      totalPoints: 0,
      doneBase: 0,
      doneLocal: 0,
      aiEditable: false,
      wizPoints: [] as WizardPointSnap[],
      pointIdx: 0,
      itemIdx: 0,
      phase: 'cred',
      manualAbnormalOpen: false,
      manualNote: '',
      retakeIdxs: [] as number[],
      abnormalIdxs: [] as number[],
      locating: false,
      locFailed: false,
      hasLoc: false,
      myLng: 0,
      myLat: 0,
      myAlt: 0,
      myAcc: 0,
      distance: -1,
      overlayMsg: '',
      submitting: false,
      pollTimer: null,
      destroyed: false,
      /** 遮罩看门狗定时器 */
      overlayWatchdog: null,
      forceExit: false
    }
  },
  computed: {
    curWizPoint(): WizardPointSnap | null {
      if (this.pointIdx < 0 || this.pointIdx >= this.wizPoints.length) return null
      return this.wizPoints[this.pointIdx]
    },
    curPoint(): TaskPoint | null {
      const wp = this.curWizPoint
      if (wp == null) return null
      const pt = this.taskPoints.find((p) => p.point_id == wp.point_id)
      return pt != null ? pt : null
    },
    curItem(): WizardItemSnap | null {
      const wp = this.curWizPoint
      if (wp == null || this.itemIdx < 0 || this.itemIdx >= wp.items.length) return null
      return wp.items[this.itemIdx]
    },
    curItemCount(): number {
      return this.curWizPoint != null ? this.curWizPoint.items.length : 0
    },
    /** 当前项是否拍照项（judge_type != 'manual'；缺省按拍照项） */
    curItemIsPhoto(): boolean {
      return this.curItem != null && this.curItem.judge_type != 'manual'
    },
    curItemExceptionText(): string {
      return this.curItem == null ? '设备不存在/无法检测，提交异常' : this.exceptionText(this.curItem.exception_type)
    },
    /** 点位序号（任务全部点位中的位置，1 起） */
    pointOrdinal(): number {
      const pt = this.curPoint
      if (pt == null) return 1
      const i = this.taskPoints.findIndex((p) => p.point_id == pt.point_id)
      return i >= 0 ? i + 1 : 1
    },
    progressWidth(): string {
      if (this.totalPoints <= 0) return '0%'
      const done = Math.min(this.doneBase + this.doneLocal, this.totalPoints)
      return Math.round((done / this.totalPoints) * 100) + '%'
    },
    /** 是否需要凭证步（扫码/NFC/围栏任一） */
    needsCred(): boolean {
      const pt = this.curPoint
      if (pt == null) return false
      return pt.credential == 'qrcode' || pt.credential == 'nfc' || pt.credential == 'any' || pt.require_fence
    },
    credOk(): boolean {
      const pt = this.curPoint
      const wp = this.curWizPoint
      if (pt == null || wp == null) return false
      const c = pt.credential
      if (c == 'qrcode') return wp.scannedNo != ''
      if (c == 'nfc') return wp.nfcCardId != ''
      if (c == 'any') return wp.scannedNo != '' || wp.nfcCardId != ''
      return true
    },
    /** 围栏未超出（无围栏点位恒 true） */
    fenceOk(): boolean {
      const pt = this.curPoint
      if (pt == null || !pt.require_fence) return true
      return this.distance >= 0 && this.distance <= pt.fence_radius
    },
    retakeItems(): WizardItemSnap[] {
      const wp = this.curWizPoint
      if (wp == null) return []
      return this.retakeIdxs.map((i) => wp.items[i]).filter((it) => it != null)
    },
    /** 收尾步统计：已完成（含已拍待识别）/ AI 检查中 / 异常（感官项已标记异常） */
    gateStats(): { done: number; recognizing: number; abnormal: number } {
      const wp = this.curWizPoint
      const r = { done: 0, recognizing: 0, abnormal: 0 }
      if (wp == null) return r
      for (let i = 0; i < wp.items.length; i++) {
        const it = wp.items[i]
        if (it.status == 'recognizing') r.recognizing += 1
        else if (it.verdict == 'abnormal') r.abnormal += 1
        else r.done += 1
      }
      return r
    },
    /** 弹窗副提示 */
    overlaySub(): string {
      if (this.overlayMsg.indexOf('AI') >= 0) return '正在识别照片，一般几秒内完成'
      return '请稍候，不要退出页面'
    },
    abnormalItems(): WizardItemSnap[] {
      const wp = this.curWizPoint
      if (wp == null) return []
      return this.abnormalIdxs.map((i) => wp.items[i]).filter((it) => it != null)
    },
    /** 底部「上一项」：主推进阶段且遮盖层未开启时显示 */
    showPrev(): boolean {
      if (this.overlayMsg != '' || this.submitting) return false
      return this.phase == 'cred' || this.phase == 'items' || this.phase == 'gate'
    },
    /** 手动模式入口：主推进阶段 + 非修改模式 + 有当前点位（修改模式点位已打卡，手动表单会拒） */
    showManualEntry(): boolean {
      if (this.modify || this.overlayMsg != '' || this.submitting) return false
      if (this.curPoint == null) return false
      return this.phase == 'cred' || this.phase == 'items' || this.phase == 'gate'
    },
    /** 是否在本点位第一步（无上一项可回退，底部「上一项」按钮隐藏；退出走返回键）。
     *  与 pointIdx 无关：中途进入（扫码/NFC/点位清单）的第一个点位就是进入者的第一页 */
    atWizardStart(): boolean {
      if (this.phase == 'cred') return true
      return this.phase == 'items' && this.itemIdx == 0 && !this.needsCred
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointIdParam = options && options.point_id ? String(options.point_id) : ''
    this.modify = options != null && options.mode == 'modify'
    if (options && options.no) {
      this.preVerifiedNo = String(options.no).trim()
    }
    const sys = uni.getSystemInfoSync()
    this.statusBarHeight = sys.statusBarHeight != null ? sys.statusBarHeight : 0
    this.load()
  },
  onUnload() {
    this.destroyed = true
    this.stopPoll()
    if (this.overlayWatchdog != null) {
      clearTimeout(this.overlayWatchdog)
      this.overlayWatchdog = null
    }
  },
  onBackPress(): boolean {
    // 手动退出（exitWizard 的 navigateBack 在 App 端同样触发 onBackPress）：放行
    if (this.forceExit) return false
    // 遮盖层（上传/AI 检查/提交中）：拦截返回并提示，避免用户误以为卡死
    if (this.overlayMsg != '' || this.submitting) {
      uni.showToast({ title: '处理中，请稍候…', icon: 'none' })
      return true
    }
    // 返回 = 直接退出巡检（不弹确认；已拍内容在云端草稿，下次可断点续检）
    return false
  },
  watch: {
    /** 遮罩看门狗：任何链路异常导致遮罩超过 75s（> 请求 30s / 上传 60s 超时）时强制解除，防永久卡死 */
    overlayMsg(v: string) {
      if (this.overlayWatchdog != null) {
        clearTimeout(this.overlayWatchdog)
        this.overlayWatchdog = null
      }
      if (v == '') return
      this.overlayWatchdog = setTimeout(() => {
        this.overlayWatchdog = null
        if (this.overlayMsg == '') return
        this.overlayMsg = ''
        this.submitting = false
        this.stopPoll()
        uni.showToast({ title: '网络较慢，请检查后重试', icon: 'none' })
      }, 75000)
    }
  },
  methods: {
    load() {
      if (this.taskId == '') {
        this.loading = false
        this.errorMsg = '缺少打卡参数'
        return
      }
      this.loading = true
      apiTaskDetail(this.taskId)
        .then((res) => {
          this.taskPoints = res.points.slice().sort((a, b) => a.sort - b.sort)
          this.totalPoints = res.total_points
          this.doneBase = res.done_points
          this.aiEditable = res.ai_result_editable ?? false
          if (this.modify) {
            this.initModify()
          } else {
            this.initNormal()
          }
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
        })
    },
    /** 修改模式：单点位，凭已有打卡逐项结论预填，提交走覆盖语义 */
    initModify() {
      const pt = this.taskPoints.find((p) => p.point_id == this.pointIdParam)
      if (pt == null) {
        this.loading = false
        this.errorMsg = '点位不属于该任务'
        return
      }
      if (pt.my_checkin == null) {
        this.loading = false
        this.errorMsg = '该点位未打卡，无需修改'
        return
      }
      if (pt.my_checkin.locked) {
        this.loading = false
        this.errorMsg = '已归档，不可修改'
        return
      }
      this.wizPoints = [freshPoint(pt)]
      this.pointIdx = 0
      this.itemIdx = 0
      // 预填：凭已有打卡逐项结论回填（best-effort，照片无法复原需重拍）
      this.prefillModify(pt.my_checkin.id)
      this.finishInit()
    },
    /** 普通模式：全部未打卡点位；逐项进度完全从云端草稿重建（本地不存快照） */
    initNormal() {
      const rest = this.taskPoints.filter((p) => p.my_checkin == null)
      if (rest.length == 0) {
        this.loading = false
        this.errorMsg = '本任务已全部打卡'
        return
      }
      this.wizPoints = rest.map((p) => freshPoint(p))
      this.pointIdx = 0
      this.itemIdx = 0
      // 扫码/NFC 进入：预核验编号写到匹配点位上（按二维码编号或 NFC 卡号匹配，匹配即视为已核验）
      if (this.preVerifiedNo != '') {
        this.wizPoints.forEach((wp) => {
          const pt = this.taskPoints.find((p) => p.point_id == wp.point_id)
          if (pt == null) return
          if (pt.qrcode_no != '' && pt.qrcode_no == this.preVerifiedNo) {
            wp.scannedNo = this.preVerifiedNo
          }
          if (pt.nfc_id != '' && pt.nfc_id == this.preVerifiedNo) {
            wp.nfcCardId = this.preVerifiedNo
          }
        })
      }
      // 点位清单指定进入：从该点位开始，余下点位按顺序继续
      if (this.pointIdParam != '') {
        const startIdx = this.wizPoints.findIndex((wp) => wp.point_id == this.pointIdParam)
        if (startIdx >= 0) {
          this.pointIdx = startIdx
          this.itemIdx = 0
        }
      }
      this.finishInit()
    },
    finishInit() {
      this.clampIndices()
      // 云端草稿重建逐项进度（唯一事实来源）→ 定位首个未完成项 → 展示
      this.restoreDrafts().then(() => {
        if (this.destroyed) return
        if (!this.modify) this.positionAtFirstIncomplete()
        // 识别中的项凭 job_id 批量查一次：仍 pending 留待收尾轮询；failed/过期回退待拍
        this.reconcileJobs()
        this.loading = false
        this.loaded = true
        this.enterPoint(true)
      })
    },
    /** 草稿重建后定位到当前点位首个未完成项（全部完成但未提交 → 停在收尾步） */
    positionAtFirstIncomplete() {
      const wp = this.curWizPoint
      if (wp == null) return
      const i = wp.items.findIndex((it) => it.status != 'done')
      this.itemIdx = i >= 0 ? i : wp.items.length
    },
    clampIndices() {
      if (this.pointIdx >= this.wizPoints.length) this.pointIdx = this.wizPoints.length - 1
      if (this.pointIdx < 0) this.pointIdx = 0
      const wp = this.curWizPoint
      if (wp == null) return
      if (this.itemIdx > wp.items.length) this.itemIdx = wp.items.length
      if (this.itemIdx < 0) this.itemIdx = 0
    },
    /** 云端草稿重建：逐项照片/AI 结论/手动项选择全部来自服务端草稿（巡检进度的唯一事实来源，
     *  本地不存快照）；一次拉取整个任务的草稿按点位分组套用 */
    restoreDrafts(): Promise<void> {
      return apiItemDrafts(this.taskId)
        .then((drafts) => {
          if (this.destroyed) return
          const byPoint: Record<string, ItemDraft[]> = {}
          drafts.forEach((d) => {
            if (byPoint[d.point_id] == null) byPoint[d.point_id] = []
            byPoint[d.point_id].push(d)
          })
          this.wizPoints.forEach((wp) => {
            const list = byPoint[wp.point_id]
            if (list == null) return
            list.forEach((d) => {
              const it = wp.items.find((x) => x.name == d.item_name)
              if (it == null) return
              if (it.judge_type == 'manual') {
                // 感官项：恢复手动选择结果
                if (d.manual_pass == null) return
                it.pass = d.manual_pass
                it.note = d.manual_pass ? '' : d.manual_note
                it.verdict = d.manual_pass ? 'pass' : 'abnormal'
                it.status = 'done'
                return
              }
              // 拍照项：照片 + AI 识别结论
              it.file_ids = d.file_ids.slice()
              it.exception_type = d.exception_type ?? ''
              it.photos = d.photos.slice()
              it.job_id = d.job_id
              it.img_error = false
              if (d.ai_status == 'done') {
                this.applyJob(it, {
                  verdict: d.ai_verdict,
                  reason: d.ai_reason,
                  reading: d.ai_reading,
                  quality_pass: d.quality_pass,
                  quality_issue: d.quality_issue
                })
              } else if (d.ai_status == 'pending') {
                it.status = 'recognizing'
              } else if (d.ai_status == 'failed') {
                it.status = 'failed'
                it.reason = d.ai_reason
              }
            })
          })
        })
        .catch(() => {
          // 草稿拉取失败按全新巡检处理（不阻断主链路）
        })
    },
    /** 恢复快照时对识别中的 job 批量查一次状态 */
    reconcileJobs() {
      const targets: Array<{ it: WizardItemSnap; jobId: string }> = []
      this.wizPoints.forEach((wp) => {
        wp.items.forEach((it) => {
          if (it.status == 'recognizing' && it.job_id != '') {
            targets.push({ it: it, jobId: it.job_id })
          }
        })
      })
      if (targets.length == 0) return
      apiAiItemJobs(targets.map((t) => t.jobId))
        .then((jobs) => {
          if (this.destroyed) return
          const map: Record<string, (typeof jobs)[0]> = {}
          jobs.forEach((j) => {
            map[j.job_id] = j
          })
          targets.forEach((t) => {
            const j = map[t.jobId]
            if (j == null || j.status == 'failed') {
              // job 过期/失败 → 该项回到待拍
              this.resetItem(t.it)
              return
            }
            if (j.status == 'done') this.applyJob(t.it, j)
          })
        })
        .catch(() => {
          // 查询失败不阻断：保持 recognizing，收尾时再轮询
        })
    },
    /** 进入当前点位：定位 + 决定首屏（凭证步 / 逐项 / 收尾） */
    enterPoint(resume: boolean) {
      this.clampIndices()
      const wp = this.curWizPoint
      this.manualAbnormalOpen = false
      this.retakeIdxs = []
      this.abnormalIdxs = []
      this.locate()
      if (wp == null) return
      // itemIdx 溢出（= items.length）表示已过完所有项，停在收尾
      if (this.itemIdx >= wp.items.length && wp.items.length > 0) {
        this.phase = 'gate'
      } else if (wp.items.length == 0) {
        this.phase = this.needsCred ? 'cred' : 'gate'
      } else if (this.needsCred && !(resume && this.credOk && this.fenceOk)) {
        this.phase = 'cred'
      } else {
        this.phase = 'items'
      }
      // 断点恢复正好停在收尾步（全部项已落定）：同样走方案 B 自动提交
      if (this.phase == 'gate') this.autoSubmitGate()
    },
    locate() {
      if (this.locating) return
      this.locating = true
      this.locFailed = false
      getLocationGcj02(
        (loc) => {
          this.hasLoc = true
          this.myLng = loc.longitude
          this.myLat = loc.latitude
          this.myAlt = loc.altitude
          this.myAcc = loc.accuracy
          if (this.curPoint != null) {
            this.distance = Math.round(
              haversine(loc.longitude, loc.latitude, this.curPoint.longitude, this.curPoint.latitude)
            )
          }
          this.locating = false
          // 定位回来时正停在收尾步：围栏刚满足条件，补一次自动提交触发
          if (this.phase == 'gate') this.autoSubmitGate()
        },
        () => {
          this.locFailed = true
          this.locating = false
        }
      )
    },
    onLocTap() {
      if (this.locFailed) this.locate()
    },
    /** 核验清单-扫码行：已通过再点不重复扫 */
    onScanRowTap() {
      if (this.curWizPoint != null && this.curWizPoint.scannedNo != '') return
      this.scanCredential()
    },
    /** 核验清单-读卡行：已通过再点不重复读 */
    onNfcRowTap() {
      if (this.curWizPoint != null && this.curWizPoint.nfcCardId != '') return
      this.nfcTap()
    },
    /** 当前项照片大图预览 */
    previewCurPhoto() {
      const it = this.curItem
      if (it == null || it.photos.length == 0 || it.img_error) return
      uni.previewImage({ urls: it.photos })
    },
    /** 指定项照片大图预览（补拍列表缩略图） */
    previewPhoto(it: WizardItemSnap) {
      if (it.photos.length == 0 || it.img_error) return
      uni.previewImage({ urls: it.photos })
    },
    scanCredential() {
      uni.scanCode({
        onlyFromCamera: true, // 禁相册选图防代扫
        success: (res) => {
          const code = extractPointCode(res.result)
          if (code == '') {
            uni.showToast({ title: '请扫描新版点位二维码', icon: 'none' })
            return
          }
          if (this.curPoint != null && this.curPoint.qrcode_no != '' && code != this.curPoint.qrcode_no) {
            uni.showToast({ title: '二维码与本点位不匹配', icon: 'none' })
            return
          }
          if (this.curWizPoint != null) {
            this.curWizPoint.scannedNo = code
          }
          uni.showToast({ title: '点位确认成功', icon: 'success' })
        },
        fail: (err) => {
          const msg = err && err.errMsg ? err.errMsg : ''
          if (msg.indexOf('cancel') < 0 && msg != '') {
            uni.showToast({ title: '扫码失败：' + msg, icon: 'none' })
          }
        }
      })
    },
    nfcTap() {
      if (!isNfcSupported()) {
        toastNfcUnavailable()
        return
      }
      uni.showLoading({ title: '请贴近 NFC 标签', mask: true })
      readCardOnce((cardId, errMsg) => {
        uni.hideLoading()
        if (cardId == null) {
          uni.showToast({ title: errMsg || 'NFC 读取失败', icon: 'none' })
          return
        }
        if (this.curPoint != null && this.curPoint.nfc_id != '' && cardId != this.curPoint.nfc_id) {
          uni.showToast({ title: 'NFC 标签与本点位不匹配', icon: 'none' })
          return
        }
        if (this.curWizPoint != null) {
          this.curWizPoint.nfcCardId = cardId
        }
        uni.showToast({ title: '点位确认成功', icon: 'success' })
      })
    },
    /**
     * 向导页内全局 NFC 贴卡（App.vue 转发，不用先点按钮）：
     * - 当前点位核验步且卡匹配 → 就地完成读卡确认，不重载页面
     * - 其他未打卡点位 → 跳到该点位并预核验（等价扫码进入）
     * - 已提交完成的点位 → 直接进入（修改模式；已归档由向导初始化拦截提示）
     * - 不属于本任务 → 回退全局任务定位（可能是今天其他任务的点位）
     */
    onGlobalNfc(cardId: string) {
      const pt = this.taskPoints.find((p) => p.nfc_id != '' && p.nfc_id == cardId)
      if (pt == null) {
        resolvePointCode(cardId)
        return
      }
      if (this.overlayMsg != '' || this.submitting) {
        uni.showToast({ title: '处理中，请稍候…', icon: 'none' })
        return
      }
      if (pt.my_checkin != null) {
        // 已提交完成的点位：进记录卡（先看后改；可改性由记录卡按锁定/任务状态判定）
        uni.redirectTo({
          url:
            '/pages/checkin/record?task_id=' + encodeURIComponent(this.taskId) +
            '&point_id=' + encodeURIComponent(pt.point_id)
        })
        return
      }
      if (this.phase == 'cred' && this.curPoint != null && this.curPoint.point_id == pt.point_id) {
        if (this.curWizPoint != null) this.curWizPoint.nfcCardId = cardId
        uni.showToast({ title: '点位确认成功', icon: 'success' })
        return
      }
      uni.redirectTo({
        url:
          '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
          '&point_id=' + encodeURIComponent(pt.point_id) + '&no=' + encodeURIComponent(cardId)
      })
    },
    /** 凭证步「开始检查」：凭证不通过不让开始该点位 */
    startItems() {
      if (!this.credOk) {
        uni.showToast({ title: '请先完成点位核验', icon: 'none' })
        return
      }
      if (!this.fenceOk) {
        uni.showToast({ title: '超出范围，走近一点再开始', icon: 'none' })
        return
      }
      const wp = this.curWizPoint
      if (wp == null) return
      this.phase = wp.items.length > 0 ? 'items' : 'gate'
    },
    /** 当前项拍照（仅相机）→ 上传原图 → 建识别 job → 立即推进下一项，不等结果（水印由服务端统一烧录） */
    takePhoto() {
      const it = this.curItem
      if (it == null || !this.curItemIsPhoto) return
      this.shootFor(it, true, 'ai')
    },
    exceptionText(exceptionType?: string) {
      if (exceptionType == 'device_missing') return '已上报：设备确实不存在（点击可重新上报）'
      if (exceptionType == 'unable_to_capture') return '已上报：现场无法拍摄（点击可重新上报）'
      return '设备不存在 / 无法拍摄？点击这里上报异常'
    },
    /** 拍照项逃生入口：明确区分设备不存在与现场无法拍摄。 */
    reportPhotoItemMissing() {
      const it = this.curItem
      if (it == null || !this.curItemIsPhoto) return
      uni.showActionSheet({
        itemList: ['设备确实不存在', '现场无法拍摄'],
        success: (r) => {
          const exceptionType = r.tapIndex == 0 ? 'device_missing' : 'unable_to_capture'
          const label = exceptionType == 'device_missing' ? '设备确实不存在' : '现场无法拍摄'
          uni.showModal({
            title: '上报项目异常',
            content: '请拍摄现场佐证照片后提交“' + label + '”异常。',
            confirmText: '拍摄佐证',
            success: (confirm) => {
              if (confirm.confirm) this.shootFor(it, true, 'escape', exceptionType)
            }
          })
        }
      })
    },
    /** 补拍步重拍：重拍 = 重新拍照上传重新建 job，停留在补拍列表 */
    retakePhoto(it: WizardItemSnap) {
      if (it.status == 'recognizing') return
      this.shootFor(it, false, 'ai')
    },
    shootFor(it: WizardItemSnap, advance: boolean, mode: 'ai' | 'escape', exceptionType = '') {
      const wp = this.curWizPoint
      if (wp == null) return
      const pointId = wp.point_id
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          // 定标压缩（1920px/q80：仍在 AI 编码分辨率之上，不损识别；体积约为原图 1/4）。
          // 只传原图：AI 识别无水印干扰；水印由服务端在打卡后统一烧录（点位/时间/坐标/巡检员）
          compressForUpload(paths[0]).then((raw) => {
            this.overlayMsg = '照片上传中…'
            let fileId = ''
            let fileUrl = ''
            apiUploadLocal(raw)
              .then((r) => {
                fileId = r.file_id
                fileUrl = r.url
                if (mode == 'escape') {
                  return apiItemDraftPhotoAbnormal({
                    task_id: this.taskId,
                    point_id: pointId,
                    name: it.name,
                    file_ids: [fileId],
                    note: exceptionType == 'device_missing' ? '设备确实不存在，已上报异常' : '现场无法拍摄，已上报异常',
                    exception_type: exceptionType
                  }).then(() => ({ job_id: '' }))
                }
                return apiAiItemJobCreate({
                  task_id: this.taskId,
                  point_id: pointId,
                  name: it.name,
                  file_ids: [fileId]
                })
              })
              .then((j) => {
                this.overlayMsg = ''
                // 展示用服务端 URL（重启后仍可加载）；本地临时路径仅兜底
                it.photos = [fileUrl != '' ? fileUrl : raw]
                it.img_error = false
                it.file_ids = [fileId]
                it.exception_type = mode == 'escape' ? exceptionType : ''
                it.file_id = fileId
                it.job_id = j.job_id
                if (mode == 'escape') {
                  it.status = 'done'
                  it.verdict = 'abnormal'
                  it.reason = exceptionType == 'device_missing' ? '设备确实不存在，已上报异常' : '现场无法拍摄，已上报异常'
                  it.reading = ''
                  it.quality_pass = true
                  it.quality_issue = ''
                  it.pass = false
                  it.note = it.reason
                  uni.showToast({ title: '异常已上报', icon: 'success' })
                } else {
                  it.status = 'recognizing'
                  it.verdict = ''
                  it.reason = ''
                  it.reading = ''
                  it.quality_pass = true
                  it.quality_issue = ''
                  it.pass = true
                  it.note = ''
                }
                // 拍照项：立即推进下一项，不等识别结果
                if (advance) this.nextStep()
              })
              .catch((e: any) => {
                this.overlayMsg = ''
                const code = e != null && typeof e.code == 'number' ? e.code : 0
                if (code == CODE_AI_DISABLED) {
                  uni.showToast({ title: 'AI 未启用，请改用手动模式', icon: 'none' })
                  return
                }
                uni.showToast({ title: (e && e.message) || '拍照失败，请重试', icon: 'none' })
              })
          })
        }
      })
    },
    /** 感官项：正常一次过 */
    tapManualOk() {
      const it = this.curItem
      if (it == null || this.curItemIsPhoto) return
      it.pass = true
      it.note = ''
      it.verdict = 'pass'
      it.status = 'done'
      this.manualAbnormalOpen = false
      this.saveManualDraft(it)
      this.nextStep()
    },
    /** 感官项：有异常 → 展开描述输入（可跳过） */
    tapManualAbnormal() {
      const it = this.curItem
      if (it == null || this.curItemIsPhoto) return
      this.manualNote = it.note
      this.manualAbnormalOpen = true
    },
    confirmManualAbnormal() {
      const it = this.curItem
      if (it == null) return
      it.pass = false
      it.note = this.manualNote.trim()
      it.verdict = 'abnormal'
      it.status = 'done'
      this.manualAbnormalOpen = false
      this.saveManualDraft(it)
      this.nextStep()
    },
    /** 手动项选择实时落云端草稿（失败仅提示不阻断；下次进入以服务端为准） */
    saveManualDraft(it: WizardItemSnap) {
      const wp = this.curWizPoint
      if (wp == null) return
      apiItemDraftManual({
        task_id: this.taskId,
        point_id: wp.point_id,
        name: it.name,
        pass: it.pass,
        note: it.note
      }).catch(() => {
        uni.showToast({ title: '网络异常，进度可能未保存', icon: 'none' })
      })
    },
    /** 推进：下一项 → 收尾；itemIdx 溢出表示待在收尾步 */
    nextStep() {
      const wp = this.curWizPoint
      if (wp == null) return
      this.manualAbnormalOpen = false
      if (this.itemIdx < wp.items.length - 1) {
        this.itemIdx += 1
      } else {
        this.itemIdx = wp.items.length
        this.phase = 'gate'
        // 方案 B：顺向到达收尾步，全部项落定且全 pass 则自动提交（异常/质量问题会分流停住）
        this.autoSubmitGate()
      }
    },
    /**
     * 方案 B 自动提交：到达收尾步时，凭证/围栏已过 + 有模板项 → 自动走提交链路
     * （轮询未落定识别 → 汇总分流：全 pass 直接落库；补拍/异常确认停住交给人）。
     * 修改模式、无模板点位（纯拍照打卡）保持手动提交。
     */
    autoSubmitGate() {
      const wp = this.curWizPoint
      if (wp == null || this.modify || this.submitting) return
      if (this.phase != 'gate' || wp.items.length == 0) return
      if (!this.credOk) return
      if (this.curPoint != null && this.curPoint.require_fence && !this.fenceOk) return
      this.submitPoint()
    },
    /** 底部「上一项」点击：回退一项（不跨点位）；兜底到本点位第一步时直接退出（正常不会到这，起点按钮已隐藏） */
    onPrevTap() {
      if (!this.prevStep()) this.exitWizard()
    },
    /**
     * 手动填写本点位：跳逐项填写表单（redirectTo 替换向导；向导进度在云端草稿，重进可续）。
     * 已核验的扫码/NFC 凭证随 no 参数带过去，表单按编号匹配二维码/NFC 自动免核验。
     */
    goManualPoint() {
      const pt = this.curPoint
      if (pt == null) return
      let url =
        '/pages/checkin/form?task_id=' + encodeURIComponent(this.taskId) +
        '&point_id=' + encodeURIComponent(pt.point_id)
      const wp = this.curWizPoint
      const preNo = wp != null ? (wp.scannedNo != '' ? wp.scannedNo : wp.nfcCardId) : ''
      if (preNo != '') url += '&no=' + encodeURIComponent(preNo)
      uni.redirectTo({ url: url })
    },
    /** 导航栏返回键：与系统返回同口径——直接退出巡检（已拍内容在云端草稿，下次可续） */
    onBackTap() {
      if (this.overlayMsg != '' || this.submitting) return
      this.exitWizard()
    },
    /**
     * 回退：上一项 → 到场确认。只在当前点位内部回退，永不跨点位——
     * 向导里只能通过「提交本点位」到达下一点位，前面的点位必然已提交（提交即封存，
     * 要改走任务明细/扫码/NFC 的记录卡入口）。
     * 返回 true=已回退；false=已在本点位第一步（此时「上一项」按钮不显示）。
     */
    prevStep(): boolean {
      const wp = this.curWizPoint
      if (wp == null) {
        return false
      }
      this.manualAbnormalOpen = false
      if (this.phase == 'gate') {
        if (wp.items.length > 0) {
          this.itemIdx = wp.items.length - 1
          this.phase = 'items'
          return true
        }
        this.phase = 'cred'
        return true
      }
      if (this.phase == 'items') {
        if (this.itemIdx > 0) {
          this.itemIdx -= 1
          return true
        }
        if (this.needsCred) {
          this.phase = 'cred'
          return true
        }
      }
      // 本点位第一步（cred 或无凭证点位的第一项）：没有上一项
      return false
    },
    /** 提交本点位：轮询未完成的识别 job → 汇总分流（全过 / 补拍 / 异常确认） */
    submitPoint() {
      const wp = this.curWizPoint
      if (wp == null || this.submitting) return
      if (!this.credOk) {
        uni.showToast({ title: '请先完成点位核验', icon: 'none' })
        return
      }
      if (this.curPoint != null && this.curPoint.require_fence && !this.fenceOk) {
        uni.showToast({ title: '超出范围，走近一点再提交', icon: 'none' })
        return
      }
      this.submitting = true
      this.overlayMsg = 'AI 检查中…'
      this.pollPointJobs()
        .then(() => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          this.routePointResult()
        })
        .catch(() => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          uni.showToast({ title: '网络异常，请重试', icon: 'none' })
        })
    },
    /** 轮询当前点位 recognizing 的 job：1.5s 批量查，全部落定或 30s 超时（按 failed）后返回 */
    pollPointJobs(): Promise<void> {
      const wp = this.curWizPoint
      if (wp == null) return Promise.resolve()
      const items = wp.items.filter((it) => it.status == 'recognizing' && it.job_id != '')
      if (items.length == 0) return Promise.resolve()
      const startedAt = Date.now()
      return new Promise<void>((resolve, reject) => {
        const tick = () => {
          if (this.destroyed) {
            resolve()
            return
          }
          const pending = items.filter((it) => it.status == 'recognizing')
          if (pending.length == 0) {
            resolve()
            return
          }
          if (Date.now() - startedAt >= POLL_TIMEOUT) {
            pending.forEach((it) => {
              it.status = 'failed'
              it.quality_issue = '识别超时'
            })
            resolve()
            return
          }
          apiAiItemJobs(pending.map((it) => it.job_id))
            .then((jobs) => {
              if (this.destroyed) {
                resolve()
                return
              }
              const map: Record<string, (typeof jobs)[0]> = {}
              jobs.forEach((j) => {
                map[j.job_id] = j
              })
              pending.forEach((it) => {
                const j = map[it.job_id]
                if (j == null) return
                if (j.status == 'done') this.applyJob(it, j)
                if (j.status == 'failed') it.status = 'failed'
              })
              this.pollTimer = setTimeout(tick, POLL_INTERVAL)
            })
            .catch(reject)
        }
        tick()
      })
    },
    stopPoll() {
      if (this.pollTimer != null) {
        clearTimeout(this.pollTimer)
        this.pollTimer = null
      }
    },
    /** job 落定：写回识别结论；abnormal 预填描述 */
    applyJob(it: WizardItemSnap, j: { verdict: string; reason: string; reading: string; quality_pass: boolean; quality_issue: string }) {
      it.status = 'done'
      it.verdict = j.verdict
      it.reason = j.reason
      it.reading = j.reading
      it.quality_pass = j.quality_pass
      it.quality_issue = j.quality_issue
      it.pass = j.verdict != 'abnormal'
      if (j.verdict == 'abnormal' && it.note == '') it.note = j.reason
    },
    /** 该项回到待拍状态（job 过期/失败） */
    resetItem(it: WizardItemSnap) {
      it.photos = []
      it.file_ids = []
      it.job_id = ''
      it.status = 'todo'
      it.verdict = ''
      it.reason = ''
      it.reading = ''
      it.quality_pass = true
      it.quality_issue = ''
      it.img_error = false
    },
    /** 汇总分流：补拍 > 异常确认 > 全过直接提交 */
    routePointResult() {
      const wp = this.curWizPoint
      if (wp == null) return
      const retake: number[] = []
      const abnormal: number[] = []
      wp.items.forEach((it, i) => {
        if (it.judge_type != 'manual') {
          // 拍照项：未拍 / 识别失败 / 质量不合格 → 补拍
          if (it.status == 'todo' || it.status == 'failed' || (it.status == 'done' && !it.quality_pass)) {
            retake.push(i)
            return
          }
          if (it.status == 'done' && it.verdict == 'abnormal') abnormal.push(i)
        } else if (!it.pass) {
          abnormal.push(i)
        }
      })
      if (retake.length > 0) {
        this.retakeIdxs = retake
        this.phase = 'retake'
        uni.vibrateShort({})
        playVoice('blurry')
        return
      }
      if (abnormal.length > 0) {
        this.abnormalIdxs = abnormal
        this.phase = 'abnormal'
        uni.vibrateShort({})
        playVoice('abnormal')
        return
      }
      this.doCheckin('normal', [])
    },
    retakeIssue(it: WizardItemSnap): string {
      if (it.status == 'todo') return '还没拍'
      if (it.status == 'failed') return it.quality_issue != '' ? it.quality_issue : '识别失败，请重拍'
      return it.quality_issue != '' ? it.quality_issue : '照片不合格'
    },
    onAbnNoteChange(payload: { item: WizardItemSnap; value: string }) {
      payload.item.note = payload.value
    },
    /** 异常确认：确认后真正 POST /checkin → 下一处 */
    confirmAbnormalSubmit() {
      this.doCheckin('abnormal', this.abnormalIdxs.slice())
    },
    /** 真正提交打卡（ai_confirmed；已打卡未锁定点位重复提交 = 覆盖修改，服务端处理） */
    doCheckin(result: 'normal' | 'abnormal', abnIdxs: number[]) {
      const wp = this.curWizPoint
      const pt = this.curPoint
      if (wp == null || pt == null || this.submitting) return
      this.submitting = true
      this.overlayMsg = '提交中…'
      const abnSet: Record<number, boolean> = {}
      abnIdxs.forEach((i) => {
        abnSet[i] = true
      })
      const checkItems = wp.items.map((it, i) => {
        const isAbn = abnSet[i] == true
        return {
          name: it.name,
          pass: !isAbn,
          note: isAbn ? it.note : '',
          photos: it.file_ids.slice(),
          exception_type: it.exception_type ?? '',
          ai_verdict: it.verdict,
          ai_reason: it.reason,
          ai_reading: it.reading
        }
      })
      const remark = wp.items
        .filter((it, i) => abnSet[i] == true && it.note != '')
        .map((it) => it.name + '：' + it.note)
        .join('\n')
      apiCheckin({
        task_id: this.taskId,
        point_id: wp.point_id,
        checkin_type: pt.credential == 'nfc' ? 'nfc' : (wp.scannedNo != '' ? 'qrcode' : (wp.nfcCardId != '' ? 'nfc' : 'fence')),
        qrcode_no: wp.scannedNo != '' ? wp.scannedNo : undefined,
        nfc_id: pt.credential == 'nfc' || pt.credential == 'any' ? (wp.nfcCardId != '' ? wp.nfcCardId : undefined) : undefined,
        longitude: this.myLng,
        latitude: this.myLat,
        altitude: this.myAlt > 0 ? this.myAlt : undefined,
        accuracy: this.myAcc > 0 ? this.myAcc : undefined,
        client_time: fmtDateTime(new Date()),
        result: result,
        ai_confirmed: true,
        remark: remark,
        check_items: checkItems
      })
        .then(() => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          this.doneLocal += 1
          this.afterPointSubmitted()
        })
        .catch((e: any) => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          const code = e != null && typeof e.code == 'number' ? e.code : 0
          if (code == CODE_CHECKIN_LOCKED) {
            uni.showToast({ title: '已归档，不可修改', icon: 'none' })
            setTimeout(() => this.exitWizard(), 800)
            return
          }
          if (code == CODE_AI_DISABLED) {
            uni.showToast({ title: 'AI 未启用，请改用手动模式', icon: 'none' })
            return
          }
          uni.showToast({ title: (e && e.message) || '提交失败，请重试', icon: 'none' })
          this.phase = 'gate'
        })
    },
    /** 提交成功：点位从待检序列移除 → 绿勾 → 自动下一点位 / 任务完成（服务端已删草稿） */
    afterPointSubmitted() {
      if (this.modify) {
        uni.showToast({ title: '已提交修改', icon: 'success' })
        setTimeout(() => this.exitWizard(), 600)
        return
      }
      this.wizPoints.splice(this.pointIdx, 1)
      if (this.wizPoints.length == 0) {
        this.phase = 'taskDone'
        playVoice('normal')
        return
      }
      this.pointIdx = Math.min(this.pointIdx, this.wizPoints.length - 1)
      this.itemIdx = 0
      this.phase = 'pointDone'
      playVoice('normal')
      setTimeout(() => {
        if (this.destroyed || this.phase != 'pointDone') return
        this.enterPoint(false)
      }, 1200)
    },
    /** 修改模式预填：凭已有打卡逐项结论回填（best-effort，照片无法复原需重拍） */
    prefillModify(checkinId: string) {
      if (checkinId == '') return
      apiCheckinItems(checkinId)
        .then((items) => {
          if (this.destroyed) return
          const wp = this.curWizPoint
          if (wp == null) return
          wp.items.forEach((it) => {
            const found = items.find((x) => x.name == it.name)
            if (found == null) return
            if (it.judge_type == 'manual') {
              // 感官项可直接回填结论
              it.pass = found.pass
              it.note = found.pass ? '' : found.ai_reason
              it.verdict = found.pass ? 'pass' : 'abnormal'
              it.status = 'done'
            } else if (found.ai_reason != '') {
              it.note = found.ai_reason
            }
          })
        })
        .catch(() => {
          // 预填失败按全新巡检处理
        })
    },
    exitWizard() {
      // App 端 navigateBack 会触发 onBackPress，先置放行标记避免被退出确认拦截
      this.forceExit = true
      uni.navigateBack({
        fail: () => {
          this.forceExit = false
          // 页面栈异常兜底：回任务 tab，避免 navigateBack 静默失败造成假死
          uni.switchTab({ url: '/pages/tasks/today' })
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

.skeleton {
  padding: 24rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.wizard {
  flex: 1;
  padding: 24rpx;
}

/* 自定义导航栏 */
.navbar {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  z-index: 990;
}

.navbar-row {
  height: 88rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.navbar-side {
  width: 160rpx;
  height: 88rpx;
  flex-direction: row;
  align-items: center;
  padding-left: 24rpx;
}

.navbar-right {
  justify-content: flex-end;
  padding-left: 0;
  padding-right: 32rpx;
}

.navbar-back {
  font-size: 64rpx;
  font-weight: 300;
  line-height: 64rpx;
}

.navbar-title {
  font-size: 36rpx;
  font-weight: 600;
}

.navbar-space {
  height: 88rpx;
}

/* 顶部进度区：白卡 + 阴影，与页面底色分层 */
.head {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.head-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.head-progress {
  font-size: 44rpx;
  font-weight: 700;
}

.head-item-pill {
  height: 56rpx;
  border-radius: 28rpx;
  align-items: center;
  justify-content: center;
  padding: 0 24rpx;
}

.head-item-pill-text {
  font-size: 28rpx;
  font-weight: 600;
}

.head-point {
  font-size: 48rpx;
  font-weight: 700;
  margin-top: 20rpx;
}

.head-building {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.head-bar-row {
  flex-direction: row;
  align-items: center;
  margin-top: 24rpx;
}

.progress {
  height: 12rpx;
  border-radius: 6rpx;
  overflow: hidden;
}

.head-bar {
  flex: 1;
}

.progress-inner {
  height: 12rpx;
  border-radius: 6rpx;
}

.head-bar-text {
  font-size: 26rpx;
  margin-left: 16rpx;
  width: 88rpx;
  text-align: right;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

/* 凭证步：核验清单 */
.cred-title {
  font-size: 36rpx;
  font-weight: 700;
  margin-bottom: 8rpx;
}

.cred-none {
  font-size: 40rpx;
  font-weight: 600;
  text-align: center;
  padding: 24rpx 0;
}

.cred-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  min-height: 104rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
  border-bottom-color: rgba(0, 0, 0, 0.05);
}

.cred-row-name {
  font-size: 40rpx;
  font-weight: 600;
}

.cred-status {
  font-size: 32rpx;
  font-weight: 600;
}

.start-hint {
  font-size: 28rpx;
  text-align: center;
  margin-top: -8rpx;
  margin-bottom: 24rpx;
}

.btn-outline {
  height: 112rpx;
  border-radius: 20rpx;
  border-width: 2rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
}

.btn-outline-text {
  font-size: 40rpx;
  font-weight: 600;
}

/* 大按钮 */
.btn-big {
  width: 100%;
  height: 140rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 24rpx;
}

.btn-big-text {
  font-size: 44rpx;
  font-weight: 700;
}

/* 收尾步 */
.gate-card {
  align-items: center;
}

.gate-title {
  font-size: 48rpx;
  font-weight: 700;
}

.gate-stats {
  flex-direction: row;
  align-items: center;
  justify-content: space-around;
  width: 100%;
  margin-top: 32rpx;
}

.gate-stat {
  align-items: center;
}

.gate-stat-num {
  font-size: 64rpx;
  font-weight: 700;
}

.gate-stat-label {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.gate-sub {
  font-size: 30rpx;
  margin-top: 24rpx;
}

/* 完成页 */
.done-pane {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 998;
  align-items: center;
  justify-content: center;
}

.done-icon {
  font-size: 200rpx;
  font-weight: 700;
  line-height: 220rpx;
}

.done-title {
  font-size: 72rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.done-btn {
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 48rpx;
  padding: 0 64rpx;
}

.done-btn-text {
  font-size: 40rpx;
  font-weight: 700;
}

.bottom-space {
  height: 64rpx;
}

/* 底部「上一项」（返回键=退出巡检，回退只走这个按钮） */
.prev-link {
  display: flex;
  flex-direction: row;
  justify-content: center;
  padding: 28rpx 0 12rpx;
}

.prev-link-text {
  font-size: 30rpx;
  padding: 12rpx 48rpx;
}

/* 处理中弹窗（居中卡片 + 转圈） */
.overlay {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 999;
  align-items: center;
  justify-content: center;
  padding: 48rpx;
}

.overlay-dialog {
  width: 520rpx;
  border-radius: 24rpx;
  padding: 56rpx 48rpx;
  align-items: center;
}

.spinner {
  width: 72rpx;
  height: 72rpx;
  border-radius: 36rpx;
  border-width: 6rpx;
  border-style: solid;
  border-color: rgba(0, 0, 0, 0.08);
  animation: quick-spin 0.9s linear infinite;
}

@keyframes quick-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.overlay-text {
  font-size: 44rpx;
  font-weight: 700;
  margin-top: 32rpx;
}

.overlay-sub {
  font-size: 28rpx;
  margin-top: 12rpx;
}
</style>
