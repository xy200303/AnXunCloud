<template>
  <div class="app-container logs-page">
    <div class="table-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="操作日志" name="operation">
          <operation-log v-if="activeTab === 'operation'" />
        </el-tab-pane>
        <el-tab-pane label="登录日志" name="login">
          <login-log v-if="activeTab === 'login'" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
// 日志管理：后端菜单为单页 /system/logs，操作日志与登录日志以 Tab 承载
import { ref } from 'vue'
import OperationLog from './components/OperationLog.vue'
import LoginLog from './components/LoginLog.vue'

const activeTab = ref('operation')
</script>

<style scoped lang="scss">
// 子组件自带 app-container 内边距，此处抵消避免双重留白
.logs-page {
  padding: 0;

  :deep(.app-container) {
    padding: 0;
  }

  :deep(.table-card) {
    padding: 0;
    background: transparent;
  }

  :deep(.filter-card) {
    margin-bottom: $spacing-lg;
    padding: $spacing-lg;
    background: $color-bg-card;
  }
}
</style>
