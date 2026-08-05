<template>
  <div class="error-page">
    <el-result icon="warning" title="404" sub-title="页面不存在或已被移除">
      <template #extra>
        <el-button type="primary" @click="$router.push('/dashboard')">返回工作台</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped lang="scss">
.error-page {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: $color-bg-page;
}
</style>
