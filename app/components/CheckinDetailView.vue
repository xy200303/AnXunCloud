<template>
  <!-- 打卡记录详情共享视图：审核弹层与独立详情页共用。
       头部（点位/巡检员/时间/结果）+ 检查项照片墙 + 异常处置 + 审核状态与意见；
       照片地址统一走 toAbsUrl，点击调 uni.previewImage 放大。 -->
  <view v-if="record != null" class="detail">
    <!-- 疑似标记横幅 -->
    <view v-if="record.is_suspect" class="suspect-bar" :style="{ backgroundColor: '#FDF3E7' }">
      <text class="suspect-text" :style="{ color: colors.warning }">⚠ 疑似异常打卡：{{ suspectText }}</text>
    </view>

    <!-- 头部：点位 + 结果 + 元信息 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <view class="head-row">
        <text class="point-name" :style="{ color: colors.textPrimary }">{{ record.point_name }}</text>
        <text
          class="result-badge"
          :style="{
            color: record.result == 'abnormal' ? colors.danger : colors.success,
            backgroundColor: record.result == 'abnormal' ? '#FDECEC' : '#E7F6EF'
          }"
        >{{ record.result == 'abnormal' ? '⚠ 有异常' : '✓ 正常' }}</text>
      </view>
      <text class="head-sub" :style="{ color: colors.textSecondary }">{{ headSub }}</text>
      <view v-if="record.checkin_time != null && record.checkin_time != ''" class="meta-row">
        <text class="meta-label" :style="{ color: colors.textSecondary }">打卡时间</text>
        <text class="meta-value" :style="{ color: colors.textRegular }">{{ record.checkin_time }}</text>
      </view>
      <view v-if="record.checkin_type != null && record.checkin_type != ''" class="meta-row">
        <text class="meta-label" :style="{ color: colors.textSecondary }">打卡方式</text>
        <text class="meta-value" :style="{ color: colors.textRegular }">{{ checkinTypeText }}</text>
      </view>
      <view v-if="record.distance_to_point != null" class="meta-row">
        <text class="meta-label" :style="{ color: colors.textSecondary }">距点位</text>
        <text class="meta-value" :style="{ color: colors.textRegular }">{{ record.distance_to_point }} 米</text>
      </view>
      <text v-if="record.remark != null && record.remark != ''" class="remark" :style="{ color: colors.textRegular }">备注：{{ record.remark }}</text>
    </view>

    <!-- AI 审核结论（有才显示） -->
    <view v-if="record.ai_verdict != null && record.ai_verdict != ''" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">AI 审核结论</text>
      <text class="info-line" :style="{ color: colors.textRegular }">结论：{{ record.ai_verdict }}</text>
      <text v-if="record.ai_reason != null && record.ai_reason != ''" class="info-line" :style="{ color: colors.textSecondary }">{{ record.ai_reason }}</text>
    </view>

    <!-- 检查项逐项明细 -->
    <view v-for="(it, i) in items" :key="i" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <view class="item-head">
        <text class="item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
        <text class="item-result" :style="{ color: it.pass ? colors.success : colors.danger }">{{ it.pass ? '✓ 正常' : '⚠ 异常' }}</text>
      </view>
      <text v-if="it.note != null && it.note != ''" class="item-note" :style="{ color: colors.textRegular }">备注：{{ it.note }}</text>
      <view v-if="itemPhotoList(it).length > 0" class="photos">
        <image
          v-for="(u, pi) in itemPhotoList(it)"
          :key="pi"
          class="photo"
          :src="u"
          mode="aspectFill"
          lazy-load
          @click="preview(itemPhotoList(it), pi)"
        />
      </view>
      <text v-else class="no-photo" :style="{ color: colors.textSecondary }">无照片</text>
      <text v-if="it.ai_reason != null && it.ai_reason != ''" class="item-ai" :style="{ color: colors.textSecondary }">AI：{{ it.ai_reason }}</text>

      <!-- 异常项处置信息（仅异常且已处置时展示） -->
      <block v-if="dispositionText(it) != ''">
        <text class="item-disp" :style="{ color: it.disposition == 'report_pending' ? colors.warning : colors.success }">处置：{{ dispositionText(it) }}</text>
        <text v-if="it.resolution_note != null && it.resolution_note != ''" class="item-note" :style="{ color: colors.textRegular }">处置说明：{{ it.resolution_note }}</text>
        <view v-if="resolutionPhotoList(it).length > 0" class="photos">
          <image
            v-for="(u, pi) in resolutionPhotoList(it)"
            :key="pi"
            class="photo"
            :src="u"
            mode="aspectFill"
            lazy-load
            @click="preview(resolutionPhotoList(it), pi)"
          />
        </view>
      </block>
    </view>
    <view v-if="items.length == 0" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="no-photo" :style="{ color: colors.textSecondary }">这次是纯打卡，没有检查项</text>
    </view>

    <!-- 整单现场照片墙 -->
    <view v-if="recordPhotoList.length > 0" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">现场照片</text>
      <view class="photos">
        <image
          v-for="(u, pi) in recordPhotoList"
          :key="pi"
          class="photo"
          :src="u"
          mode="aspectFill"
          lazy-load
          @click="preview(recordPhotoList, pi)"
        />
      </view>
    </view>

    <!-- 审核状态与审核意见 -->
    <view v-if="record.audit_status != null && record.audit_status != ''" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">审核结果</text>
      <text class="info-line" :style="{ color: auditColor }">{{ auditText }}<text v-if="record.audit_at != null && record.audit_at != ''"> · {{ record.audit_at }}</text></text>
      <text
        v-if="record.audit_remark != null && record.audit_remark != ''"
        class="info-line"
        :style="{ color: record.audit_status == 'rejected' ? colors.danger : colors.textSecondary }"
      >意见：{{ record.audit_remark }}</text>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { toAbsUrl } from '@/utils/url'

