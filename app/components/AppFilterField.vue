<template>
  <view
    class="app-filter-field border-default"
    hover-class="hover-dim"
    
    @click="$emit('click')"
  >
    <text class="app-filter-field-text"  :class="(selected ? 'text-main' : 'text-secondary')">{{ text }}</text>
    <text class="app-filter-field-arrow text-secondary" >▾</text>
  </view>
</template>

<script lang="ts">


export default {
  props: {
    text: { type: String, default: '' },
    selected: { type: Boolean, default: false },
  },
  emits: ['click']
}
</script>

<style scoped>
.app-filter-field {
  min-height: 72rpx;
  max-width: 100%;
  flex: 1;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 0 20rpx;
  margin-right: 16rpx;
}

.app-filter-field-text {
  flex: 1;
  overflow: hidden;
  font-size: 26rpx;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.app-filter-field-arrow {
  margin-left: 8rpx;
  font-size: 24rpx;
}
</style>
