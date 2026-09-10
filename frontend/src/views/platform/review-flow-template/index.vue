<!-- 审批流程模板（平台管理，仅超管）：平台默认打卡审批流程 + 报告签字流程。
     未配置审批流程的租户/项目回落到此默认；再未配置时回落代码内置默认。 -->
<template>
  <div class="app-container">
    <div class="table-card">
      <el-alert
        type="info"
        :closable="false"
        title="此处为平台默认：新开通租户与未自行配置的租户/项目均回落到此默认。修改即时生效（对已有自定义配置的租户无影响）。报告签字流程保存为空表示报告生成后直接归档。"
        style="margin-bottom: 16px"
      />
      <ReviewFlowEditor :api="flowApi" :slot-options="slotOptions" save-perm="platform:reviewflow:update" />
      <ReviewFlowEditor kind="report" :api="reportFlowApi" :slot-options="slotOptions" save-perm="platform:reviewflow:update" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getPostTemplateReviewFlow, savePostTemplateReviewFlow, getPostTemplateReportReviewFlow, savePostTemplateReportReviewFlow, listPostTemplateDutyBindings, type ReviewFlowStep, type PostDutyBindingView } from '@/api/post'
import ReviewFlowEditor from '@/components/ReviewFlowEditor.vue'

const flowApi = {
  listFlow: getPostTemplateReviewFlow,
  saveFlow: (s: ReviewFlowStep[]) => savePostTemplateReviewFlow(s) as Promise<unknown>
}

const reportFlowApi = {
  listFlow: getPostTemplateReportReviewFlow,
  saveFlow: (s: ReviewFlowStep[]) => savePostTemplateReportReviewFlow(s) as Promise<unknown>
}

const dutyList = ref<PostDutyBindingView[]>([])
const slotOptions = computed(() => dutyList.value.map((d) => ({ slot: d.slot, name: d.name })))

onMounted(async () => {
  dutyList.value = await listPostTemplateDutyBindings()
})
</script>
