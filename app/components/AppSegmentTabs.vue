<template>
  <!-- 分段页签：内部基于 uni-segmented-control（styleType=text：文字 + 选中下划线，品牌色 uni.scss $uni-color-primary），
       对外 value/items/change 接口不变（value↔下标换算在本组件内完成） -->
  <view class="app-segment-tabs bg-card border-default" >
    <uni-segmented-control
      :values="labels"
      :current="curIdx"
      style-type="text"
      :active-color="'#2B5AED'"
      @clickItem="onClickItem"
    />
  </view>
</template>

<script lang="ts">


type TabItem = { value: string; label: string }

export default {
  props: {
    value: { type: String, default: '' },
    items: { type: Array, default: () => [] as TabItem[] },
  },
  emits: ['change'],
  computed: {
    labels(): string[] {
      return (this.items as TabItem[]).map((it) => it.label)
    },
    curIdx(): number {
      const i = (this.items as TabItem[]).findIndex((it) => it.value == this.value)
      return i >= 0 ? i : 0
    }
  },
  methods: {
    onClickItem(e: { currentIndex: number }) {
      const it = (this.items as TabItem[])[e.currentIndex]
      if (it != null) this.$emit('change', it.value)
    }
  }
}
</script>

<style scoped>
.app-segment-tabs {
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  padding: 0 32rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}
</style>
