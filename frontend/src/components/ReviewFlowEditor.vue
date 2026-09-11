<!-- 审核流程节点链设计器（类钉钉审批流）：开始 → 环节卡片 → … → 结束，连线上「+」插入环节。
     环节分人工（引用职责槽位）与 AI 审核闸门（按结果分流：finish/next/reject/goto:N，仅可向后跳到人工环节）；
     报告签字链不支持 AI 环节与跳转，仅 name+slot+mode。租户级 / 平台模板 / 项目级三处共用。 -->
<template>
  <div class="flow-editor">
    <div class="flow-head" :class="{ 'only-badge': hideTitle }">
      <span v-if="!hideTitle" class="flow-title">{{ title }}</span>
      <el-tag v-if="sourceLabel" size="small" :type="source === 'platform' || source === 'tenant' ? 'warning' : source === 'project' ? 'success' : 'info'" effect="plain">
        {{ sourceLabel }}
      </el-tag>
    </div>
    <el-alert type="info" :closable="false" :title="tip" class="flow-tip" />
    <div v-loading="loading" class="flow-canvas">
      <div class="flow-node start-node">开始</div>

      <template v-for="(step, idx) in steps" :key="idx">
        <div class="flow-join">
          <div class="join-line" />
          <el-dropdown v-if="canEdit" trigger="click" @command="(c: string) => insertStep(idx, c as 'manual' | 'ai')">
            <span class="join-add" title="添加环节"><el-icon :size="12"><Plus /></el-icon></span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="manual"><el-icon><User /></el-icon>人工审核</el-dropdown-item>
                <el-dropdown-item v-if="allowAi" command="ai"><el-icon><Cpu /></el-icon>AI 审核</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>

        <div class="node-card" :class="{ 'ai-card': step.kind === 'ai', clickable: canEdit }" @click="canEdit && openEdit(idx)">
          <div class="card-head" :class="{ 'ai-head': step.kind === 'ai' }">
            <el-icon v-if="step.kind === 'ai'" class="head-icon"><Cpu /></el-icon>
            <el-icon v-else class="head-icon"><User /></el-icon>
            <span class="card-title">{{ step.name || (step.kind === 'ai' ? 'AI 审核' : '未命名环节') }}</span>
            <template v-if="canEdit">
              <el-icon class="head-op" :class="{ disabled: idx === 0 }" title="上移" @click.stop="moveStep(idx, -1)"><Top /></el-icon>
              <el-icon class="head-op" :class="{ disabled: idx === steps.length - 1 }" title="下移" @click.stop="moveStep(idx, 1)"><Bottom /></el-icon>
              <el-icon class="head-op danger" title="删除" @click.stop="removeStep(idx)"><Delete /></el-icon>
            </template>
          </div>
          <div class="card-body">
            <template v-if="step.kind === 'ai'">
              <div v-for="b in aiSummary(step, idx)" :key="b.label" class="branch-line" :class="{ invalid: b.invalid }">
                {{ b.label }} → {{ b.target }}<template v-if="b.invalid">（目标失效，请修正）</template>
              </div>
            </template>
            <div v-else class="human-line">
              <span>{{ slotLabel(step.slot) }}</span>
              <el-tag v-if="kind === 'report'" size="small" effect="plain">{{ step.mode === 'all' ? '全部人' : '任一人' }}</el-tag>
            </div>
          </div>
        </div>
      </template>

      <div v-if="!steps.length" class="flow-empty">{{ emptyText }}</div>

      <div class="flow-join">
        <div class="join-line" />
        <el-dropdown v-if="canEdit" trigger="click" @command="(c: string) => insertStep(steps.length, c as 'manual' | 'ai')">
          <span class="join-add" title="添加环节"><el-icon :size="12"><Plus /></el-icon></span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="manual"><el-icon><User /></el-icon>人工审核</el-dropdown-item>
              <el-dropdown-item v-if="allowAi" command="ai"><el-icon><Cpu /></el-icon>AI 审核</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <div class="flow-node end-node">结束</div>
    </div>

    <div class="flow-footer">
      <span />
      <el-button v-if="canEdit" type="primary" size="small" :loading="saving" @click="handleSave">保存审批流程</el-button>
    </div>

    <el-drawer v-model="drawerVisible" :title="editingStep?.kind === 'ai' ? 'AI 审核环节' : '人工审核环节'" size="400px">
      <template v-if="editingStep">
        <el-form label-position="top">
          <el-form-item label="环节名称">
            <el-input v-model="editingStep.name" maxlength="32" :placeholder="editingStep.kind === 'ai' ? '留空默认为「AI 审核」' : '如：主管审核'" />
          </el-form-item>
          <template v-if="editingStep.kind !== 'ai'">
            <el-form-item label="审核人（按岗位）">
              <el-select v-model="editingStep.slot" placeholder="选择职责槽位" style="width: 100%">
                <el-option v-for="s in slotOptions" :key="s.slot" :label="s.name" :value="s.slot" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="kind === 'report'" label="签字方式">
              <el-radio-group v-model="editingStep.mode">
                <el-radio value="any">任一人</el-radio>
                <el-radio value="all">全部人</el-radio>
              </el-radio-group>
            </el-form-item>
          </template>
          <template v-else>
            <el-form-item v-for="br in branchDefs" :key="br.field" :label="br.title">
              <el-select v-model="editingStep[br.field]" style="width: 100%">
                <el-option v-for="o in branchOptions(br.field)" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </el-form-item>
            <el-alert
              v-if="stepHasInvalidRoute(editingIdx)"
              type="error"
              :closable="false"
              title="存在非法的跳转目标：只能跳到本环节之后的人工环节，请修正后再保存"
            />
          </template>
        </el-form>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Top, Bottom, Delete, Plus, User, Cpu } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import type { ReviewFlowStep, ReviewFlowView } from '@/api/post'

