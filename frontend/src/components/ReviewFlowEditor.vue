<!-- 审批流程设计器（钉钉/飞书式约束结构布局，纯 DOM 垂直链）：开始 → 环节卡片 → … → 结束，连线上「⊕」插入环节。
     AI 环节卡片下半部分为三分支泳道（无异常/有异常/存疑失败），点泳道弹出去向配置（finish/next/reject/goto:N，仅可向后跳到人工环节）；
     goto 泳道 hover 高亮目标环节卡。环节分人工（引用职责槽位）与 AI 审核闸门；报告签字链不支持 AI 环节与跳转，仅 name+slot+mode。
     租户级 / 平台模板 / 项目级三处共用。 -->
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
      <div v-if="canEdit" class="flow-toolbar">
        <el-dropdown v-if="allowAi" trigger="click" @command="(c: string) => insertStep(steps.length, c as 'manual' | 'ai')">
          <el-button type="primary" size="small">
            <el-icon class="btn-icon"><Plus /></el-icon>添加环节
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="manual"><el-icon><User /></el-icon>人工审核</el-dropdown-item>
              <el-dropdown-item command="ai"><el-icon><Cpu /></el-icon>AI 审核</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button v-else type="primary" size="small" @click="insertStep(steps.length, 'manual')">
          <el-icon class="btn-icon"><Plus /></el-icon>添加环节
        </el-button>
      </div>

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

        <div
          class="node-card"
          :class="{ 'ai-card': step.kind === 'ai', clickable: canEdit, 'goto-highlight': hoverGotoTarget === idx }"
          @click="canEdit && openEdit(idx)"
        >
          <div class="card-head" :class="{ 'ai-head': step.kind === 'ai' }">
            <el-icon v-if="step.kind === 'ai'" class="head-icon"><Cpu /></el-icon>
            <el-icon v-else class="head-icon"><User /></el-icon>
            <span class="card-title">{{ step.name || (step.kind === 'ai' ? 'AI 审核' : '未命名环节') }}</span>
            <span class="card-seq">环节{{ idx + 1 }}</span>
            <template v-if="canEdit">
              <el-icon class="head-op" :class="{ disabled: idx === 0 }" title="上移" @click.stop="moveStep(idx, -1)"><Top /></el-icon>
              <el-icon class="head-op" :class="{ disabled: idx === steps.length - 1 }" title="下移" @click.stop="moveStep(idx, 1)"><Bottom /></el-icon>
              <el-icon class="head-op danger" title="删除" @click.stop="removeStep(idx)"><Delete /></el-icon>
            </template>
          </div>

          <!-- 人工环节：岗位名（报告链带签字方式 tag） -->
          <div v-if="step.kind !== 'ai'" class="card-body">
            <div class="human-line">
              <span>{{ slotLabel(step.slot) }}</span>
              <el-tag v-if="kind === 'report'" size="small" effect="plain">{{ step.mode === 'all' ? '全部人' : '任一人' }}</el-tag>
            </div>
          </div>

          <!-- AI 环节：三分支泳道，点击配置去向 -->
          <div v-else class="branch-lanes" @click.stop>
            <el-popover
              v-for="b in aiSummary(step, idx)"
              :key="b.field"
              trigger="click"
              placement="bottom"
              :width="260"
              :disabled="!canEdit"
            >
              <template #reference>
                <div
                  class="lane"
                  :class="{ clickable: canEdit }"
                  @mouseenter="onLaneEnter(b)"
                  @mouseleave="onLaneLeave"
                >
                  <div class="lane-head" :style="{ background: b.bg, color: b.color }">{{ b.text }}</div>
                  <div class="lane-body" :class="{ invalid: b.invalid }">
                    {{ b.target }}<template v-if="b.invalid">（目标失效，请修正）</template>
                  </div>
                  <div v-if="canEdit" class="lane-foot">
                    <el-icon :size="11"><EditPen /></el-icon>点击配置
                  </div>
                </div>
              </template>
              <div class="lane-pop">
                <div class="lane-pop-title" :style="{ color: b.color }">{{ b.text }}去向</div>
                <el-select
                  :model-value="step[b.field] || b.fallback"
                  style="width: 100%"
                  @update:model-value="(v: string) => (step[b.field] = v)"
                >
                  <el-option v-for="o in branchOptionsFor(idx, b.field)" :key="o.value" :label="o.label" :value="o.value" />
                </el-select>
                <div class="lane-pop-hint">只能跳到本环节之后的人工环节；「无异常」不允许直接打回</div>
              </div>
            </el-popover>
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
import { Top, Bottom, Delete, Plus, User, Cpu, EditPen } from '@element-plus/icons-vue'
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
  const lane = canEdit.value ? '；点击 AI 卡片上的分支泳道可直接配置该分支去向' : ''
  if (kind.value === 'maint') {
    return '维保登记按环节顺序逐级审核：空流程 = 登记即生效；AI 环节 = 标签核对结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员' + lane + '。'
  }
  return '打卡记录按环节顺序逐级审核：当前环节名单成员通过后进入下一环节，末环节通过才生效；驳回即打回。空流程 = 打卡默认通过；AI 环节 = 按结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员' + lane + '。'
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

