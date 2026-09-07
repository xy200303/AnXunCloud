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
      <text class="selection-title" :style="{ color: colors.textPrimary }">{{ title }}</text>
      <view class="selection-clear" hover-class="hover-dim" @click="$emit('clear')">
        <text :style="{ color: colors.danger }">清空</text>
      </view>
    </view>
    <view v-if="loading" class="selection-state">
      <text :style="{ color: colors.textSecondary }">加载中…</text>
    </view>
    <view v-else-if="error != ''" class="selection-state" hover-class="hover-dim" @click="$emit('retry')">
      <text :style="{ color: colors.danger }">{{ error }}</text>
      <text class="selection-retry" :style="{ color: colors.primary }">重试</text>
    </view>
    <view v-else-if="items.length == 0" class="selection-state">
      <text :style="{ color: colors.textSecondary }">{{ emptyText }}</text>
    </view>
    <scroll-view v-else scroll-y class="selection-scroll" :show-scrollbar="false">
      <view
        v-for="item in items"
        :key="item.id"
        class="selection-item"
        hover-class="hover-dim"
        @click="$emit('toggle', item.id)"
      >
        <text :style="{ color: selectedIds.indexOf(item.id) >= 0 ? colors.primary : colors.textPrimary }">
          {{ selectedIds.indexOf(item.id) >= 0 ? '✓ ' : '○ ' }}{{ item.name }}
        </text>
        <text v-if="item.warning" class="selection-warning" :style="{ color: colors.warning }">{{ item.warning }}</text>
      </view>
    </scroll-view>
    <view class="selection-done" hover-class="hover-dim" @click="$emit('close')">
      <text :style="{ color: colors.primary }">完成</text>
    </view>
  </AppBottomSheet>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
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
    colors: { type: Object, default: () => Colors as ColorTokens }
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
