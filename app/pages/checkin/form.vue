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
        <text class="sec-title" :style="{ color: colors.textPrimary }">打卡确认</text>
        <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="scanCredential">
          <text class="btn-outline-text" :style="{ color: colors.primary }">扫点位二维码</text>
        </view>
        <view v-if="point != null && point.credential == 'any'" class="btn-outline cred-gap" :style="{ borderColor: colors.primary }" @click="nfcTap">
          <text class="btn-outline-text" :style="{ color: colors.primary }">刷 NFC 卡</text>
        </view>
        <text v-if="point != null && point.credential == 'any'" class="cred-tip" :style="{ color: colors.textSecondary }">扫码或刷卡，确认一种就行</text>
      </view>
      <view v-else-if="scannedNo != '' && point != null && point.credential != 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">打卡确认</text>
        <text class="cred-ok" :style="{ color: colors.success }">已扫码确认：{{ scannedNo }}</text>
      </view>
      <view v-else-if="point && (point.credential == 'nfc' || point.credential == 'any') && nfcCardId != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">打卡确认</text>
        <text class="cred-ok" :style="{ color: colors.success }">已刷卡确认：{{ nfcCardId }}</text>
      </view>
      <view v-else-if="point && point.credential == 'nfc'" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">打卡确认</text>
        <view class="btn-outline" :style="{ borderColor: colors.primary }" @click="nfcTap">
          <text class="btn-outline-text" :style="{ color: colors.primary }">刷 NFC 卡</text>
        </view>
      </view>

      <!-- 点位设备提醒横幅（v1.7 展示增强，零新增动作）：逾期/报废红、临期黄；点击滚动到设备项 -->
      <view
        v-if="equipBanner.show"
        class="equip-banner"
        :style="{ backgroundColor: equipBanner.danger ? colors.danger : colors.warning }"
        @click="scrollToEquip"
      >
        <text class="equip-banner-text" :style="{ color: colors.white }">{{ equipBanner.text }}</text>
        <text class="equip-banner-arrow" :style="{ color: colors.white }">></text>
      </view>

      <!-- 检查项列表 -->
      <view v-if="items.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="sec-head">
          <text class="sec-title" :style="{ color: colors.textPrimary }">检查项</text>
          <text class="sec-action" :style="{ color: colors.primary }" @click="allNormal">全部正常</text>
        </view>
        <view v-for="(it, idx) in items" :key="idx" class="item" :class="{ 'equip-anchor-mark': isEquipAuto(it) }">
          <view class="item-head">
            <view class="item-texts">
              <text class="item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
              <text v-if="it.requirement != ''" class="item-req" :style="{ color: colors.textSecondary }">{{ it.requirement }}</text>
              <text v-if="it.photo_required == 'required' && !isEquipAuto(it)" class="item-req" :style="{ color: colors.warning }">必拍照片</text>
            </view>
            <!-- 台账有效期合成项：服务端逐台自动判定，只读展示（不可人工改判） -->
            <view v-if="isEquipAuto(it)" class="equip-state-wrap">
              <text class="equip-state" :style="{ color: equipStateColor(it.auto_judge) }">{{ equipStateText(it.auto_judge) }}</text>
              <text v-if="it.auto_judge != null && it.auto_judge.has_pending_register" class="equip-pending" :style="{ color: colors.primary }">已登记待确认</text>
            </view>
            <view v-else-if="!isEquipSpot(it)" class="item-toggle">
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
          <!-- 标签抽查合成项（v1.7）：必拍 1 张 + 生产日期/维修日期（服务端四规则比对，结论由服务端给出） -->
          <view v-if="isEquipSpot(it)" class="spot-block" :style="{ borderColor: colors.border }">
            <view class="spot-row">
              <text class="spot-label" :style="{ color: colors.textRegular }">生产日期</text>
              <picker mode="date" :value="it.spot_mfg" :disabled="it.spot_label_missing" @change="it.spot_mfg = $event.detail.value">
                <view class="spot-picker" :style="{ borderColor: colors.border, color: it.spot_mfg != '' ? colors.textPrimary : colors.textSecondary }">
                  {{ it.spot_mfg != '' ? it.spot_mfg : '选择日期' }}
                </view>
              </picker>
            </view>
            <view class="spot-row">
              <text class="spot-label" :style="{ color: colors.textRegular }">维修日期</text>
              <picker v-if="!it.spot_no_sticker" mode="date" :value="it.spot_maint" :disabled="it.spot_label_missing" @change="it.spot_maint = $event.detail.value">
                <view class="spot-picker" :style="{ borderColor: colors.border, color: it.spot_maint != '' ? colors.textPrimary : colors.textSecondary }">
                  {{ it.spot_maint != '' ? it.spot_maint : '选择日期' }}
                </view>
              </picker>
              <text
                class="spot-check"
                :style="{ color: it.spot_no_sticker ? colors.primary : colors.textSecondary }"
                @click="it.spot_no_sticker = !it.spot_no_sticker"
              >{{ it.spot_no_sticker ? '✓ 无贴纸' : '无贴纸' }}</text>
            </view>
            <view class="spot-row">
              <text
                class="spot-check"
                :style="{ color: it.spot_label_missing ? colors.danger : colors.textSecondary }"
                @click="toggleSpotMissing(it)"
              >{{ it.spot_label_missing ? '✓ 标签缺失/无法辨认' : '标签缺失/无法辨认' }}</text>
              <text
                v-if="it.photos.length > 0 && !it.spot_label_missing"
                class="spot-ai"
                :style="{ color: colors.primary }"
                @click="aiReadLabel(it)"
              >{{ it.spot_ai_loading ? 'AI 识别中…' : 'AI 读标签' }}</text>
            </view>
            <text class="spot-hint" :style="{ color: colors.textSecondary }">抽查只核对不改台账；比对不符将转经理审核</text>
          </view>

          <!-- 台账有效期合成项：该设备临期/逾期/缺数据时的「拍新标签」入口（逐台独立；照片即登记凭证，提交时系统自动核对） -->
          <view v-if="isEquipAuto(it) && canLabelPhoto(it)" class="spot-block" :style="{ borderColor: colors.border }">
            <view class="photos">
              <image
                v-for="(ph, pi) in it.photos"
                :key="pi"
                class="photo"
                :src="ph"
                mode="aspectFill"
                lazy-load
                @longpress="removePhoto(it.photos, pi)"
              />
              <view
                v-if="it.photos.length < 3"
                class="photo-add"
                :style="{ borderColor: colors.border }"
                @click="takePhotos(it.photos, 3)"
              >
                <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍新标签</text>
              </view>
            </view>
            <text class="spot-hint" :style="{ color: colors.textSecondary }">已维保？拍新维修标签（至多 3 张）；提交时系统自动核对，核对通过自动更新台账</text>
          </view>
          <!-- 台账有效期合成项：自动判定备注（逾期/缺数据）只读展示 -->
          <text v-if="isEquipAuto(it) && it.note != ''" class="item-req" :style="{ color: equipStateColor(it.auto_judge) }">{{ it.note }}</text>
          <!-- 异常备注（选异常必填） -->
          <textarea
            v-if="!it.pass && !isEquipAuto(it)"
            v-model="it.note"
            class="item-note"
            :style="{ borderColor: colors.border, color: colors.textPrimary }"
            placeholder="请填写该项异常情况（必填）"
            :maxlength="200"
          />
          <!-- 异常项处置方式：默认上报待处理；现场已处理须拍处置照片留痕 -->
          <view v-if="!it.pass && !isEquipAuto(it) && !isEquipSpot(it)" class="disp-row">
            <text
              class="toggle-btn"
              :style="it.disposition != 'on_site_resolved' ? { color: colors.white, backgroundColor: colors.danger } : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
              @click="setDisposition(it, 'report_pending')"
            >上报待处理</text>
            <text
              class="toggle-btn"
              :style="it.disposition == 'on_site_resolved' ? { color: colors.white, backgroundColor: colors.success } : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
              @click="setDisposition(it, 'on_site_resolved')"
            >现场已处理</text>
          </view>
          <view v-if="!it.pass && !isEquipAuto(it) && !isEquipSpot(it) && it.disposition == 'on_site_resolved'" class="photos">
            <image
              v-for="(ph, pi) in it.res_photos"
              :key="pi"
              class="photo"
              :src="ph"
              mode="aspectFill"
              lazy-load
              @longpress="removePhoto(it.res_photos, pi)"
            />
            <view
              v-if="it.res_photos.length < 3"
              class="photo-add"
              :style="{ borderColor: colors.border }"
              @click="takePhotos(it.res_photos, 3)"
            >
              <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍处置照片</text>
            </view>
          </view>
          <text v-if="!it.pass && !isEquipAuto(it) && !isEquipSpot(it) && it.disposition == 'on_site_resolved'" class="spot-hint" :style="{ color: colors.textSecondary }">必拍至少 1 张处置后的照片，作为已处理凭证</text>
          <!-- 该项照片（异常项与必拍项展示；一项一图硬约束，最多 1 张，重拍先长按删除） -->
          <view v-if="showItemPhotos(it)" class="photos">
            <image
              v-for="(ph, pi) in it.photos"
              :key="pi"
              class="photo"
              :src="ph"
              mode="aspectFill"
              lazy-load
              @longpress="removePhoto(it.photos, pi)"
            />
            <view
              v-if="it.photos.length < 1"
              class="photo-add"
              :style="{ borderColor: colors.border }"
              @click="takePhotos(it.photos, 1)"
            >
              <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
            </view>
          </view>
        </view>
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
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiTaskDetail, apiCheckin, apiCheckinItems, apiUploadLocal, apiAiItemJobCreate, apiAiItemJobs, TaskPoint, CheckinResult, CheckinItemAI, CheckinReqPayload, EquipmentAutoJudge } from '@/services/api'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'
import { extractPointCode } from '@/utils/scan'
import { getLocationGcj02 } from '@/utils/geo'
import { compressForUpload } from '@/utils/image'
import { enqueueOfflineCheckin, uuidv7, NETWORK_ERR_PREFIX, OfflinePhoto } from '@/utils/offline'

