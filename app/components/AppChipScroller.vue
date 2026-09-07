<template>
  <scroll-view scroll-x class="app-chip-scroller" :show-scrollbar="false">
    <view class="app-chip-row">
      <view
        v-for="item in items"
        :key="item.value"
        class="app-chip"
        hover-class="hover-dim"
        :style="value == item.value
          ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
          : { backgroundColor: colors.bgCard, borderColor: colors.border }"
        @click="$emit('change', item.value)"
      >
        <text class="app-chip-text" :style="{ color: value == item.value ? colors.primary : colors.textSecondary }">{{ item.label }}</text>
      </view>
    </view>
  </scroll-view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

type ChipItem = { value: string; label: string }

export default {
  props: {
    value: { type: String, default: '' },
    items: { type: Array, default: () => [] as ChipItem[] },
    colors: { type: Object, default: () => Colors as ColorTokens }
  },
  emits: ['change']
}
</script>

<style scoped>
.app-chip-scroller {
  width: 100%;
  margin: 8rpx 0 16rpx;
}

.app-chip-row {
  flex-direction: row;
  padding: 4rpx 0 12rpx;
}

.app-chip {
  min-height: 64rpx;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 999rpx;
  align-items: center;
  justify-content: center;
  padding: 0 24rpx;
  margin-right: 16rpx;
}

.app-chip-text {
  font-size: 26rpx;
  white-space: nowrap;
}
</style>
