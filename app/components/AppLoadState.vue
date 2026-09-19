<template>
  <view v-if="loading && showSkeleton" class="app-load-state app-load-state-skeleton">
    <view
      v-for="index in skeletonCount"
      :key="index"
      class="app-load-skeleton-block bg-border"
      
    ></view>
  </view>
  <!-- 加载中（非骨架场景）：官方 uni-load-more 转圈态 -->
  <uni-load-more v-else-if="loading" status="loading" :content-text="{ contentrefresh: '加载中…' }" :color="'#86909C'" />
  <view v-else-if="!loaded && error != ''" class="app-load-state app-load-state-empty">
    <text class="app-load-title text-regular" >{{ error }}</text>
    <view class="app-load-action" hover-class="hover-dim" @click="$emit('retry')">
      <text  class="text-brand">{{ retryText }}</text>
    </view>
  </view>
  <view v-else-if="loaded && empty" class="app-load-state app-load-state-empty">
    <text class="app-load-title text-regular" >{{ emptyTitle }}</text>
    <text v-if="emptySub != ''" class="app-load-sub text-secondary" >{{ emptySub }}</text>
  </view>
</template>

<script lang="ts">


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
