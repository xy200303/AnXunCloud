<template>
  <view
    class="app-filter-field"
    hover-class="hover-dim"
    :style="{ borderColor: colors.border }"
    @click="$emit('click')"
  >
    <text class="app-filter-field-text" :style="{ color: selected ? colors.textPrimary : colors.textSecondary }">{{ text }}</text>
    <text class="app-filter-field-arrow" :style="{ color: colors.textSecondary }">▾</text>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

export default {
  props: {
    text: { type: String, default: '' },
    selected: { type: Boolean, default: false },
    colors: { type: Object, default: () => Colors as ColorTokens }
  },
  emits: ['click']
}
</script>

<style scoped>
.app-filter-field {
  min-height: 72rpx;
  max-width: 100%;
  flex: 1;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 0 20rpx;
  margin-right: 16rpx;
}

.app-filter-field-text {
  flex: 1;
  overflow: hidden;
  font-size: 26rpx;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.app-filter-field-arrow {
  margin-left: 8rpx;
  font-size: 24rpx;
}
</style>