const props = defineProps<{
  api: {
    listFlow: () => Promise<ReviewFlowView>
    saveFlow: (steps: ReviewFlowStep[]) => Promise<unknown>
  }
  slotOptions: { slot: string; name: string }[]
  savePerm: string
  kind?: 'checkin' | 'report' | 'maint'
  /** 选项卡内使用时隐藏标题（标签页已承担命名），仅保留来源徽标 */
  hideTitle?: boolean
}>()

const userStore = useUserStore()
const canEdit = computed(() => userStore.hasPerm(props.savePerm))
const kind = computed(() => props.kind || 'checkin')
const allowAi = computed(() => kind.value !== 'report')
const MAX_STEPS = 5

const title = computed(() => (kind.value === 'report' ? '报告签字流程' : kind.value === 'maint' ? '维保审核流程' : '打卡审批流程'))
const tip = computed(() => {
  if (kind.value === 'report') {
    return '报告生成后按环节顺序签字审核；每个环节可要求任一人或全部人员完成。保存空流程表示报告生成后直接归档。'
  }
  if (kind.value === 'maint') {
    return '维保登记按环节顺序逐级审核：空流程 = 登记即生效；AI 环节 = 标签核对结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员。'
  }
  return '打卡记录按环节顺序逐级审核：当前环节名单成员通过后进入下一环节，末环节通过才生效；驳回即打回。空流程 = 打卡默认通过；AI 环节 = 按结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员。'
})
const emptyText = computed(() =>
  kind.value === 'report' ? '未配置环节 —— 报告生成后将直接归档' : kind.value === 'maint' ? '未配置环节 —— 维保登记将登记即生效' : '未配置环节 —— 打卡记录将默认通过'
)
const emptySaveConfirm = computed(() =>
  kind.value === 'report'
    ? '当前流程为空，保存后报告生成后将直接归档，确定保存吗？'
    : kind.value === 'maint'
      ? '当前流程为空，保存后维保登记将登记即生效，确定保存吗？'
      : '当前流程为空，保存后打卡记录将默认通过，确定保存吗？'
)

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
    steps.value = data.steps.map((s) =>
      s.kind === 'ai'
        ? { slot: '', name: s.name, kind: 'ai' as const, on_pass: s.on_pass || 'finish', on_abnormal: s.on_abnormal || 'finish', on_review: s.on_review || 'next' }
        : { ...s }
    )
    source.value = data.source
  } finally {
    loading.value = false
  }
}

function slotLabel(slot: string) {
  return props.slotOptions.find((s) => s.slot === slot)?.name || (slot ? `未知槽位（${slot}）` : '未选择审核人')
}