/** 整单照片（对齐后端 photos 数组元素：优先水印图） */
export type CheckinDetailPhoto = {
  url: string
  watermarked_url?: string
}

/** 检查项（对齐详情接口 check_items；调用处可多带字段，不影响渲染） */
export type CheckinDetailItem = {
  name: string
  pass: boolean
  note?: string
  photo_urls?: string[]
  ai_verdict?: string | null
  ai_reason?: string | null
  /** ''=未处置 / on_site_resolved=现场已处理 / maintenance_registered=已登记维保 / report_pending=上报待处理 */
  disposition?: string
  resolution_note?: string
  resolution_photo_urls?: string[]
}

/** 打卡详情视图数据（对齐 GET /inspection/checkins/:id 返回；审核列表记录在调用处补齐同名字段） */
export type CheckinDetailRecord = {
  point_name: string
  community_name?: string
  inspector_name?: string
  plan_name?: string
  checkin_time?: string
  /** qrcode/fence/nfc/offline */
  checkin_type?: string
  distance_to_point?: number | null
  /** normal/abnormal */
  result: string
  remark?: string
  is_suspect?: boolean
  suspect_reason?: string
  photos?: CheckinDetailPhoto[]
  check_items?: CheckinDetailItem[]
  /** pending/passed/rejected/auto_pass */
  audit_status?: string
  audit_at?: string | null
  audit_remark?: string
  ai_verdict?: string
  ai_reason?: string
}

type ViewData = {
  colors: ColorTokens
  shadow: string
}

function checkinTypeTextOf(t: string): string {
  if (t == 'qrcode') return '扫二维码'
  if (t == 'nfc') return '刷 NFC 卡'
  if (t == 'fence') return '到场确认'
  if (t == 'offline') return '离线补传'
  return t
}