/** 检查项视图模型：模板项 + 录入状态 */
type ItemView = {
  name: string
  requirement: string
  photo_required: string
  pass: boolean
  note: string
  /** 水印烧录后的本地路径，提交时上传换 file_id */
  photos: string[]
  /** 判定方式（equipment_validity=台账有效期：服务端自动判定，UI 只读展示） */
  judge_type: string
  /** 台账有效期自动判定（judge_type=equipment_validity 时后端透出） */
  auto_judge: EquipmentAutoJudge | null
  /** 标签抽查项（judge_type=equipment_date_spot）录入字段 */
  spot_mfg: string
  spot_maint: string
  spot_no_sticker: boolean
  spot_label_missing: boolean
  spot_ai_loading: boolean
  /** 异常项处置方式：'' 未选（按 report_pending 处理）/ on_site_resolved 现场已处理 / report_pending 上报待处理 */
  disposition: '' | 'on_site_resolved' | 'report_pending'
  /** 处置照片本地路径（disposition=on_site_resolved 时必传 ≥1 张，提交时上传换 file_id） */
  res_photos: string[]
}

/** 台账有效期合成项（v1.6：绑定即启用、逐台独立——点位每台在用设备一条合成项，只读展示） */
function isEquipAuto(it: ItemView): boolean {
  return it.judge_type == 'equipment_validity' && it.auto_judge != null
}

