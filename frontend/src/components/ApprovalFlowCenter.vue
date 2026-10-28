<template>
  <div class="table-card">
    <el-tabs v-model="activeTab">
      <el-tab-pane v-for="flow in flows" :key="flow.code" :label="flow.label" :name="flow.code" />
    </el-tabs>
    <ReviewFlowEditor
      v-if="activeFlow"
      :key="activeFlow.code"
      :api="flowApi"
      :slot-options="slotOptions"
      :save-perm="scope === 'platform' ? 'platform:reviewflow:update' : 'system:reviewflow:update'"
      :kind="activeFlow.kind"
      :title="activeFlow.label"
      :preview-api="previewApi"
      hide-title
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { FLOW_CODES, getFlow, saveFlow, type FlowCode, type ReviewFlowStep } from '@/api/post'
import { getFlowPreview } from '@/api/community'
import { listRoles } from '@/api/role'
import type { RoleItem } from '@/api/types'
import ReviewFlowEditor from '@/components/ReviewFlowEditor.vue'

const props = defineProps<{ scope: 'platform' | 'tenant'; communityId?: string }>()
const flows: { code: FlowCode; label: string; kind: 'checkin' | 'maint' | 'report' }[] = [
  { code: 'checkin_review', label: '打卡复核', kind: 'checkin' },
  { code: 'maint_review', label: '维保审核', kind: 'maint' },
  { code: 'workorder_review', label: '工单审核', kind: 'report' },
  { code: 'report_review', label: '报告审核与签署', kind: 'report' },
  { code: 'acceptance_review', label: '承接查验签署', kind: 'report' }
]
if (!FLOW_CODES.every((code) => flows.some((flow) => flow.code === code))) throw new Error('审批流程目录不完整')
const activeTab = ref<FlowCode>(flows[0].code)
const activeFlow = computed(() => flows.find((flow) => flow.code === activeTab.value))
const flowApi = {
  listFlow: () => getFlow(activeTab.value, props.scope),
  saveFlow: (steps: ReviewFlowStep[]) => saveFlow(activeTab.value, steps, props.scope)
}
const previewApi = computed(() => props.communityId ? () => getFlowPreview(props.communityId!, activeTab.value) : undefined)
const roles = ref<RoleItem[]>([])
const slotOptions = computed(() => roles.value.map((role) => ({ role_code: role.code, name: role.name })))
onMounted(async () => {
  roles.value = (await listRoles({ page: 1, page_size: 100 })).list
})
</script>