function auditTextOf(s: string): string {
  if (s == 'auto_pass') return 'AI 自动通过'
  if (s == 'pending') return '待人工审核'
  if (s == 'pass' || s == 'passed') return '审核通过'
  if (s == 'rejected') return '已驳回'
  return s
}

export default {
  props: {
    record: { type: Object, default: null }
  },
  data(): ViewData {
    return {
      colors: Colors,
      shadow: ShadowCard
    }
  },
  computed: {
    rec(): CheckinDetailRecord {
      return (this.record as CheckinDetailRecord | null) ?? { point_name: '', result: '' }
    },
    items(): CheckinDetailItem[] {
      return this.rec.check_items ?? []
    },
    headSub(): string {
      const parts = [this.rec.community_name, this.rec.inspector_name].filter((s) => s != null && s != '')
      if (this.rec.plan_name != null && this.rec.plan_name != '') parts.push(this.rec.plan_name)
      return parts.join(' · ')
    },
    suspectText(): string {
      const r = this.rec.suspect_reason
      return r != null && r != '' ? r : '时间/位置存疑'
    },
    checkinTypeText(): string {
      return checkinTypeTextOf(this.rec.checkin_type ?? '')
    },
    auditText(): string {
      return auditTextOf(this.rec.audit_status ?? '')
    },
    auditColor(): string {
      const s = this.rec.audit_status
      if (s == 'rejected') return Colors.danger
      if (s == 'pending') return Colors.warning
      return Colors.success
    },
    /** 整单现场照片：优先水印图，统一绝对地址 */
    recordPhotoList(): string[] {
      return (this.rec.photos ?? []).map((p) => toAbsUrl(p.watermarked_url != null && p.watermarked_url != '' ? p.watermarked_url : p.url))
    }
  },
  methods: {
    itemPhotoList(it: CheckinDetailItem): string[] {
      return (it.photo_urls ?? []).map(toAbsUrl)
    },
    resolutionPhotoList(it: CheckinDetailItem): string[] {
      return (it.resolution_photo_urls ?? []).map(toAbsUrl)
    },
    dispositionText(it: CheckinDetailItem): string {
      if (it.disposition == 'on_site_resolved') return '现场已处理'
      if (it.disposition == 'maintenance_registered') return '已登记维保'
      if (it.disposition == 'report_pending') return '上报待处理'
      return ''
    },
    preview(urls: string[], idx: number) {
      if (urls.length == 0) return
      uni.previewImage({ urls: urls, current: urls[idx] })
    }
  }
}
</script>

<style scoped>
.detail {
  flex: 1;
}

.suspect-bar {
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
}

.suspect-text {
  font-size: 26rpx;
}

.card {
  border-radius: 24rpx;
  padding: 28rpx;
  margin-bottom: 24rpx;
}

.head-row {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.point-name {
  font-size: 36rpx;
  font-weight: 600;
  flex: 1;
}

.result-badge {
  font-size: 26rpx;
  font-weight: 600;
  padding: 8rpx 20rpx;
  border-radius: 999rpx;
}

.head-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.meta-row {
  flex-direction: row;
  justify-content: space-between;
  margin-top: 20rpx;
}

.meta-label {
  font-size: 28rpx;
}

.meta-value {
  font-size: 28rpx;
}

.remark {
  font-size: 26rpx;
  margin-top: 20rpx;
  line-height: 1.6;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 8rpx;
}

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
  line-height: 1.6;
}

.item-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.item-name {
  font-size: 32rpx;
  font-weight: 600;
  flex: 1;
}

.item-result {
  font-size: 28rpx;
  font-weight: 600;
}

.item-note {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.item-ai {
  font-size: 26rpx;
  margin-top: 16rpx;
  line-height: 1.6;
}

.item-disp {
  font-size: 26rpx;
  margin-top: 16rpx;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.no-photo {
  font-size: 26rpx;
  margin-top: 16rpx;
}
</style>