/** 标签抽查合成项（v1.7：触发才出现；必拍+填日期，服务端四规则比对） */
function isEquipSpot(it: ItemView): boolean {
  return it.judge_type == 'equipment_date_spot'
}

/** 台账有效期项「拍新标签」入口：逾期/缺数据（或后端仍下发展示登记入口）时才出现 */
function canLabelPhoto(it: ItemView): boolean {
  const aj = it.auto_judge
  if (aj == null) return false
  return aj.show_register || aj.status == 'overdue' || aj.status == 'no_data'
}

/**
 * 组装逐项提交载荷：
 * - 台账有效期项仅在有新标签照片时上送（photos=新标签照片，服务端 AI 核对；pass 忽略）
 * - 异常项（非抽查）带处置方式；「现场已处理」带处置照片 file_id（未选按上报待处理）
 */
function toCheckinPayload(it: ItemView, photoIds: string[], resIds: string[]): CheckinItemReqPayload {
  if (isEquipAuto(it)) {
    return { name: it.name, pass: true, note: '', photos: photoIds }
  }
  const spot = isEquipSpot(it)
  const disp = !it.pass && !spot ? (it.disposition != '' ? it.disposition : 'report_pending') : ''
  return {
    name: it.name,
    pass: it.pass,
    note: it.note.trim(),
    photos: photoIds,
    spot_manufacture_date: spot ? it.spot_mfg : undefined,
    spot_maintenance_date: spot ? it.spot_maint : undefined,
    spot_no_sticker: spot ? it.spot_no_sticker : undefined,
    spot_label_missing: spot ? it.spot_label_missing : undefined,
    disposition: disp != '' ? disp : undefined,
    resolution_file_ids: disp == 'on_site_resolved' && resIds.length > 0 ? resIds : undefined,
    resolution_note: disp == 'on_site_resolved' && it.note.trim() != '' ? it.note.trim() : undefined
  }
}

