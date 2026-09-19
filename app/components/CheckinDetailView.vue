<template>
  <!-- 打卡记录详情共享视图：审核弹层与独立详情页共用。
       头部（点位/巡检员/时间/结果）+ 检查项照片墙 + 异常处置 + 审核状态与意见；
       照片地址统一走 toAbsUrl，点击调 uni.previewImage 放大。 -->
  <view v-if="record != null" class="detail">
    <!-- 疑似标记横幅 -->
    <view v-if="record.is_suspect" class="suspect-bar" :style="{ backgroundColor: '#FDF3E7' }">
      <text class="suspect-text text-warning" >⚠ 疑似异常打卡：{{ suspectText }}</text>
    </view>

    <!-- 头部：点位 + 结果 + 元信息（§18.3 uni-card 分组容器） -->
    <uni-card :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <view class="head-row">
        <text class="point-name text-main" >{{ record.point_name }}</text>
        <!-- 记录级结论徽章：官方 uni-tag（custom-style 对齐原浅底深字配色） -->
        <uni-tag
          :text="record.result == 'abnormal' ? '⚠ 有异常' : '✓ 正常'"
          :custom-style="record.result == 'abnormal' ? 'color:#D54941;background-color:#FDECEC;border-color:transparent' : 'color:#2BA471;background-color:#E7F6EF;border-color:transparent'"
        />
      </view>
      <text class="head-sub text-secondary" >{{ headSub }}</text>
      <view v-if="record.checkin_time != null && record.checkin_time != ''" class="meta-row">
        <text class="meta-label text-secondary" >打卡时间</text>
        <uni-dateformat class="meta-value text-regular"  :date="record.checkin_time" format="yyyy-MM-dd hh:mm:ss" />
      </view>
      <view v-if="record.checkin_type != null && record.checkin_type != ''" class="meta-row">
        <text class="meta-label text-secondary" >打卡方式</text>
        <text class="meta-value text-regular" >{{ checkinTypeText }}</text>
      </view>
      <view v-if="record.distance_to_point != null" class="meta-row">
        <text class="meta-label text-secondary" >距点位</text>
        <text class="meta-value text-regular" >{{ record.distance_to_point }} 米</text>
      </view>
      <text v-if="record.remark != null && record.remark != ''" class="remark text-regular" >备注：{{ record.remark }}</text>
    </uni-card>

    <!-- AI 审核结论（有才显示） -->
    <uni-card v-if="record.ai_verdict != null && record.ai_verdict != ''" :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <text class="sec-title text-main" >AI 审核结论</text>
      <text class="info-line text-regular" >结论：{{ record.ai_verdict }}</text>
      <text v-if="record.ai_reason != null && record.ai_reason != ''" class="info-line text-secondary" >{{ record.ai_reason }}</text>
    </uni-card>

    <!-- 检查项逐项明细 -->
    <uni-card v-for="(it, i) in items" :key="it.name" :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <view class="item-head">
        <text class="item-name text-main" >{{ it.name }}</text>
        <uni-tag :text="itemResultTextOf(it.result ?? '', it.exception_type)" :type="itemResultTagType(it)" :inverted="true" size="small" />
      </view>
      <!-- 观察点 tag 快照：异常 tag 红色实心，正常 tag 灰色空心（官方 uni-tag） -->
      <view v-if="it.tags != null && it.tags.length > 0" class="tag-row">
        <uni-tag
          v-for="(t, ti) in it.tags"
          :key="ti"
          :text="t"
          size="small"
          :circle="true"
          :type="isAbnTag(it, t) ? 'error' : 'default'"
          :inverted="!isAbnTag(it, t)"
          :custom-style="isAbnTag(it, t) ? 'margin-right:16rpx;margin-bottom:12rpx' : 'color:#86909C;border-color:#E5E6EB;margin-right:16rpx;margin-bottom:12rpx'"
        />
      </view>
      <text v-if="it.note != null && it.note != ''" class="item-note text-regular" >备注：{{ it.note }}</text>
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
      <text v-else class="no-photo text-secondary" >无照片</text>
      <text v-if="it.ai_reason != null && it.ai_reason != ''" class="item-ai text-secondary" >AI：{{ it.ai_reason }}</text>

    </uni-card>
    <uni-card v-if="items.length == 0" :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <text class="no-photo text-secondary" >这次是纯打卡，没有检查项</text>
    </uni-card>

    <!-- 整单现场照片墙 -->
    <uni-card v-if="recordPhotoList.length > 0" :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <text class="sec-title text-main" >现场照片</text>
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
    </uni-card>

    <!-- 审核状态与审核意见 -->
    <uni-card v-if="record.audit_status != null && record.audit_status != ''" :is-shadow="true" :border="false" margin="0 0 24rpx 0" padding="28rpx" spacing="0">
      <text class="sec-title text-main" >审核结果</text>
      <text class="info-line" :style="{ color: auditColor }">{{ auditText }}<text v-if="record.audit_at != null && record.audit_at != ''"> · {{ record.audit_at }}</text></text>
      <text
        v-if="record.audit_remark != null && record.audit_remark != ''"
        class="info-line"
         :class="(record.audit_status == 'rejected' ? 'text-danger' : 'text-secondary')"
      >意见：{{ record.audit_remark }}</text>
    </uni-card>
  </view>
</template>

<script lang="ts">

import { toAbsUrl } from '@/utils/url'
import { checkinTypeTextOf, itemResultTextOf, itemResultColorKeyOf } from '@/utils/format'

/** 整单照片（对齐后端 photos 数组元素：优先水印图） */
export type CheckinDetailPhoto = {
  url: string
  watermarked_url?: string
}

/** 检查项（对齐详情接口 check_items；调用处可多带字段，不影响渲染） */
export type CheckinDetailItem = {
  name: string
  /** 三态结论：normal 正常 / abnormal 异常 / escaped 无法检查 */
  result?: string
  /** 无法检查原因（仅 escaped 态有意义：device_missing/unable_to_capture/camera_broken/label_missing） */
  exception_type?: string
  note?: string
  photo_urls?: string[]
  ai_verdict?: string | null
  ai_reason?: string | null
  /** 观察点 tag 快照（空/缺省=无观察点） */
  tags?: string[]
  /** 异常观察点 tag 列表（红色高亮展示） */
  abnormal_tags?: string[]
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
      if (s == 'rejected') return '#D54941'
      if (s == 'pending') return '#ED7B2F'
      return '#2BA471'
    },
    /** 整单现场照片：优先水印图，统一绝对地址 */
    recordPhotoList(): string[] {
      return (this.rec.photos ?? []).map((p) => toAbsUrl(p.watermarked_url != null && p.watermarked_url != '' ? p.watermarked_url : p.url))
    }
  },
  methods: {
    itemResultTextOf: itemResultTextOf,
    /** 三态 → uni-tag type（uni-tag 用 error 命名，danger 色系映射） */
    itemResultTagType(it: CheckinDetailItem): 'success' | 'warning' | 'error' {
      const k = itemResultColorKeyOf(it.result ?? '')
      return k == 'danger' ? 'error' : k
    },
    isAbnTag(it: CheckinDetailItem, t: string): boolean {
      return it.abnormal_tags != null && it.abnormal_tags.indexOf(t) >= 0
    },
    itemPhotoList(it: CheckinDetailItem): string[] {
      return (it.photo_urls ?? []).map(toAbsUrl)
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


/* 观察点 tag 快照行（芯片为 uni-tag） */
.tag-row {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
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
