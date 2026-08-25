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

    <!-- 极简连续打卡 -->
    <view v-else class="content">
      <!-- 顶部进度区 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="progress-big" :style="{ color: colors.textPrimary }">已完成 {{ donePoints }} / {{ totalPoints }}</text>
        <view class="progress" :style="{ backgroundColor: colors.border }">
          <view class="progress-inner" :style="{ width: progressWidth, backgroundColor: colors.success }"></view>
        </view>
        <text class="point-name" :style="{ color: colors.textPrimary }">{{ point ? point.point_name : '' }}</text>
        <text class="point-building" :style="{ color: colors.textSecondary }">{{ point && point.building_name != '' ? point.building_name : '未分区' }}</text>
      </view>

      <!-- 凭证区 -->
      <view v-if="point != null && point.credential == 'qrcode'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text v-if="scannedNo != ''" class="cred-ok" :style="{ color: colors.success }">✓ 已扫码</text>
        <view v-else class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
          <text class="btn-outline-text" :style="{ color: colors.primary }">扫码打卡</text>
        </view>
      </view>
      <view v-else-if="point != null && point.credential == 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text v-if="nfcCardId != ''" class="cred-ok" :style="{ color: colors.success }">✓ 已读卡</text>
        <view v-else class="btn-outline" :style="{ borderColor: colors.primary }" @click="nfcTap">
          <text class="btn-outline-text" :style="{ color: colors.primary }">读卡打卡</text>
        </view>
      </view>
      <view v-else-if="point != null && point.credential == 'any'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text v-if="scannedNo != '' || nfcCardId != ''" class="cred-ok" :style="{ color: colors.success }">✓ 已核验</text>
        <block v-else>
          <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
            <text class="btn-outline-text" :style="{ color: colors.primary }">扫码打卡</text>
          </view>
          <view class="btn-outline cred-gap" :style="{ borderColor: colors.primary }" @click="nfcTap">
            <text class="btn-outline-text" :style="{ color: colors.primary }">读卡打卡</text>
          </view>
        </block>
      </view>
      <!-- 电子围栏：自动定位一次，大字显示在/超范围 -->
      <view v-if="point != null && point.require_fence" class="card" :style="{ backgroundColor: colors.bgCard }" @click="onLocTap">
        <text v-if="locating" class="fence-text" :style="{ color: colors.textSecondary }">定位中…</text>
        <text v-else-if="locFailed" class="fence-text" :style="{ color: colors.danger }">定位失败，点我重试</text>
        <text v-else-if="distance >= 0 && distance <= point.fence_radius" class="fence-text" :style="{ color: colors.success }">在范围内（{{ distance }}米）</text>
        <text v-else-if="distance >= 0" class="fence-text" :style="{ color: colors.danger }">超出范围（{{ distance }}米）</text>
      </view>

      <!-- 拍照区 -->
      <view class="card photo-card" :style="{ backgroundColor: colors.bgCard }">
        <!-- 未拍：巨大圆形拍照按钮 -->
        <view v-if="photos.length == 0" class="big-shot" :style="{ backgroundColor: colors.primary }" @click="takePhoto">
          <text class="big-shot-text" :style="{ color: colors.white }">拍一张照片</text>
        </view>
        <!-- 已拍：缩略图横排 + 重拍/提交 -->
        <block v-else>
          <view class="photos">
            <image
              v-for="(ph, pi) in photos"
              :key="pi"
              class="photo"
              :src="ph"
              mode="aspectFill"
              @click="removePhoto(pi)"
            />
            <view
              v-if="photos.length < 3"
              class="photo-add"
              :style="{ borderColor: colors.border }"
              @click="takePhoto"
            >
              <text class="photo-add-text" :style="{ color: colors.textSecondary }">再拍一张</text>
            </view>
          </view>
          <view class="btn-row">
            <view class="btn-retake" :style="{ borderColor: colors.primary }" @click="retakeAll">
              <text class="btn-retake-text" :style="{ color: colors.primary }">重拍</text>
            </view>
            <view class="btn-submit" :style="{ backgroundColor: colors.success }" @click="submit(false)">
              <text class="btn-submit-text" :style="{ color: colors.white }">提交</text>
            </view>
          </view>
        </block>
      </view>

      <!-- 完整模式入口（不显眼，给熟手留后路） -->
      <text class="full-link" :style="{ color: colors.textSecondary }" @click="goFullMode">切换到完整模式</text>
      <view class="bottom-space"></view>
    </view>

    <!-- 提交中全屏遮盖（上传/AI 判定） -->
    <view v-if="overlayMsg != ''" class="overlay" :style="{ backgroundColor: colors.mask }">
      <text class="overlay-text" :style="{ color: colors.white }">{{ overlayMsg }}</text>
    </view>

    <!-- 结果反馈全屏遮盖层 -->
    <view v-if="resultState != ''" class="overlay" :style="{ backgroundColor: resultBg }">
      <text class="result-icon" :style="{ color: colors.white }">{{ resultIcon }}</text>
      <text class="result-title" :style="{ color: colors.white }">{{ resultTitle }}</text>
      <text v-if="resultMsg != ''" class="result-msg" :style="{ color: colors.white }">{{ resultMsg }}</text>
      <view v-if="resultState == 'abnormal'" class="result-btn" :style="{ backgroundColor: colors.white }" @click="goNext">
        <text class="result-btn-text" :style="{ color: colors.danger }">知道了，去下一处</text>
      </view>
      <view v-if="resultState == 'blurry' && maxAttempts > 0 && attempts >= maxAttempts" class="result-btn" :style="{ backgroundColor: colors.white }" @click="submit(true)">
        <text class="result-btn-text" :style="{ color: colors.warning }">总是拍不好？强制提交</text>
      </view>
      <view v-if="resultState == 'error'" class="result-btn" :style="{ backgroundColor: colors.white }" @click="closeResult">
        <text class="result-btn-text" :style="{ color: colors.danger }">重试</text>
      </view>
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
import { apiTaskDetail, apiCheckin, apiUploadLocal, TaskPoint, CheckinResult } from '@/services/api'
import { burnWatermark } from '@/utils/watermark'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { getLocationGcj02 } from '@/utils/geo'
import { playVoice, VoiceType } from '@/utils/voice'
import { useAuthStore } from '@/stores/auth'

