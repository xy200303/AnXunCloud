<template>
  <view class="app-segment-tabs" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
    <view
      v-for="item in items"
      :key="item.value"
      class="app-segment-tab"
      hover-class="hover-dim"
      @click="$emit('change', item.value)"
    >
      <text
        class="app-segment-tab-text"
        :style="{ color: value == item.value ? colors.primary : colors.textRegular, fontWeight: value == item.value ? 600 : 400 }"
      >{{ item.label }}</text>
      <view
        class="app-segment-tab-line"
        :style="{ backgroundColor: value == item.value ? colors.primary : 'transparent' }"
      ></view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

type TabItem = { value: string; label: string }

export default {
  props: {
    value: { type: String, default: '' },
    items: { type: Array, default: () => [] as TabItem[] },
    colors: { type: Object, default: () => Colors as ColorTokens }
  },
  emits: ['change']
}
</script>

<style scoped>
.app-segment-tabs {
  flex-direction: row;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  padding: 0 32rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.app-segment-tab {
  flex: 1;
  min-height: 88rpx;
  align-items: center;
  justify-content: flex-end;
  padding-top: 16rpx;
}

.app-segment-tab-text {
  min-height: 44rpx;
  font-size: 30rpx;
  text-align: center;
}

.app-segment-tab-line {
  width: 48rpx;
  height: 6rpx;
  border-radius: 3rpx;
  margin-top: 12rpx;
  margin-bottom: 12rpx;
}
</style>