// ===== 分支定义与 goto 路由合法性（N 为 1 起序号，只能指向后面的人工环节） =====
type BranchField = 'on_pass' | 'on_abnormal' | 'on_review'
const BRANCHES: { field: BranchField; text: string; color: string; bg: string; fallback: string }[] = [
  { field: 'on_pass', text: '无异常', color: '#2ba471', bg: '#e7f6ef', fallback: 'finish' },
  { field: 'on_abnormal', text: '有异常', color: '#d54941', bg: '#fdecec', fallback: 'finish' },
  { field: 'on_review', text: '存疑失败', color: '#ed7b2f', bg: '#fdf3e7', fallback: 'next' }
]

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
    const raw = v.slice(5)
    if (raw.startsWith('!')) return '目标环节已删除'
    const n = Number(raw)
    const t = steps.value[n - 1]
    return t ? `跳到「${t.name || `环节${n}`}」` : '目标环节不存在'
  }
  return v
}

function aiSummary(step: ReviewFlowStep, idx: number) {
  return BRANCHES.map((b) => {
    const route = step[b.field] || b.fallback
    // 合法 goto 的目标环节下标（0 起），用于泳道 hover 高亮目标卡片
    let gotoIdx = -1
    if (route.startsWith('goto:')) {
      const n = Number(route.slice(5))
      if (Number.isInteger(n) && n > idx + 1 && n <= steps.value.length && steps.value[n - 1].kind !== 'ai') gotoIdx = n - 1
    }
    return {
      field: b.field,
      text: b.text,
      color: b.color,
      bg: b.bg,
      fallback: b.fallback,
      target: routeLabel(step[b.field], b.fallback),
      invalid: !routeValid(idx, step[b.field], b.field !== 'on_pass'),
      gotoIdx
    }
  })
}

// goto 泳道 hover 高亮目标环节卡
const hoverGotoTarget = ref(-1)
function onLaneEnter(b: { gotoIdx: number }) {
  hoverGotoTarget.value = b.gotoIdx
}
function onLaneLeave() {
  hoverGotoTarget.value = -1
}

// ===== goto 序号重映射：环节增删移后按旧序号 → 新序号改写；目标被删除置为失效标记 goto:!N（标红待修正） =====
function remapGoto(map: (oldIdx: number) => number | null) {
  for (const s of steps.value) {
    if (s.kind !== 'ai') continue
    for (const f of ['on_pass', 'on_abnormal', 'on_review'] as const) {
      const v = s[f]
      if (!v || !v.startsWith('goto:') || v.slice(5).startsWith('!')) continue
      const n = Number(v.slice(5))
      if (!Number.isInteger(n)) continue
      const t = map(n - 1)
      s[f] = t === null ? `goto:!${n}` : `goto:${t + 1}`
    }
  }
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
  remapGoto((old) => (old < at ? old : old + 1))
  openEdit(at)
}

