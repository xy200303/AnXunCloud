<template>
  <div class="app-container">
    <div class="table-card plan-tabs-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="巡检计划" name="inspection" />
        <el-tab-pane label="报告计划" name="report" />
      </el-tabs>
    </div>
    <inspection-plan-panel v-if="activeTab === 'inspection'" />
    <report-plan-panel v-else />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import InspectionPlanPanel from './InspectionPlanPanel.vue'
import ReportPlanPanel from './ReportPlanPanel.vue'

// 计划任务：巡检计划（周期→任务）与报告计划（周期→报告）同一入口的两个面板
const activeTab = ref('inspection')
</script>

<style scoped lang="scss">
.plan-tabs-card {
  padding-bottom: 0;
  margin-bottom: $spacing-lg;

  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }
}
</style>
