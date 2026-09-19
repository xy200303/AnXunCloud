<template>
  <!-- 底部动作面板：内部基于 uni-popup（type=bottom），遮罩/动画/安全区交给官方；
       微信式列表 + 独立取消行，遮罩点击即取消。对外 props/事件接口不变 -->
  <uni-popup
    ref="popup"
    type="bottom"
    :mask-background-color="'rgba(0, 0, 0, 0.45)'"
    @maskClick="onCancel"
  >
    <view class="sheet-panel">
      <view class="sheet-body bg-card" >
        <view v-if="title != ''" class="sheet-row sheet-title-row border-default" >
          <text class="sheet-title text-secondary" >{{ title }}</text>
        </view>
        <view
          v-for="(item, idx) in items"
          :key="idx"
          class="sheet-row"
          :style="idx > 0 ? { borderTopWidth: '1rpx', borderTopStyle: 'solid', borderTopColor: '#E5E6EB' } : {}"
          hover-class="hover-dim"
          @click="onSelect(idx)"
        >
          <text class="sheet-row-text text-main" >{{ item }}</text>
        </view>
      </view>
      <!-- 取消行与列表间留出页面底色间隔（微信式分组） -->
      <view class="sheet-gap bg-page" ></view>
      <view class="sheet-row sheet-cancel bg-card"  hover-class="hover-dim" @click="onCancel">
        <text class="sheet-row-text text-regular" >{{ cancelText }}</text>
      </view>
    </view>
  </uni-popup>
</template>

<script lang="ts">


type SheetData = {
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
    }
  },
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
  },
  methods: {
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
.sheet-panel {
  width: 100%;
  flex-direction: column;
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
</style>
