<template>
  <!-- 底部半屏弹层：内部基于 wd-popup（position=bottom），遮罩/动画/底部安全区交给 wot；对外 props/事件接口不变 -->
  <wd-popup
    :model-value="visible"
    position="bottom"
    :modal-style="'background: ' + maskColor"
    :safe-area-inset-bottom="true"
    custom-style="border-radius: 24rpx 24rpx 0 0; overflow: hidden;"
    @close="$emit('close')"
  >
    <view
      class="app-bottom-sheet"
      :style="{ backgroundColor: backgroundColor, height: height, maxHeight: maxHeight }"
    >
      <slot />
    </view>
  </wd-popup>
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
  emits: ['close']
}
</script>

<style scoped>
.app-bottom-sheet {
  width: 100%;
  flex-direction: column;
  border-radius: 24rpx 24rpx 0 0;
  overflow: hidden;
}
</style>
