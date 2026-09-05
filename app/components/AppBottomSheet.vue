<template>
  <view v-if="visible" class="app-sheet-mask" :style="{ backgroundColor: maskColor }" @click="onMaskClick">
    <view
      class="app-bottom-sheet"
      :style="{ backgroundColor: backgroundColor, height: height, maxHeight: maxHeight }"
      @click.stop="noop"
    >
      <slot />
    </view>
  </view>
</template>

<script lang="ts">
export default {
  props: {
    visible: { type: Boolean, default: false },
    maskColor: { type: String, default: 'rgba(0, 0, 0, 0.45)' },
    backgroundColor: { type: String, default: '#ffffff' },
    height: { type: String, default: 'auto' },
    maxHeight: { type: String, default: '80vh' }
  },
  emits: ['close'],
  methods: {
    noop() {},
    onMaskClick() {
      this.$emit('close')
    }
  }
}
</script>

<style scoped>
.app-sheet-mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 99;
  justify-content: flex-end;
}

.app-bottom-sheet {
  width: 100%;
  flex-shrink: 0;
  flex-direction: column;
  border-radius: 24rpx 24rpx 0 0;
  overflow: hidden;
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
