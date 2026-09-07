<template>
  <view v-if="visible && (loadingMore || noMore)" class="app-list-footer">
    <text class="app-list-footer-text" :style="{ color: colors.textSecondary }">{{ loadingMore ? loadingText : noMoreText }}</text>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

export default {
  props: {
    loadingMore: { type: Boolean, default: false },
    noMore: { type: Boolean, default: false },
    visible: { type: Boolean, default: true },
    loadingText: { type: String, default: '加载中…' },
    noMoreText: { type: String, default: '没有更多了' },
    colors: { type: Object, default: () => Colors as ColorTokens }
  }
}
</script>

<style scoped>
.app-list-footer {
  min-height: 72rpx;
  align-items: center;
  justify-content: center;
  padding: 16rpx 24rpx 32rpx;
}

.app-list-footer-text {
  font-size: 24rpx;
}
</style>
