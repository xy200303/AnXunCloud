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
// 日志管理单页：外层白卡承载 Tab，子面板内的卡片样式摊平避免嵌套白卡
.logs-page {
  :deep(.log-panel) {
    .filter-card {
      background: transparent;
      border-radius: 0;
      padding: 0 0 $spacing-lg;
      margin-bottom: $spacing-lg;
      border-bottom: 1px solid $color-border;
    }

    .table-card {
      background: transparent;
      padding: 0;
    }
  }
}
</style>
