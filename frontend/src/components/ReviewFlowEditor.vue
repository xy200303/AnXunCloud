<!-- 打卡审批流程编辑器（扩展方案 §3）：租户级默认 / 平台模板 / 项目级覆盖三处共用。
     流程 = 有序环节列表，每个环节引用一个职责槽位；环节名单解析与授权复用槽位体系。 -->
<template>
  <div class="flow-editor">
    <div class="flow-head">
      <span class="flow-title">打卡审批流程</span>
      <el-tag v-if="sourceLabel" size="small" :type="source === 'platform' || source === 'tenant' ? 'warning' : source === 'project' ? 'success' : 'info'" effect="plain">
        {{ sourceLabel }}
      </el-tag>
    </div>
    <el-alert
      type="info"
      :closable="false"
      title="打卡记录按环节顺序逐级审核：当前环节名单成员通过后进入下一环节，末环节通过才生效；驳回即打回。环节审核人 = 负责岗位在该项目编制里的在职成员"
      class="flow-tip"
    />
    <div v-loading="loading">
      <div v-for="(step, idx) in steps" :key="idx" class="flow-step">
        <span class="step-no">{{ idx + 1 }}</span>
        <el-input v-model="step.name" placeholder="环节名称" maxlength="32" class="step-name" :disabled="!canEdit" />
        <el-select v-model="step.slot" placeholder="审核人（按岗位）" class="step-slot" :disabled="!canEdit">
          <el-option v-for="s in slotOptions" :key="s.slot" :label="s.name" :value="s.slot" />
        </el-select>
        <template v-if="canEdit">
          <el-button :icon="Top" circle size="small" :disabled="idx === 0" @click="move(idx, -1)" />
          <el-button :icon="Bottom" circle size="small" :disabled="idx === steps.length - 1" @click="move(idx, 1)" />
          <el-button :icon="Delete" circle size="small" type="danger" plain :disabled="steps.length <= 1" @click="steps.splice(idx, 1)" />
        </template>
      </div>
      <div class="flow-footer">
        <el-button v-if="canEdit" :icon="Plus" size="small" :disabled="steps.length >= 5" @click="addStep">添加环节</el-button>
        <el-button v-if="canEdit" type="primary" size="small" :loading="saving" @click="handleSave">保存审批流程</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Top, Bottom, Delete, Plus } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import type { ReviewFlowStep, ReviewFlowView } from '@/api/post'

const props = defineProps<{
  api: {
    listFlow: () => Promise<ReviewFlowView>
    saveFlow: (steps: ReviewFlowStep[]) => Promise<unknown>
  }
  slotOptions: { slot: string; name: string }[]
  savePerm: string
}>()

const userStore = useUserStore()
const canEdit = computed(() => userStore.hasPerm(props.savePerm))

const loading = ref(false)
const saving = ref(false)
const steps = ref<ReviewFlowStep[]>([])
const source = ref('')

const SOURCE_LABELS: Record<string, string> = {
  project: '项目覆盖',
  tenant: '租户默认',
  platform: '平台默认',
  default: '内置默认'
}
const sourceLabel = computed(() => SOURCE_LABELS[source.value] || '')

async function fetchFlow() {
  loading.value = true
  try {
    const data = await props.api.listFlow()
    steps.value = data.steps.map((s) => ({ ...s }))
    source.value = data.source
  } finally {
    loading.value = false
  }
}

function move(idx: number, dir: number) {
  const j = idx + dir
  const tmp = steps.value[idx]
  steps.value[idx] = steps.value[j]
  steps.value[j] = tmp
}

function addStep() {
  steps.value.push({ slot: '', name: '' })
}

async function handleSave() {
  for (const s of steps.value) {
    if (!s.name.trim() || !s.slot) {
      ElMessage.warning('每个环节都须填写名称并选择审核人')
      return
    }
  }
  saving.value = true
  try {
    await props.api.saveFlow(steps.value.map((s) => ({ slot: s.slot, name: s.name.trim() })))
    ElMessage.success('审批流程已保存')
    fetchFlow()
  } finally {
    saving.value = false
  }
}

onMounted(fetchFlow)
</script>

<style scoped lang="scss">
.flow-editor {
  margin-top: $spacing-lg;
  border-top: 1px dashed $color-border;
  padding-top: $spacing-lg;
}
.flow-head {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  margin-bottom: $spacing-sm;
}
.flow-title {
  font-weight: 600;
  color: $color-text-primary;
}
.flow-tip {
  margin-bottom: $spacing-md;
}
.flow-step {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  margin-bottom: $spacing-sm;
}
.step-no {
  width: 40rpx;
  min-width: 22px;
  height: 22px;
  line-height: 22px;
  text-align: center;
  border-radius: 50%;
  background: $color-primary-light;
  color: $color-primary;
  font-size: 12px;
}
.step-name {
  width: 220px;
}
.step-slot {
  flex: 1;
}
.flow-footer {
  display: flex;
  justify-content: space-between;
  margin-top: $spacing-sm;
}
</style>
