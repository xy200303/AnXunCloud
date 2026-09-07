<template>
  <view class="credential-wrap">
    <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="cred-title" :style="{ color: colors.textPrimary }">到场打卡</text>
      <text v-if="!needsCred" class="cred-none" :style="{ color: colors.success }">✓ 本点位直接拍照就行</text>
      <view v-if="point != null && (point.credential == 'qrcode' || point.credential == 'any')" hover-class="hover-dim" class="cred-row" @click="$emit('scan')">
        <text class="cred-row-name" :style="{ color: colors.textPrimary }">扫点位二维码</text>
        <text v-if="wizPoint != null && wizPoint.scannedNo != ''" class="cred-status" :style="{ color: colors.success }">✓ 已完成</text>
        <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
      </view>
      <view v-if="point != null && (point.credential == 'nfc' || point.credential == 'any')" hover-class="hover-dim" class="cred-row" @click="$emit('nfc')">
        <text class="cred-row-name" :style="{ color: colors.textPrimary }">刷 NFC 卡</text>
        <text v-if="wizPoint != null && wizPoint.nfcCardId != ''" class="cred-status" :style="{ color: colors.success }">✓ 已完成</text>
        <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
      </view>
      <view v-if="point != null && point.require_fence" hover-class="hover-dim" class="cred-row" @click="$emit('location')">
        <text class="cred-row-name" :style="{ color: colors.textPrimary }">位置确认</text>
        <text v-if="locating" class="cred-status" :style="{ color: colors.textSecondary }">定位中…</text>
        <text v-else-if="locFailed" class="cred-status" :style="{ color: colors.danger }">失败，点我重试</text>
        <text v-else-if="distance >= 0 && point != null && distance <= point.fence_radius" class="cred-status" :style="{ color: colors.success }">✓ 在范围内（{{ distance }}米）</text>
        <text v-else-if="distance >= 0" class="cred-status" :style="{ color: colors.danger }">超出范围（{{ distance }}米）</text>
        <text v-else class="cred-status" :style="{ color: colors.textSecondary }">去完成 ›</text>
      </view>
    </view>
    <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: credOk && fenceOk ? colors.success : colors.border }" @click="$emit('start')">
      <text class="btn-big-text" :style="{ color: credOk && fenceOk ? colors.white : colors.textSecondary }">开始检查</text>
    </view>
    <text v-if="!(credOk && fenceOk)" class="start-hint" :style="{ color: colors.textSecondary }">先完成上面的确认，再开始检查</text>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import type { WizardPointSnap } from '@/utils/checkinWizard'
import type { TaskPoint } from '@/services/api'

export default {
  props: {
    point: { type: Object as () => TaskPoint | null, default: null },
    wizPoint: { type: Object as () => WizardPointSnap | null, default: null },
    needsCred: { type: Boolean, default: false },
    credOk: { type: Boolean, default: false },
    fenceOk: { type: Boolean, default: false },
    locating: { type: Boolean, default: false },
    locFailed: { type: Boolean, default: false },
    distance: { type: Number, default: -1 },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: ['scan', 'nfc', 'location', 'start']
}
</script>

<style scoped>
.credential-wrap { width: 100%; }
.card { border-radius: 24rpx; padding: 32rpx; margin-bottom: 24rpx; }
.cred-title { font-size: 36rpx; font-weight: 700; margin-bottom: 8rpx; }
.cred-none { font-size: 40rpx; font-weight: 600; text-align: center; padding: 24rpx 0; }
.cred-row { flex-direction: row; align-items: center; justify-content: space-between; min-height: 104rpx; border-bottom-width: 1rpx; border-bottom-style: solid; border-bottom-color: rgba(0, 0, 0, 0.05); }
.cred-row-name { font-size: 40rpx; font-weight: 600; }
.cred-status { font-size: 32rpx; font-weight: 600; }
.btn-big { width: 100%; height: 140rpx; border-radius: 20rpx; align-items: center; justify-content: center; margin-bottom: 24rpx; }
.btn-big-text { font-size: 44rpx; font-weight: 700; }
.start-hint { font-size: 28rpx; text-align: center; margin-top: -8rpx; margin-bottom: 24rpx; }
</style>
