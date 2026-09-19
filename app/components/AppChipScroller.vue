<template>
  <!-- 横向筛选 chips（选项较多/动态数量场景；内芯 = scroll-view + uni-tag，选中实心品牌色）。
       对外 props/事件不变；≤4 个固定选项的筛选请用 uni-segmented-control -->
  <scroll-view scroll-x class="app-chip-scroller" :show-scrollbar="false">
    <view class="app-chip-row">
      <uni-tag
        v-for="item in items"
        :key="item.value"
        :text="item.label"
        :inverted="value != item.value"
        :circle="true"
        :custom-style="value == item.value ? 'background-color:#EAEFFF;border-color:#2B5AED;color:#2B5AED;margin-right:16rpx;white-space:nowrap' : 'background-color:#FFFFFF;border-color:#E5E6EB;color:#86909C;margin-right:16rpx;white-space:nowrap'"
        @click="$emit('change', item.value)"
      />
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
</style>
