<template>
  <!-- 横向筛选 chips（选项较多/动态数量场景；内芯 = scroll-view + wd-tag，选中品牌色）。
       对外 props/事件不变；≤4 个固定选项的筛选请用 AppSegmentTabs -->
  <scroll-view scroll-x class="app-chip-scroller" :show-scrollbar="false">
    <view class="app-chip-row">
      <wd-tag
        v-for="item in items"
        :key="item.value"
        :type="value == item.value ? 'primary' : 'default'"
        round
        custom-class="app-chip"
        @click="$emit('change', item.value)"
      >{{ item.label }}</wd-tag>
    </view>
  </scroll-view>
</template>

<script lang="ts">
type ChipItem = { value: string; label: string }

export default {
  props: {
    value: { type: String, default: '' },
    items: { type: Array, default: () => [] as ChipItem[] },
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
  align-items: center;
  padding: 4rpx 0 12rpx;
}

.app-chip {
  margin-right: 16rpx;
  white-space: nowrap;
}
</style>
