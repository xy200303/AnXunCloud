<template>
  <!-- 列表底部加载态：内部基于 uni-load-more（官方 loading 图标 + 文案），对外 props 接口不变 -->
  <uni-load-more
    v-if="visible && (loadingMore || noMore)"
    :status="loadingMore ? 'loading' : 'noMore'"
    :content-text="{ contentrefresh: loadingText, contentnomore: noMoreText }"
    :color="'#86909C'"
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
