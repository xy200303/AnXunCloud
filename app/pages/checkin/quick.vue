<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
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
      <view v-if="phase != 'taskDone'" class="head" :style="{ backgroundColor: colors.bgCard }">
        <text class="head-progress" :style="{ color: colors.textPrimary }">
          点位 {{ pointOrdinal }}/{{ totalPoints }}<text v-if="phase == 'items'"> · 本项 {{ itemIdx + 1 }}/{{ curItemCount }}</text>
        </text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: colors.success }"></view>
        </view>
        <text class="head-point" :style="{ color: colors.textPrimary }">{{ curPoint != null ? curPoint.point_name : '' }}</text>
        <text class="head-building" :style="{ color: colors.textSecondary }">{{ curPoint != null && curPoint.building_name != '' ? curPoint.building_name : '未分区' }}</text>
      </view>

      <!-- 凭证核验步 -->
      <block v-if="phase == 'cred'">
        <view v-if="curPoint != null && curPoint.credential == 'qrcode'" class="card" :style="{ backgroundColor: colors.bgCard }">
          <text v-if="curWizPoint != null && curWizPoint.scannedNo != ''" class="cred-ok" :style="{ color: colors.success }">✓ 已扫码</text>
          <view v-else class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
            <text class="btn-outline-text" :style="{ color: colors.primary }">扫码核验</text>
          </view>
        </view>
        <view v-else-if="curPoint != null && curPoint.credential == 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
          <text v-if="curWizPoint != null && curWizPoint.nfcCardId != ''" class="cred-ok" :style="{ color: colors.success }">✓ 已读卡</text>
          <view v-else class="btn-outline" :style="{ borderColor: colors.primary }" @click="nfcTap">
            <text class="btn-outline-text" :style="{ color: colors.primary }">读卡核验</text>
          </view>
        </view>
        <view v-else-if="curPoint != null && curPoint.credential == 'any'" class="card" :style="{ backgroundColor: colors.bgCard }">
          <text v-if="curWizPoint != null && (curWizPoint.scannedNo != '' || curWizPoint.nfcCardId != '')" class="cred-ok" :style="{ color: colors.success }">✓ 已核验</text>
          <block v-else>
            <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
              <text class="btn-outline-text" :style="{ color: colors.primary }">扫码核验</text>
            </view>
            <view class="btn-outline cred-gap" :style="{ borderColor: colors.primary }" @click="nfcTap">
              <text class="btn-outline-text" :style="{ color: colors.primary }">读卡核验</text>
            </view>
          </block>
        </view>
        <!-- 电子围栏：自动定位一次，大字显示在/超范围 -->
        <view v-if="curPoint != null && curPoint.require_fence" class="card" :style="{ backgroundColor: colors.bgCard }" @click="onLocTap">
          <text v-if="locating" class="fence-text" :style="{ color: colors.textSecondary }">定位中…</text>
          <text v-else-if="locFailed" class="fence-text" :style="{ color: colors.danger }">定位失败，点我重试</text>
          <text v-else-if="distance >= 0 && curPoint != null && distance <= curPoint.fence_radius" class="fence-text" :style="{ color: colors.success }">在范围内（{{ distance }}米）</text>
          <text v-else-if="distance >= 0" class="fence-text" :style="{ color: colors.danger }">超出范围（{{ distance }}米）</text>
        </view>
        <view
          class="btn-big"
          :style="{ backgroundColor: credOk && fenceOk ? colors.success : colors.info }"
          @click="startItems"
        >
          <text class="btn-big-text" :style="{ color: colors.white }">开始检查</text>
        </view>
      </block>

      <!-- 逐项卡片步 -->
      <block v-if="phase == 'items' && curItem != null">
        <!-- 拍照项 -->
        <view v-if="curItemIsPhoto" class="item-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="item-name" :style="{ color: colors.textPrimary }">{{ curItem.name }}</text>
          <text class="item-hint" :style="{ color: colors.textSecondary }">{{ curItem.requirement != '' ? curItem.requirement : '拍一张该项的照片' }}</text>
          <view v-if="curItem.status == 'todo' || curItem.status == 'failed'" class="big-shot" :style="{ backgroundColor: colors.primary }" @click="takePhoto">
            <text class="big-shot-text" :style="{ color: colors.white }">拍一张照片</text>
          </view>
          <block v-else>
            <text class="shot-ok" :style="{ color: colors.success }">✓ 已拍照{{ curItem.status == 'recognizing' ? '，AI 检查中' : '' }}</text>
            <view class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="takePhoto">
              <text class="btn-outline-text" :style="{ color: colors.primary }">重新拍</text>
            </view>
          </block>
        </view>
        <!-- 感官项 -->
        <view v-else class="item-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="item-name" :style="{ color: colors.textPrimary }">{{ curItem.name }}</text>
          <text class="item-hint" :style="{ color: colors.textSecondary }">{{ curItem.requirement != '' ? curItem.requirement : '这项正常吗？' }}</text>
          <view class="btn-big btn-normal" :style="{ backgroundColor: colors.success }" @click="tapManualOk">
            <text class="btn-big-text" :style="{ color: colors.white }">正常</text>
          </view>
          <view class="btn-big" :style="{ backgroundColor: colors.danger }" @click="tapManualAbnormal">
            <text class="btn-big-text" :style="{ color: colors.white }">有异常</text>
          </view>
          <block v-if="manualAbnormalOpen">
            <textarea
              v-model="manualNote"
              class="manual-note"
              :style="{ borderColor: colors.border, color: colors.textPrimary, backgroundColor: colors.bgPage }"
              placeholder="说说哪里不对劲（可不填）"
              :maxlength="200"
            />
            <view class="btn-big" :style="{ backgroundColor: colors.danger }" @click="confirmManualAbnormal">
              <text class="btn-big-text" :style="{ color: colors.white }">确认</text>
            </view>
          </block>
        </view>
      </block>

      <!-- 点位收尾步 -->
      <block v-if="phase == 'gate'">
        <view class="card gate-card" :style="{ backgroundColor: colors.bgCard }">
          <text class="gate-title" :style="{ color: colors.textPrimary }">本点位 {{ curItemCount }} 项已过完</text>
          <text class="gate-sub" :style="{ color: colors.textSecondary }">提交后 AI 统一检查</text>
        </view>
        <view class="btn-big" :style="{ backgroundColor: colors.success }" @click="submitPoint">
          <text class="btn-big-text" :style="{ color: colors.white }">提交本点位</text>
        </view>
      </block>

      <!-- 补拍步（质量不合格 / 识别失败） -->
      <block v-if="phase == 'retake'">
        <text class="phase-title" :style="{ color: colors.danger }">这几项要重新拍</text>
        <view v-for="(it, i) in retakeItems" :key="i" class="card retake-card" :style="{ backgroundColor: colors.bgCard }">
          <view class="retake-texts">
            <text class="retake-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
            <text class="retake-issue" :style="{ color: colors.danger }">{{ retakeIssue(it) }}</text>
          </view>
          <view
            class="retake-btn"
            :style="{ backgroundColor: it.status == 'recognizing' ? colors.info : colors.primary }"
            @click="retakePhoto(it)"
          >
            <text class="retake-btn-text" :style="{ color: colors.white }">{{ it.status == 'recognizing' ? '检查中' : '重拍' }}</text>
          </view>
        </view>
        <view class="btn-big" :style="{ backgroundColor: colors.success }" @click="submitPoint">
          <text class="btn-big-text" :style="{ color: colors.white }">重新提交本点位</text>
        </view>
      </block>

      <!-- 异常确认步（红屏） -->
      <block v-if="phase == 'abnormal'">
        <text class="phase-title" :style="{ color: colors.danger }">发现异常</text>
        <view v-for="(it, i) in abnormalItems" :key="i" class="card" :style="{ backgroundColor: colors.bgCard }">
          <text class="abn-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
          <textarea
            v-if="aiEditable"
            class="abn-note"
            :style="{ borderColor: colors.border, color: colors.textPrimary, backgroundColor: colors.bgPage }"
            :value="it.note"
            placeholder="补充说明（可不填）"
            :maxlength="200"
            @input="onAbnNoteInput(it, $event)"
          />
          <text v-else class="abn-reason" :style="{ color: colors.danger }">{{ it.note != '' ? it.note : 'AI 判断该项异常' }}</text>
        </view>
        <view class="btn-big" :style="{ backgroundColor: colors.danger }" @click="confirmAbnormalSubmit">
          <text class="btn-big-text" :style="{ color: colors.white }">确认，去下一处</text>
        </view>
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
        <view class="done-btn" :style="{ backgroundColor: colors.white }" @click="exitWizard">
          <text class="done-btn-text" :style="{ color: colors.success }">返回</text>
        </view>
      </view>

      <!-- 底部「上一项」 -->
      <text v-if="showPrev" class="prev-link" :style="{ color: colors.textSecondary }" @click="prevStep">上一项</text>
      <view class="bottom-space"></view>
    </view>

    <!-- 上传 / AI 检查 / 提交中全屏遮盖 -->
    <view v-if="overlayMsg != ''" class="overlay" :style="{ backgroundColor: colors.mask }">
      <text class="overlay-text" :style="{ color: colors.white }">{{ overlayMsg }}</text>
    </view>

    <!-- 水印烧录用隐藏 canvas（屏外，尺寸由数据驱动） -->
    <canvas
      canvas-id="wmCanvasQuick"
      :style="{ width: canvasW + 'px', height: canvasH + 'px', position: 'fixed', left: '-9999px', top: '-9999px' }"
    ></canvas>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import {
  apiTaskDetail,
  apiCheckin,
  apiCheckinItems,
  apiUploadLocal,
  apiAiItemJobCreate,
  apiAiItemJobs,
  CODE_AI_DISABLED,
  CODE_CHECKIN_LOCKED,
  TaskPoint
} from '@/services/api'
import { burnWatermark } from '@/utils/watermark'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { getLocationGcj02 } from '@/utils/geo'
import { playVoice } from '@/utils/voice'
import { useAuthStore } from '@/stores/auth'
import {
  WizardSnap,
  WizardPointSnap,
  WizardItemSnap,
  wizardSnapKey,
  wizardModifySnapKey,
  loadWizardSnap,
  saveWizardSnap,
  clearWizardSnap
} from '@/utils/checkinWizard'

