<template>
  <!-- 底部动作面板（自绘，替代 uni.showActionSheet）：微信式列表 + 独立取消行，遮罩点击即取消 -->
  <view v-if="visible" class="sheet-mask" :style="{ backgroundColor: colors.mask }" @click="onCancel">
    <view class="sheet-panel" @click.stop="noop">
      <view class="sheet-body" :style="{ backgroundColor: colors.bgCard }">
        <view v-if="title != ''" class="sheet-row sheet-title-row" :style="{ borderBottomColor: colors.border }">
          <text class="sheet-title" :style="{ color: colors.textSecondary }">{{ title }}</text>
        </view>
        <view
          v-for="(item, idx) in items"
          :key="idx"
          class="sheet-row"
          :style="idx > 0 ? { borderTopWidth: '1rpx', borderTopStyle: 'solid', borderTopColor: colors.border } : {}"
          hover-class="hover-dim"
          @click="onSelect(idx)"
        >
          <text class="sheet-row-text" :style="{ color: colors.textPrimary }">{{ item }}</text>
        </view>
      </view>
      <!-- 取消行与列表间留出页面底色间隔（微信式分组） -->
      <view class="sheet-gap" :style="{ backgroundColor: colors.bgPage }"></view>
      <view class="sheet-row sheet-cancel" :style="{ backgroundColor: colors.bgCard }" hover-class="hover-dim" @click="onCancel">
        <text class="sheet-row-text" :style="{ color: colors.textRegular }">{{ cancelText }}</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

type SheetData = {
  colors: ColorTokens
}

export default {
  props: {
    visible: { type: Boolean, default: false },
    title: { type: String, default: '' },
    items: { type: Array, default: () => [] as string[] },
    cancelText: { type: String, default: '取消' }
  },
  emits: ['select', 'cancel', 'update:visible'],
  data(): SheetData {
    return {
      colors: Colors
    }
  },
  methods: {
    noop() {},
    onSelect(idx: number) {
      this.$emit('update:visible', false)
      this.$emit('select', idx)
    },
    onCancel() {
      this.$emit('update:visible', false)
      this.$emit('cancel')
    }
  }
}
</script>

<style scoped>
.sheet-mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
  justify-content: flex-end;
  animation: sheet-fade-in 180ms ease-out;
}

.sheet-panel {
  width: 100%;
  flex-shrink: 0;
  flex-direction: column;
  animation: sheet-slide-up 220ms ease-out;
}

.sheet-body {
  border-radius: 24rpx 24rpx 0 0;
  overflow: hidden;
}

.sheet-row {
  height: 110rpx;
  align-items: center;
  justify-content: center;
}

.sheet-title-row {
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.sheet-title {
  font-size: 26rpx;
}

.sheet-row-text {
  font-size: 32rpx;
}

.sheet-gap {
  height: 16rpx;
}

.sheet-cancel {
  /* 底部安全区：取消行不被全面屏手势条遮挡 */
  padding-bottom: env(safe-area-inset-bottom);
}

@keyframes sheet-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes sheet-slide-up {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}
</style>
