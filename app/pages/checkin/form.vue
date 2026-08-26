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

    <!-- 打卡表单 -->
    <view v-else class="content">
      <!-- 点位头部 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="point-name" :style="{ color: colors.textPrimary }">{{ point ? point.point_name : '' }}</text>
        <text class="point-sub" :style="{ color: colors.textSecondary }">编号：{{ point ? point.qrcode_no : '' }}</text>
        <!-- GPS 行 -->
        <view class="loc-row" @click="onLocTap">
          <text v-if="locating" class="loc-text" :style="{ color: colors.textSecondary }">定位中…</text>
          <text v-else-if="locFailed" class="loc-text" :style="{ color: colors.danger }">定位失败，点我重试</text>
          <text v-else class="loc-text" :style="{ color: distColor }">{{ distText }}</text>
        </view>
      </view>

      <!-- 凭证校验区：任一（any）时扫码/NFC 两个入口并列，核验其一即可 -->
      <view v-if="needScan" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">点位凭证</text>
        <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
          <text class="btn-outline-text" :style="{ color: colors.primary }">扫码校验点位</text>
        </view>
        <view v-if="point != null && point.credential == 'any'" class="btn-outline cred-gap" :style="{ borderColor: colors.primary }" @click="nfcTap">
          <text class="btn-outline-text" :style="{ color: colors.primary }">NFC 校验</text>
        </view>
        <text v-if="point != null && point.credential == 'any'" class="cred-tip" :style="{ color: colors.textSecondary }">扫码或 NFC 任一方式核验即可</text>
      </view>
      <view v-else-if="scannedNo != '' && point != null && point.credential != 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">点位凭证</text>
        <text class="cred-ok" :style="{ color: colors.success }">已扫码校验：{{ scannedNo }}</text>
      </view>
      <view v-else-if="point && (point.credential == 'nfc' || point.credential == 'any') && nfcCardId != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">点位凭证</text>
        <text class="cred-ok" :style="{ color: colors.success }">已 NFC 校验：{{ nfcCardId }}</text>
      </view>
      <view v-else-if="point && point.credential == 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">点位凭证</text>
        <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="nfcTap">
          <text class="btn-outline-text" :style="{ color: colors.primary }">NFC 校验</text>
        </view>
      </view>

      <!-- 检查项列表 -->
      <view v-if="items.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="sec-head">
          <text class="sec-title" :style="{ color: colors.textPrimary }">检查项</text>
          <text class="sec-action" :style="{ color: colors.primary }" @click="allNormal">全部正常</text>
        </view>
        <view v-for="(it, idx) in items" :key="idx" class="item">
          <view class="item-head">
            <view class="item-texts">
              <text class="item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
              <text v-if="it.requirement != ''" class="item-req" :style="{ color: colors.textSecondary }">{{ it.requirement }}</text>
              <text v-if="it.photo_required == 'required'" class="item-req" :style="{ color: colors.warning }">必拍照片</text>
            </view>
            <view class="item-toggle">
              <text
                class="toggle-btn"
                :style="it.pass ? { color: colors.white, backgroundColor: colors.success } : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
                @click="setPass(idx, true)"
              >正常</text>
              <text
                class="toggle-btn"
                :style="!it.pass ? { color: colors.white, backgroundColor: colors.danger } : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
                @click="setPass(idx, false)"
              >异常</text>
            </view>
          </view>
          <!-- 异常备注（选异常必填） -->
          <textarea
            v-if="!it.pass"
            v-model="it.note"
            class="item-note"
            :style="{ borderColor: colors.border, color: colors.textPrimary }"
            placeholder="请填写该项异常情况（必填）"
            :maxlength="200"
          />
          <!-- 该项照片（异常项与必拍项展示） -->
          <view v-if="showItemPhotos(it)" class="photos">
            <image
              v-for="(ph, pi) in it.photos"
              :key="pi"
              class="photo"
              :src="ph"
              mode="aspectFill"
              @longpress="removePhoto(it.photos, pi)"
            />
            <view
              v-if="it.photos.length < 3"
              class="photo-add"
              :style="{ borderColor: colors.border }"
              @click="takePhotos(it.photos, 3)"
            >
              <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
            </view>
          </view>
        </view>
      </view>

      <!-- 现场照片（整单，必传 ≥1 张） -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">现场照片（必传）</text>
        <view class="photos">
          <image
            v-for="(ph, pi) in scenePhotos"
            :key="pi"
            class="photo"
            :src="ph"
            mode="aspectFill"
            @longpress="removePhoto(scenePhotos, pi)"
          />
          <view
            v-if="scenePhotos.length < 6"
            class="photo-add"
            :style="{ borderColor: colors.border }"
            @click="takePhotos(scenePhotos, 6)"
          >
            <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
          </view>
        </view>
        <text class="photo-tip" :style="{ color: colors.textSecondary }">最多 6 张，长按照片可删除</text>
      </view>

      <!-- 整单备注 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">备注</text>
        <textarea
          v-model="remark"
          class="remark"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="整体情况说明（有异常项时必填）"
          :maxlength="500"
        />
      </view>

      <!-- 提交 -->
      <view class="btn-primary" :style="{ backgroundColor: submitting ? colors.info : colors.primary }" @click="submit">
        <text class="btn-primary-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : '提交打卡' }}</text>
      </view>
      <view class="bottom-space"></view>
    </view>

    <!-- 水印烧录用隐藏 canvas（屏外，尺寸由数据驱动） -->
    <canvas
      canvas-id="wmCanvas"
      :style="{ width: canvasW + 'px', height: canvasH + 'px', position: 'fixed', left: '-9999px', top: '-9999px' }"
    ></canvas>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiTaskDetail, apiCheckin, apiCheckinItems, apiUploadLocal, TaskPoint, CheckinResult, CheckinItemAI, CheckinReqPayload } from '@/services/api'