/** 向导阶段：cred 凭证 / items 逐项 / gate 提交本点位 / retake 补拍 / abnormal 异常确认 / pointDone 点位完成 / taskDone 任务完成 */
type Phase = 'cred' | 'items' | 'gate' | 'retake' | 'abnormal' | 'pointDone' | 'taskDone'

/** AI 轮询节奏：pending 时 1.5s 间隔批量查，30s 超时按 failed 处理（契约 §2） */
const POLL_INTERVAL = 1500
const POLL_TIMEOUT = 30000

type QuickData = {
  colors: ColorTokens
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
  /** 与点位距离（米），-1 表示未知 */
  distance: number
  /** 遮盖层文案（空 = 不显示） */
  overlayMsg: string
  submitting: boolean
  /** 轮询定时器 */
  pollTimer: any
  /** 页面已卸载（停止轮询回调写状态） */
  destroyed: boolean
  snapKey: string
  canvasW: number
  canvasH: number
  inspectorName: string
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

/** 二维码内容归一化：兼容短链接贴纸（/p/{code}）与早期 scheme 前缀（与 form.vue / 后端 normalizeQRCode 一致） */
function normalizeCode(v: string): string {
  const prefix = 'inspection://checkin?no='
  let s = (v || '').trim()
  if (s.indexOf(prefix) == 0) s = s.substring(prefix.length)
  const m = s.match(/\/p\/([A-Za-z0-9]+)\/?(?:[?#].*)?$/)
  if (m != null) s = m[1]
  return s
}

/** 由点位模板生成向导项初始状态 */
function freshItem(name: string, requirement: string, judgeType: string): WizardItemSnap {
  return {
    name: name,
    requirement: requirement,
    judge_type: judgeType,
    photos: [],
    file_keys: [],
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
  data(): QuickData {
    return {
      colors: Colors,
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
      distance: -1,
      overlayMsg: '',
      submitting: false,
      pollTimer: null,
      destroyed: false,
      snapKey: '',
      canvasW: 0,
      canvasH: 0,
      inspectorName: ''
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
    abnormalItems(): WizardItemSnap[] {
      const wp = this.curWizPoint
      if (wp == null) return []
      return this.abnormalIdxs.map((i) => wp.items[i]).filter((it) => it != null)
    },
    /** 底部「上一项」：主推进阶段且遮盖层未开启时显示 */
    showPrev(): boolean {
      if (this.overlayMsg != '' || this.submitting) return false
      return this.phase == 'cred' || this.phase == 'items' || this.phase == 'gate'
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointIdParam = options && options.point_id ? String(options.point_id) : ''
    this.modify = options != null && options.mode == 'modify'
    if (options && options.no) {
      this.preVerifiedNo = normalizeCode(String(options.no))
    }
    const auth = useAuthStore()
    this.inspectorName = auth.userInfo != null ? auth.userInfo.name : ''
    this.load()
  },
  onUnload() {
    this.destroyed = true
    this.stopPoll()
  },
  onBackPress(): boolean {
    // 遮盖层（上传/AI 检查/提交中）与点位完成过渡页：拦截返回，不打断流程
    if (this.overlayMsg != '' || this.submitting || this.phase == 'pointDone') return true
    if (this.phase == 'taskDone') return false
    if (this.phase == 'retake' || this.phase == 'abnormal') {
      // 收尾页返回 = 回到提交本点位
      this.phase = 'gate'
      this.persist()
      return true
    }
    this.prevStep()
    return true
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
    /** 修改模式：单点位，预填已有内容，提交走覆盖语义 */
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
      this.snapKey = wizardModifySnapKey(this.taskId, pt.point_id)
      const snap = loadWizardSnap(this.snapKey)
      if (snap != null && snap.points.length > 0 && snap.points[0].point_id == pt.point_id) {
        this.wizPoints = snap.points
        this.pointIdx = 0
        this.itemIdx = snap.itemIdx
      } else {
        this.wizPoints = [freshPoint(pt)]
        this.pointIdx = 0
        this.itemIdx = 0
        // 预填：凭已有打卡逐项结论回填（best-effort，照片无法复原需重拍）
        this.prefillModify(pt.my_checkin.id)
      }
      this.finishInit()
    },
    /** 普通模式：全部未打卡点位；有本地快照直接恢复（继续巡检） */
    initNormal() {
      this.snapKey = wizardSnapKey(this.taskId)
      const snap = loadWizardSnap(this.snapKey)
      let resumed = false
      if (snap != null && !snap.modify) {
        // 与服务端对账：仅保留仍存在且未打卡的点位
        const alive = snap.points.filter((wp) => {
          if (wp.status == 'submitted') return false
          const pt = this.taskPoints.find((p) => p.point_id == wp.point_id)
          return pt != null && pt.my_checkin == null
        })
        if (alive.length > 0) {
          this.wizPoints = alive
          this.pointIdx = Math.min(Math.max(snap.pointIdx, 0), alive.length - 1)
          this.itemIdx = Math.max(snap.itemIdx, 0)
          resumed = true
        }
      }
      if (!resumed) {
        const rest = this.taskPoints.filter((p) => p.my_checkin == null)
        if (rest.length == 0) {
          this.loading = false
          this.errorMsg = '本任务已全部打卡'
          return
        }
        this.wizPoints = rest.map((p) => freshPoint(p))
        this.pointIdx = 0
        this.itemIdx = 0
      }
      // 扫码进入：预核验编号写到匹配点位上
      if (this.preVerifiedNo != '') {
        this.wizPoints.forEach((wp) => {
          const pt = this.taskPoints.find((p) => p.point_id == wp.point_id)
          if (pt != null && pt.qrcode_no != '' && pt.qrcode_no == this.preVerifiedNo) {
            wp.scannedNo = this.preVerifiedNo
          }
        })
      }
      this.finishInit()
    },
    finishInit() {
      this.loading = false
      this.loaded = true
      this.clampIndices()
      // 识别中的项凭 job_id 批量查一次：仍 pending 留待收尾轮询；failed/过期回退待拍
      this.reconcileJobs()
      this.enterPoint(true)
    },
    clampIndices() {
      if (this.pointIdx >= this.wizPoints.length) this.pointIdx = this.wizPoints.length - 1
      if (this.pointIdx < 0) this.pointIdx = 0
      const wp = this.curWizPoint
      if (wp == null) return
      if (this.itemIdx > wp.items.length) this.itemIdx = wp.items.length
      if (this.itemIdx < 0) this.itemIdx = 0
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
          this.persist()
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
      this.persist()
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
          if (this.curPoint != null) {
            this.distance = Math.round(
              haversine(loc.longitude, loc.latitude, this.curPoint.longitude, this.curPoint.latitude)
            )
          }
          this.locating = false
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
    scanCredential() {
      uni.scanCode({
        onlyFromCamera: true, // 禁相册选图防代扫
        success: (res) => {
          const code = normalizeCode(res.result)
          if (this.curPoint != null && this.curPoint.qrcode_no != '' && code != this.curPoint.qrcode_no) {
            uni.showToast({ title: '二维码与本点位不匹配', icon: 'none' })
            return
          }
          if (this.curWizPoint != null) {
            this.curWizPoint.scannedNo = code
            this.persist()
          }
          uni.showToast({ title: '点位校验成功', icon: 'success' })
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
          this.persist()
        }
        uni.showToast({ title: '点位校验成功', icon: 'success' })
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
      this.persist()
    },
    /** 水印行：时间+点位 / 经纬度+距离 / 巡检员（同 form.vue） */
    wmLines(): string[] {
      const lines: string[] = []
      const name = this.curPoint != null ? this.curPoint.point_name : ''
      lines.push(fmtDateTime(new Date()) + ' ' + name)
      if (this.hasLoc) {
        let l = this.myLng.toFixed(6) + ',' + this.myLat.toFixed(6)
        if (this.distance >= 0) l += ' 距点位 ' + this.distance + 'm'
        lines.push(l)
      }
      if (this.inspectorName != '') {
        lines.push('巡检员：' + this.inspectorName)
      }
      return lines
    },
    /** 当前项拍照（仅相机）→ 水印 → 上传 → 建识别 job → 立即推进下一项，不等结果 */
    takePhoto() {
      const it = this.curItem
      if (it == null || !this.curItemIsPhoto) return
      this.shootFor(it, true)
    },
    /** 补拍步重拍：重拍 = 重新拍照上传重新建 job，停留在补拍列表 */
    retakePhoto(it: WizardItemSnap) {
      if (it.status == 'recognizing') return
      this.shootFor(it, false)
    },
    shootFor(it: WizardItemSnap, advance: boolean) {
      const wp = this.curWizPoint
      if (wp == null) return
      const pointId = wp.point_id
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          this.overlayMsg = '照片处理中…'
          let burned = ''
          let fileKey = ''
          burnWatermark(paths[0], this.wmLines(), 'wmCanvasQuick', this)
            .then((b) => {
              burned = b
              this.overlayMsg = '照片上传中…'
              return apiUploadLocal(b)
            })
            .then((r) => {
              fileKey = r.file_key
              return apiAiItemJobCreate({
                task_id: this.taskId,
                point_id: pointId,
                name: it.name,
                file_keys: [fileKey]
              })
            })
            .then((j) => {
              this.overlayMsg = ''
              it.photos = [burned]
              it.file_keys = [fileKey]
              it.job_id = j.job_id
              it.status = 'recognizing'
              it.verdict = ''
              it.reason = ''
              it.reading = ''
              it.quality_pass = true
              it.quality_issue = ''
              this.persist()
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
      this.persist()
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
      this.persist()
      this.nextStep()
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
      }
      this.persist()
    },
    /** 回退：上一项 → 凭证步 → 上一点位 → 任务第一步时退出向导 */
    prevStep() {
      const wp = this.curWizPoint
      if (wp == null) {
        this.exitWizard()
        return
      }
      this.manualAbnormalOpen = false
      if (this.phase == 'gate') {
        if (wp.items.length > 0) {
          this.itemIdx = wp.items.length - 1
          this.phase = 'items'
          this.persist()
          return
        }
        this.phase = 'cred'
        this.persist()
        return
      }
      if (this.phase == 'items') {
        if (this.itemIdx > 0) {
          this.itemIdx -= 1
          this.persist()
          return
        }
        if (this.needsCred) {
          this.phase = 'cred'
          this.persist()
          return
        }
      }
      // phase == 'cred' 或无凭证点位的第一项：上一点位 / 退出
      if (this.pointIdx > 0) {
        this.pointIdx -= 1
        const prev = this.curWizPoint
        this.itemIdx = prev != null && prev.items.length > 0 ? prev.items.length - 1 : 0
        this.phase = prev != null && prev.items.length > 0 ? 'items' : 'gate'
        this.locate()
        this.persist()
        return
      }
      this.exitWizard()
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
          this.persist()
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
      it.file_keys = []
      it.job_id = ''
      it.status = 'todo'
      it.verdict = ''
      it.reason = ''
      it.reading = ''
      it.quality_pass = true
      it.quality_issue = ''
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
        this.persist()
        return
      }
      if (abnormal.length > 0) {
        this.abnormalIdxs = abnormal
        this.phase = 'abnormal'
        uni.vibrateShort({})
        playVoice('abnormal')
        this.persist()
        return
      }
      this.doCheckin('normal', [])
    },
    retakeIssue(it: WizardItemSnap): string {
      if (it.status == 'todo') return '还没拍'
      if (it.status == 'failed') return it.quality_issue != '' ? it.quality_issue : '识别失败，请重拍'
      return it.quality_issue != '' ? it.quality_issue : '照片不合格'
    },
    onAbnNoteInput(it: WizardItemSnap, e: any) {
      it.note = e != null && e.detail != null ? String(e.detail.value) : ''
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
          photos: it.file_keys.slice(),
          ai_verdict: it.verdict,
          ai_reason: it.reason,
          ai_reading: it.reading
        }
      })
      const photoRefs: Array<{ item: string; file_key: string }> = []
      wp.items.forEach((it) => {
        it.file_keys.forEach((k) => {
          photoRefs.push({ item: it.name, file_key: k })
        })
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
        client_time: fmtDateTime(new Date()),
        result: result,
        ai_confirmed: true,
        remark: remark,
        check_items: checkItems,
        photos: photoRefs
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
    /** 提交成功：点位从快照移除 → 绿勾 → 自动下一点位 / 任务完成 */
    afterPointSubmitted() {
      if (this.modify) {
        clearWizardSnap(this.snapKey)
        uni.showToast({ title: '已提交修改', icon: 'success' })
        setTimeout(() => this.exitWizard(), 600)
        return
      }
      // 提交成功的点位从快照移除
      this.wizPoints.splice(this.pointIdx, 1)
      if (this.wizPoints.length == 0) {
        clearWizardSnap(this.snapKey)
        this.phase = 'taskDone'
        playVoice('normal')
        return
      }
      this.pointIdx = Math.min(this.pointIdx, this.wizPoints.length - 1)
      this.itemIdx = 0
      this.persist()
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
          this.persist()
        })
        .catch(() => {
          // 预填失败按全新巡检处理
        })
    },
    persist() {
      if (this.snapKey == '') return
      const snap: WizardSnap = {
        task_id: this.taskId,
        modify: this.modify,
        pointIdx: this.pointIdx,
        itemIdx: this.itemIdx,
        points: this.wizPoints,
        saved_at: 0
      }
      saveWizardSnap(this.snapKey, snap)
    },
    exitWizard() {
      uni.navigateBack()
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

/* 顶部进度区 */
.head {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.head-progress {
  font-size: 48rpx;
  font-weight: 700;
}

.progress {
  height: 24rpx;
  border-radius: 12rpx;
  overflow: hidden;
  margin-top: 16rpx;
}

.progress-inner {
  height: 24rpx;
  border-radius: 12rpx;
}

.head-point {
  font-size: 56rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.head-building {
  font-size: 32rpx;
  margin-top: 8rpx;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

/* 凭证步 */
.cred-ok {
  font-size: 40rpx;
  font-weight: 600;
  text-align: center;
}

.fence-text {
  font-size: 44rpx;
  font-weight: 700;
  text-align: center;
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

.cred-gap {
  margin-top: 20rpx;
}

/* 大按钮 */
.btn-big {
  height: 120rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 24rpx;
}

.btn-big-text {
  font-size: 44rpx;
  font-weight: 700;
}

.btn-normal {
  margin-top: 32rpx;
}

/* 逐项卡片 */
.item-card {
  border-radius: 24rpx;
  padding: 48rpx 32rpx;
  align-items: center;
  margin-bottom: 24rpx;
}

.item-name {
  font-size: 56rpx;
  font-weight: 700;
  text-align: center;
}

.item-hint {
  font-size: 34rpx;
  text-align: center;
  margin-top: 16rpx;
  line-height: 48rpx;
}

.big-shot {
  width: 320rpx;
  height: 320rpx;
  border-radius: 160rpx;
  align-items: center;
  justify-content: center;
  margin-top: 48rpx;
}

.big-shot-text {
  font-size: 40rpx;
  font-weight: 700;
}

.shot-ok {
  font-size: 44rpx;
  font-weight: 700;
  margin-top: 48rpx;
}

.reshot {
  width: 100%;
  margin-top: 32rpx;
}

.manual-note {
  width: 100%;
  height: 192rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: solid;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 8rpx;
  margin-bottom: 24rpx;
}

/* 收尾步 */
.gate-card {
  align-items: center;
}

.gate-title {
  font-size: 48rpx;
  font-weight: 700;
}

.gate-sub {
  font-size: 30rpx;
  margin-top: 12rpx;
}

.phase-title {
  font-size: 56rpx;
  font-weight: 700;
  text-align: center;
  margin: 24rpx 0;
}

/* 补拍列表 */
.retake-card {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.retake-texts {
  flex: 1;
}

.retake-name {
  font-size: 40rpx;
  font-weight: 700;
}

.retake-issue {
  font-size: 30rpx;
  margin-top: 8rpx;
}

.retake-btn {
  width: 160rpx;
  height: 96rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-left: 24rpx;
}

.retake-btn-text {
  font-size: 36rpx;
  font-weight: 700;
}

/* 异常确认 */
.abn-name {
  font-size: 40rpx;
  font-weight: 700;
}

.abn-reason {
  font-size: 34rpx;
  margin-top: 12rpx;
  line-height: 48rpx;
}

.abn-note {
  width: 100%;
  height: 160rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: solid;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 16rpx;
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

/* 上一项 */
.prev-link {
  font-size: 30rpx;
  padding: 24rpx;
  align-self: flex-start;
}

.bottom-space {
  height: 64rpx;
}

/* 遮盖层 */
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

.overlay-text {
  font-size: 56rpx;
  font-weight: 700;
}
</style>
