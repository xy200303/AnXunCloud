<template>
  <AppBottomSheet
    :visible="visible"
    :mask-color="maskColor"
    :background-color="backgroundColor"
    :height="height"
    :max-height="maxHeight"
    @close="$emit('close')"
  >
    <view class="selection-header">
      <text class="selection-title text-main" >{{ title }}</text>
      <view class="selection-clear" hover-class="hover-dim" @click="$emit('clear')">
        <text  class="text-danger">清空</text>
      </view>
    </view>
    <view v-if="loading" class="selection-state">
      <text  class="text-secondary">加载中…</text>
    </view>
    <view v-else-if="error != ''" class="selection-state" hover-class="hover-dim" @click="$emit('retry')">
      <text  class="text-danger">{{ error }}</text>
      <text class="selection-retry text-brand" >重试</text>
    </view>
    <view v-else-if="items.length == 0" class="selection-state">
      <text  class="text-secondary">{{ emptyText }}</text>
    </view>
    <scroll-view v-else scroll-y class="selection-scroll" :show-scrollbar="false">
      <view
        v-for="item in items"
        :key="item.id"
        class="selection-item"
        hover-class="hover-dim"
        @click="$emit('toggle', item.id)"
      >
        <text :style="{ color: selectedIds.indexOf(item.id) >= 0 ? '#2B5AED' : '#1F2329' }">
          {{ selectedIds.indexOf(item.id) >= 0 ? '✓ ' : '○ ' }}{{ item.name }}
        </text>
        <text v-if="item.warning" class="selection-warning text-warning" >{{ item.warning }}</text>
      </view>
    </scroll-view>
    <view class="selection-done" hover-class="hover-dim" @click="$emit('close')">
      <text  class="text-brand">完成</text>
    </view>
  </AppBottomSheet>
</template>

<script lang="ts">

import AppBottomSheet from '@/components/AppBottomSheet.vue'

type SelectionItem = { id: string; name: string; warning?: string }

export default {
  components: { AppBottomSheet },
  props: {
    visible: { type: Boolean, default: false },
    title: { type: String, default: '请选择' },
    items: { type: Array, default: () => [] as SelectionItem[] },
    selectedIds: { type: Array, default: () => [] as string[] },
    loading: { type: Boolean, default: false },
    error: { type: String, default: '' },
    emptyText: { type: String, default: '暂无可选项' },
    height: { type: String, default: 'auto' },
    maxHeight: { type: String, default: '80vh' },
    maskColor: { type: String, default: 'rgba(0, 0, 0, 0.45)' },
    backgroundColor: { type: String, default: '#ffffff' },
  },
  emits: ['close', 'clear', 'retry', 'toggle']
}
</script>

<style scoped>
.selection-header,
.selection-item {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.selection-header {
  padding: 24rpx 32rpx;
}

.selection-title,
.selection-header text,
.selection-item text,
.selection-done text {
  font-size: 30rpx;
}

.selection-title {
  font-weight: 600;
}

.selection-clear,
.selection-done {
  min-height: 48rpx;
  align-items: center;
  justify-content: center;
  padding: 8rpx 16rpx;
}

.selection-state {
  align-items: center;
  padding: 42rpx 32rpx;
}

.selection-retry {
  margin-top: 12rpx;
}

.selection-scroll {
  height: 52vh;
  max-height: 720rpx;
}

.selection-item {
  min-height: 96rpx;
  padding: 0 32rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
  border-top-color: #f0f0f0;
}

.selection-warning {
  font-size: 24rpx !important;
}

.selection-done {
  padding: 20rpx 32rpx;
}
</style>
