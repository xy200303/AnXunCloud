<template>
  <div class="app-container">
    <el-tabs v-model="activeTab" class="plan-tabs">
      <el-tab-pane label="巡检计划" name="inspection">
        <inspection-plan-panel v-if="activeTab === 'inspection'" />
      </el-tab-pane>
      <el-tab-pane label="报告计划" name="report">
        <report-plan-panel v-if="activeTab === 'report'" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import InspectionPlanPanel from './InspectionPlanPanel.vue'
import ReportPlanPanel from './ReportPlanPanel.vue'

// 计划任务：巡检计划（周期→任务）与报告计划（周期→报告）同一入口的两个面板
const activeTab = ref('inspection')
</script>

<style scoped>
.plan-tabs :deep(.el-tabs__item) {
  font-size: 15px;
}
</style>
