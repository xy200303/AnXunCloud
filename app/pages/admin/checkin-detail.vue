<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 加载中 / 失败 -->
    <view v-if="loading" class="hint">
      <text class="hint-text" :style="{ color: colors.textSecondary }">加载中…</text>
    </view>
    <view v-else-if="!loaded" class="hint">
      <text class="hint-text" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="hint-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <block v-else-if="d != null">
      <!-- 疑似标记横幅 -->
      <view v-if="d.is_suspect" class="suspect-bar" :style="{ backgroundColor: '#FDF3E7' }">
        <text class="suspect-text" :style="{ color: colors.warning }">⚠ 疑似异常打卡：{{ d.suspect_reason != '' ? d.suspect_reason : '时间/位置存疑' }}</text>
      </view>

      <!-- 头部：点位 + 结果 + 元信息 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <view class="head-row">
          <text class="point-name" :style="{ color: colors.textPrimary }">{{ d.point_name }}</text>
          <text
            class="result-badge"
            :style="{
              color: d.result == 'abnormal' ? colors.danger : colors.success,
              backgroundColor: d.result == 'abnormal' ? '#FDECEC' : '#E7F6EF'
            }"
          >{{ d.result == 'abnormal' ? '⚠ 有异常' : '✓ 正常' }}</text>
        </view>
        <text class="head-sub" :style="{ color: colors.textSecondary }">{{ d.community_name }} · {{ d.inspector_name }}<text v-if="d.plan_name != ''"> · {{ d.plan_name }}</text></text>
        <view class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">打卡时间</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ d.checkin_time }}</text>
        </view>
        <view class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">打卡方式</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ checkinTypeText }}</text>
        </view>
        <view v-if="d.distance_to_point != null" class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">距点位</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ d.distance_to_point }} 米</text>
        </view>
        <view class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">审核状态</text>
          <text class="meta-value" :style="{ color: auditColor }">{{ auditText }}</text>
        </view>
        <text v-if="d.remark != ''" class="remark" :style="{ color: colors.textRegular }">备注：{{ d.remark }}</text>
      </view>

      <!-- 逐项明细 -->
      <view v-for="(it, i) in d.check_items" :key="i" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <view class="item-head">
          <text class="item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
          <text class="item-result" :style="{ color: it.pass ? colors.success : colors.danger }">{{ it.pass ? '✓ 正常' : '⚠ 异常' }}</text>
        </view>
        <image
          v-if="it.photo_urls != null && it.photo_urls.length > 0"
          :src="it.photo_urls[0]"
          class="item-photo"
          mode="aspectFill"
          @click="preview(it)"
        />
        <text v-if="it.ai_reason != null && it.ai_reason != ''" class="item-ai" :style="{ color: colors.textSecondary }">AI：{{ it.ai_reason }}</text>
        <text v-if="it.note != ''" class="item-note" :style="{ color: colors.textRegular }">备注：{{ it.note }}</text>
      </view>
      <view v-if="d.check_items.length == 0" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <text class="hint-text" :style="{ color: colors.textSecondary }">这次是纯打卡，没有检查项</text>
      </view>

      <view class="bottom-space"></view>
    </block>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { apiAdminCheckinDetail, AdminCheckinDetail } from '@/services/api'

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
  data() {
    return {
      colors: Colors,
      shadow: ShadowCard,
      checkinId: '',
      loading: true,
      loaded: false,
      errorMsg: '加载失败',
      d: null as AdminCheckinDetail | null
    }
  },
  computed: {
    checkinTypeText(): string {
      return this.d == null ? '' : checkinTypeTextOf(this.d.checkin_type)
    },
    auditText(): string {
      return this.d == null ? '' : auditTextOf(this.d.audit_status)
    },
    auditColor(): string {
      if (this.d == null) return Colors.textRegular
      if (this.d.audit_status == 'rejected') return Colors.danger
      if (this.d.audit_status == 'pending') return Colors.warning
      return Colors.success
    }
  },
  onLoad(options: any) {
    this.checkinId = options && options.id ? String(options.id) : ''
    this.load()
  },
  methods: {
    load() {
      if (this.checkinId == '') {
        this.loading = false
        this.errorMsg = '缺少参数'
        return
      }
      this.loading = !this.loaded
      apiAdminCheckinDetail(this.checkinId)
        .then((d) => {
          this.d = d
          this.loading = false
          this.loaded = true
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
        })
    },
    preview(it: { photo_urls?: string[] }) {
      if (it.photo_urls == null || it.photo_urls.length == 0) return
      uni.previewImage({ urls: it.photo_urls })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.hint {
  align-items: center;
  padding-top: 192rpx;
}

.hint-text {
  font-size: 30rpx;
}

.hint-retry {
  margin-top: 24rpx;
  font-size: 30rpx;
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

.item-photo {
  width: 100%;
  height: 380rpx;
  border-radius: 16rpx;
  margin-top: 20rpx;
}

.item-ai {
  font-size: 26rpx;
  margin-top: 16rpx;
  line-height: 1.6;
}

.item-note {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.bottom-space {
  height: 48rpx;
}
</style>