/** 结果反馈状态：'' 无 / normal 正常 / abnormal 异常 / review 待复核 / blurry 质量失败 / error 其他错误 */
type ResultState = '' | 'normal' | 'abnormal' | 'review' | 'blurry' | 'error'

/** 业务错误码：照片质量不达标 */
const CODE_QUALITY_FAIL = 43107

type QuickData = {
  colors: ColorTokens
  taskId: string
  pointId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  point: TaskPoint | null
  totalPoints: number
  donePoints: number
  locating: boolean
  locFailed: boolean
  hasLoc: boolean
  myLng: number
  myLat: number
  /** 与点位距离（米），-1 表示未知 */
  distance: number
  /** 扫码核验后的二维码编号（空 = 未核验） */
  scannedNo: string
  /** NFC 核验后读到的卡片 UID（空 = 未核验） */
  nfcCardId: string
  /** 水印烧录后的本地照片路径（最多 3 张） */
  photos: string[]
  /** 提交中遮盖层文案（空 = 不显示） */
  overlayMsg: string
  submitting: boolean
  resultState: ResultState
  resultMsg: string
  /** 质量失败已重试次数 */
  attempts: number
  /** 质量放行次数上限（43107 信封 data.max_attempts） */
  maxAttempts: number
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

export default {
  data(): QuickData {
    return {
      colors: Colors,
      taskId: '',
      pointId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      point: null,
      totalPoints: 0,
      donePoints: 0,
      locating: false,
      locFailed: false,
      hasLoc: false,
      myLng: 0,
      myLat: 0,
      distance: -1,
      scannedNo: '',
      nfcCardId: '',
      photos: [] as string[],
      overlayMsg: '',
      submitting: false,
      resultState: '',
      resultMsg: '',
      attempts: 0,
      maxAttempts: 0,
      canvasW: 0,
      canvasH: 0,
      inspectorName: ''
    }
  },
  computed: {
    progressWidth(): string {
      if (this.totalPoints <= 0) return '0%'
      return Math.round((this.donePoints / this.totalPoints) * 100) + '%'
    },
    /** 围栏未超出（无围栏点位恒 true；定位未成功时按可提交，由后端距离兜底） */
    fenceOk(): boolean {
      if (this.point == null || !this.point.require_fence) return true
      return this.distance >= 0 && this.distance <= this.point.fence_radius
    },
    credentialOk(): boolean {
      if (this.point == null) return false
      const c = this.point.credential
      if (c == 'qrcode') return this.scannedNo != ''
      if (c == 'nfc') return this.nfcCardId != ''
      if (c == 'any') return this.scannedNo != '' || this.nfcCardId != ''
      return true
    },
    resultIcon(): string {
      if (this.resultState == 'normal') return '✓'
      if (this.resultState == 'review') return '!'
      return '✕'
    },
    resultTitle(): string {
      if (this.resultState == 'normal') return '正常'
      if (this.resultState == 'abnormal') return '发现异常'
      if (this.resultState == 'review') return '已拍照，等管理员确认'
      if (this.resultState == 'blurry') return '照片不合格'
      return '出错了'
    },
    resultBg(): string {
      if (this.resultState == 'normal') return Colors.success
      if (this.resultState == 'review') return Colors.warning
      return Colors.danger
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointId = options && options.point_id ? String(options.point_id) : ''
    // 扫码进入时带来的二维码编号，视为已核验凭证（同 form.vue）
    if (options && options.no) {
      this.scannedNo = normalizeCode(String(options.no))
    }
    const auth = useAuthStore()
    this.inspectorName = auth.userInfo != null ? auth.userInfo.name : ''
    this.load()
  },
  methods: {
    load() {
      if (this.taskId == '' || this.pointId == '') {
        this.loading = false
        this.errorMsg = '缺少打卡参数'
        return
      }
      this.loading = true
      apiTaskDetail(this.taskId)
        .then((res) => {
          const pt = res.points.find((p: TaskPoint) => p.point_id == this.pointId)
          if (pt == null) {
            this.loading = false
            this.errorMsg = '点位不属于该任务'
            return
          }
          if (pt.my_checkin != null) {
            this.loading = false
            this.errorMsg = '该点位已打卡'
            return
          }
          this.point = pt
          this.totalPoints = res.total_points
          this.donePoints = res.done_points
          this.loading = false
          this.loaded = true
          // 加载完成自动定位一次
          this.locate()
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
        })
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
          if (this.point != null) {
            this.distance = Math.round(
              haversine(loc.longitude, loc.latitude, this.point.longitude, this.point.latitude)
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
      // 定位失败时可点击重试
      if (this.locFailed) this.locate()
    },
    scanCredential() {
      uni.scanCode({
        onlyFromCamera: true, // 禁相册选图防代扫
        success: (res) => {
          const code = normalizeCode(res.result)
          if (this.point != null && this.point.qrcode_no != '' && code != this.point.qrcode_no) {
            uni.showToast({ title: '二维码与本点位不匹配', icon: 'none' })
            return
          }
          this.scannedNo = code
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
        if (this.point != null && this.point.nfc_id != '' && cardId != this.point.nfc_id) {
          uni.showToast({ title: 'NFC 标签与本点位不匹配', icon: 'none' })
          return
        }
        this.nfcCardId = cardId
        uni.showToast({ title: '点位校验成功', icon: 'success' })
      })
    },
    /** 水印行：时间+点位 / 经纬度+距离 / 巡检员（同 form.vue） */
    wmLines(): string[] {
      const lines: string[] = []
      const name = this.point != null ? this.point.point_name : ''
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
    /** 拍照（仅相机）→ 水印烧录 → 追加（最多 3 张） */
    takePhoto() {
      if (this.photos.length >= 3) return
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          uni.showLoading({ title: '处理中…', mask: true })
          burnWatermark(paths[0], this.wmLines(), 'wmCanvasQuick', this)
            .then((burned) => {
              this.photos.push(burned)
              uni.hideLoading()
            })
            .catch(() => uni.hideLoading())
        }
      })
    },
    /** 点缩略图删除该张（大按钮确认，无长按手势） */
    removePhoto(idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) this.photos.splice(idx, 1)
        }
      })
    },
    /** 重拍：清空已拍照片，直接拉起相机 */
    retakeAll() {
      this.photos = []
      this.takePhoto()
    },
    /**
     * 提交：逐张上传换 file_key → result=auto 打卡（AI 代判）。
     * force=true 为质量失败超放行次数后的强制提交（成功反馈按待复核处理）。
     */
    submit(force: boolean) {
      if (this.submitting) return
      if (!force) {
        if (this.photos.length == 0) return
        if (!this.credentialOk) {
          uni.showToast({ title: '请先完成点位核验', icon: 'none' })
          return
        }
        if (this.point != null && this.point.require_fence && !this.fenceOk) {
          uni.showToast({ title: '超出范围，走近一点再打卡', icon: 'none' })
          return
        }
        if (!this.hasLoc) {
          uni.showToast({ title: '定位失败，请重试', icon: 'none' })
          this.locate()
          return
        }
      }
      this.resultState = ''
      // 极简模式仅在线可用（离线/AI 关闭时后端拒绝 auto）
      uni.getNetworkType({
        success: (net) => {
          if (net.networkType == 'none') {
            uni.showToast({ title: '没网络，连上网再打卡', icon: 'none' })
            return
          }
          this.doSubmit(force)
        },
        fail: () => {
          this.doSubmit(force)
        }
      })
    },
    doSubmit(force: boolean) {
      const pt = this.point as TaskPoint
      this.submitting = true
      this.overlayMsg = '照片上传中…'
      const photoRefs: Array<{ item: string; file_key: string }> = []
      let chain: Promise<void> = Promise.resolve()
      this.photos.forEach((p) => {
        chain = chain
          .then(() => apiUploadLocal(p))
          .then((r) => {
            photoRefs.push({ item: '', file_key: r.file_key })
          })
      })
      chain
        .then(() => {
          this.overlayMsg = 'AI 检查中…'
          return apiCheckin({
            task_id: this.taskId,
            point_id: this.pointId,
            checkin_type: pt.credential == 'nfc' ? 'nfc' : (this.scannedNo != '' ? 'qrcode' : (this.nfcCardId != '' ? 'nfc' : 'fence')),
            qrcode_no: this.scannedNo != '' ? this.scannedNo : undefined,
            nfc_id: pt.credential == 'nfc' || pt.credential == 'any' ? (this.nfcCardId != '' ? this.nfcCardId : undefined) : undefined,
            longitude: this.myLng,
            latitude: this.myLat,
            client_time: fmtDateTime(new Date()),
            result: 'auto',
            force: force ? true : undefined,
            remark: '',
            photos: photoRefs
          })
        })
        .then((res: CheckinResult) => {
          this.submitting = false
          this.overlayMsg = ''
          this.handleResult(res, force)
        })
        .catch((e: any) => {
          this.submitting = false
          this.overlayMsg = ''
          this.handleError(e)
        })
    },
    /** 同步判定结果分流：正常 / 异常 / 待复核（force 强制提交成功按待复核展示） */
    handleResult(res: CheckinResult, force: boolean) {
      const abnormalItems = (res.ai_items ?? []).filter((it) => it.verdict == 'abnormal')
      if (force) {
        this.showResult('review', '已提交，等管理员确认', 'review', false)
        return
      }
      if (res.audit_status == 'auto_pass' && abnormalItems.length == 0) {
        this.showResult('normal', '', 'normal', false)
        // 1.2s 后自动去下一处
        setTimeout(() => {
          this.goNext()
        }, 1200)
        return
      }
      if (abnormalItems.length > 0 || res.ai_verdict == 'abnormal') {
        const lines = abnormalItems.map((it) => it.name + (it.reason != '' ? '：' + it.reason : ''))
        const msg = lines.length > 0 ? lines.join('\n') : (res.ai_reason != '' ? res.ai_reason : '')
        this.showResult('abnormal', msg, 'abnormal', true)
        return
      }
      // pending 且无异常项 → 待复核，1.2s 后自动去下一处
      this.showResult('review', '', 'review', false)
      setTimeout(() => {
        this.goNext()
      }, 1200)
    },
    /** 提交失败分流：43107 质量失败留在本点位重拍；其余大字报错可重试 */
    handleError(e: any) {
      const code = e != null && typeof e.code == 'number' ? e.code : 0
      const msg = e != null && e.message != null && e.message != '' ? e.message : '提交失败，请重试'
      if (code == CODE_QUALITY_FAIL) {
        this.attempts += 1
        const max = e != null && e.data != null && typeof e.data.max_attempts == 'number' ? e.data.max_attempts : 0
        if (max > 0) this.maxAttempts = max
        this.showResult('blurry', msg, 'blurry', true)
        return
      }
      this.showResult('error', msg, null, false)
    },
    /** 展示结果遮盖层：震动（异常/质量失败）+ 语音播报 */
    showResult(state: ResultState, msg: string, voice: VoiceType | null, vibrate: boolean) {
      this.resultState = state
      this.resultMsg = msg
      if (vibrate) {
        uni.vibrateShort({})
      }
      if (voice != null) {
        playVoice(voice)
      }
    },
    /** 关闭结果层（错误重试：回到拍照区，照片保留可直接再提交或重拍） */
    closeResult() {
      this.resultState = ''
      this.resultMsg = ''
    },
    /** 去下一处：重拉任务找第一个未打卡点位；全部完成则返回任务详情 */
    goNext() {
      this.resultState = ''
      this.resultMsg = ''
      apiTaskDetail(this.taskId)
        .then((res) => {
          const next = res.points
            .filter((p: TaskPoint) => p.my_checkin == null)
            .sort((a: TaskPoint, b: TaskPoint) => a.sort - b.sort)[0]
          if (next != null) {
            uni.redirectTo({
              url:
                '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
                '&point_id=' + encodeURIComponent(next.point_id)
            })
            return
          }
          uni.showToast({ title: '本任务已全部打卡', icon: 'none' })
          setTimeout(() => {
            uni.navigateBack()
          }, 800)
        })
        .catch(() => {
          // 拉取失败不阻断：直接返回任务详情（onShow 会刷新状态）
          uni.navigateBack()
        })
    },
    /** 切换完整模式（熟手后路）：替换当前页跳原打卡表单，参数透传 */
    goFullMode() {
      let url =
        '/pages/checkin/form?task_id=' + encodeURIComponent(this.taskId) +
        '&point_id=' + encodeURIComponent(this.pointId)
      if (this.scannedNo != '') {
        url += '&no=' + encodeURIComponent(this.scannedNo)
      }
      uni.redirectTo({ url: url })
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

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

/* 顶部进度区 */
.progress-big {
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

.point-name {
  font-size: 56rpx; /* FontSize.display */
  font-weight: 700;
  margin-top: 24rpx;
}

.point-building {
  font-size: 32rpx;
  margin-top: 8rpx;
}

/* 凭证区 */
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

/* 拍照区 */
.photo-card {
  align-items: center;
}

.big-shot {
  width: 320rpx;
  height: 320rpx;
  border-radius: 160rpx;
  align-items: center;
  justify-content: center;
  margin: 32rpx 0;
}

.big-shot-text {
  font-size: 40rpx;
  font-weight: 700;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  justify-content: center;
  width: 100%;
}

.photo {
  width: 200rpx;
  height: 200rpx;
  border-radius: 16rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.photo-add {
  width: 200rpx;
  height: 200rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: dashed;
  align-items: center;
  justify-content: center;
  margin-bottom: 16rpx;
}

.photo-add-text {
  font-size: 30rpx;
}

.btn-row {
  flex-direction: row;
  width: 100%;
  margin-top: 16rpx;
}

.btn-retake {
  flex: 1;
  height: 112rpx;
  border-radius: 20rpx;
  border-width: 2rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
  margin-right: 20rpx;
}

.btn-retake-text {
  font-size: 40rpx;
  font-weight: 600;
}

.btn-submit {
  flex: 2;
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-submit-text {
  font-size: 44rpx;
  font-weight: 700;
}

/* 完整模式入口 */
.full-link {
  font-size: 26rpx;
  text-align: center;
  padding: 24rpx;
}

.bottom-space {
  height: 64rpx;
}

/* 遮盖层（提交中 / 结果反馈） */
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

.result-icon {
  font-size: 200rpx;
  font-weight: 700;
  line-height: 220rpx;
}

.result-title {
  font-size: 72rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.result-msg {
  font-size: 40rpx;
  margin-top: 32rpx;
  text-align: center;
  line-height: 56rpx;
}

.result-btn {
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 48rpx;
  padding: 0 64rpx;
}

.result-btn-text {
  font-size: 40rpx;
  font-weight: 700;
}
</style>
