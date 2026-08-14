<template>
  <!-- App 端 PDF 预览：web-view 内嵌后端 pdf.js 查看器（报告正文在内嵌页内渲染） -->
  <web-view :src="src"></web-view>
</template>

<script lang="ts">
export default {
  data() {
    return {
      src: ''
    }
  },
  onLoad(options: any) {
    if (options != null && options['src'] != null) {
      this.src = decodeURIComponent(String(options['src']))
    }
  }
}
</script>