/** 台账有效期状态文案（逐台：auto_judge 即该设备自己的判定；报废日已过优先） */
function equipStateText(aj: EquipmentAutoJudge): string {
  if (aj.scrap_due) return '已过报废日期' + ((aj.scrap_date || '') != '' ? '（' + aj.scrap_date + '）' : '') + '，应停用更换'
  if (aj.status == 'no_data') return '台账数据缺失，请补录'
  const due = aj.next_due_date
  if (due == '') return ''
  if (aj.status == 'overdue') return '已逾期 ' + aj.overdue_days + ' 天（到期日 ' + due + '）'
  if (aj.status == 'warning') {
    const days = Math.round((new Date(due.replace(/-/g, '/')).getTime() - todayZero()) / 86400000)
    return '将于 ' + days + ' 天内到期（' + due + '）'
  }
  return '台账有效（至 ' + due + '）'
}

function equipStateColor(aj: EquipmentAutoJudge): string {
  if (aj.scrap_due) return Colors.danger
  if (aj.status == 'overdue') return Colors.danger
  if (aj.status == 'warning') return Colors.warning
  if (aj.status == 'no_data') return Colors.info
  return Colors.success
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
  /** 海拔/定位精度（米，0=未取得；仅随打卡上送作参考展示） */
  myAlt: number
  myAcc: number
  /** 与点位距离（米），-1 表示未知 */
  distance: number
  /** 扫码核验后的二维码编号（空 = 未核验） */
  scannedNo: string
  /** NFC 核验后读到的卡片 UID（十六进制，提交时作 nfc_id；空 = 未核验） */
  nfcCardId: string
  items: ItemView[]
  remark: string
  submitting: boolean
  photoBusy: boolean
  /** AI 照片质量拦截计数与放行上限（43107 分支用；达到上限允许强制提交转人工复核） */
  qualityAttempts: number
  maxAttempts: number
  /** 强制提交标记（用户确认后重发带 force=true） */
  forceSubmit: boolean
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
      myAlt: 0,
      myAcc: 0,
      distance: -1,
      scannedNo: '',
      nfcCardId: '',
      items: [] as ItemView[],
      remark: '',
      submitting: false,
      photoBusy: false,
      qualityAttempts: 0,
      maxAttempts: 3,
      forceSubmit: false
    }
  },
  computed: {
    /** 点位设备横幅：统计台账合成项临期/逾期/报废台数 */
    equipBanner(): { show: boolean; danger: boolean; text: string } {
      let warn = 0
      let bad = 0
      this.items.forEach((it) => {
        if (!isEquipAuto(it) || it.auto_judge == null) return
        if (it.auto_judge.scrap_due || it.auto_judge.status == 'overdue') {
          bad++
        } else if (it.auto_judge.status == 'warning') {
          warn++
        }
      })
      if (bad > 0) return { show: true, danger: true, text: '该点位 ' + bad + ' 台设备已逾期/应报废' + (warn > 0 ? '，' + warn + ' 台临期' : '') }
      if (warn > 0) return { show: true, danger: false, text: '该点位 ' + warn + ' 台设备临期' }
      return { show: false, danger: false, text: '' }
    },
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
      this.scannedNo = String(options.no).trim()
    }
    this.load()
  },
  methods: {
    isEquipAuto,
    isEquipSpot,
    canLabelPhoto,
    equipStateText,
    equipStateColor,
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
          // 预核验编号按点位匹配：命中 NFC 卡号 → 视为已刷卡核验；否则按二维码编号（原扫码进入行为）
          if (this.scannedNo != '' && pt.nfc_id != '' && pt.nfc_id == this.scannedNo && pt.qrcode_no != this.scannedNo) {
            this.nfcCardId = this.scannedNo
            this.scannedNo = ''
          }
          this.items = pt.check_items.map((c) => {
            const it: ItemView = {
              name: c.name,
              requirement: c.requirement,
              photo_required: c.photo_required,
              pass: true,
              note: '',
              photos: [],
              judge_type: c.judge_type,
              auto_judge: c.auto_judge ?? null,
              spot_mfg: '', spot_maint: '', spot_no_sticker: false, spot_label_missing: false, spot_ai_loading: false,
              disposition: '', res_photos: [] as string[]
            }
            // 台账有效期合成项：按服务端逐台判定预置结论（逾期判异常，客户端不可改，提交时不上送）
            if (isEquipAuto(it) && it.auto_judge != null) {
              const aj = it.auto_judge
              if (aj.status == 'overdue') {
                it.pass = false
                it.note = '设备维保逾期：编号 ' + aj.equipment_code + ' 到期日 ' + aj.next_due_date
                if (aj.has_pending_register) it.note += '（已登记维保待确认）'
              } else if (aj.status == 'no_data') {
                it.note = '台账数据缺失待补录'
              }
            }
            return it
          })
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
          this.myAlt = loc.altitude
          this.myAcc = loc.accuracy
          // 点位未录坐标（0,0）时距离无意义，不计算（distText 随之不显示）
          if (this.point != null && (this.point.longitude != 0 || this.point.latitude != 0)) {
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
          const code = extractPointCode(res.result)
          if (code == '') {
            uni.showToast({ title: '请扫描新版点位二维码', icon: 'none' })
            return
          }
          if (this.point != null && this.point.qrcode_no != '' && code != this.point.qrcode_no) {
            uni.showToast({ title: '二维码与本点位不匹配', icon: 'none' })
            return
          }
          this.scannedNo = code
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
        // 卡片 UID 须与点位备案的「NFC 卡号」一致（与扫码校验同规则；后端提交时还会再比对一次）
        if (this.point != null && this.point.nfc_id != '' && cardId != this.point.nfc_id) {
          uni.showToast({ title: 'NFC 标签与本点位不匹配', icon: 'none' })
          return
        }
        this.nfcCardId = cardId
        uni.showToast({ title: '点位确认成功', icon: 'success' })
      })
    },
    /** 表单页内全局 NFC 贴卡（App.vue 转发，不用先点按钮）：卡号匹配本点位即完成刷卡确认 */
    onGlobalNfc(cardId: string) {
      if (this.point != null && this.point.nfc_id != '' && this.point.nfc_id == cardId) {
        this.nfcCardId = cardId
        uni.showToast({ title: '点位确认成功', icon: 'success' })
        return
      }
      uni.showToast({ title: '这张卡不是本点位的 NFC 卡', icon: 'none' })
    },
    setPass(idx: number, pass: boolean) {
      // 台账有效期项由服务端判定，不接受人工改判
      if (isEquipAuto(this.items[idx])) return
      const it = this.items[idx]
      it.pass = pass
      // 改回正常时清掉处置选择与处置照片，避免误带旧凭证
      if (pass) {
        it.disposition = ''
        it.res_photos = []
      } else if (it.disposition == '') {
        it.disposition = 'report_pending'
      }
    },
    /** 异常项处置方式切换：改回「上报待处理」时清掉已拍处置照片 */
    setDisposition(it: ItemView, value: '' | 'on_site_resolved' | 'report_pending') {
      it.disposition = value
      if (value != 'on_site_resolved') it.res_photos = []
    },
    /** 一键全部正常并清空逐项备注；不跳过必拍照片校验；台账有效期项（服务端判定）不动 */
    allNormal() {
      this.items.forEach((it) => {
        if (isEquipAuto(it)) return
        it.pass = true
        it.note = ''
        it.disposition = ''
        it.res_photos = []
      })
      const needPhoto = this.items.some((it) => !isEquipAuto(it) && it.photo_required == 'required' && it.photos.length == 0)
      if (needPhoto) {
        uni.showToast({ title: '仍有必拍照片项，请拍照', icon: 'none' })
      }
    },
    showItemPhotos(it: ItemView): boolean {
      // 台账有效期项不要求照片；抽查项必拍；异常项与模板必拍项均展示照片区
      if (isEquipAuto(it)) return false
      if (isEquipSpot(it)) return true
      return !it.pass || it.photo_required == 'required'
    },
    /** 抽查项「标签缺失」开关：勾选后清空日期（日期免填） */
    toggleSpotMissing(it: ItemView) {
      it.spot_label_missing = !it.spot_label_missing
      if (it.spot_label_missing) {
        it.spot_mfg = ''
        it.spot_maint = ''
      }
    },
    /** 抽查项 AI 读标签预填（拍照后可用；失败/超时允许手填，不阻塞） */
    aiReadLabel(it: ItemView) {
      if (it.spot_ai_loading || it.photos.length == 0) return
      it.spot_ai_loading = true
      // 本地照片先上传换 file_id 再建识别任务（抽查合成项经后端按前缀+触发重算放行）
      apiUploadLocal(it.photos[0])
        .then((up) => apiAiItemJobCreate({ task_id: this.taskId, point_id: this.pointId, name: it.name, file_ids: [up.file_id] }))
        .then((j) => this.pollLabelJob(it, j.job_id, 8))
        .catch((e: any) => {
          it.spot_ai_loading = false
          uni.showToast({ title: (e && e.message) || 'AI 识别不可用，请手填日期', icon: 'none' })
        })
    },
    /** 轮询读标签结果：M2020-05|W2025-03 紧凑格式解析预填（巡检员可改，提交以确认值为准） */
    pollLabelJob(it: ItemView, jobId: string, retries: number) {
      setTimeout(() => {
        apiAiItemJobs([jobId])
          .then((jobs) => {
            const j = jobs[0]
            if (j != null && j.status == 'done') {
              const m = (j.reading || '').match(/M(\d{4}-\d{2}|无)\|W(\d{4}-\d{2}|无)/)
              if (m != null) {
                if (m[1] != '无') it.spot_mfg = m[1] + '-01'
                if (m[2] != '无') {
                  it.spot_maint = m[2] + '-01'
                  it.spot_no_sticker = false
                } else {
                  it.spot_no_sticker = true
                }
                uni.showToast({ title: '已预填日期，请核对', icon: 'none' })
              } else {
                uni.showToast({ title: '未读到日期，请手填', icon: 'none' })
              }
              it.spot_ai_loading = false
              return
            }
            if (j != null && j.status == 'failed') {
              uni.showToast({ title: 'AI 识别失败，请手填日期', icon: 'none' })
              it.spot_ai_loading = false
              return
            }
            if (retries > 0) {
              this.pollLabelJob(it, jobId, retries - 1)
            } else {
              it.spot_ai_loading = false
              uni.showToast({ title: 'AI 识别超时，请手填日期', icon: 'none' })
            }
          })
          .catch(() => {
            it.spot_ai_loading = false
          })
      }, 1500)
    },
    /** 横幅点击：滚动到第一个设备合成项 */
    scrollToEquip() {
      uni.pageScrollTo({ selector: '.equip-anchor-mark', duration: 200, fail: () => {} })
    },
    /** 拍照（仅相机防相册作弊）→ 定标压缩（1920px/q80）后入列表；一项一图硬约束（max=1）；水印由服务端在打卡后统一烧录 */
    takePhotos(list: string[], max: number) {
      const remain = max - list.length
      if (remain <= 0 || this.photoBusy || this.submitting) return
      this.photoBusy = true
      uni.chooseImage({
        count: remain,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) {
            this.photoBusy = false
            return
          }
          Promise.all(paths.slice(0, remain).map((p) => compressForUpload(p)))
            .then((compressed) => compressed.forEach((p) => list.push(p)))
            .catch(() => uni.showToast({ title: '照片处理失败，请重试', icon: 'none' }))
            .finally(() => { this.photoBusy = false })
        },
        fail: () => { this.photoBusy = false }
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
    /** 提交前校验，返回错误文案（空串 = 通过） */
    validate(): string {
      if (this.point == null) return '点位信息缺失'
      if (this.point.credential == 'nfc' && this.nfcCardId == '') return '请先刷 NFC 卡'
      if (this.point.credential == 'qrcode' && this.scannedNo == '') return '请先扫点位二维码'
      if (this.point.credential == 'any' && this.scannedNo == '' && this.nfcCardId == '') return '请扫码或刷 NFC 卡'
      if (!this.hasLoc) return '请先完成定位'
      if (this.point.require_fence && this.distance > this.point.fence_radius) {
        return '距点位 ' + this.distance + ' m，超出围栏半径 ' + this.point.fence_radius + ' m'
      }
      for (let i = 0; i < this.items.length; i++) {
        const it = this.items[i]
        // 台账有效期项：服务端判定，无照片/备注约束（备注已按状态预置）
        if (isEquipAuto(it)) continue
        // 标签抽查项：必拍 1 张；日期必填（无贴纸免维修日期；标签缺失全免）
        if (isEquipSpot(it)) {
          if (it.photos.length == 0) return '「' + it.name + '」须拍 1 张标签/设备照片'
          if (it.spot_label_missing) continue
          if (it.spot_mfg == '') return '「' + it.name + '」请填写生产日期（读不到则勾选标签缺失）'
          if (!it.spot_no_sticker && it.spot_maint == '') return '「' + it.name + '」请填写维修日期或选「无贴纸」'
          continue
        }
        if (!it.pass && it.note.trim() == '') {
          return '请填写「' + it.name + '」异常备注'
        }
        if ((!it.pass || it.photo_required == 'required') && it.photos.length == 0) {
          return '「' + it.name + '」须至少拍 1 张照片'
        }
        // 「现场已处理」须拍处置照片留痕
        if (!it.pass && it.disposition == 'on_site_resolved' && it.res_photos.length == 0) {
          return '「' + it.name + '」选了现场已处理，请拍处置照片'
        }
      }
      // 后端硬约束：异常打卡整单备注必填
      const abnormal = this.items.some((it) => !it.pass)
      if (abnormal && this.remark.trim() == '') return '存在异常项，请填写整单备注说明'
      return ''
    },
    submit() {
      if (this.submitting) return
      if (this.photoBusy) {
        uni.showToast({ title: '照片处理中，请稍候', icon: 'none' })
        return
      }
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
      // 台账有效期项仅带新标签照片时上送；照片留空待补传回填（含处置照片 kind=resolution）
      const subs = this.items.filter((it) => !isEquipAuto(it) || it.photos.length > 0)
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
        check_items: subs.map((it) => toCheckinPayload(it, [], []))
      }
      // 照片保留本地路径（不删本地文件）：item=检查项名（照片唯一归属逐项）；处置照片标 kind=resolution
      const photosLocal: OfflinePhoto[] = []
      this.items.forEach((it) => {
        it.photos.forEach((p) => {
          photosLocal.push({ item: it.name, local_path: p })
        })
        it.res_photos.forEach((p) => {
          photosLocal.push({ item: it.name, local_path: p, kind: 'resolution' })
        })
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
    /** 在线提交：上传照片换 file_id → POST /checkin；网络异常转离线暂存 */
    submitOnline() {
      const pt = this.point as TaskPoint
      this.submitting = true
      uni.showLoading({ title: '上传中…', mask: true })
      // 台账有效期合成项仅带新标签照片时上送（服务端 AI 核对，可信自动回写台账）；
      // 抽查项须上送（含 spot_* 字段）；逐项照片 file_id 与提交项同序，处置照片单列 resKeys
      const subs = this.items.filter((it) => !isEquipAuto(it) || it.photos.length > 0)
      const itemKeys: string[][] = subs.map(() => [])
      const resKeys: string[][] = subs.map(() => [])
      let chain: Promise<void> = Promise.resolve()
      subs.forEach((it, idx) => {
        it.photos.forEach((p) => {
          chain = chain
            .then(() => apiUploadLocal(p))
            .then((r) => {
              itemKeys[idx].push(r.file_id)
            })
        })
        it.res_photos.forEach((p) => {
          chain = chain
            .then(() => apiUploadLocal(p))
            .then((r) => {
              resKeys[idx].push(r.file_id)
            })
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
            altitude: this.myAlt > 0 ? this.myAlt : undefined,
            accuracy: this.myAcc > 0 ? this.myAcc : undefined,
            client_time: fmtDateTime(new Date()),
            result: abnormal ? 'abnormal' : 'normal',
            remark: this.remark.trim(),
            check_items: subs.map((it, idx) => toCheckinPayload(it, itemKeys[idx], resKeys[idx])),
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

/* 标签抽查项 */
.spot-block {
  margin-top: 16rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 24rpx;
}

.spot-row {
  flex-direction: row;
  align-items: center;
  margin-bottom: 16rpx;
}

.spot-label {
  font-size: 26rpx;
  width: 140rpx;
}

.spot-picker {
  height: 72rpx;
  min-width: 240rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  justify-content: center;
  font-size: 28rpx;
}

.spot-check {
  font-size: 26rpx;
  margin-left: 24rpx;
  padding: 8rpx 16rpx;
}

.spot-ai {
  font-size: 26rpx;
  padding: 8rpx 16rpx;
}

.spot-hint {
  font-size: 22rpx;
}

/* 点位设备提醒横幅 */
.equip-banner {
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.equip-banner-text {
  font-size: 26rpx;
  flex: 1;
}

.equip-banner-arrow {
  font-size: 26rpx;
  margin-left: 16rpx;
}

/* 台账有效期项：服务端判定结果展示 + 「拍新标签」入口 */
.equip-state-wrap {
  flex-direction: column;
  align-items: flex-end;
  margin-left: 24rpx;
}

.equip-state {
  font-size: 26rpx;
}

.equip-pending {
  font-size: 22rpx;
  margin-top: 4rpx;
}

/* 处置方式选择 */
.disp-row {
  flex-direction: row;
  margin-top: 16rpx;
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
