<template>
  <span class="status-dot" :class="`due-${state}`" />
</template>

<script setup lang="ts">
// 设备到期状态灯：normal 绿 / warning 黄 / overdue 红 / none 灰 / scrap 红晕 / label_missing 虚线圈
defineProps<{ state: string }>()
</script>

<style scoped lang="scss">
.status-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;

  &.due-normal {
    background: $color-success;
  }

  &.due-warning {
    background: $color-warning;
  }

  &.due-overdue {
    background: $color-danger;
  }

  &.due-none {
    background: $color-text-placeholder;
  }

  &.due-scrap {
    background: $color-danger;
    box-shadow: 0 0 0 4px rgba($color-danger, 0.25);
  }

  &.due-label_missing {
    background: $color-text-placeholder;
    border: 2px dashed $color-text-secondary;
    box-sizing: border-box;
  }
}
</style>
