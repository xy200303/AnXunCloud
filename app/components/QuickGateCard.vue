<template>
  <view class="gate-wrap">
    <view class="card gate-card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <text class="gate-title" :style="{ color: colors.textPrimary }">本点位 {{ itemCount }} 项已过完</text>
      <view class="gate-stats">
        <view class="gate-stat"><text class="gate-stat-num" :style="{ color: colors.success }">{{ stats.done }}</text><text class="gate-stat-label" :style="{ color: colors.textSecondary }">已完成</text></view>
        <view class="gate-stat"><text class="gate-stat-num" :style="{ color: colors.primary }">{{ stats.recognizing }}</text><text class="gate-stat-label" :style="{ color: colors.textSecondary }">AI 检查中</text></view>
        <view class="gate-stat"><text class="gate-stat-num" :style="{ color: colors.danger }">{{ stats.abnormal }}</text><text class="gate-stat-label" :style="{ color: colors.textSecondary }">异常</text></view>
      </view>
      <text class="gate-sub" :style="{ color: colors.textSecondary }">{{ stats.recognizing > 0 ? '提交后将等待 AI 检查完成' : '提交后 AI 统一检查' }}</text>
    </view>
    <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.success }" @click="$emit('submit')">
      <text class="btn-big-text" :style="{ color: colors.white }">提交本点位</text>
    </view>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'

type GateStats = { done: number; recognizing: number; abnormal: number }

export default {
  props: {
    itemCount: { type: Number, default: 0 },
    stats: { type: Object as () => GateStats, required: true },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: ['submit']
}
</script>

<style scoped>
.gate-wrap { width: 100%; }
.card { border-radius: 24rpx; padding: 32rpx; margin-bottom: 24rpx; }
.gate-card { align-items: center; }
.gate-title { font-size: 48rpx; font-weight: 700; }
.gate-stats { flex-direction: row; align-items: center; justify-content: space-around; width: 100%; margin-top: 32rpx; }
.gate-stat { align-items: center; }
.gate-stat-num { font-size: 64rpx; font-weight: 700; }
.gate-stat-label { font-size: 28rpx; margin-top: 8rpx; }
.gate-sub { font-size: 30rpx; margin-top: 24rpx; }
.btn-big { width: 100%; height: 140rpx; border-radius: 20rpx; align-items: center; justify-content: center; margin-bottom: 24rpx; }
.btn-big-text { font-size: 44rpx; font-weight: 700; }
</style>
