<template>
  <!-- 单选数据选择器：原生 picker(mode=selector) 封装，系统级渲染（Android/iOS/鸿蒙均原生），长列表原生滚动。
       value=-1 表示未选（显示 placeholder）；选中变化 emit change(下标) -->
  <picker mode="selector" :range="range" :value="value < 0 ? 0 : value" :disabled="disabled" @change="onChange">
    <view class="picker-field border-default" :class="[boxBg, { 'picker-field-large': large }]" hover-class="hover-dim" >
      <text class="picker-field-text" :class="value < 0 || value >= range.length ? 'text-secondary' : 'text-main'">{{ displayText }}</text>
      <text class="text-secondary">›</text>
    </view>
  </picker>
</template>

<script lang="ts">
export default {
  props: {
    /** 选项文案数组（下标即值） */
    range: { type: Array, default: () => [] as string[] },
    /** 选中下标，-1=未选 */
    value: { type: Number, default: -1 },
    placeholder: { type: String, default: '请选择' },
    disabled: { type: Boolean, default: false },
    /** 大表单行（与登录/注册输入框同高 104rpx） */
    large: { type: Boolean, default: false },
    /** 底色工具类（bg-card / bg-page 等全局类） */
    boxBg: { type: String, default: 'bg-card' }
  },
  emits: ['change'],
  computed: {
    displayText(): string {
      if (this.value < 0 || this.value >= this.range.length) return this.placeholder
      return String(this.range[this.value])
    }
  },
  methods: {
    onChange(e: any) {
      this.$emit('change', Number(e.detail.value))
    }
  }
}
</script>

<style scoped>
.picker-field {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 22rpx 24rpx;
}

.picker-field-large {
  height: 104rpx;
  border-radius: 20rpx;
  padding: 0 32rpx;
}

.picker-field-text {
  font-size: 30rpx;
}
</style>
