<template>
  <!-- 列表底部加载态：内部基于 wd-loadmore，对外 props 接口不变 -->
  <wd-loadmore
    v-if="visible && (loadingMore || noMore)"
    :state="loadingMore ? 'loading' : 'finished'"
    :loading-text="loadingText"
    :finished-text="noMoreText"
  />
</template>

<script lang="ts">
export default {
  props: {
    loadingMore: { type: Boolean, default: false },
    noMore: { type: Boolean, default: false },
    visible: { type: Boolean, default: true },
    loadingText: { type: String, default: '加载中…' },
    noMoreText: { type: String, default: '没有更多了' },
  }
}
</script>