// ===== goto 路由合法性（N 为 1 起序号，只能指向后面的人工环节） =====
function routeValid(idx: number, route: string | undefined, allowReject: boolean): boolean {
  if (!route) return true
  if (route.startsWith('goto:')) {
    const n = Number(route.slice(5))
    if (!Number.isInteger(n) || n <= idx + 1 || n > steps.value.length) return false
    return steps.value[n - 1].kind !== 'ai'
  }
  if (route === 'finish' || route === 'next') return true
  if (route === 'reject') return allowReject
  return false
}

function stepHasInvalidRoute(idx: number): boolean {
  const s = steps.value[idx]
  if (!s || s.kind !== 'ai') return false
  return !routeValid(idx, s.on_pass, false) || !routeValid(idx, s.on_abnormal, true) || !routeValid(idx, s.on_review, true)
}

function routeLabel(route: string | undefined, fallback: string): string {
  const v = route || fallback
  if (v === 'finish') return '直接生效'
  if (v === 'next') return '进入下一环节'
  if (v === 'reject') return '直接打回'
  if (v.startsWith('goto:')) {
    const n = Number(v.slice(5))
    const t = steps.value[n - 1]
    return t ? `跳到「${t.name || `环节${n}`}」` : '目标环节不存在'
  }
  return v
}

function aiSummary(step: ReviewFlowStep, idx: number) {
  return [
    { label: '无异常', target: routeLabel(step.on_pass, 'finish'), invalid: !routeValid(idx, step.on_pass, false) },
    { label: '有异常', target: routeLabel(step.on_abnormal, 'finish'), invalid: !routeValid(idx, step.on_abnormal, true) },
    { label: '存疑/失败', target: routeLabel(step.on_review, 'next'), invalid: !routeValid(idx, step.on_review, true) }
  ]
}

// ===== 节点增删移 =====
function insertStep(at: number, type: 'manual' | 'ai') {
  if (steps.value.length >= MAX_STEPS) {
    ElMessage.warning(`审批链最多 ${MAX_STEPS} 个环节`)
    return
  }
  const step: ReviewFlowStep =
    type === 'ai'
      ? { slot: '', name: '', kind: 'ai', on_pass: 'finish', on_abnormal: 'finish', on_review: 'next' }
      : { slot: '', name: '', ...(kind.value === 'report' ? { mode: 'any' as const } : {}) }
  steps.value.splice(at, 0, step)
  openEdit(at)
}

function moveStep(idx: number, dir: number) {
  const j = idx + dir
  if (j < 0 || j >= steps.value.length) return
  const arr = steps.value
  const tmp = arr[idx]
  arr[idx] = arr[j]
  arr[j] = tmp
}

function removeStep(idx: number) {
  steps.value.splice(idx, 1)
}

// ===== 编辑抽屉 =====
const editingIdx = ref(-1)
const editingStep = computed(() => (editingIdx.value >= 0 ? steps.value[editingIdx.value] : null))
const drawerVisible = computed({
  get: () => editingIdx.value >= 0,
  set: (v) => {
    if (!v) editingIdx.value = -1
  }
})

function openEdit(idx: number) {
  editingIdx.value = idx
}

const branchDefs = [
  { field: 'on_pass' as const, title: '无异常时' },
  { field: 'on_abnormal' as const, title: '有异常时' },
  { field: 'on_review' as const, title: '存疑 / 审核失败时' }
]

function branchOptions(field: 'on_pass' | 'on_abnormal' | 'on_review') {
  const opts: { value: string; label: string }[] = []
  if (field === 'on_review') opts.push({ value: 'next', label: '进入下一环节（默认）' })
  else opts.push({ value: 'finish', label: '直接生效（默认）' })
  if (field !== 'on_review') opts.push({ value: 'next', label: '进入下一环节' })
  if (field !== 'on_pass') opts.push({ value: 'reject', label: '直接打回' })
  for (let j = editingIdx.value + 1; j < steps.value.length; j++) {
    const t = steps.value[j]
    if (t.kind === 'ai') continue
    opts.push({ value: `goto:${j + 1}`, label: `跳到环节${j + 1}：${t.name || '未命名环节'}` })
  }
  return opts
}

