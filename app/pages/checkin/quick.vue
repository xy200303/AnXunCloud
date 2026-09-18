<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 自定义导航栏：左返回（返回=退出巡检）、中标题、右侧「?」=逃生（仅逐项步、非台账项显示） -->
    <view class="navbar" :style="{ backgroundColor: colors.primary, paddingTop: statusBarHeight + 'px' }">
      <view class="navbar-row">
        <view hover-class="hover-dim" class="navbar-side" @click="onBackTap">
          <text class="navbar-back" :style="{ color: colors.white }">‹</text>
        </view>
        <text class="navbar-title" :style="{ color: colors.white }">{{ manualMode ? '手动巡检' : '连续巡检' }}</text>
        <view class="navbar-side navbar-right">
          <view v-if="showEscapeEntry" hover-class="hover-dim" class="navbar-help" :style="{ borderColor: colors.white }" @click="onEscapeTap">
            <text class="navbar-help-text" :style="{ color: colors.white }">?</text>
          </view>
        </view>
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
            <text class="head-item-pill-text" :style="{ color: colors.primary }">第 {{ itemIdx + 1 }}/{{ curItemCount }} 项</text>
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

      <!-- 凭证步（无感化：核验齐全自动进入第一项，无底部操作栏） -->
      <QuickCredentialCard
        v-if="phase == 'cred'"
        :point="curPoint"
        :wiz-point="curWizPoint"
        :needs-cred="needsCred"
        :cred-ok="credOk"
        :fence-ok="fenceOk"
        :locating="locating"
        :loc-failed="locFailed"
        :distance="fenceDispDistance"
        :cred-flash="credFlash"
        :colors="colors"
        :shadow="shadow"
        @scan-fallback="scanCredential"
        @nfc-tap="onNfcRowTap"
        @retry-location="onLocTap"
        @start="startItems"
      />

      <!-- 逐项卡片步 -->
      <block v-if="phase == 'items' && curItem != null">
        <QuickItemCard
          :item="curItem"
          :is-photo="curItemIsPhoto"
          :is-spot="curItemIsSpot"
          :equip-judge="curEquipJudge"
          :manual-mode="manualMode"
          :exception-label="curItemExceptionText"
          :colors="colors"
          :shadow="shadow"
          @take-photo="takePhoto"
          @preview-photo="previewCurPhoto"
          @image-error="curItem.img_error = true"
          @retry-upload="retryCurUpload"
          @open-tags="tagPickerShow = true"
          @equip-label-photo="takeEquipLabelPhoto"
          @escape-undo="undoEscape"
        />
      </block>

      <!-- 提交卡 gate：逐项实时进度 + 置灰等待（主按钮在底部操作栏） -->
      <QuickGateCard
        v-if="phase == 'gate'"
        :item-count="curItemCount"
        :stats="gateStats"
        :rows="gateRows"
        :submit-error="gateSubmitError"
        :colors="colors"
        :shadow="shadow"
        @skip="onGateSkip"
        @retry="onGateRetry"
      />

      <!-- 补拍清算（质量不合格 / 识别失败） -->
      <block v-if="phase == 'retake'">
        <QuickIssuePanel
          :items="retakeItems"
          :colors="colors"
          :shadow="shadow"
          @preview="previewPhoto"
          @image-error="$event.img_error = true"
          @retake="retakePhoto"
          @manual-confirm="manualConfirmItem"
        />
      </block>

      <!-- 异常不再单设处置页（处置是甲方线下的事）：异常项随点位直接提交，服务端强制人工审核 -->
      <!-- 点位完成（绿勾，自动下一点位） -->
      <view v-if="phase == 'pointDone'" class="done-pane" :style="{ backgroundColor: colors.success }">
        <text class="done-icon" :style="{ color: colors.white }">✓</text>
        <text class="done-title" :style="{ color: colors.white }">本点位完成</text>
      </view>

      <!-- 任务完成 -->
      <view v-if="phase == 'taskDone'" class="done-pane" :style="{ backgroundColor: colors.success }">
        <text class="done-icon" :style="{ color: colors.white }">✓</text>
        <text class="done-title" :style="{ color: colors.white }">任务完成</text>
        <view hover-class="hover-dim" class="done-btn" :style="{ backgroundColor: colors.white }" @click="exitWizard">
          <text hover-class="hover-dim" class="done-btn-text" :style="{ color: colors.success }">返回</text>
        </view>
      </view>

      <!-- 档位切换：仅凭证步；AI 档"改用人工填写"，手动档"改回 AI 自动识别"（双向可切，进度在云端草稿） -->
      <view v-if="showManualEntry" hover-class="hover-dim" class="manual-link" @click="switchMode">
        <text hover-class="hover-dim" class="manual-link-text" :style="{ color: colors.textSecondary }">{{ manualMode ? '改回 AI 自动识别 ›' : 'AI 不好使？改用人工填写 ›' }}</text>
      </view>

      <view class="bottom-space"></view>
    </view>

    <!-- 底部操作栏（文档流底部随整页滚动，不固定；凭证步不显示）：状态-按钮对照见方案第五节 -->
    <WizardBottomBar
      v-if="barCfg.visible"
      :colors="colors"
      :primary-text="barCfg.primaryText"
      :primary-kind="barCfg.primaryKind"
      :secondary-text="barCfg.secondaryText"
      :secondary-kind="barCfg.secondaryKind"
      :prev-visible="barCfg.prevVisible"
      :manual-visible="showModeSwitch"
      :manual-text="manualMode ? '改回 AI 自动识别' : 'AI 不好使？改用人工填写'"
      @primary="onBarPrimary"
      @secondary="onBarSecondary"
      @prev="onPrevTap"
      @manual="switchMode"
    />

    <!-- 上传 / 提交中弹窗 -->
    <view v-if="overlayMsg != ''" class="overlay" :style="{ backgroundColor: colors.mask }">
      <view class="overlay-dialog" :style="{ backgroundColor: colors.bgCard }">
        <view class="spinner" :style="{ borderTopColor: colors.primary }"></view>
        <text class="overlay-text" :style="{ color: colors.textPrimary }">{{ overlayMsg }}</text>
        <text class="overlay-sub" :style="{ color: colors.textSecondary }">{{ overlaySub }}</text>
      </view>
    </view>

    <!-- 逃生「?」：异常类型面板 + 佐证拍摄确认（自绘，替代原生 showActionSheet/showModal）；抽查项多一个「标签磨损无法辨认」；无法拍摄/相机故障选完直接上报不进此弹窗 -->
    <AppActionSheet
      :visible="escapeSheetShow"
      :items="escapeSheetItems"
      @update:visible="escapeSheetShow = $event"
      @select="onEscapeSheetSelect"
    />
    <AppDialog
      :visible="escapeDlgShow"
      kind="warning"
      title="上报无法检查"
      :content="'请拍摄现场佐证照片后提交“' + escapeTypeText(escapeType) + '”。'"
      confirm-text="拍摄佐证"
      cancel-text="取消"
      @update:visible="escapeDlgShow = $event"
      @confirm="onEscapeDlgConfirm"
    />
    <!-- 记录已归档提示（自绘，替代原生 showModal）：确认后退出向导 -->
    <AppDialog
      :visible="lockedDlgShow"
      kind="warning"
      title="提示"
      content="该记录已归档，不可修改"
      confirm-text="知道了"
      @update:visible="lockedDlgShow = $event"
      @confirm="exitWizard"
    />

    <!-- 观察点下拉多选半屏（勾选异常观察点） -->
    <QuickTagPicker
      :visible="tagPickerShow"
      title="观察点"
      :tags="pickerTags"
      :selected="pickerSelected"
      confirm-text="完成"
      :colors="colors"
      @update:visible="tagPickerShow = $event"
      @toggle="toggleCurTag"
      @confirm="tagPickerShow = false"
    />
    <!-- 「⚠ 有异常」底部半屏：观察点多选 + 备注 + 确认，确认后自动下一项 -->
    <QuickTagPicker
      :visible="abnSheetShow"
      title="有异常"
      :tags="pickerTags"
      :selected="pickerSelected"
      with-note
      :note="manualNote"
      confirm-text="确认异常，下一项"
      :colors="colors"
      @update:visible="abnSheetShow = $event"
      @toggle="toggleCurTag"
      @update:note="manualNote = $event"
      @confirm="confirmManualAbnormal"
    />
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
  apiItemDraftDelete,
  apiPointCredSave,
  CODE_AI_DISABLED,
  CODE_CHECKIN_LOCKED,
  CheckinReqPayload,
  EquipmentAutoJudge,
  ItemDraft,
  PointCredDraft,
  TaskPoint
} from '@/services/api'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { NETWORK_ERR_PREFIX, enqueueOfflineCheckin } from '@/utils/offline'
import { extractPointCode, resolvePointCode } from '@/utils/scan'
import { getLocationGcj02, getLocationCached } from '@/utils/geo'
import { playVoice } from '@/utils/voice'
import { compressForUpload } from '@/utils/image'
import { WizardPointSnap, WizardItemSnap } from '@/utils/checkinWizard'
import QuickItemCard from '@/components/QuickItemCard.vue'
import QuickIssuePanel from '@/components/QuickIssuePanel.vue'
import QuickCredentialCard from '@/components/QuickCredentialCard.vue'
import QuickGateCard from '@/components/QuickGateCard.vue'
import WizardBottomBar from '@/components/WizardBottomBar.vue'
import QuickTagPicker from '@/components/QuickTagPicker.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppActionSheet from '@/components/AppActionSheet.vue'

/** 向导阶段：cred 凭证 / items 逐项 / gate 提交本点位 / retake 补拍 / pointDone 点位完成 / taskDone 任务完成 */
type Phase = 'cred' | 'items' | 'gate' | 'retake' | 'pointDone' | 'taskDone'

/** gate 逐项进度行（与 QuickGateCard rows prop 结构一致） */
type GateRow = { key: string; name: string; stage: string; canSkip: boolean; pending: boolean; item: WizardItemSnap }

/** AI 轮询节奏：pending 时 1.5s 间隔批量查（契约 §2） */
const POLL_INTERVAL = 1500
/** 轮询总时长：须覆盖 队列等待 + 后端 AI 超时（默认 180s），留足余量；后端任务必然落定（done/failed），不单方面判死 */
const POLL_TIMEOUT = 190000

/** 后台任务（拍照异步化：压缩→上传→建 job 离主流程，串行执行弱网防雪崩） */
type BgJob = {
  it: WizardItemSnap
  pointId: string
  /** 本地照片路径（槽位已即时展示；上传成功后才换成服务端 URL） */
  raw: string
  mode: 'ai' | 'escape' | 'manual'
  exceptionType: string
  /** 拍摄定位回写落定 promise（建 job/落草稿前短等，坐标尽量带齐；null=无） */
  shootReady: Promise<void> | null
}

/** 底部操作栏配置（状态-按钮对照表，方案 §五） */
type BarCfg = {
  visible: boolean
  primaryText: string
  primaryKind: 'primary' | 'success' | 'danger' | 'disabled'
  primaryAction: '' | 'take-photo' | 'next' | 'manual-ok' | 'submit' | 'skip-ai'
  secondaryText: string
  secondaryKind: 'primary' | 'success' | 'danger' | 'disabled'
  secondaryAction: '' | 'manual-abnormal' | 'next'
  prevVisible: boolean
}

