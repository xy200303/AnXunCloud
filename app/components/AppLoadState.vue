<template>
  <view v-if="loading && showSkeleton" class="app-load-state app-load-state-skeleton">
    <view
      v-for="index in skeletonCount"
      :key="index"
      class="app-load-skeleton-block"
      :style="{ backgroundColor: colors.border }"
    ></view>
  </view>
  <view v-else-if="!loaded && error != ''" class="app-load-state app-load-state-empty">
    <text class="app-load-title" :style="{ color: colors.textRegular }">{{ error }}</text>
    <view class="app-load-action" hover-class="hover-dim" @click="$emit('retry')">
      <text :style="{ color: colors.primary }">{{ retryText }}</text>
    </view>
  </view>
  <view v-else-if="loaded && empty" class="app-load-state app-load-state-empty">
    <text class="app-load-title" :style="{ color: colors.textRegular }">{{ emptyTitle }}</text>
    <text v-if="emptySub != ''" class="app-load-sub" :style="{ color: colors.textSecondary }">{{ emptySub }}</text>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

export default {
  props: {
    loading: { type: Boolean, default: false },
    loaded: { type: Boolean, default: false },
    empty: { type: Boolean, default: false },
    error: { type: String, default: '' },
    emptyTitle: { type: String, default: '暂无数据' },
    emptySub: { type: String, default: '' },
    retryText: { type: String, default: '重试' },
    skeletonCount: { type: Number, default: 3 },
    showSkeleton: { type: Boolean, default: true },
    colors: { type: Object, default: () => Colors as ColorTokens }
  },
  emits: ['retry']
}
</script>

<style scoped>
.app-load-state {
  width: 100%;
}

.app-load-state-skeleton {
  padding: 24rpx;
}

.app-load-skeleton-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.app-load-skeleton-block:last-child {
  height: 96rpx;
}

.app-load-state-empty {
  align-items: center;
  padding: 160rpx 32rpx 48rpx;
}

.app-load-title {
  font-size: 34rpx;
  text-align: center;
  margin-bottom: 16rpx;
}

.app-load-sub {
  font-size: 26rpx;
  text-align: center;
  line-height: 1.5;
}

.app-load-action {
  min-height: 48rpx;
  padding: 12rpx 32rpx;
  align-items: center;
  justify-content: center;
}

.app-load-action text {
  font-size: 30rpx;
}
</style>