// ===== 保存 =====
function serializeStep(s: ReviewFlowStep): ReviewFlowStep {
  if (s.kind === 'ai') {
    return {
      slot: '',
      name: s.name.trim() || 'AI 审核',
      kind: 'ai',
      on_pass: s.on_pass || 'finish',
      on_abnormal: s.on_abnormal || 'finish',
      on_review: s.on_review || 'next'
    }
  }
  const base: ReviewFlowStep = { slot: s.slot, name: s.name.trim() }
  if (kind.value === 'report') base.mode = s.mode || 'any'
  return base
}

async function handleSave() {
  if (!steps.value.length) {
    const ok = await ElMessageBox.confirm(emptySaveConfirm.value, '保存空流程', {
      confirmButtonText: '保存',
      cancelButtonText: '取消',
      type: 'warning'
    })
      .then(() => true)
      .catch(() => false)
    if (!ok) return
  }
  for (let i = 0; i < steps.value.length; i++) {
    const s = steps.value[i]
    if (s.kind === 'ai') {
      if (!s.name.trim()) s.name = 'AI 审核'
      if (stepHasInvalidRoute(i)) {
        ElMessage.warning(`第 ${i + 1} 个环节「${s.name}」存在非法的跳转目标：只能跳到之后的人工环节`)
        return
      }
    } else if (!s.name.trim() || !s.slot) {
      ElMessage.warning(`第 ${i + 1} 个人工环节须填写名称并选择审核人`)
      return
    }
  }
  saving.value = true
  try {
    await props.api.saveFlow(steps.value.map(serializeStep))
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
.flow-head.only-badge {
  justify-content: flex-end;
}
.flow-title {
  font-weight: 600;
  color: $color-text-primary;
}
.flow-tip {
  margin-bottom: $spacing-md;
}
.flow-canvas {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: $spacing-sm 0;
}
.flow-node {
  padding: 4px 28px;
  border-radius: 16px;
  font-size: 13px;
  color: $color-white;
  line-height: 20px;
}
.start-node {
  background: $color-primary;
}
.end-node {
  background: $color-text-secondary;
}
.flow-join {
  position: relative;
  display: flex;
  justify-content: center;
  width: 100%;
  height: 32px;
}
.join-line {
  width: 2px;
  height: 100%;
  background: $color-border-dark;
}
.join-add {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: $color-white;
  border: 1px solid $color-primary;
  color: $color-primary;
  cursor: pointer;
  &:hover {
    background: $color-primary;
    color: $color-white;
  }
}
.node-card {
  width: 300px;
  border: 1px solid $color-border;
  border-radius: 8px;
  background: $color-bg-card;
  box-shadow: 0 1px 4px rgba(31, 35, 41, 0.06);
  overflow: hidden;
  &.clickable {
    cursor: pointer;
    &:hover {
      border-color: $color-primary;
    }
  }
  &.ai-card {
    border-color: $color-primary;
    &.clickable:hover {
      box-shadow: 0 2px 8px rgba(43, 90, 237, 0.18);
    }
  }
}
.card-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px $spacing-sm;
  background: $color-table-header;
  border-bottom: 1px solid $color-border;
  &.ai-head {
    background: $color-primary-light;
    color: $color-primary;
  }
}
.head-icon {
  font-size: 14px;
  color: $color-text-secondary;
  .ai-head & {
    color: $color-primary;
  }
}
.card-title {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: $color-text-primary;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  .ai-head & {
    color: $color-primary;
  }
}
.head-op {
  font-size: 13px;
  color: $color-text-secondary;
  cursor: pointer;
  &:hover {
    color: $color-primary;
  }
  &.danger:hover {
    color: $color-danger;
  }
  &.disabled {
    color: $color-text-placeholder;
    cursor: not-allowed;
  }
}
.card-body {
  padding: $spacing-sm $spacing-md;
  font-size: 12px;
  color: $color-text-regular;
}
.human-line {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
}
.branch-line {
  line-height: 20px;
  &.invalid {
    color: $color-danger;
  }
}
.flow-empty {
  margin: $spacing-sm 0;
  font-size: 12px;
  color: $color-text-secondary;
}
.flow-footer {
  display: flex;
  justify-content: space-between;
  margin-top: $spacing-sm;
}
</style>
