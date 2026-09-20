<template>
  <!-- 分段页签：内部基于 wd-segmented（card 主题，品牌色由全局 CSS 变量注入），
       对外 value/items/change 接口不变（options 直接吃 {value,label}） -->
  <wd-segmented
    custom-class="app-segment-tabs"
    :value="value"
    :options="items"
    @change="onChange"
  />
</template>

<script lang="ts">
type TabItem = { value: string; label: string }

export default {
  props: {
    value: { type: String, default: '' },
    items: { type: Array, default: () => [] as TabItem[] },
  },
  emits: ['change'],
  methods: {
    onChange(option: { value: string }) {
      this.$emit('change', option.value)
    }
  }
}
</script>

<style scoped>
.app-segment-tabs {
  margin-bottom: 24rpx;
}
</style>
