<template>
  <!-- 底部半屏弹层：内部基于 uni-popup（type=bottom），遮罩/动画/底部安全区交给官方；对外 props/事件接口不变 -->
  <uni-popup
    ref="popup"
    type="bottom"
    :mask-background-color="maskColor"
    border-radius="24rpx 24rpx 0 0"
    @maskClick="$emit('close')"
  >
    <view
      class="app-bottom-sheet"
      :style="{ backgroundColor: backgroundColor, height: height, maxHeight: maxHeight }"
    >
      <slot />
    </view>
  </uni-popup>
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
  watch: {
    visible(v: boolean) {
      const popup: any = this.$refs.popup
      if (popup == null) return
      if (v) popup.open()
      else popup.close()
    }
  },
  mounted() {
    if (this.visible) {
      const popup: any = this.$refs.popup
      if (popup != null) popup.open()
    }
  }
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