import { burnWatermark } from '@/utils/watermark'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { getLocationGcj02 } from '@/utils/geo'
import { enqueueOfflineCheckin, uuidv7, NETWORK_ERR_PREFIX, OfflinePhoto } from '@/utils/offline'
import { useAuthStore } from '@/stores/auth'

/** 检查项视图模型：模板项 + 录入状态 */
type ItemView = {
  name: string
  requirement: string
  photo_required: string
  pass: boolean
  note: string
  /** 水印烧录后的本地路径，提交时上传换 file_key */
  photos: string[]
}

type FormData = {
  colors: ColorTokens
  taskId: string
  pointId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  point: TaskPoint | null
  locating: boolean
  locFailed: boolean
  hasLoc: boolean
  myLng: number
  myLat: number
  /** 与点位距离（米），-1 表示未知 */
  distance: number
  /** 扫码核验后的二维码编号（空 = 未核验） */
  scannedNo: string
  /** NFC 核验后读到的卡片 UID（十六进制，提交时作 nfc_id；空 = 未核验） */
  nfcCardId: string
  items: ItemView[]
  scenePhotos: string[]
  remark: string
  submitting: boolean
  /** AI 照片质量拦截计数与放行上限（43107 分支用；达到上限允许强制提交转人工复核） */
  qualityAttempts: number
  maxAttempts: number
  /** 强制提交标记（用户确认后重发带 force=true） */
  forceSubmit: boolean
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

/** 二维码内容归一化：兼容短链接贴纸（/p/{code}）与早期 scheme 前缀（与后端 normalizeQRCode 一致） */
function normalizeCode(v: string): string {
  const prefix = 'inspection://checkin?no='
  let s = (v || '').trim()
  if (s.indexOf(prefix) == 0) s = s.substring(prefix.length)
  const m = s.match(/\/p\/([A-Za-z0-9]+)\/?(?:[?#].*)?$/)
  if (m != null) s = m[1]
  return s
}

export default {
  data(): FormData {
    return {
      colors: Colors,
      taskId: '',
      pointId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      point: null,
      locating: false,
      locFailed: false,
      hasLoc: false,
      myLng: 0,
      myLat: 0,
      distance: -1,
      scannedNo: '',
      nfcCardId: '',
      items: [] as ItemView[],
      scenePhotos: [] as string[],
      remark: '',
      submitting: false,
      qualityAttempts: 0,
      maxAttempts: 3,
      forceSubmit: false,
      canvasW: 0,
      canvasH: 0,
      inspectorName: ''
    }
  },
  computed: {
    /** qrcode/any 凭证点位且未核验 → 显示凭证校验入口（any 并列扫码+NFC） */
    needScan(): boolean {
      if (this.point == null) return false
      const c = this.point.credential
      if (c == 'qrcode') return this.scannedNo == ''
      if (c == 'any') return this.scannedNo == '' && this.nfcCardId == ''
      return false
    },
    distText(): string {
      if (this.distance < 0) return ''
      let t = '距点位 ' + this.distance + ' m'
      if (this.point != null && this.point.require_fence) {
        t += this.distance <= this.point.fence_radius ? ' · ✓围栏内' : ' · 超出围栏'
      }
      return t
    },
    distColor(): string {
      if (this.point != null && this.point.require_fence && this.distance >= 0) {
        return this.distance <= this.point.fence_radius ? Colors.success : Colors.danger
      }
      return Colors.textRegular
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointId = options && options.point_id ? String(options.point_id) : ''
    // 扫码进入时带来的二维码编号，视为已核验凭证
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
          this.items = pt.check_items.map((c) => ({
            name: c.name,
            requirement: c.requirement,
            photo_required: c.photo_required,
            pass: true,
            note: '',
            photos: []
          }))
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
        // 卡片 UID 须与点位备案的「NFC 卡号」一致（与扫码校验同规则；后端提交时还会再比对一次）
        if (this.point != null && this.point.nfc_id != '' && cardId != this.point.nfc_id) {
          uni.showToast({ title: 'NFC 标签与本点位不匹配', icon: 'none' })
          return
        }
        this.nfcCardId = cardId
        uni.showToast({ title: '点位校验成功', icon: 'success' })
      })
    },
    setPass(idx: number, pass: boolean) {
      this.items[idx].pass = pass
    },
    /** 一键全部正常并清空逐项备注；不跳过必拍照片校验 */
    allNormal() {
      this.items.forEach((it) => {
        it.pass = true
        it.note = ''
      })
      const needPhoto = this.items.some((it) => it.photo_required == 'required' && it.photos.length == 0)
      if (needPhoto) {
        uni.showToast({ title: '仍有必拍照片项，请拍照', icon: 'none' })
      }
    },
    showItemPhotos(it: ItemView): boolean {
      // 异常项与模板必拍项均展示照片区
      return !it.pass || it.photo_required == 'required'
    },
    /** 拍照（仅相机防相册作弊）→ 水印烧录 → 入列表；list 为响应式数组引用 */
    takePhotos(list: string[], max: number) {
      const remain = max - list.length
      if (remain <= 0) return
      uni.chooseImage({
        count: remain,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          uni.showLoading({ title: '处理中…', mask: true })
          const lines = this.wmLines()
          let chain: Promise<void> = Promise.resolve()
          paths.forEach((p) => {
            chain = chain
              .then(() => burnWatermark(p, lines, 'wmCanvas', this))
              .then((burned) => {
                list.push(burned)
              })
          })
          chain
            .then(() => uni.hideLoading())
            .catch(() => uni.hideLoading())
        }
      })
    },
    removePhoto(list: string[], idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) list.splice(idx, 1)
        }
      })
    },
    /** 水印行：时间+点位 / 经纬度+距离 / 巡检员（无则跳过） */
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
    /** 提交前校验，返回错误文案（空串 = 通过） */
    validate(): string {
      if (this.point == null) return '点位信息缺失'
      if (this.point.credential == 'nfc' && this.nfcCardId == '') return '请先完成 NFC 校验'
      if (this.point.credential == 'qrcode' && this.scannedNo == '') return '请先扫码校验点位'
      if (this.point.credential == 'any' && this.scannedNo == '' && this.nfcCardId == '') return '请扫码或 NFC 校验点位'
      if (!this.hasLoc) return '请先完成定位'
      if (this.point.require_fence && this.distance > this.point.fence_radius) {
        return '距点位 ' + this.distance + ' m，超出围栏半径 ' + this.point.fence_radius + ' m'
      }
      for (let i = 0; i < this.items.length; i++) {
        const it = this.items[i]
        if (!it.pass && it.note.trim() == '') {
          return '请填写「' + it.name + '」异常备注'
        }
        if ((!it.pass || it.photo_required == 'required') && it.photos.length == 0) {
          return '「' + it.name + '」须至少拍 1 张照片'
        }
      }
      if (this.scenePhotos.length == 0) return '请至少拍 1 张现场照片'
      // 后端硬约束：异常打卡整单备注必填
      const abnormal = this.items.some((it) => !it.pass)
      if (abnormal && this.remark.trim() == '') return '存在异常项，请填写整单备注说明'
      return ''
    },
    submit() {
      if (this.submitting) return
      const err = this.validate()
      if (err != '') {
        uni.showToast({ title: err, icon: 'none' })
        return
      }
      // 无网直接离线暂存（不进上传/提交流程）；取网络类型失败按在线处理
      uni.getNetworkType({
        success: (net) => {
          if (net.networkType == 'none') {
            this.saveOffline()
          } else {
            this.submitOnline()
          }
        },
        fail: () => {
          this.submitOnline()
        }
      })
    },
    /** 离线暂存：req 带客户端 UUIDv7 幂等 ID，照片保留本地路径，入队后视同成功返回 */
    saveOffline() {
      const pt = this.point as TaskPoint
      const abnormal = this.items.some((it) => !it.pass)
      const req: CheckinReqPayload = {
        id: uuidv7(),
        task_id: this.taskId,
        point_id: this.pointId,
        checkin_type: pt.credential == 'nfc' ? 'nfc' : (this.scannedNo != '' ? 'qrcode' : (this.nfcCardId != '' ? 'nfc' : 'fence')),
        qrcode_no: pt.credential == 'nfc' ? undefined : (this.scannedNo != '' ? this.scannedNo : undefined),
        nfc_id: pt.credential == 'nfc' || pt.credential == 'any' ? (this.nfcCardId != '' ? this.nfcCardId : undefined) : undefined,
        longitude: this.myLng,
        latitude: this.myLat,
        client_time: fmtDateTime(new Date()),
        result: abnormal ? 'abnormal' : 'normal',
        remark: this.remark.trim(),
        check_items: this.items.map((it) => ({
          name: it.name,
          pass: it.pass,
          note: it.note.trim(),
          photos: []
        })),
        photos: []
      }
      // 照片保留本地路径（不删本地文件）：item=检查项名，整单现场照片 item=''
      const photosLocal: OfflinePhoto[] = []
      this.items.forEach((it) => {
        it.photos.forEach((p) => {
          photosLocal.push({ item: it.name, local_path: p })
        })
      })
      this.scenePhotos.forEach((p) => {
        photosLocal.push({ item: '', local_path: p })
      })
      enqueueOfflineCheckin(req, photosLocal)
      uni.hideLoading()
      this.submitting = false
      uni.showModal({
        title: pt.point_name,
        content: '当前无网络，打卡已离线暂存，网络恢复后自动补传',
        showCancel: false,
        confirmText: '知道了',
        success: () => {
          uni.navigateBack()
        }
      })
    },
    /**
     * 轮询逐项 AI 结论：2.5s 后取一次，全部项仍无结论且 retries>0 时再补一次；
     * 接口失败/超时静默 resolve(null)（记录已提交，结论只是提醒，拿不到不打扰）。
     */
    fetchAiItems(checkinId: string, retries: number): Promise<CheckinItemAI[] | null> {
      return new Promise((resolve) => {
        setTimeout(() => {
          apiCheckinItems(checkinId)
            .then((items) => {
              const hasVerdict = items.some((it) => it.ai_verdict != '')
              if (!hasVerdict && retries > 0) {
                this.fetchAiItems(checkinId, retries - 1).then(resolve)
              } else {
                resolve(items)
              }
            })
            .catch(() => resolve(null))
        }, 2500)
      })
    },
    /** 提交结果提示：有存疑项弹「AI 初判存疑」（提醒不阻断，记录已提交）；否则原成功弹窗 */
    showSubmitResult(pt: TaskPoint, res: CheckinResult, aiItems: CheckinItemAI[] | null) {
      uni.hideLoading()
      this.submitting = false
      const suspicious = (aiItems ?? []).filter((it) => it.ai_verdict == 'review' || it.ai_verdict == 'error')
      if (suspicious.length > 0) {
        const aiLines = suspicious.map((it) => it.name + (it.ai_reason != '' ? ' - ' + it.ai_reason : ''))
        uni.showModal({
          title: 'AI 初判存疑',
          content: aiLines.join('\n') + '\n请确认或重新拍摄',
          cancelText: '重新打卡',
          confirmText: '仍要提交',
          success: () => {
            // 两个出口均返回任务页：记录已提交，「重新打卡」需管理端驳回/重开后方可再次打卡
            uni.navigateBack()
          }
        })
        return
      }
      const tp = res.task_progress
      const lines = ['打卡成功，任务进度 ' + tp.done_points + '/' + tp.total_points]
      if (res.is_suspect) {
        lines.push('⚠ ' + (res.suspect_reason != '' ? res.suspect_reason : '本次打卡被标记为疑似异常'))
      }
      if (this.items.some((it) => !it.pass)) {
        lines.push('异常已记录，已通知管理员')
      }
      uni.showModal({
        title: pt.point_name,
        content: lines.join('\n'),
        showCancel: false,
        confirmText: '知道了',
        success: () => {
          uni.navigateBack()
        }
      })
    },
    /** 在线提交：上传照片换 file_key → POST /checkin；网络异常转离线暂存 */
    submitOnline() {
      const pt = this.point as TaskPoint
      this.submitting = true
      uni.showLoading({ title: '上传中…', mask: true })
      // 逐项照片 file_key（与 items 同序）+ 整单 photos 引用（项照片 item=项名，整单 item=''）
      const itemKeys: string[][] = this.items.map(() => [])
      const photoRefs: Array<{ item: string; file_key: string }> = []
      let chain: Promise<void> = Promise.resolve()
      this.items.forEach((it, idx) => {
        it.photos.forEach((p) => {
          chain = chain
            .then(() => apiUploadLocal(p))
            .then((r) => {
              itemKeys[idx].push(r.file_key)
              photoRefs.push({ item: it.name, file_key: r.file_key })
            })
        })
      })
      this.scenePhotos.forEach((p) => {
        chain = chain
          .then(() => apiUploadLocal(p))
          .then((r) => {
            photoRefs.push({ item: '', file_key: r.file_key })
          })
      })
      chain
        .then(() => {
          uni.showLoading({ title: '提交中…', mask: true })
          const abnormal = this.items.some((it) => !it.pass)
          return apiCheckin({
            task_id: this.taskId,
            point_id: this.pointId,
            checkin_type: pt.credential == 'nfc' ? 'nfc' : (this.scannedNo != '' ? 'qrcode' : (this.nfcCardId != '' ? 'nfc' : 'fence')),
            qrcode_no: pt.credential == 'nfc' ? undefined : (this.scannedNo != '' ? this.scannedNo : undefined),
            nfc_id: pt.credential == 'nfc' || pt.credential == 'any' ? (this.nfcCardId != '' ? this.nfcCardId : undefined) : undefined,
            longitude: this.myLng,
            latitude: this.myLat,
            client_time: fmtDateTime(new Date()),
            result: abnormal ? 'abnormal' : 'normal',
            remark: this.remark.trim(),
            check_items: this.items.map((it, idx) => ({
              name: it.name,
              pass: it.pass,
              note: it.note.trim(),
              photos: itemKeys[idx]
            })),
            photos: photoRefs,
            force: this.forceSubmit || undefined
          })
        })
        .then((res: CheckinResult) => {
          // AI 审核为后端异步执行：启用时延迟轮询逐项结论（2.5s×2，超时静默按无结论处理），
          // 未启用直接走原成功提示，不白等
          if (res.ai_enabled) {
            uni.showLoading({ title: 'AI 初判中…', mask: true })
            return this.fetchAiItems(res.checkin_id, 1).then((aiItems) => {
              this.showSubmitResult(pt, res, aiItems)
            })
          }
          this.showSubmitResult(pt, res, null)
        })
        .catch((e: Error & { code?: number; data?: any }) => {
          // 网络类错误（请求失败/上传失败）→ 转离线暂存；业务错误原样提示
          if (e.message != null && e.message.indexOf(NETWORK_ERR_PREFIX) == 0) {
            this.saveOffline()
            return
          }
          uni.hideLoading()
          this.submitting = false
          // AI 照片质量不合格（43107）：提示重拍并计数，达到放行上限允许强制提交转人工复核
          if (e.code === 43107) {
            this.qualityAttempts += 1
            const max = e.data != null && typeof e.data.max_attempts == 'number' ? e.data.max_attempts : this.maxAttempts
            this.maxAttempts = max
            if (this.qualityAttempts >= max) {
              uni.showModal({
                title: '照片仍未通过检查',
                content: e.message + '\n已达重拍上限，可强制提交，由管理员人工复核。',
                confirmText: '强制提交',
                cancelText: '重新拍摄',
                success: (r) => {
                  if (r.confirm) {
                    this.forceSubmit = true
                    this.submit()
                  }
                }
              })
              return
            }
            uni.showModal({
              title: '照片不合格',
              content: e.message + '\n请重新拍摄（第 ' + this.qualityAttempts + ' 次，' + max + ' 次后可强制提交）',
              showCancel: false,
              confirmText: '重新拍摄'
            })
            return
          }
          uni.showToast({ title: e.message, icon: 'none' })
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

.point-name {
  font-size: 36rpx;
  font-weight: 600;
}

.point-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.loc-row {
  margin-top: 16rpx;
}

.loc-text {
  font-size: 28rpx;
}

.sec-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
}

.sec-action {
  font-size: 28rpx;
  padding: 8rpx 16rpx;
}

.cred-ok {
  font-size: 28rpx;
  margin-top: 16rpx;
}

.cred-gap {
  margin-top: 16rpx;
}

.cred-tip {
  font-size: 24rpx;
  margin-top: 12rpx;
  text-align: center;
}

.btn-outline {
  height: 88rpx;
  border-radius: 20rpx;
  border-width: 2rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
  margin-top: 16rpx;
}

.btn-outline-text {
  font-size: 30rpx;
}

.item {
  margin-top: 24rpx;
}

.item-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.item-texts {
  flex: 1;
}

.item-name {
  font-size: 30rpx;
}

.item-req {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.item-toggle {
  flex-direction: row;
  margin-left: 24rpx;
}

.toggle-btn {
  font-size: 26rpx;
  padding: 10rpx 24rpx;
  border-radius: 12rpx;
  margin-left: 12rpx;
}

.item-note {
  width: 100%;
  height: 144rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 16rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.photo-add {
  width: 160rpx;
  height: 160rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: dashed;
  align-items: center;
  justify-content: center;
  margin-bottom: 16rpx;
}

.photo-add-text {
  font-size: 26rpx;
}

.photo-tip {
  font-size: 24rpx;
}

.remark {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
}

.btn-primary {
  height: 104rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 8rpx;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

.bottom-space {
  height: 64rpx;
}
</style>