function moveStep(idx: number, dir: number) {
  const j = idx + dir
  if (j < 0 || j >= steps.value.length) return
  const arr = steps.value
  const tmp = arr[idx]
  arr[idx] = arr[j]
  arr[j] = tmp
  remapGoto((old) => (old === idx ? j : old === j ? idx : old))
}

function removeStep(idx: number) {
  steps.value.splice(idx, 1)
  remapGoto((old) => (old === idx ? null : old > idx ? old - 1 : old))
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

// 分支去向选项：泳道 popover 与抽屉共用（校验规则：on_pass 不允许 reject；goto 只能向后跳到人工环节）
function branchOptionsFor(idx: number, field: BranchField) {
  const opts: { value: string; label: string }[] = []
  if (field === 'on_review') opts.push({ value: 'next', label: '进入下一环节（默认）' })
  else opts.push({ value: 'finish', label: '直接生效（默认）' })
  if (field !== 'on_review') opts.push({ value: 'next', label: '进入下一环节' })
  if (field !== 'on_pass') opts.push({ value: 'reject', label: '直接打回' })
  for (let j = idx + 1; j < steps.value.length; j++) {
    const t = steps.value[j]
    if (t.kind === 'ai') continue
    opts.push({ value: `goto:${j + 1}`, label: `跳到环节${j + 1}：${t.name || '未命名环节'}` })
  }
  return opts
}

function branchOptions(field: BranchField) {
  return branchOptionsFor(editingIdx.value, field)
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
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: $spacing-xxl $spacing-md $spacing-lg;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  background: $color-bg-page;
}
.flow-toolbar {
  position: absolute;
  top: $spacing-md;
  right: $spacing-md;
}
.btn-icon {
  margin-right: 4px;
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
  box-shadow: 0 1px 4px rgba(31, 35, 41, 0.15);
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
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
  &.clickable {
    cursor: pointer;
    &:hover {
      border-color: $color-primary;
    }
  }
  &.ai-card {
    width: 440px;
    border-color: $color-primary;
    &.clickable:hover {
      box-shadow: 0 2px 8px rgba(43, 90, 237, 0.18);
    }
  }
  &.goto-highlight {
    border-color: $color-primary;
    box-shadow: 0 0 0 2px rgba(43, 90, 237, 0.35);
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
.card-seq {
  flex-shrink: 0;
  font-size: 11px;
  color: $color-text-placeholder;
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
  min-height: 22px;
}
// AI 分支泳道：卡片下半部分，三列等宽
.branch-lanes {
  display: flex;
  gap: $spacing-sm;
  padding: $spacing-sm;
  cursor: default;
}
.lane {
  flex: 1;
  min-width: 0;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  background: $color-table-header;
  overflow: hidden;
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
  &.clickable {
    cursor: pointer;
    &:hover {
      border-color: $color-primary;
      box-shadow: 0 1px 4px rgba(43, 90, 237, 0.15);
    }
  }
}
.lane-head {
  padding: 3px $spacing-sm;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
}
.lane-body {
  padding: $spacing-sm;
  font-size: 12px;
  line-height: 18px;
  color: $color-text-regular;
  text-align: center;
  word-break: break-all;
  &.invalid {
    color: $color-danger;
  }
}
.lane-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding-bottom: $spacing-xs;
  font-size: 11px;
  color: $color-text-placeholder;
  .clickable:hover & {
    color: $color-primary;
  }
}
.lane-pop-title {
  margin-bottom: $spacing-sm;
  font-size: 13px;
  font-weight: 600;
}
.lane-pop-hint {
  margin-top: $spacing-sm;
  font-size: 11px;
  color: $color-text-placeholder;
  line-height: 16px;
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