type QuickData = {
  colors: ColorTokens
  shadow: string
  /** 状态栏高度（px），自定义导航栏占位用 */
  statusBarHeight: number
  taskId: string
  pointIdParam: string
  /** true = 单点位修改模式（覆盖提交） */
  modify: boolean
  /** true = 手动档向导（mode=manual）：不建 AI job，拍照后「这项正常吗？」，提交 ai_confirmed=false 转人工复核 */
  manualMode: boolean
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
  /** 服务端 AI 识别能力是否启用（任务详情透出）；未启用强制手动档且不显示档位切换 */
  aiEnabled: boolean
  /** 自动提交武装：仅会话内逐项顺向推进到 gate 时置真；断点恢复/切档重进不武装（停在 gate 等用户点提交） */
  gateArmed: boolean
  /** 向导范围内点位（仅未提交；快照持久化对象） */
  wizPoints: WizardPointSnap[]
  pointIdx: number
  itemIdx: number
  phase: Phase
  /** 「有异常」半屏 / 观察点多选半屏 */
  abnSheetShow: boolean
  tagPickerShow: boolean
  manualNote: string
  /** 补拍 / 异常处置列表（当前点位 items 下标） */
  retakeIdxs: number[]
  abnormalIdxs: number[]
  locating: boolean
  locFailed: boolean
  /** 凭证步定位失败已自动重试过一次（方案 §13.2：失败自动重试一次，仍失败给「点我重试」） */
  locRetried: boolean
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
  /** 收尾步上次提交失败原因（空 = 无；gate 卡红色状态条，重试成功后清除） */
  gateSubmitError: string
  /** 后台任务队列与轮询（拍照全异步：上传/建job/轮询全部离主流程） */
  bgQueue: BgJob[]
  bgRunning: boolean
  /** 后台轮询活跃标记（防并行多条轮询链） */
  bgPolling: boolean
  bgPollTimer: any
  /** job_id → 开始轮询时间（190s 超时判 failed） */
  jobStartTimes: Record<string, number>
  /** 已「跳过识别」的 job（轮询落定不再回写，保持人工确认结论） */
  skippedJobs: Record<string, boolean>
  /** 页面已卸载（停止轮询回调写状态） */
  destroyed: boolean
  /** 遮罩看门狗定时器 */
  overlayWatchdog: any
  /** 相机链路锁，避免连续点击拉起多个相机 */
  captureBusy: boolean
  captureToken: number
  /** 手动退出放行标记（exitWizard 时 onBackPress 不拦截） */
  forceExit: boolean
  /** 逃生入口：异常类型面板 / 佐证确认弹窗状态 */
  escapeSheetShow: boolean
  escapeDlgShow: boolean
  lockedDlgShow: boolean
  /** 面板选择确认期间暂存的当前项与异常类型 */
  escapeItem: WizardItemSnap | null
  escapeType: string
  /** 凭证步：核验通过绿色过渡态 / 自动进入防重 / 内嵌扫码窗 */
  credFlash: boolean
  credAutoStarted: boolean
  scanView: any
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

/** 由点位模板生成向导项初始状态（台账有效期项按服务端 auto_judge 预置结论，巡检员不可改） */
function freshItem(name: string, requirement: string, guide: string, judgeType: string, autoJudge: EquipmentAutoJudge | null, tags: string[], photoRequired: string): WizardItemSnap {
  const it: WizardItemSnap = {
    name: name,
    requirement: requirement,
    guide: guide,
    judge_type: judgeType,
    auto_judge: autoJudge,
    tags: tags,
    abnormal_tags: [],
    photo_required: photoRequired,
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
    note: '',
  }
  // 台账有效期合成项（v1.6 逐台独立）：按服务端判定预置展示态，初始化即落定；
  // 不上送、不参与人工分流（服务端提交时实时逐台判定，逾期记录级强制异常兜底）
  if (judgeType == 'equipment_validity' && autoJudge != null) {
    it.status = 'done'
    if (autoJudge.status == 'overdue') {
      it.pass = false
      it.verdict = 'abnormal' // 收尾统计可见异常台数；提交结果以服务端为准
      it.note = '设备维保逾期：编号 ' + autoJudge.equipment_code + ' 到期日 ' + autoJudge.next_due_date
      if (autoJudge.has_pending_register) it.note += '（已登记维保待确认）'
    } else {
      it.pass = true
      it.verdict = 'pass'
      if (autoJudge.status == 'no_data') it.note = '台账数据缺失待补录'
    }
  }
  return it
}

/** 由点位模板生成向导点位初始状态 */
function freshPoint(p: TaskPoint): WizardPointSnap {
  return {
    point_id: p.point_id,
    status: 'doing',
    scannedNo: '',
    nfcCardId: '',
    items: (p.check_items || []).map((c) => freshItem(c.name, c.requirement, c.guide ?? '', c.judge_type, c.auto_judge ?? null, c.tags ?? [], c.photo_required ?? ''))
  }
}

export default {
  components: { QuickItemCard, QuickIssuePanel, QuickCredentialCard, QuickGateCard, WizardBottomBar, QuickTagPicker, AppDialog, AppActionSheet },
  data(): QuickData {
    return {
      colors: Colors,
      shadow: ShadowCard,
      statusBarHeight: 0,
      taskId: '',
      pointIdParam: '',
      modify: false,
      manualMode: false,
      preVerifiedNo: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      taskPoints: [] as TaskPoint[],
      totalPoints: 0,
      doneBase: 0,
      doneLocal: 0,
      aiEnabled: true,
      gateArmed: false,
      wizPoints: [] as WizardPointSnap[],
      pointIdx: 0,
      itemIdx: 0,
      phase: 'cred',
      abnSheetShow: false,
      tagPickerShow: false,
      manualNote: '',
      retakeIdxs: [] as number[],
      abnormalIdxs: [] as number[],
      locating: false,
      locFailed: false,
      locRetried: false,
      hasLoc: false,
      myLng: 0,
      myLat: 0,
      myAlt: 0,
      myAcc: 0,
      distance: -1,
      overlayMsg: '',
      submitting: false,
      gateSubmitError: '',
      bgQueue: [] as BgJob[],
      bgRunning: false,
      bgPolling: false,
      bgPollTimer: null,
      jobStartTimes: {} as Record<string, number>,
      skippedJobs: {} as Record<string, boolean>,
      destroyed: false,
      overlayWatchdog: null,
      captureBusy: false,
      captureToken: 0,
      forceExit: false,
      escapeSheetShow: false,
      escapeDlgShow: false,
      /** 记录已归档提示弹窗（确认后退出向导） */
      lockedDlgShow: false,
      escapeItem: null,
      escapeType: '',
      credFlash: false,
      credAutoStarted: false,
      scanView: null
    }
  },
  computed: {
    /** 点位索引缓存：避免进度区和当前点位在每次响应式更新时重复线性查找 */
    taskPointIndex(): Record<string, { point: TaskPoint; ordinal: number }> {
      const index: Record<string, { point: TaskPoint; ordinal: number }> = {}
      this.taskPoints.forEach((point, ordinal) => {
        index[point.point_id] = { point: point, ordinal: ordinal + 1 }
      })
      return index
    },
    curWizPoint(): WizardPointSnap | null {
      if (this.pointIdx < 0 || this.pointIdx >= this.wizPoints.length) return null
      return this.wizPoints[this.pointIdx]
    },
    curPoint(): TaskPoint | null {
      const wp = this.curWizPoint
      if (wp == null) return null
      return this.taskPointIndex[wp.point_id]?.point || null
    },
    curItem(): WizardItemSnap | null {
      const wp = this.curWizPoint
      if (wp == null || this.itemIdx < 0 || this.itemIdx >= wp.items.length) return null
      return wp.items[this.itemIdx]
    },
    curItemCount(): number {
      return this.curWizPoint != null ? this.curWizPoint.items.length : 0
    },
    /** 当前项是否拍照项（manual 感官项与 equipment_validity 台账有效期项之外的类型，含标签抽查合成项；缺省按拍照项） */
    curItemIsPhoto(): boolean {
      return this.curItem != null && this.curItem.judge_type != 'manual' && this.curItem.judge_type != 'equipment_validity'
    },
    /** 当前项是否标签抽查合成项（交互同普通拍照项；仅逃生面板多「标签磨损无法辨认」与读数行展示用） */
    curItemIsSpot(): boolean {
      return this.curItem != null && this.curItem.judge_type == 'equipment_date_spot'
    },
    /** 当前台账有效期项的自动判定（非该类型/未关联台账为 null，走人工分支） */
    curEquipJudge(): EquipmentAutoJudge | null {
      const it = this.curItem
      if (it == null || it.judge_type != 'equipment_validity') return null
      return it.auto_judge ?? null
    },
    curItemExceptionText(): string {
      return this.curItem == null ? '' : this.exceptionText(this.curItem.exception_type)
    },
    /** 逃生面板选项：设备不存在（拍佐证）/ 现场无法拍摄 / 相机故障（拍不了直接上报）；抽查项追加「标签磨损无法辨认」（exception_type=label_missing，仅抽查项可用） */
    escapeSheetItems(): string[] {
      const base = ['设备确实不存在', '现场无法拍摄', '相机故障']
      if (this.curItemIsSpot) base.push('标签磨损无法辨认')
      return base
    },
    /** 点位序号（任务全部点位中的位置，1 起） */
    pointOrdinal(): number {
      const pt = this.curPoint
      if (pt == null) return 1
      return this.taskPointIndex[pt.point_id]?.ordinal || 1
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
    /** 围栏未超出（无围栏点位恒 true；云端凭证草稿已核验围栏的直接通过，无需等重新定位） */
    fenceOk(): boolean {
      const pt = this.curPoint
      if (pt == null || !pt.require_fence) return true
      if (pt.longitude == 0 && pt.latitude == 0) return true // 点位未录坐标：后端跳过围栏校验，前端一致放行
      const wp = this.curWizPoint
      if (wp != null && wp.fence_distance != null && wp.fence_distance >= 0 && wp.fence_distance <= pt.fence_radius) return true
      return this.distance >= 0 && this.distance <= pt.fence_radius
    },
    /** 凭证卡围栏行展示距离：实时定位优先，未定位时回退云端草稿恢复的核验距离 */
    fenceDispDistance(): number {
      if (this.distance >= 0) return this.distance
      const wp = this.curWizPoint
      if (wp != null && wp.fence_distance != null && wp.fence_distance >= 0) return wp.fence_distance
      return -1
    },
    retakeItems(): WizardItemSnap[] {
      const wp = this.curWizPoint
      if (wp == null) return []
      return this.retakeIdxs.map((i) => wp.items[i]).filter((it) => it != null)
    },
    /** gate 逐项实时进度行：待补传 / 上传中 / 排队中 / 识别中（落定即从列表勾掉） */
    gateRows(): GateRow[] {
      const wp = this.curWizPoint
      if (wp == null) return []
      const rows: GateRow[] = []
      wp.items.forEach((it, i) => {
        if ((it.pending_local ?? '') != '') {
          rows.push({ key: it.name + ':' + i, name: it.name, stage: '待补传', canSkip: false, pending: true, item: it })
          return
        }
        if (it.status == 'recognizing') {
          const stage = it.file_ids.length == 0 ? '上传中…' : it.job_id == '' ? '排队中…' : '识别中…'
          rows.push({ key: it.name + ':' + i, name: it.name, stage: stage, canSkip: it.job_id != '', pending: false, item: it })
        }
      })
      return rows
    },
    /** gate 未落定项数（>0 时提交按钮置灰「等待处理（还剩 N 项）」） */
    gateRemain(): number {
      return this.gateRows.length
    },
    gateSettled(): boolean {
      return this.gateRemain == 0
    },
    /** 收尾步统计：已落定 / 处理中 / 异常 / 无法检查（逃生项 exception_type 非空，不计入异常） */
    gateStats(): { done: number; processing: number; abnormal: number; escaped: number } {
      const wp = this.curWizPoint
      const r = { done: 0, processing: 0, abnormal: 0, escaped: 0 }
      if (wp == null) return r
      for (let i = 0; i < wp.items.length; i++) {
        const it = wp.items[i]
        if ((it.pending_local ?? '') != '' || it.status == 'recognizing') r.processing += 1
        else if ((it.exception_type ?? '') != '') r.escaped += 1
        else if (it.verdict == 'abnormal' || it.abnormal_tags.length > 0) r.abnormal += 1
        else r.done += 1
      }
      return r
    },
    /** 弹窗副提示 */
    overlaySub(): string {
      if (this.overlayMsg.indexOf('AI') >= 0) return '正在识别照片，一般几秒内完成'
      return '请稍候，不要退出页面'
    },
    /** 观察点半屏数据源（当前项；无当前项时空数组） */
    pickerTags(): string[] {
      return this.curItem != null ? this.curItem.tags : []
    },
    pickerSelected(): string[] {
      return this.curItem != null ? this.curItem.abnormal_tags : []
    },
    /** 导航条右侧逃生「?」：逐项步所有非台账项显示（设备不存在/无法拍摄与是否需要拍照无关） */
    showEscapeEntry(): boolean {
      if (this.phase != 'items' || this.curItem == null) return false
      if (this.overlayMsg != '' || this.submitting) return false
      return this.curItem.judge_type != 'equipment_validity'
    },
    /** 手动模式入口：主推进阶段 + 非修改/手动档 + 有当前点位（修改模式点位已打卡；手动档已是手动） */
    /** 逐项步底栏档位切换链：AI 档/手动档双向可切（草稿在云端，互切不丢进度） */
    showModeSwitch(): boolean {
      if (!this.aiEnabled) return false
      if (this.modify || this.overlayMsg != '' || this.submitting || this.captureBusy) return false
      if (this.curPoint == null) return false
      return this.phase == 'items'
    },
    showManualEntry(): boolean {
      if (!this.aiEnabled) return false
      if (this.modify || this.overlayMsg != '' || this.submitting || this.captureBusy) return false
      if (this.curPoint == null) return false
      // 档位在进门时定：仅凭证步提供切手动档入口（方案 §13.4）
      return this.phase == 'cred'
    },
    /** 是否在本点位第一步（无上一项可回退，底部「上一项」隐藏；退出走返回键）。
     *  与 pointIdx 无关：中途进入（扫码/NFC/点位清单）的第一个点位就是进入者的第一页 */
    atWizardStart(): boolean {
      if (this.phase == 'cred') return true
      return this.phase == 'items' && this.itemIdx == 0 && !this.needsCred
    },
    /**
     * 底部操作栏状态-按钮对照（方案 §五唯一真相）：
     * 逐项 必拍未拍=[📷 拍照片]；选拍/免拍未拍与手动档已拍=[✓ 正常][⚠ 有异常]；
     * AI 档已拍（回退查看）/台账抽查就绪/逃生已上报=[下一项 ›]；
     * gate 全部落定=[提交本点位]，未落定=[等待处理（还剩 N 项）]置灰；
     * 补拍=[重新提交本点位]；异常处置=[确认，去下一处]；凭证步不显示操作栏。
     */
    barCfg(): BarCfg {
      const none: BarCfg = { visible: false, primaryText: '', primaryKind: 'primary', primaryAction: '', secondaryText: '', secondaryKind: 'danger', secondaryAction: '', prevVisible: false }
      if (!this.loaded || this.loading) return none
      const ph = this.phase
      const prevOk = (ph == 'items' || ph == 'gate') && !this.atWizardStart && this.overlayMsg == '' && !this.submitting
      const single = (text: string, kind: BarCfg['primaryKind'], action: BarCfg['primaryAction'], prev: boolean): BarCfg => ({
        visible: true, primaryText: text, primaryKind: kind, primaryAction: action, secondaryText: '', secondaryKind: 'danger', secondaryAction: '', prevVisible: prev
      })
      const dual = (prev: boolean): BarCfg => ({
        visible: true, primaryText: '✓ 正常', primaryKind: 'success', primaryAction: 'manual-ok', secondaryText: '⚠ 有异常', secondaryKind: 'danger', secondaryAction: 'manual-abnormal', prevVisible: prev
      })
      if (ph == 'items') {
        const it = this.curItem
        if (it == null) return none
        const escaped = (it.exception_type ?? '') != ''
        if (this.curEquipJudge != null) return single('下一项 ›', 'primary', 'next', prevOk)
        if (escaped) return single('下一项 ›', 'primary', 'next', prevOk)
        if (this.manualMode) {
          // 手动档必拍项未拍：唯一拍照入口是底栏大按钮；已拍/选拍/免拍：作答双按钮
          if (it.judge_type != 'manual' && (it.photo_required ?? '') == 'required' && it.file_ids.length == 0 && it.photos.length == 0) {
            return single('📷 拍照片', 'primary', 'take-photo', prevOk)
          }
          if (it.status == 'done') return single('下一项 ›', 'primary', 'next', prevOk)
          return dual(prevOk)
        }
        if (it.judge_type == 'manual') {
          // 感官项：作答双按钮；已作答（回退查看）：下一项
          if (it.status == 'done') return single('下一项 ›', 'primary', 'next', prevOk)
          return dual(prevOk)
        }
        // AI 档拍照/抽查项：未拍=拍照大按钮；已拍无停留态（拍完自动跳走），回退查看=下一项；
        // 失败/不合格：重拍走照片槽「重新拍照」（唯一重拍入口），失败另给「跳过识别」次级按钮
        if (it.status == 'failed') {
          return {
            visible: true, primaryText: '跳过识别', primaryKind: 'primary', primaryAction: 'skip-ai',
            secondaryText: '下一项 ›', secondaryKind: 'primary', secondaryAction: 'next', prevVisible: prevOk
          }
        }
        if (it.status == 'todo') return single('📷 拍照片', 'primary', 'take-photo', prevOk)
        return single('下一项 ›', 'primary', 'next', prevOk)
      }
      if (ph == 'gate') {
        if (!this.gateSettled) return single('等待处理（还剩 ' + this.gateRemain + ' 项）', 'disabled', '', prevOk)
        return single('提交本点位', 'success', 'submit', prevOk)
      }
      if (ph == 'retake') {
        if (!this.gateSettled) return single('等待处理（还剩 ' + this.gateRemain + ' 项）', 'disabled', '', false)
        return single('重新提交本点位', 'success', 'submit', false)
      }
      return none
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointIdParam = options && options.point_id ? String(options.point_id) : ''
    this.modify = options != null && options.mode == 'modify'
    this.manualMode = options != null && options.mode == 'manual'
    if (options && options.no) {
      this.preVerifiedNo = String(options.no).trim()
    }
    const sys = uni.getSystemInfoSync()
    this.statusBarHeight = sys.statusBarHeight != null ? sys.statusBarHeight : 0
    uni.onNetworkStatusChange(this.onNetChange)
    this.load()
  },
  onShow() {
    // 回到页面（如接电话切回）：自动补传待补传照片（无网/处理中自动跳过）
    this.retryPendingItems()
    this.ensureBgPoll()
    // 回到凭证步：重开内嵌扫码窗（onHide 已关闭，避免原生视图遮盖其他页面）
    if (this.loaded && this.phase == 'cred') this.openCredScan()
  },
  onHide() {
    // 内嵌扫码窗是原生视图、盖在 webview 上层：页面不可见时关闭
    this.closeCredScan()
  },
  onUnload() {
    this.destroyed = true
    this.captureToken += 1
    this.captureBusy = false
    this.bgQueue = []
    uni.offNetworkStatusChange(this.onNetChange)
    this.stopBgPoll()
    this.closeCredScan()
    if (this.overlayWatchdog != null) {
      clearTimeout(this.overlayWatchdog)
      this.overlayWatchdog = null
    }
  },
  onBackPress(): boolean {
    // 手动退出（exitWizard 的 navigateBack 在 App 端同样触发 onBackPress）：放行
    if (this.forceExit) return false
    // 遮盖层（上传/提交中）：拦截返回并提示，避免用户误以为卡死
    if (this.overlayMsg != '' || this.submitting || this.captureBusy) {
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
        this.captureToken += 1
        this.captureBusy = false
        uni.showToast({ title: '网络较慢，请检查后重试', icon: 'none' })
      }, 75000)
    },
    /** 内嵌扫码窗生命周期收进 phase 状态机：进凭证步开，离开凭证步必须 close */
    phase(v: Phase) {
      if (v == 'cred') this.openCredScan()
      else this.closeCredScan()
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
          this.aiEnabled = res.ai_enabled ?? false
          // AI 未启用：整任务强制手动档（没有可切回的 AI 档）
          if (!this.aiEnabled) this.manualMode = true
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
            apiPointCredSave({ task_id: this.taskId, point_id: wp.point_id, checkin_type: 'qrcode', cred_no: this.preVerifiedNo, fence_distance: 0 })
          }
          if (pt.nfc_id != '' && pt.nfc_id == this.preVerifiedNo) {
            wp.nfcCardId = this.preVerifiedNo
            apiPointCredSave({ task_id: this.taskId, point_id: wp.point_id, checkin_type: 'nfc', cred_no: this.preVerifiedNo, fence_distance: 0 })
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
        // 识别中的项凭 job_id 批量查一次：仍 pending 转入后台轮询；failed/过期回退待拍
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
    /** 云端草稿重建：逐项照片/AI 结论/手动项选择 + 凭证核验结果全部来自服务端草稿（巡检进度的唯一事实来源，
     *  本地不存快照）；按点位并行拉取（凭证草稿在按点位查询的响应顶层），单点失败按该点全新巡检处理 */
    restoreDrafts(): Promise<void> {
      const wps = this.wizPoints.slice()
      if (wps.length == 0) return Promise.resolve()
      return Promise.all(
        wps.map((wp) =>
          apiItemDrafts(this.taskId, wp.point_id)
            .then((res) => ({ wp: wp, items: res.items, credential: res.credential }))
            .catch(() => null)
        )
      ).then((results) => {
        if (this.destroyed) return
        results.forEach((r) => {
          if (r == null) return
          this.applyPointDrafts(r.wp, r.items)
          this.applyCredDraft(r.wp, r.credential)
        })
      })
    },
    /** 单点位逐项草稿套用：按 draft_kind 单字段分发（ai=识别结论 / manual=人工结论 / escape=已逃生）；
     *  shoot_* 拍摄时空信息随草稿带回，再次保存坐标不丢 */
    applyPointDrafts(wp: WizardPointSnap, list: ItemDraft[]) {
      list.forEach((d) => {
        const it = wp.items.find((x) => x.name == d.item_name)
        if (it == null) return
        it.shoot_lng = d.shoot_lng
        it.shoot_lat = d.shoot_lat
        it.shoot_at = d.shoot_at
        it.file_ids = d.file_ids.slice()
        it.photos = d.photos.slice()
        it.img_error = false
        // 逃生草稿：恢复为已上报「无法检查」（escaped 不是异常：不标 verdict、不进异常统计/处置）
        if (d.draft_kind == 'escape') {
          it.exception_type = d.exception_type ?? ''
          it.abnormal_tags = []
          it.pass = false
          it.verdict = ''
          it.note = d.ai_reason != '' ? d.ai_reason : d.manual_note
          it.status = 'done'
          return
        }
        // 人工结论草稿：感官项 + 手动档向导的模板项 + 手动档答过又切回 AI 档的项（拍照项照片/结论/tag 都走 manual 草稿）
        if (d.draft_kind == 'manual') {
          it.abnormal_tags = (d.abnormal_tags ?? []).filter((t) => it.tags.indexOf(t) >= 0)
          it.exception_type = ''
          // 台账有效期合成项无草稿，每次按服务端最新判定重建
          if (d.manual_pass == null) return
          // 必拍项的草稿没有照片 = 无效完成（老数据/异常残留），不恢复，保持待拍
          if (it.judge_type != 'manual' && (it.photo_required ?? '') == 'required' && d.file_ids.length == 0) return
          it.pass = d.manual_pass
          it.note = d.manual_pass ? '' : d.manual_note
          it.verdict = d.manual_pass ? 'pass' : 'abnormal'
          it.status = 'done'
          // 非感官项的人工结论（手动档答过切回 AI 档）：标来源，提交时 ai_verdict 置空转人工复核（服务端校验同口径）
          if (it.judge_type != 'manual') it.manual_confirmed = true
          return
        }
        // 识别草稿（draft_kind=ai）：照片 + AI 识别结论（异常观察点 tag 随草稿恢复；抽查项日期由服务端从 ai_reading 解析，前端不展示录入）
        it.exception_type = d.exception_type ?? ''
        it.job_id = d.job_id
        it.abnormal_tags = (d.abnormal_tags ?? []).filter((t) => it.tags.indexOf(t) >= 0)
        if (d.ai_status == 'done') {
          this.applyJob(it, {
            verdict: d.ai_verdict,
            reason: d.ai_reason,
            reading: d.ai_reading,
            quality_pass: d.quality_pass,
            quality_issue: d.quality_issue,
            abnormal_tags: d.abnormal_tags ?? []
          })
        } else if (d.ai_status == 'pending') {
          it.status = 'recognizing'
        } else if (d.ai_status == 'failed') {
          it.status = 'failed'
          it.reason = d.ai_reason
        }
      })
    },
    /** 凭证核验草稿恢复：扫码/NFC 回填编号（会话内预核验优先），围栏回填核验距离使围栏判定直接通过（方案 §14.2） */
    applyCredDraft(wp: WizardPointSnap, cred: PointCredDraft | null) {
      if (cred == null || cred.checkin_type == '') return
      wp.cred_type = cred.checkin_type
      wp.cred_verified_at = cred.verified_at ?? ''
      if (cred.checkin_type == 'qrcode') {
        if (wp.scannedNo == '') wp.scannedNo = cred.cred_no
      } else if (cred.checkin_type == 'nfc') {
        if (wp.nfcCardId == '') wp.nfcCardId = cred.cred_no
      } else if (cred.checkin_type == 'fence') {
        wp.fence_distance = cred.fence_distance
      }
    },
    /** 恢复快照时对识别中的 job 批量查一次状态；仍 pending 的转入后台轮询 */
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
          let hasPending = false
          targets.forEach((t) => {
            const j = map[t.jobId]
            if (j == null || j.status == 'failed') {
              // job 过期/失败 → 该项回到待拍
              this.resetItem(t.it)
              return
            }
            if (j.status == 'done') {
              this.applyJob(t.it, j)
              return
            }
            // 仍 pending：转入后台轮询（重新计时）
            this.jobStartTimes[t.jobId] = Date.now()
            hasPending = true
          })
          if (hasPending) this.ensureBgPoll()
          this.maybeGateSettled()
        })
        .catch(() => {
          // 查询失败不阻断：保持 recognizing，转后台轮询继续等
          targets.forEach((t) => {
            this.jobStartTimes[t.jobId] = Date.now()
          })
          this.ensureBgPoll()
        })
    },
    /** 进入当前点位：定位 + 决定首屏（凭证步 / 逐项 / 收尾） */
    enterPoint(resume: boolean) {
      this.clampIndices()
      const wp = this.curWizPoint
      this.abnSheetShow = false
      this.tagPickerShow = false
      this.retakeIdxs = []
      this.abnormalIdxs = []
      this.credFlash = false
      this.credAutoStarted = false
      this.locRetried = false
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
      // 断点恢复/切档重进停在收尾步：不自动提交（gateArmed=false），等用户自己点「提交本点位」
      this.gateArmed = false
      // 内嵌扫码窗：phase 未变化时 watcher 不触发（如同相位连续进点位），这里显式开/关
      if (this.phase == 'cred') this.openCredScan()
      else this.closeCredScan()
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
          // 点位未录坐标（0,0）时距离无意义，不计算
          if (this.curPoint != null && (this.curPoint.longitude != 0 || this.curPoint.latitude != 0)) {
            this.distance = Math.round(
              haversine(loc.longitude, loc.latitude, this.curPoint.longitude, this.curPoint.latitude)
            )
            // 围栏通过即落凭证草稿（断点恢复直接放行）；仅纯围栏点位——有扫码/NFC 的点位凭证以编号草稿为准，
            // 避免单行 upsert 被围栏行覆盖导致扫码编号丢失
            const pt = this.curPoint
            if (pt != null && pt.require_fence && this.distance <= pt.fence_radius && pt.credential != 'qrcode' && pt.credential != 'nfc' && pt.credential != 'any') {
              this.saveCredDraft('fence', '', this.distance)
            }
          }
          this.locating = false
          // 凭证步：定位回来围栏刚满足条件，核验齐全则自动进入第一项（方案 §13.2 自动获取一次）
          if (this.phase == 'cred') this.maybeAutoStartCred()
          // 定位回来时正停在收尾步：围栏刚满足条件，补一次自动提交触发
          if (this.phase == 'gate') this.autoSubmitGate()
        },
        () => {
          this.locating = false
          // 凭证步定位失败自动重试一次，仍失败给「点我重试」（方案 §13.2）
          if (this.phase == 'cred' && !this.locRetried) {
            this.locRetried = true
            this.locate()
            return
          }
          this.locFailed = true
        }
      )
    },
    onLocTap() {
      // 非定位中即可点击重新定位（走近后主动刷新围栏距离，不必等失败）
      if (!this.locating) {
        this.locRetried = true // 手动重试后不再自动重试
        this.locate()
      }
    },
    /**
     * 凭证步核验齐全 → 「✓ 核验通过」绿色过渡 0.6s → 自动进入第一项（方案 §十三：取消「开始检查」按钮）。
     * 触发点：扫码成功 / NFC 贴卡命中 / 定位回来。
     */
    maybeAutoStartCred() {
      if (this.destroyed || this.phase != 'cred' || this.credAutoStarted) return
      if (!this.needsCred) return
      if (!this.credOk || !this.fenceOk) return
      this.credAutoStarted = true
      this.credFlash = true
      this.closeCredScan()
      uni.vibrateShort({})
      setTimeout(() => {
        if (this.destroyed) return
        this.credFlash = false
        if (this.phase != 'cred') return
        this.startItems()
      }, 600)
    },
    /** 凭证步全屏扫码（内嵌扫码窗故障时的备用入口，uni.scanCode） */
    scanCredential() {
      uni.scanCode({
        onlyFromCamera: true, // 禁相册选图防代扫
        success: (res) => {
          this.onCredScanResult(res.result)
        },
        fail: (err) => {
          const msg = err && err.errMsg ? err.errMsg : ''
          if (msg.indexOf('cancel') < 0 && msg != '') {
            uni.showToast({ title: '扫码失败：' + msg, icon: 'none' })
          }
        }
      })
    },
    /** 凭证核验通过即落云端草稿（断点恢复用；apiPointCredSave 内部已吞错，弱网不阻塞） */
    saveCredDraft(type: 'qrcode' | 'nfc' | 'fence', credNo: string, fenceDistance: number) {
      const wp = this.curWizPoint
      if (wp == null || this.modify) return
      apiPointCredSave({
        task_id: this.taskId,
        point_id: wp.point_id,
        checkin_type: type,
        cred_no: credNo,
        fence_distance: fenceDistance
      })
    },
    /** 扫码结果统一处理（内嵌扫码窗 onmarked / 全屏扫码 success 共用） */
    onCredScanResult(raw: string) {
      const code = extractPointCode(raw)
      if (code == '') {
        uni.showToast({ title: '请扫描新版点位二维码', icon: 'none' })
        this.reopenCredScan()
        return
      }
      if (this.curPoint != null && this.curPoint.qrcode_no != '' && code != this.curPoint.qrcode_no) {
        uni.showToast({ title: '二维码与本点位不匹配', icon: 'none' })
        this.reopenCredScan()
        return
      }
      this.closeCredScan()
      if (this.curWizPoint != null) {
        this.curWizPoint.scannedNo = code
        this.saveCredDraft('qrcode', code, this.distance >= 0 ? this.distance : 0)
      }
      uni.vibrateShort({})
      // 围栏不过则提示并留在凭证步（缺哪项哪行红字说明）
      if (!this.fenceOk) {
        uni.showToast({ title: this.fenceTip(), icon: 'none' })
        return
      }
      this.maybeAutoStartCred()
    },
    /** 扫码结果无效：内嵌窗重开持续识别 */
    reopenCredScan() {
      setTimeout(() => {
        if (!this.destroyed && this.phase == 'cred') this.openCredScan()
      }, 300)
    },
    /**
     * 内嵌摄像头扫码（方案 §13.3）：plus.barcode 在凭证卡上部开固定高度原生扫码视图，
     * 持续识别，扫到即震动+点亮+自动关闭。原生视图盖在 webview 上层不随滚动（凭证步内容短不滚动，正好成立）；
     * 离开凭证步必须 close（生命周期收进 phase watcher / onHide / onUnload）。
     */
    openCredScan() {
      // #ifdef APP-PLUS
      if (this.scanView != null || this.destroyed) return
      const pt = this.curPoint
      if (pt == null || (pt.credential != 'qrcode' && pt.credential != 'any')) return
      const wp = this.curWizPoint
      if (wp == null || wp.scannedNo != '') return
      // any 同屏锁死：NFC 已核验则扫码窗不再开启
      if (pt.credential == 'any' && wp.nfcCardId != '') return
      setTimeout(() => {
        if (this.destroyed || this.phase != 'cred' || this.scanView != null) return
        uni.createSelectorQuery()
          .in(this)
          .select('#cred-scan-slot')
          .boundingClientRect((rect: any) => {
            if (this.destroyed || this.phase != 'cred' || this.scanView != null) return
            if (rect == null || rect.width == 0) return // 占位未渲染：保留全屏扫码备用入口
            try {
              const bc: any = (plus as any).barcode
              const view = bc.create('credScanView', [bc.QR], {
                top: Math.round(rect.top) + 'px',
                left: Math.round(rect.left) + 'px',
                width: Math.round(rect.width) + 'px',
                height: Math.round(rect.height) + 'px'
              })
              view.onmarked = (_type: number, result: string) => {
                this.onCredScanResult(result)
              }
              view.start()
              this.scanView = view
            } catch (_e) {
              // 创建失败：回退全屏扫码入口（卡上常驻「点这里全屏扫码」）
            }
          })
          .exec()
      }, 400)
      // #endif
    },
    closeCredScan() {
      // #ifdef APP-PLUS
      if (this.scanView != null) {
        try {
          this.scanView.close()
        } catch (_e) {}
        this.scanView = null
      }
      // #endif
    },
    /** 核验清单-NFC 行点按（iOS 手动触发；Android 常驻监听贴卡即亮，点按同样可用） */
    onNfcRowTap() {
      const wp = this.curWizPoint
      if (wp != null && wp.nfcCardId != '') return
      // any 同屏锁死：扫码已核验则 NFC 入口失效
      if (this.curPoint != null && this.curPoint.credential == 'any' && wp != null && wp.scannedNo != '') {
        uni.showToast({ title: '已通过二维码核验', icon: 'none' })
        return
      }
      this.nfcTap()
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
        this.applyNfcCard(cardId)
      })
    },
    /** NFC 卡号命中核验（凭证步按钮读取与全局贴卡接管共用） */
    applyNfcCard(cardId: string) {
      if (this.curPoint != null && this.curPoint.nfc_id != '' && cardId != this.curPoint.nfc_id) {
        uni.showToast({ title: '这不是本点位的卡', icon: 'none' })
        return
      }
      if (this.curWizPoint != null) {
        this.curWizPoint.nfcCardId = cardId
        this.saveCredDraft('nfc', cardId, this.distance >= 0 ? this.distance : 0)
      }
      uni.vibrateShort({})
      if (!this.fenceOk) {
        uni.showToast({ title: this.fenceTip(), icon: 'none' })
        return
      }
      this.maybeAutoStartCred()
    },
    /**
     * 向导页内全局 NFC 贴卡（App.vue 转发，不用先点按钮）：
     * - 凭证步接管：贴卡 UID 命中当前点位 → 直接点亮 NFC ✓，不跳页；不命中 → toast「这不是本点位的卡」；
     *   离开凭证步/离开向导归还全局路由（App.vue 每次事件按当前页分发，天然归还）
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
      if (this.overlayMsg != '' || this.submitting || this.captureBusy) {
        uni.showToast({ title: '处理中，请稍候…', icon: 'none' })
        return
      }
      // 凭证步接管：贴卡只为当前点位核验服务
      if (this.phase == 'cred') {
        if (this.curPoint != null && pt.point_id == this.curPoint.point_id) {
          const cred = this.curPoint.credential
          if (cred != 'nfc' && cred != 'any') {
            uni.showToast({ title: '本点位请扫码核验', icon: 'none' })
            return
          }
          const wp = this.curWizPoint
          if (wp != null && wp.nfcCardId != '') return
          // any 同屏锁死：扫码已核验则 NFC 入口失效
          if (cred == 'any' && wp != null && wp.scannedNo != '') {
            uni.showToast({ title: '已通过二维码核验', icon: 'none' })
            return
          }
          this.applyNfcCard(cardId)
          return
        }
        uni.showToast({ title: '这不是本点位的卡', icon: 'none' })
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
      uni.redirectTo({
        url:
          '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
          '&point_id=' + encodeURIComponent(pt.point_id) + '&no=' + encodeURIComponent(cardId)
      })
    },
    /** 围栏提示（含阈值）：X=点位围栏半径，Y=当前距离 */
    fenceTip(): string {
      const pt = this.curPoint
      if (pt == null) return '超出围栏范围，走近一点再试'
      return '需在 ' + pt.fence_radius + ' 米内，当前 ' + (this.distance >= 0 ? this.distance + ' 米' : '距离未知')
    },
    /** 进入逐项步（凭证齐全时由自动核验/回退继续入口触发） */
    startItems() {
      if (!this.credOk) {
        uni.showToast({ title: '请先完成点位核验', icon: 'none' })
        return
      }
      if (!this.fenceOk) {
        uni.showToast({ title: this.fenceTip(), icon: 'none' })
        return
      }
      const wp = this.curWizPoint
      if (wp == null) return
      this.phase = wp.items.length > 0 ? 'items' : 'gate'
    },
    /**
     * 当前项拍照（仅相机）。AI 档拍完即走：本地照片立即进槽、立即推进下一项，
     * 压缩→上传→建 job→识别轮询全部进后台队列；手动档/感官项拍完停留作答（✓正常/⚠有异常）。
     * （水印由服务端统一烧录）
     */
    takePhoto() {
      const it = this.curItem
      if (it == null || it.status == 'recognizing') return
      if (it.judge_type == 'equipment_validity') {
        this.takeEquipLabelPhoto()
        return
      }
      if (this.manualMode || it.judge_type == 'manual') {
        // 手动档/感官项：照片累加（≤3），上传同步完成（作答草稿需 file_id），不自动跳
        if (it.photos.length >= 3) {
          uni.showToast({ title: '照片至多 3 张', icon: 'none' })
          return
        }
        this.shootFor(it, false, 'manual')
        return
      }
      if (!this.curItemIsPhoto) return
      this.shootFor(it, true, 'ai')
    },
    /** 逃生类型文案（佐证确认弹窗/已上报回显共用） */
    escapeTypeText(exceptionType: string): string {
      if (exceptionType == 'device_missing') return '设备确实不存在'
      if (exceptionType == 'unable_to_capture') return '现场无法拍摄'
      if (exceptionType == 'camera_broken') return '相机故障'
      if (exceptionType == 'label_missing') return '标签磨损无法辨认'
      return ''
    },
    exceptionText(exceptionType?: string) {
      if (exceptionType == 'device_missing') return '已上报：设备确实不存在'
      if (exceptionType == 'unable_to_capture') return '已上报：现场无法拍摄'
      if (exceptionType == 'camera_broken') return '已上报：相机故障'
      if (exceptionType == 'label_missing') return '已上报：标签磨损无法辨认'
      return ''
    },
    /** 导航条右侧逃生「?」：面板选择异常类型（设备不存在/现场无法拍摄，抽查项另有标签磨损），再确认拍摄佐证 */
    onEscapeTap() {
      const it = this.curItem
      if (it == null || this.captureBusy || this.overlayMsg != '' || this.submitting) return
      if (!this.curItemIsPhoto && !(this.manualMode && (it.photo_required ?? '') != 'none')) return
      if (this.manualMode && it.photo_required == 'none') return
      this.escapeItem = it
      this.escapeSheetShow = true
    },
    onEscapeSheetSelect(idx: number) {
      const types = ['device_missing', 'unable_to_capture', 'camera_broken', 'label_missing']
      this.escapeType = types[idx] ?? ''
      // 无法拍摄/相机故障：拍不了照还要佐证是矛盾的——选完类型直接上报（跳过佐证拍摄确认框）
      if (this.escapeType == 'unable_to_capture' || this.escapeType == 'camera_broken') {
        const it = this.escapeItem
        if (it != null) this.reportEscapeNoPhoto(it, this.escapeType)
        return
      }
      this.escapeDlgShow = true
    },
    /** 无佐证逃生（现场无法拍摄/相机故障）：直接落逃生草稿并本地落定 escaped（非异常，不进异常处置/统计），自动下一项 */
    reportEscapeNoPhoto(it: WizardItemSnap, exceptionType: 'unable_to_capture' | 'camera_broken') {
      const wp = this.curWizPoint
      if (wp == null) return
      if (it.job_id != '') this.skippedJobs[it.job_id] = true // 进行中的识别 job 落定不回写
      it.photos = []
      it.file_ids = []
      it.job_id = ''
      it.exception_type = exceptionType
      it.abnormal_tags = []
      it.pending_local = ''
      it.pending_mode = ''
      it.pending_exception_type = ''
      it.shoot_lng = undefined
      it.shoot_lat = undefined
      it.shoot_at = undefined
      it.manual_confirmed = false
      it.status = 'done'
      it.verdict = ''
      it.reason = this.escapeTypeText(exceptionType) + '，已上报'
      it.reading = ''
      it.quality_pass = true
      it.quality_issue = ''
      it.pass = false
      it.note = it.reason
      it.img_error = false
      apiItemDraftPhotoAbnormal({
        task_id: this.taskId,
        point_id: wp.point_id,
        name: it.name,
        note: it.reason,
        exception_type: exceptionType
      }).catch(() => {
        uni.showToast({ title: '网络异常，进度可能未保存', icon: 'none' })
      })
      uni.showToast({ title: '已上报无法检查', icon: 'success' })
      this.nextStep()
    },
    onEscapeDlgConfirm() {
      const it = this.escapeItem
      if (it != null) this.shootFor(it, true, 'escape', this.escapeType)
    },
    /**
     * 逃生佐证落草稿：device_missing/unable_to_capture/camera_broken 走 photo-abnormal 草稿（fileId 空=无佐证直接上报）；
     * label_missing 仅抽查项可用（模板项草稿接口会拒），AI 档改走 AI 读标签 job 落照片草稿
     * （服务端提交时从草稿 ai_reading 解析日期；label_missing 短路不读数），手动档不落草稿仅随提交上送。
     */
    saveEscapeDraft(it: WizardItemSnap, pointId: string, fileId: string, exceptionType: string): Promise<{ job_id: string }> {
      const shoot = this.pickShoot(it)
      if (exceptionType == 'label_missing') {
        if (this.manualMode) return Promise.resolve({ job_id: '' })
        return apiAiItemJobCreate({ task_id: this.taskId, point_id: pointId, name: it.name, file_ids: [fileId], ...shoot })
      }
      return apiItemDraftPhotoAbnormal({
        task_id: this.taskId,
        point_id: pointId,
        name: it.name,
        file_ids: fileId != '' ? [fileId] : undefined,
        note: this.escapeTypeText(exceptionType) + '，已上报',
        exception_type: exceptionType as 'device_missing' | 'unable_to_capture' | 'camera_broken',
        ...shoot
      }).then(() => ({ job_id: '' }))
    },
    /** 补拍步重拍：重拍 = 重新拍照上传重新建 job（后台队列），停留在补拍列表 */
    retakePhoto(it: WizardItemSnap) {
      if (it.status == 'recognizing') return
      this.shootFor(it, false, 'ai')
    },
    /**
     * 识别失败/超时逃生 + gate 逐项「跳过识别」（方案 §六，转人工复核，不让巡检员死等）：
     * AI 基础设施故障不阻断巡检——该项转人工确认（巡检员自行核对，异常用观察点勾选表达）。
     * 服务端 validateConfirmedAI 对 pending/failed 草稿放行并把记录转人工复核。
     * 质量不合格（模糊/翻拍）仍须重拍。
     */
    manualConfirmItem(it: WizardItemSnap) {
      if (it.status != 'failed' && it.status != 'recognizing') return
      if (it.job_id != '') this.skippedJobs[it.job_id] = true // 轮询落定不再回写，保持人工确认结论
      it.status = 'done'
      it.quality_pass = true
      it.quality_issue = ''
      it.verdict = ''
      it.reason = 'AI 未识别，巡检员现场确认'
      it.pass = true
      it.manual_confirmed = true
      uni.showToast({ title: '已转人工确认，异常请勾选观察点', icon: 'none' })
      this.maybeGateSettled()
    },
    onGateSkip(it: WizardItemSnap) {
      this.manualConfirmItem(it)
    },
    onGateRetry(it: WizardItemSnap) {
      const wp = this.curWizPoint
      if (wp == null) return
      this.retryPendingUpload(it, wp.point_id)
    },
    /**
     * 拍照统一入口。AI/escape 档「拍完即走」（方案 §4.1）：本地照片立即进槽、立即推进，
     * 压缩→上传→建 job 进后台队列串行执行；manual 档（手动档/感官项）上传同步完成（作答草稿需 file_id）。
     */
    shootFor(it: WizardItemSnap, advance: boolean, mode: 'ai' | 'escape' | 'manual', exceptionType = '') {
      const wp = this.curWizPoint
      if (wp == null || this.captureBusy || this.overlayMsg != '' || this.submitting) return
      const pointId = wp.point_id
      const captureToken = this.captureToken + 1
      this.captureToken = captureToken
      this.captureBusy = true
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          if (!this.captureIsCurrent(captureToken)) return
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) {
            this.captureBusy = false
            return
          }
          const localPath = paths[0]
          if (mode == 'manual') {
            // 手动档/感官项：照片立即本地显示，上传同步（遮罩）完成后换服务端 URL 并记 file_id；停留作答不自动跳
            it.photos.push(localPath)
            it.img_error = false
            this.captureShootInfo(it) // 异步回写拍摄时空信息，作答落草稿时随 shoot_* 上送
            this.captureBusy = false
            this.overlayMsg = '照片上传中…'
            compressForUpload(localPath)
              .then((raw) => apiUploadLocal(raw))
              .then((r) => {
                if (this.destroyed) return
                this.overlayMsg = ''
                const i = it.photos.indexOf(localPath)
                if (i >= 0) it.photos.splice(i, 1, r.url)
                it.file_ids.push(r.file_id)
              })
              .catch((e: any) => {
                if (this.destroyed) return
                this.overlayMsg = ''
                const msg = (e && e.message) || ''
                // 网络异常：压缩图保留在项上置待补传态，联网/回页后自动重试
                if (msg.indexOf(NETWORK_ERR_PREFIX) == 0) {
                  compressForUpload(localPath)
                    .then((raw) => this.markPendingUpload(it, raw, 'manual', ''))
                    .catch(() => this.markPendingUpload(it, localPath, 'manual', ''))
                  uni.showToast({ title: '网络异常，照片已保留，联网后自动补传', icon: 'none' })
                  return
                }
                const i = it.photos.indexOf(localPath)
                if (i >= 0) it.photos.splice(i, 1)
                uni.showToast({ title: msg || '上传失败，请重试', icon: 'none' })
              })
            return
          }
          // AI / escape：拍完即走——本地照片立即进槽，后台队列依次完成 压缩→上传→建 job→并发识别→轮询落定
          it.photos = [localPath] // 一图一位：重拍=替换
          it.img_error = false
          it.file_ids = []
          it.job_id = ''
          // 新照片 = 重新判定：异常观察点 tag 重置为全正常（AI 结果回来后按 abnormal_tags 再预标记）
          it.abnormal_tags = []
          it.exception_type = mode == 'escape' ? exceptionType : ''
          it.status = 'recognizing'
          it.verdict = ''
          it.reason = ''
          it.reading = ''
          it.quality_pass = true
          it.quality_issue = ''
          it.pending_local = ''
          it.pending_mode = ''
          it.pending_exception_type = ''
          it.manual_confirmed = false // 重拍/新拍 = 重新走识别链路，清除人工确认标记（ai_verdict 恢复透传）
          // 拍摄时空信息：shoot_at 同步写入，坐标异步回写；后台链路建 job/落草稿前短等其落定
          const shootReady = this.captureShootInfo(it)
          this.captureBusy = false
          this.enqueueBg(it, pointId, localPath, mode, exceptionType, shootReady)
          // 拍照项：立即推进下一项，不等上传、不等识别结果
          if (advance) this.nextStep()
        },
        fail: () => {
          if (this.captureToken == captureToken) this.captureBusy = false
        }
      })
    },
    captureIsCurrent(token: number): boolean {
      return !this.destroyed && this.captureToken == token
    },
    /**
     * 拍照成功即记录拍摄时刻并异步回写 GCJ-02 坐标（30s 缓存定位，方案 §14.3 防作弊数据源）：
     * shoot_at 同步写入；定位失败仅坐标缺省（null 不抛错），不阻塞拍完即走主流程。
     * 返回回写落定 promise，后台上传链路建 job/落草稿前短等（见 awaitShootReady），坐标尽量带齐。
     */
    captureShootInfo(it: WizardItemSnap): Promise<void> {
      it.shoot_at = fmtDateTime(new Date())
      it.shoot_lng = undefined
      it.shoot_lat = undefined
      return getLocationCached()
        .then((loc) => {
          if (loc == null) return
          it.shoot_lng = loc.longitude
          it.shoot_lat = loc.latitude
        })
        .catch(() => {})
    },
    /** 取该项拍摄时空信息上送字段（缺省不送） */
    pickShoot(it: WizardItemSnap): { shoot_lng?: number; shoot_lat?: number; shoot_at?: string } {
      const r: { shoot_lng?: number; shoot_lat?: number; shoot_at?: string } = {}
      if (it.shoot_lng != null) r.shoot_lng = it.shoot_lng
      if (it.shoot_lat != null) r.shoot_lat = it.shoot_lat
      if (it.shoot_at != null && it.shoot_at != '') r.shoot_at = it.shoot_at
      return r
    },
    /** 上送前短等拍摄定位回写落定（最多 3s；超时/失败放弃坐标不阻塞，shoot_at 拍照时已写入） */
    awaitShootReady(p: Promise<void> | null): Promise<void> {
      if (p == null) return Promise.resolve()
      return Promise.race([p, new Promise<void>((resolve) => setTimeout(resolve, 3000))])
        .then(() => {})
        .catch(() => {})
    },
    /** 后台任务入队并启动泵（串行执行，弱网防雪崩） */
    enqueueBg(it: WizardItemSnap, pointId: string, raw: string, mode: 'ai' | 'escape' | 'manual', exceptionType: string, shootReady: Promise<void> | null = null) {
      this.bgQueue.push({ it: it, pointId: pointId, raw: raw, mode: mode, exceptionType: exceptionType, shootReady: shootReady })
      this.pumpBgQueue()
    },
    /** 后台队列泵：取下一个任务执行（压缩→上传→建 job/落草稿），完成后递归取下一个 */
    pumpBgQueue() {
      if (this.bgRunning || this.destroyed) return
      const job = this.bgQueue.shift()
      if (job == null) {
        this.maybeGateSettled()
        return
      }
      this.bgRunning = true
      const it = job.it
      let compressed = job.raw
      compressForUpload(job.raw)
        .then((raw) => {
          compressed = raw
          return apiUploadLocal(raw)
        })
        .then((r) => {
          // 上送前短等拍摄定位回写落定（坐标尽量带齐；超时/失败放弃坐标不阻塞）
          return this.awaitShootReady(job.shootReady).then(() => {
            if (this.destroyed) return null
            // 槽位照片已不是这张（用户重拍/替换过）：本次结果过期丢弃，不覆盖新照片
            if (it.photos.length == 0 || it.photos[0] != job.raw) return null
            if (job.mode == 'escape') {
              return this.saveEscapeDraft(it, job.pointId, r.file_id, job.exceptionType).then((j) => ({ fileId: r.file_id, url: r.url, jobId: j.job_id }))
            }
            return apiAiItemJobCreate({
              task_id: this.taskId,
              point_id: job.pointId,
              name: it.name,
              file_ids: [r.file_id],
              ...this.pickShoot(it)
            }).then((j) => ({ fileId: r.file_id, url: r.url, jobId: j.job_id }))
          })
        })
        .then((res) => {
          if (this.destroyed) return
          this.bgRunning = false
          if (res != null) {
            this.applyPhotoSuccess(it, compressed, res.fileId, res.url, res.jobId, job.mode, job.exceptionType)
            if (res.jobId != '') {
              this.jobStartTimes[res.jobId] = Date.now()
              this.ensureBgPoll()
            }
          }
          this.pumpBgQueue()
          this.maybeGateSettled()
        })
        .catch((e: any) => {
          if (this.destroyed) return
          this.bgRunning = false
          // 槽位仍是这张本地照片才处理失败态（重拍过则丢弃本次失败）
          if (it.photos.length > 0 && it.photos[0] == job.raw) {
            const code = e != null && typeof e.code == 'number' ? e.code : 0
            if (code == CODE_AI_DISABLED) {
              uni.showToast({ title: 'AI 未启用，请改用手动模式', icon: 'none' })
              this.resetItem(it)
            } else {
              const msg = (e && e.message) || ''
              // 网络异常（上传/建 job）：压缩图保留在项上置待补传态，联网/回页后自动补传
              if (msg.indexOf(NETWORK_ERR_PREFIX) == 0) {
                this.markPendingUpload(it, compressed, job.mode, job.exceptionType)
                uni.showToast({ title: '网络异常，照片已保留，联网自动补传', icon: 'none' })
              } else {
                uni.showToast({ title: msg || '照片上传失败，请重新拍', icon: 'none' })
                this.resetItem(it)
              }
            }
          }
          this.pumpBgQueue()
          this.maybeGateSettled()
        })
    },
    /** 上传+建 job 成功后的逐项落定（后台队列与待补传重试共用）；escape=逃生佐证直接落定 escaped（不是异常：不标 verdict、不进异常处置/统计） */
    applyPhotoSuccess(it: WizardItemSnap, raw: string, fileId: string, fileUrl: string, jobId: string, mode: 'ai' | 'escape' | 'manual', exceptionType: string) {
      it.pending_local = ''
      it.pending_mode = ''
      it.pending_exception_type = ''
      // 展示用服务端 URL（重启后仍可加载）；本地临时路径仅兜底
      const url = fileUrl != '' ? fileUrl : raw
      it.img_error = false
      it.photos = [url]
      it.file_ids = [fileId]
      it.exception_type = mode == 'escape' ? exceptionType : ''
      it.file_id = fileId
      it.job_id = jobId
      if (mode == 'escape') {
        it.status = 'done'
        it.verdict = ''
        it.reason = this.escapeTypeText(exceptionType) + '，已上报'
        it.reading = ''
        it.quality_pass = true
        it.quality_issue = ''
        it.pass = false
        it.note = it.reason
        uni.showToast({ title: '已上报无法检查', icon: 'success' })
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
    },
    /** 上传失败置待补传态：压缩图保留在项上（本地路径仅会话内有效），槽位显示橙色状态条「照片还没传上去」 */
    markPendingUpload(it: WizardItemSnap, raw: string, mode: 'ai' | 'escape' | 'manual', exceptionType: string) {
      it.pending_local = raw
      it.pending_mode = mode
      it.pending_exception_type = exceptionType
      it.photos = [raw]
      it.img_error = false
      it.file_ids = []
      it.job_id = ''
      it.status = 'failed'
      it.quality_issue = '照片待补传'
    },
    /** 待补传项重试：重新上传保留的本地压缩图 → 继续原链路（ai=建识别 job / escape=异常上报草稿），成功清 pending 标记 */
    retryPendingUpload(it: WizardItemSnap, pointId: string) {
      const raw = it.pending_local
      if (raw == null || raw == '') return
      if (this.captureBusy || this.submitting || this.overlayMsg != '' || this.destroyed) return
      const mode: 'ai' | 'escape' | 'manual' = it.pending_mode == 'escape' ? 'escape' : it.pending_mode == 'manual' ? 'manual' : 'ai'
      const exceptionType = it.pending_exception_type ?? ''
      this.captureBusy = true
      this.overlayMsg = '照片补传中…'
      let fileId = ''
      let fileUrl = ''
      apiUploadLocal(raw)
        .then((r) => {
          fileId = r.file_id
          fileUrl = r.url
          if (mode == 'escape') {
            return this.saveEscapeDraft(it, pointId, fileId, exceptionType)
          }
          if (mode == 'manual') {
            return { job_id: '' }
          }
          return apiAiItemJobCreate({
            task_id: this.taskId,
            point_id: pointId,
            name: it.name,
            file_ids: [fileId],
            ...this.pickShoot(it)
          })
        })
        .then((j) => {
          if (this.destroyed) return
          this.overlayMsg = ''
          this.captureBusy = false
          if (mode == 'manual') {
            // 手动档：照片只上传落项，不建 AI job
            it.pending_local = ''
            it.pending_mode = ''
            it.pending_exception_type = ''
            it.status = 'todo'
            it.quality_issue = ''
            it.photos = [fileUrl != '' ? fileUrl : raw]
            it.file_ids = [fileId]
            it.img_error = false
          } else {
            this.applyPhotoSuccess(it, raw, fileId, fileUrl, j.job_id, mode, exceptionType)
            if (j.job_id != '') {
              this.jobStartTimes[j.job_id] = Date.now()
              this.ensureBgPoll()
            }
          }
          if (mode != 'escape') uni.showToast({ title: '照片补传成功', icon: 'success' })
          // 继续补传其余待补传项（逐项串行，弱网防雪崩）
          this.retryNextPending()
          this.maybeGateSettled()
        })
        .catch((e: any) => {
          if (this.destroyed) return
          this.overlayMsg = ''
          this.captureBusy = false
          const code = e != null && typeof e.code == 'number' ? e.code : 0
          if (code == CODE_AI_DISABLED) {
            uni.showToast({ title: 'AI 未启用，请改用手动模式', icon: 'none' })
            return
          }
          // 仍是网络异常：照片继续保留待补传；业务错误原样提示（本地图失效可在卡片上点「重新拍照」）
          const msg = (e && e.message) || ''
          if (msg.indexOf(NETWORK_ERR_PREFIX) == 0) {
            uni.showToast({ title: '仍无法上传，照片已保留，联网后自动重试', icon: 'none' })
            return
          }
          uni.showToast({ title: msg || '补传失败，请重试', icon: 'none' })
        })
    },
    /** 逐项扫描向导快照，补传第一个待补传项（成功后由其 then 链继续下一项） */
    retryNextPending() {
      if (this.captureBusy || this.submitting || this.overlayMsg != '' || this.destroyed) return
      for (let p = 0; p < this.wizPoints.length; p++) {
        const wp = this.wizPoints[p]
        for (let i = 0; i < wp.items.length; i++) {
          const it = wp.items[i]
          if ((it.pending_local ?? '') != '') {
            this.retryPendingUpload(it, wp.point_id)
            return
          }
        }
      }
    },
    /** 回到页面 / 网络恢复时自动补传：无网或处理中则跳过（loaded 前快照未建，不跑） */
    retryPendingItems() {
      if (!this.loaded || this.destroyed) return
      if (this.captureBusy || this.submitting || this.overlayMsg != '') return
      uni.getNetworkType({
        success: (net) => {
          if (net.networkType == 'none') return
          this.retryNextPending()
        },
        fail: () => {}
      })
    },
    /** 卡片待补传橙色状态条点击：补传当前项 */
    retryCurUpload() {
      const it = this.curItem
      const wp = this.curWizPoint
      if (it == null || wp == null || (it.pending_local ?? '') == '') return
      this.retryPendingUpload(it, wp.point_id)
    },
    /** 网络状态变化：恢复连接即自动补传待补传项 */
    onNetChange(res: { isConnected?: boolean }) {
      if (res != null && res.isConnected == true) this.retryPendingItems()
    },
    /** 台账有效期合成项「已维保？拍新标签」（仅相机，至多 3 张）：照片即登记凭证，提交时服务端 AI 核对 */
    takeEquipLabelPhoto() {
      const it = this.curItem
      if (it == null || it.judge_type != 'equipment_validity' || this.captureBusy || this.submitting) return
      if (it.file_ids.length >= 3) {
        uni.showToast({ title: '新标签照片至多 3 张', icon: 'none' })
        return
      }
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          const path = (res.tempFilePaths || [])[0]
          if (path == null) return
          this.captureBusy = true
          this.overlayMsg = '上传中…'
          compressForUpload(path)
            .then((p) => apiUploadLocal(p))
            .then((up) => {
              it.photos.push(up.url)
              it.file_ids.push(up.file_id)
              it.img_error = false
            })
            .catch((e: any) => {
              uni.showToast({ title: (e && e.message) || '上传失败，请重试', icon: 'none' })
            })
            .finally(() => {
              this.captureBusy = false
              this.overlayMsg = ''
            })
        }
      })
    },
    /** 观察点勾选切换（观察点半屏/有异常半屏共用）：默认全部正常，勾选为异常（再勾恢复）；非空即该项判异常 */
    toggleCurTag(tag: string) {
      const it = this.curItem
      if (it == null || it.tags.indexOf(tag) < 0) return
      const i = it.abnormal_tags.indexOf(tag)
      if (i >= 0) it.abnormal_tags.splice(i, 1)
      else it.abnormal_tags.push(tag)
    },
    /** 该项可走「这项正常吗？」作答：感官项；或手动档的非台账项（必拍项须先拍照或逃生上报） */
    curItemManualAnswerable(it: WizardItemSnap | null, toastWhy: boolean): boolean {
      if (it == null || it.status == 'done') return false
      if (it.judge_type == 'equipment_validity') return false
      if (it.judge_type != 'manual' && !this.manualMode) return false
      if (this.manualMode && it.judge_type != 'manual' && it.photo_required == 'required' && it.file_ids.length == 0 && (it.exception_type ?? '') == '') {
        if (toastWhy) uni.showToast({ title: '请先拍 1 张该项照片', icon: 'none' })
        return false
      }
      return true
    },
    /** 正常一次过（感官项 / 手动档拍照项同一套动作） */
    tapManualOk() {
      const it = this.curItem
      if (!this.curItemManualAnswerable(it, true)) return
      const item = it as WizardItemSnap
      item.pass = true
      item.note = ''
      item.verdict = 'pass'
      item.status = 'done'
      this.abnSheetShow = false
      this.saveManualDraft(item)
      this.nextStep()
    },
    /** 有异常 → 底部半屏（观察点多选+备注+确认），确认后自动下一项 */
    tapManualAbnormal() {
      const it = this.curItem
      if (!this.curItemManualAnswerable(it, true)) return
      this.manualNote = (it as WizardItemSnap).note
      this.abnSheetShow = true
    },
    confirmManualAbnormal() {
      const it = this.curItem
      if (!this.curItemManualAnswerable(it, true)) return
      const item = it as WizardItemSnap
      item.pass = false
      item.note = this.manualNote.trim()
      item.verdict = 'abnormal'
      item.status = 'done'
      this.abnSheetShow = false
      this.saveManualDraft(item)
      this.nextStep()
    },
    /** 手动项选择实时落云端草稿（含手动档拍照项的照片 file_ids 与异常观察点 tag；失败仅提示不阻断，下次进入以服务端为准） */
    saveManualDraft(it: WizardItemSnap) {
      const wp = this.curWizPoint
      if (wp == null) return
      apiItemDraftManual({
        task_id: this.taskId,
        point_id: wp.point_id,
        name: it.name,
        pass: it.pass,
        note: it.note,
        file_ids: it.file_ids.length > 0 ? it.file_ids.slice() : undefined,
        abnormal_tags: it.abnormal_tags.length > 0 ? it.abnormal_tags.slice() : undefined,
        ...this.pickShoot(it)
      }).catch(() => {
        uni.showToast({ title: '网络异常，进度可能未保存', icon: 'none' })
      })
    },
    /** 推进：下一项 → 收尾；itemIdx 溢出表示待在收尾步 */
    nextStep() {
      const wp = this.curWizPoint
      if (wp == null) return
      this.abnSheetShow = false
      this.tagPickerShow = false
      if (this.itemIdx < wp.items.length - 1) {
        this.itemIdx += 1
      } else {
        this.itemIdx = wp.items.length
        this.phase = 'gate'
        // 方案 B：顺向到达收尾步武装自动提交（断点恢复/切档重进不武装，见 enterPoint）
        this.gateArmed = true
        this.autoSubmitGate()
      }
    },
    /**
     * 方案 B 自动提交：到达收尾步且全部落定（后台上传/识别都已结束）时，凭证/围栏已过 + 有模板项 → 自动走提交链路
     * （汇总分流：全 pass 直接落库；补拍/异常处置停住交给人）。未落定时停在 gate 阻塞等待（逐项实时进度）。
     * 修改模式、无模板点位（纯拍照打卡）保持手动提交。
     */
    autoSubmitGate() {
      const wp = this.curWizPoint
      if (!this.gateArmed) return // 仅会话内顺向推进到 gate 才允许自动提交
      if (wp == null || this.modify || this.submitting) return
      if (this.phase != 'gate' || wp.items.length == 0) return
      if (!this.gateSettled) return
      if (!this.credOk) return
      if (this.curPoint != null && this.curPoint.require_fence && !this.fenceOk) return
      this.submitPoint()
    },
    /** 后台任务/轮询落定后回调：gate 步全部落定即触发自动提交（按钮同步亮起） */
    maybeGateSettled() {
      if (this.destroyed) return
      if (this.phase != 'gate') return
      if (!this.gateSettled) return
      this.autoSubmitGate()
    },
    /** 底部「上一项」点击：回退一项（不跨点位）；兜底到本点位第一步时直接退出（正常不会到这，起点按钮已隐藏） */
    onPrevTap() {
      if (!this.prevStep()) this.exitWizard()
    },
    /**
     * 档位切换（双向）：AI 档 → 手动档（?mode=manual）；手动档 → AI 档（去掉 mode）。
     * redirectTo 替换当前向导；进度在云端草稿，互切重进可续。已核验的扫码/NFC 凭证随 no 参数带过去免核验。
     */
    switchMode() {
      const pt = this.curPoint
      if (pt == null) return
      let url =
        '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
        '&point_id=' + encodeURIComponent(pt.point_id)
      if (!this.manualMode) url += '&mode=manual'
      const wp = this.curWizPoint
      const preNo = wp != null ? (wp.scannedNo != '' ? wp.scannedNo : wp.nfcCardId) : ''
      if (preNo != '') url += '&no=' + encodeURIComponent(preNo)
      uni.redirectTo({ url: url })
    },
    /** 导航栏返回键：与系统返回同口径——直接退出巡检（已拍内容在云端草稿，下次可续） */
    onBackTap() {
      if (this.overlayMsg != '' || this.submitting || this.captureBusy) return
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
      this.abnSheetShow = false
      this.tagPickerShow = false
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
    /**
     * 提交本点位：gate 是全流程唯一的等待点——未落定项（上传中/识别中/待补传）在这里阻塞，
     * 落定后按钮亮起，点击直接汇总分流（全过 / 补拍 / 异常处置）。识别等待由后台轮询承担，不再有遮罩轮询。
     */
    submitPoint() {
      const wp = this.curWizPoint
      if (wp == null || this.submitting) return
      if (!this.gateSettled) {
        uni.showToast({ title: '还有 ' + this.gateRemain + ' 项在处理中…', icon: 'none' })
        return
      }
      if (!this.credOk) {
        uni.showToast({ title: '请先完成点位核验', icon: 'none' })
        return
      }
      if (this.curPoint != null && this.curPoint.require_fence && !this.fenceOk) {
        uni.showToast({ title: this.fenceTip(), icon: 'none' })
        return
      }
      this.routePointResult()
    },
    /** 后台 AI 轮询：有未落定 job 时 1.5s 批量查，落定写回；190s 超时按 failed 处理（可重拍或跳过识别） */
    ensureBgPoll() {
      if (this.destroyed || this.bgPolling) return
      this.bgPolling = true
      this.bgTick()
    },
    bgTick() {
      if (this.destroyed) {
        this.bgPolling = false
        return
      }
      const targets: Array<{ it: WizardItemSnap; jobId: string }> = []
      this.wizPoints.forEach((wp) => {
        wp.items.forEach((it) => {
          if (it.status == 'recognizing' && it.job_id != '' && this.skippedJobs[it.job_id] != true) {
            targets.push({ it: it, jobId: it.job_id })
          }
        })
      })
      if (targets.length == 0) {
        this.bgPolling = false
        this.maybeGateSettled()
        return
      }
      apiAiItemJobs(targets.map((t) => t.jobId))
        .then((jobs) => {
          if (this.destroyed) return
          const map: Record<string, (typeof jobs)[0]> = {}
          jobs.forEach((j) => {
            map[j.job_id] = j
          })
          const now = Date.now()
          targets.forEach((t) => {
            if (this.skippedJobs[t.jobId] == true) return
            const j = map[t.jobId]
            const started = this.jobStartTimes[t.jobId] != null ? this.jobStartTimes[t.jobId] : now
            if (j == null) {
              // job 过期查不到：按超时判失败
              if (now - started >= POLL_TIMEOUT) {
                t.it.status = 'failed'
                t.it.quality_issue = '识别超时，可重拍或跳过识别'
              }
              return
            }
            if (j.status == 'done') {
              this.applyJob(t.it, j)
              return
            }
            if (j.status == 'failed') {
              t.it.status = 'failed'
              return
            }
            if (now - started >= POLL_TIMEOUT) {
              t.it.status = 'failed'
              t.it.quality_issue = '识别超时，可重拍或跳过识别'
            }
          })
        })
        .catch(() => {
          // 查询失败下轮再试（不阻断）
        })
        .finally(() => {
          if (this.destroyed) {
            this.bgPolling = false
            return
          }
          this.bgPollTimer = setTimeout(() => this.bgTick(), POLL_INTERVAL)
          this.maybeGateSettled()
        })
    },
    stopBgPoll() {
      if (this.bgPollTimer != null) {
        clearTimeout(this.bgPollTimer)
        this.bgPollTimer = null
      }
      this.bgPolling = false
    },
    /** job 落定：写回识别结论；abnormal 预填描述；AI 判出的异常观察点 tag 预标记（用户可手动取消） */
    applyJob(it: WizardItemSnap, j: { verdict: string; reason: string; reading: string; quality_pass: boolean; quality_issue: string; abnormal_tags?: string[] }) {
      it.status = 'done'
      it.verdict = j.verdict
      it.reason = j.reason
      it.reading = j.reading
      it.quality_pass = j.quality_pass
      it.quality_issue = j.quality_issue
      it.abnormal_tags = (j.abnormal_tags ?? []).filter((t) => it.tags.indexOf(t) >= 0)
      // 异常 tag 非空即该项判异常（与服务端强制口径一致）
      it.pass = j.verdict != 'abnormal' && it.abnormal_tags.length == 0
      if (!it.pass && it.note == '') it.note = j.reason
    },
    /** 逃生撤销（选错"设备不存在/无法拍摄"回到待拍）：清本地状态+删云端草稿；
     *  进行中的 AI job 标记跳过（落定不回写），服务端草稿行删后 worker 回写自然不落库 */
    undoEscape() {
      const it = this.curItem
      const wp = this.curWizPoint
      if (it == null || wp == null || (it.exception_type ?? '') == '') return
      if (it.job_id != '') this.skippedJobs[it.job_id] = true
      const name = it.name
      this.resetItem(it)
      it.exception_type = ''
      it.pass = false
      it.note = ''
      apiItemDraftDelete({ task_id: this.taskId, point_id: wp.point_id, name: name }).catch(() => {})
      uni.showToast({ title: '已撤销，请重新拍照', icon: 'none' })
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
      it.abnormal_tags = []
    },
    /** 汇总分流：补拍 > 异常处置 > 全过直接提交 */
    routePointResult() {
      const wp = this.curWizPoint
      if (wp == null) return
      const retake: number[] = []
      const abnormal: number[] = []
      // 手动档：未作答项（todo）阻断提交并跳回该项（无 AI 识别，无补拍步）
      if (this.manualMode) {
        for (let i = 0; i < wp.items.length; i++) {
          const si = wp.items[i]
          if (si.judge_type != 'equipment_validity' && si.status == 'todo') {
            this.itemIdx = i
            this.phase = 'items'
            uni.showToast({ title: '「' + si.name + '」还未检查', icon: 'none' })
            return
          }
        }
      }
      wp.items.forEach((it, i) => {
        // 台账有效期合成项：服务端判定（逾期记录级转人工），不参与向导的补拍/异常分流
        if (it.judge_type == 'equipment_validity') return
        // 异常观察点 tag 非空即该项判异常（服务端同口径强制）；无描述时以 tag 列表预填
        const tagAbn = it.abnormal_tags.length > 0
        if (tagAbn && it.note == '') it.note = '异常观察点：' + it.abnormal_tags.join('、')
        if (this.manualMode) {
          // 手动档：无 AI 分流，巡检员手选异常或点选了异常观察点 tag → 异常；逃生项（escaped）不算异常不升级记录
          if ((it.exception_type ?? '') == '' && (!it.pass || tagAbn)) abnormal.push(i)
          return
        }
        if (it.judge_type != 'manual') {
          // 拍照项（含标签抽查合成项）：未拍 / 识别失败 / 质量不合格 → 补拍
          if (it.status == 'todo' || it.status == 'failed' || (it.status == 'done' && !it.quality_pass)) {
            retake.push(i)
            return
          }
          if (it.status == 'done' && (it.verdict == 'abnormal' || tagAbn)) abnormal.push(i)
        } else if (!it.pass || tagAbn) {
          // 感官项：巡检员手选异常或点选了异常观察点 tag
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
        // 异常项不再走处置页：直接随点位提交（result=abnormal，服务端强制人工审核）
        this.abnormalIdxs = abnormal
        uni.vibrateShort({})
        playVoice('abnormal')
        this.doCheckin('abnormal', abnormal)
        return
      }
      this.doCheckin('normal', [])
    },
    /** 真正提交打卡（AI 档 ai_confirmed=true 采纳逐项 AI 结论；手动档 false 无草稿校验、记录标「未经 AI 识别」转人工复核；已打卡未锁定点位重复提交 = 覆盖修改，服务端处理） */
    doCheckin(result: 'normal' | 'abnormal', abnIdxs: number[]) {
      const wp = this.curWizPoint
      const pt = this.curPoint
      if (wp == null || pt == null || this.submitting || this.captureBusy) return
      this.submitting = true
      this.gateSubmitError = ''
      // 带新标签照片的提交后端会同步 AI 核对（最长约 15s）：遮罩文案说明在核对，避免误以为卡死
      const hasLabel = wp.items.some((it) => it.judge_type == 'equipment_validity' && it.file_ids.length > 0)
      this.overlayMsg = hasLabel && !this.manualMode ? 'AI 核对新标签中…' : '提交中…'
      const abnSet: Record<number, boolean> = {}
      abnIdxs.forEach((i) => {
        abnSet[i] = true
      })
      // 台账有效期合成项默认不上送（服务端按点位实时逐台判定追加快照）；
      // 但已拍新标签照片的需上送（photos=新标签照片，服务端同步 AI 核对，结论供经理确认，
      // equipment.ai_auto_confirm 开时才自动回写台账）；abnSet 键为 wp.items 原始下标，过滤时保留
      const checkItems: CheckinItemReqPayload[] = []
      wp.items.forEach((it, i) => {
        if (it.judge_type == 'equipment_validity') {
          if (it.file_ids.length > 0) {
            checkItems.push({ name: it.name, result: 'normal', note: '', photos: it.file_ids.slice() })
          }
          return
        }
        const isAbn = abnSet[i] == true
        const escaped = (it.exception_type ?? '') != ''
        const tagAbn = it.abnormal_tags.length > 0
        // 抽查合成项：只交照片（+逃生 exception_type=label_missing），结论由服务端四规则比对（日期从该项 AI 读标签草稿 ai_reading 解析），客户端 result 忽略
        const isSpot = it.judge_type == 'equipment_date_spot'
        // 处置措施功能已砍（甲方线下处置）：异常项只带结论/备注/照片，服务端见异常即强制人工审核
        checkItems.push({
          name: it.name,
          result: escaped ? 'escaped' : isAbn || tagAbn ? 'abnormal' : 'normal',
          note: isAbn || escaped ? it.note : '',
          photos: it.file_ids.slice(),
          exception_type: escaped ? (it.exception_type as CheckinItemReqPayload['exception_type']) : undefined,
          ai_verdict: isSpot || this.manualMode || it.manual_confirmed == true ? '' : it.verdict,
          ai_reason: isSpot || this.manualMode || it.manual_confirmed == true ? '' : it.reason,
          ai_reading: isSpot || this.manualMode || it.manual_confirmed == true ? '' : it.reading,
          abnormal_tags: tagAbn ? it.abnormal_tags.slice() : undefined
        })
      })
      const remark = wp.items
        .filter((it, i) => abnSet[i] == true && it.note != '')
        .map((it) => it.name + '：' + it.note)
        .join('\n')
      // checkin_type 优先取凭证草稿恢复的显式方式（断点恢复口径唯一），无草稿再按现有组合推导
      const credType = wp.cred_type == 'qrcode' || wp.cred_type == 'nfc' || wp.cred_type == 'fence' ? wp.cred_type : ''
      const req: CheckinReqPayload = {
        task_id: this.taskId,
        point_id: wp.point_id,
        checkin_type: credType != '' ? credType : pt.credential == 'nfc' ? 'nfc' : wp.scannedNo != '' ? 'qrcode' : wp.nfcCardId != '' ? 'nfc' : 'fence',
        qrcode_no: wp.scannedNo != '' ? wp.scannedNo : undefined,
        nfc_id: pt.credential == 'nfc' || pt.credential == 'any' ? (wp.nfcCardId != '' ? wp.nfcCardId : undefined) : undefined,
        longitude: this.myLng,
        latitude: this.myLat,
        altitude: this.myAlt > 0 ? this.myAlt : undefined,
        accuracy: this.myAcc > 0 ? this.myAcc : undefined,
        client_time: fmtDateTime(new Date()),
        result: result,
        ai_confirmed: !this.manualMode,
        remark: remark,
        check_items: checkItems
      }
      apiCheckin(req)
        .then(() => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          this.gateSubmitError = ''
          this.doneLocal += 1
          this.afterPointSubmitted()
        })
        .catch((e: any) => {
          if (this.destroyed) return
          this.submitting = false
          this.overlayMsg = ''
          const code = e != null && typeof e.code == 'number' ? e.code : 0
          if (code == CODE_CHECKIN_LOCKED) {
            this.lockedDlgShow = true
            return
          }
          if (code == CODE_AI_DISABLED) {
            uni.showToast({ title: 'AI 未启用，请改用手动模式', icon: 'none' })
            return
          }
          // 网络异常：照片在拍照时已上传（file_id 在 req 内），整单离线暂存，网络恢复后自动补传（幂等键防重）
          const msg = (e && e.message) || ''
          if (msg.indexOf(NETWORK_ERR_PREFIX) == 0) {
            enqueueOfflineCheckin(req, [])
            this.gateSubmitError = ''
            this.doneLocal += 1
            uni.showToast({ title: '网络异常，打卡已离线暂存，恢复后自动补传', icon: 'none' })
            this.afterPointSubmitted()
            return
          }
          // 收尾步持久红色状态条：自动提交失败后不止 toast，留在 gate 卡上直到重试成功
          this.gateSubmitError = msg || '提交失败'
          uni.showToast({ title: msg || '提交失败，请重试', icon: 'none' })
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
            // 异常观察点 tag 随上次结论回填（修改模式重新拍照后会被重置，重新由 AI/人工标记）
            it.abnormal_tags = (found.abnormal_tags ?? []).filter((t) => it.tags.indexOf(t) >= 0)
            if (it.judge_type == 'manual') {
              // 感官项可直接回填结论（三态：normal 回填正常；abnormal/escaped 回填异常态，重新作答可覆盖）
              const ok = found.result == 'normal'
              it.pass = ok
              it.note = ok ? '' : found.ai_reason
              it.verdict = ok ? 'pass' : 'abnormal'
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
    /** 底部操作栏主按钮分发（动作由 barCfg 状态表给出） */
    onBarPrimary() {
      const a = this.barCfg.primaryAction
      if (a == 'take-photo') this.takePhoto()
      else if (a == 'next') this.nextStep()
      else if (a == 'manual-ok') this.tapManualOk()
      else if (a == 'submit') this.submitPoint()
      else if (a == 'skip-ai') {
        const it = this.curItem
        if (it != null) this.manualConfirmItem(it)
      }
    },
    onBarSecondary() {
      const a = this.barCfg.secondaryAction
      if (a == 'manual-abnormal') this.tapManualAbnormal()
      else if (a == 'next') this.nextStep()
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
  padding-right: 24rpx;
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

/* 导航条右侧逃生「?」（仅逐项步、非台账项显示） */
.navbar-help {
  width: 56rpx;
  height: 56rpx;
  border-radius: 28rpx;
  border-width: 3rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
}

.navbar-help-text {
  font-size: 36rpx;
  font-weight: 700;
  line-height: 48rpx;
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

/* 手动模式入口（灰字小链接，在内容流内，底部操作栏上方） */
.manual-link {
  display: flex;
  flex-direction: row;
  justify-content: center;
  padding: 12rpx 0 24rpx;
}

.manual-link-text {
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
