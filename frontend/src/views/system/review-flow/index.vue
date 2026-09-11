<!-- 审批流程（系统管理，租户级）：打卡审批流程 + 维保审核流程 + 报告签字流程租户默认配置（扩展方案 §3）。
     生效顺序：项目级覆盖（小区管理 → 编制）→ 租户默认（本页）→ 平台默认（平台管理 → 审批流程模板）。 -->
<template>
  <div class="app-container">
    <div class="table-card">
      <el-alert
        type="info"
        :closable="false"
        title="此处为公司级默认，对本公司全部小区生效；单个小区可在「小区管理 → 编制」中单独配置。打卡审批流程管打卡记录的逐级审批（空流程=默认通过）；维保审核流程管维保登记的审核（空流程=登记即生效）；报告签字流程管报告生成后的签字审核（保存空流程表示生成后直接归档）。"
        style="margin-bottom: 16px"
      />
      <ReviewFlowEditor :api="flowApi" :slot-options="slotOptions" save-perm="system:reviewflow:update" />
      <ReviewFlowEditor kind="maint" :api="maintFlowApi" :slot-options="slotOptions" save-perm="system:reviewflow:update" />
      <ReviewFlowEditor kind="report" :api="reportFlowApi" :slot-options="slotOptions" save-perm="system:reviewflow:update" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getReviewFlow, saveReviewFlow, getReportReviewFlow, saveReportReviewFlow, getMaintReviewFlow, saveMaintReviewFlow, listPostDutyBindings, type ReviewFlowStep, type PostDutyBindingView } from '@/api/post'
import ReviewFlowEditor from '@/components/ReviewFlowEditor.vue'

const flowApi = {
  listFlow: getReviewFlow,
  saveFlow: (s: ReviewFlowStep[]) => saveReviewFlow(s) as Promise<unknown>
}

const maintFlowApi = {
  listFlow: getMaintReviewFlow,
  saveFlow: (s: ReviewFlowStep[]) => saveMaintReviewFlow(s) as Promise<unknown>
}

const reportFlowApi = {
  listFlow: getReportReviewFlow,
  saveFlow: (s: ReviewFlowStep[]) => saveReportReviewFlow(s) as Promise<unknown>
}

// 环节槽位选项（复用职责槽位列表）
const dutyList = ref<PostDutyBindingView[]>([])
const slotOptions = computed(() => dutyList.value.map((d) => ({ slot: d.slot, name: d.name })))

onMounted(async () => {
  dutyList.value = await listPostDutyBindings()
})
</script>
