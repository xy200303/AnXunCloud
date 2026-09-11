<!-- 审批流程图设计器（Vue Flow + dagre 自动布局）：开始 → 环节节点 → 结束，AI 环节三分支语义色连线，
     主链边上「+」插入环节；AI 环节右侧彩色锚点支持拖线配置分支去向（goto:N/finish/reject，仅可向后跳到人工环节）。
     环节分人工（引用职责槽位）与 AI 审核闸门；报告签字链不支持 AI 环节与跳转，仅 name+slot+mode。租户级 / 平台模板 / 项目级三处共用。 -->
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
      <VueFlow
        :nodes="nodes"
        :edges="edges"
        :fit-view-on-init="true"
        :min-zoom="0.2"
        :max-zoom="1.5"
        :nodes-draggable="true"
        :nodes-connectable="canEdit"
        :edges-focusable="false"
        :delete-key-code="null"
        :is-valid-connection="isValidConnection"
        @connect="onConnect"
        @connect-start="onConnectStart"
        @connect-end="onConnectEnd"
      >
        <Background :gap="18" pattern-color="#dcdfe6" />
        <Controls :show-interactive="false" />

        <template #node-start>
          <div class="vf-pill start-pill" :class="{ 'connect-blocked': isBlockedTarget('start') }">开始</div>
          <Handle id="chain-s" type="source" :position="Position.Bottom" class="ghost-handle" :connectable="false" />
        </template>

        <template #node-step="{ data }">
          <div
            v-if="steps[data.idx]"
            class="node-card"
            :class="{ 'ai-card': steps[data.idx].kind === 'ai', clickable: canEdit, 'connect-blocked': isBlockedTarget(`step-${data.idx}`) }"
            @click="canEdit && openEdit(data.idx)"
          >
            <div class="card-head" :class="{ 'ai-head': steps[data.idx].kind === 'ai' }">
              <el-icon v-if="steps[data.idx].kind === 'ai'" class="head-icon"><Cpu /></el-icon>
              <el-icon v-else class="head-icon"><User /></el-icon>
              <span class="card-title">{{ steps[data.idx].name || (steps[data.idx].kind === 'ai' ? 'AI 审核' : '未命名环节') }}</span>
              <span class="card-seq">环节{{ data.idx + 1 }}</span>
              <template v-if="canEdit">
                <el-icon class="head-op" :class="{ disabled: data.idx === 0 }" title="上移" @click.stop="moveStep(data.idx, -1)"><Top /></el-icon>
                <el-icon class="head-op" :class="{ disabled: data.idx === steps.length - 1 }" title="下移" @click.stop="moveStep(data.idx, 1)"><Bottom /></el-icon>
                <el-icon class="head-op danger" title="删除" @click.stop="removeStep(data.idx)"><Delete /></el-icon>
              </template>
            </div>
            <div class="card-body">
              <template v-if="steps[data.idx].kind === 'ai'">
                <div v-for="b in aiSummary(steps[data.idx], data.idx)" :key="b.field" class="branch-line" :class="{ invalid: b.invalid }">
                  <span class="branch-dot" :style="{ background: b.color }" />
                  <span class="branch-text">{{ b.label }} → {{ b.target }}<template v-if="b.invalid">（目标失效，请修正）</template></span>
                  <Handle
                    :id="b.field"
                    type="source"
                    :position="Position.Right"
                    class="branch-source nodrag"
                    :style="{ background: b.color }"
                    :connectable="canEdit"
                    :title="`拖线设置「${b.label}」去向`"
                  />
                </div>
              </template>
              <div v-else class="human-line">
                <span>{{ slotLabel(steps[data.idx].slot) }}</span>
                <el-tag v-if="kind === 'report'" size="small" effect="plain">{{ steps[data.idx].mode === 'all' ? '全部人' : '任一人' }}</el-tag>
              </div>
            </div>
            <Handle id="chain-t" type="target" :position="Position.Top" class="ghost-handle" :connectable="false" />
            <Handle id="chain-s" type="source" :position="Position.Bottom" class="ghost-handle" :connectable="false" />
            <Handle v-if="steps[data.idx].kind !== 'ai'" id="branch-t" type="target" :position="Position.Left" class="branch-target" :connectable="canEdit" />
          </div>
        </template>

        <template #node-end>
          <div class="vf-pill end-pill">结束</div>
          <Handle id="chain-t" type="target" :position="Position.Top" class="ghost-handle" :connectable="false" />
          <Handle id="branch-t" type="target" :position="Position.Left" class="branch-target" :connectable="canEdit" />
        </template>

        <template #node-reject>
          <div class="vf-pill reject-pill" :class="{ 'connect-blocked': isBlockedTarget('reject') }">打回</div>
          <Handle id="branch-t" type="target" :position="Position.Left" class="branch-target" :connectable="canEdit" />
        </template>

        <template #edge-chain="{ sourceX, sourceY, targetX, targetY, data, markerEnd }">
          <BaseEdge :path="`M ${sourceX} ${sourceY} L ${targetX} ${targetY}`" :marker-end="markerEnd" class="chain-edge" />
          <EdgeLabelRenderer v-if="canEdit">
            <div class="edge-add nodrag nopan" :style="{ transform: `translate(-50%, -50%) translate(${(sourceX + targetX) / 2}px, ${(sourceY + targetY) / 2}px)` }">
              <el-dropdown trigger="click" @command="(c: string) => insertStep(data.at, c as 'manual' | 'ai')">
                <span class="join-add" title="添加环节"><el-icon :size="12"><Plus /></el-icon></span>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="manual"><el-icon><User /></el-icon>人工审核</el-dropdown-item>
                    <el-dropdown-item v-if="allowAi" command="ai"><el-icon><Cpu /></el-icon>AI 审核</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </EdgeLabelRenderer>
        </template>

        <template #edge-branch="p">
          <BaseEdge :id="p.id" :path="branchEdgePath(p)[0]" :marker-end="p.markerEnd" :style="p.style" :class="{ 'edge-invalid': p.data.invalid }" />
          <EdgeLabelRenderer>
            <div
              class="branch-label nodrag nopan"
              :class="{ invalid: p.data.invalid }"
              :style="{ transform: `translate(-50%, -50%) translate(${branchEdgePath(p)[1]}px, ${branchEdgePath(p)[2]}px)`, borderColor: p.data.color, color: p.data.color }"
            >
              {{ p.data.text }}<template v-if="p.data.invalid">·目标失效</template>
            </div>
          </EdgeLabelRenderer>
        </template>
      </VueFlow>

      <div v-if="!steps.length" class="flow-empty">{{ emptyText }}</div>
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
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Top, Bottom, Delete, Plus, User, Cpu } from '@element-plus/icons-vue'
import { VueFlow, Handle, Position, BaseEdge, EdgeLabelRenderer, MarkerType, useVueFlow, getBezierPath } from '@vue-flow/core'
import type { Connection, Edge, EdgeProps, Node, OnConnectStartParams } from '@vue-flow/core'
import { Controls } from '@vue-flow/controls'
import { Background } from '@vue-flow/background'
import { graphlib, layout as dagreLayout } from '@dagrejs/dagre'
import { useUserStore } from '@/store/user'
import type { ReviewFlowStep, ReviewFlowView } from '@/api/post'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

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
  const drag = canEdit.value ? '；可直接从 AI 环节右侧彩色锚点拖线到后续环节 / 结束 / 打回节点配置分支去向' : ''
  if (kind.value === 'maint') {
    return '维保登记按环节顺序逐级审核：空流程 = 登记即生效；AI 环节 = 标签核对结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员' + drag + '。'
  }
  return '打卡记录按环节顺序逐级审核：当前环节名单成员通过后进入下一环节，末环节通过才生效；驳回即打回。空流程 = 打卡默认通过；AI 环节 = 按结果分流（无异常 / 有异常 / 存疑各配去向，可跳到后续人工环节）；人工环节审核人 = 负责岗位在该项目编制里的在职成员' + drag + '。'
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
const BRANCHES: { field: BranchField; text: string; color: string; fallback: string }[] = [
  { field: 'on_pass', text: '无异常', color: '#2ba471', fallback: 'finish' },
  { field: 'on_abnormal', text: '有异常', color: '#d54941', fallback: 'finish' },
  { field: 'on_review', text: '存疑失败', color: '#ed7b2f', fallback: 'next' }
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
  return BRANCHES.map((b) => ({
    field: b.field,
    color: b.color,
    label: b.text,
    target: routeLabel(step[b.field], b.fallback),
    invalid: !routeValid(idx, step[b.field], b.field !== 'on_pass')
  }))
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

// ===== 画布：dagre 分层自动布局（TB），steps 变化即重算；用户可临时拖动节点，重算后回弹 =====
const NODE_W = 280
const PILL_W = 96
const PILL_H = 34
const MANUAL_H = 92
const AI_H = 156

const { fitView } = useVueFlow()
const nodes = ref<Node[]>([])

const rejectUsed = computed(() => steps.value.some((s) => s.kind === 'ai' && (s.on_pass === 'reject' || s.on_abnormal === 'reject' || s.on_review === 'reject')))
// 打回节点：可编辑时恒显示（拖线目标）；只读时仅在有 reject 分支时显示
const showRejectNode = computed(() => canEdit.value || rejectUsed.value)

function stepNodeId(idx: number) {
  return `step-${idx}`
}

function branchTargetNode(idx: number, route: string | undefined, fallback: string): string {
  const v = route || fallback
  if (v === 'finish') return 'end'
  if (v === 'reject') return 'reject'
  if (v === 'next') return idx + 1 < steps.value.length ? stepNodeId(idx + 1) : 'end'
  if (v.startsWith('goto:')) {
    const n = Number(v.slice(5))
    if (Number.isInteger(n) && n >= 1 && n <= steps.value.length) return stepNodeId(n - 1)
    return 'end' // 失效跳转：落到结束节点并标红提示
  }
  return 'end'
}

function buildEdges(): Edge[] {
  const list: Edge[] = []
  const chainIds = ['start', ...steps.value.map((_, i) => stepNodeId(i)), 'end']
  for (let k = 0; k < chainIds.length - 1; k++) {
    list.push({
      id: `chain-${k}`,
      source: chainIds[k],
      target: chainIds[k + 1],
      sourceHandle: 'chain-s',
      targetHandle: 'chain-t',
      type: 'chain',
      selectable: false,
      focusable: false,
      data: { at: k },
      style: { stroke: '#c9cdd4', strokeWidth: 1.5 },
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: '#c9cdd4' }
    })
  }
  steps.value.forEach((s, i) => {
    if (s.kind !== 'ai') return
    for (const b of BRANCHES) {
      const target = branchTargetNode(i, s[b.field], b.fallback)
      if (target === 'reject' && !showRejectNode.value) continue
      const invalid = !routeValid(i, s[b.field], b.field !== 'on_pass')
      list.push({
        id: `branch-${i}-${b.field}`,
        source: stepNodeId(i),
        sourceHandle: b.field,
        target,
        targetHandle: 'branch-t',
        type: 'branch',
        selectable: false,
        focusable: false,
        data: { text: b.text, color: b.color, invalid },
        style: { stroke: invalid ? '#d54941' : b.color, strokeWidth: 2, ...(invalid ? { strokeDasharray: '5 4' } : {}) },
        markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16, color: b.color }
      })
    }
  })
  return list
}

const edges = computed<Edge[]>(() => buildEdges())

function branchEdgePath(p: EdgeProps) {
  return getBezierPath({
    sourceX: p.sourceX,
    sourceY: p.sourceY,
    sourcePosition: p.sourcePosition,
    targetX: p.targetX,
    targetY: p.targetY,
    targetPosition: p.targetPosition
  })
}

function rebuild() {
  const g = new graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'TB', nodesep: 56, ranksep: 84, marginx: 24, marginy: 24 })

  const dims: Record<string, { w: number; h: number }> = {
    start: { w: PILL_W, h: PILL_H },
    end: { w: PILL_W, h: PILL_H }
  }
  steps.value.forEach((s, i) => {
    dims[stepNodeId(i)] = { w: NODE_W, h: s.kind === 'ai' ? AI_H : MANUAL_H }
  })
  if (showRejectNode.value) dims.reject = { w: PILL_W, h: PILL_H }
  for (const [id, d] of Object.entries(dims)) g.setNode(id, { width: d.w, height: d.h })
  for (const e of buildEdges()) g.setEdge(e.source, e.target)
  dagreLayout(g)

  const pos = (id: string) => {
    const n = g.node(id)
    const d = dims[id]
    return { x: n.x - d.w / 2, y: n.y - d.h / 2 }
  }

  const list: Node[] = [
    { id: 'start', type: 'start', position: pos('start'), data: {}, selectable: false, connectable: false },
    ...steps.value.map<Node>((_, i) => ({
      id: stepNodeId(i),
      type: 'step',
      position: pos(stepNodeId(i)),
      data: { idx: i },
      selectable: false,
      connectable: canEdit.value
    })),
    { id: 'end', type: 'end', position: pos('end'), data: {}, selectable: false, connectable: canEdit.value }
  ]
  if (showRejectNode.value) {
    let rejectPos = pos('reject')
    if (!rejectUsed.value) {
      // 尚无分支指向打回节点：不参与布局约束，固定放到结束节点右侧
      const endN = g.node('end')
      rejectPos = { x: endN.x + PILL_W / 2 + 120, y: endN.y - PILL_H / 2 }
    }
    list.push({ id: 'reject', type: 'reject', position: rejectPos, data: {}, selectable: false, connectable: canEdit.value })
  }
  nodes.value = list
  nextTick(() => fitView({ padding: 0.2 }))
}

watch(steps, rebuild, { deep: true })
watch(canEdit, rebuild)

// ===== 拖线配置分支去向：AI 环节右侧三个彩色锚点 → 目标节点 =====
const connectSource = ref<{ idx: number; field: BranchField } | null>(null)

function onConnectStart(p: OnConnectStartParams) {
  if (p.handleType === 'source' && p.nodeId?.startsWith('step-') && (p.handleId || '').startsWith('on_')) {
    connectSource.value = { idx: Number(p.nodeId.slice(5)), field: p.handleId as BranchField }
  }
}

function onConnectEnd() {
  connectSource.value = null
}

function isValidConnection(conn: Connection): boolean {
  if (!canEdit.value) return false
  const field = conn.sourceHandle as BranchField | null
  if (!conn.source?.startsWith('step-') || !field || !(field + '').startsWith('on_')) return false
  if (conn.targetHandle !== 'branch-t') return false
  const i = Number(conn.source.slice(5))
  const t = conn.target
  if (t === 'end') return true
  if (t === 'reject') return field !== 'on_pass'
  if (t?.startsWith('step-')) {
    const j = Number(t.slice(5))
    return j > i && steps.value[j]?.kind !== 'ai'
  }
  return false
}

function denyReason(conn: Connection): string {
  const field = conn.sourceHandle
  const t = conn.target
  if (t === 'reject' && field === 'on_pass') return '「无异常」分支不允许直接打回'
  if (t?.startsWith('step-') && conn.source?.startsWith('step-')) {
    const i = Number(conn.source.slice(5))
    const j = Number(t.slice(5))
    if (j <= i) return '只能连接到本环节之后的环节'
    if (steps.value[j]?.kind === 'ai') return '分支去向不能是 AI 环节，请选择人工环节'
  }
  return '无法连接到该节点'
}

function onConnect(conn: Connection) {
  if (!isValidConnection(conn)) {
    ElMessage.warning(denyReason(conn))
    return
  }
  const idx = Number(conn.source.slice(5))
  const field = conn.sourceHandle as BranchField
  let v: string
  if (conn.target === 'end') v = 'finish'
  else if (conn.target === 'reject') v = 'reject'
  else {
    const j = Number(conn.target.slice(5))
    v = j === idx + 1 ? 'next' : `goto:${j + 1}`
  }
  steps.value[idx][field] = v
  const b = BRANCHES.find((x) => x.field === field)
  ElMessage.success(`已设置「${b?.text || field}」去向：${routeLabel(v, b?.fallback || '')}`)
}

// 拖线过程中目标节点的禁用态（视觉提示）
function isBlockedTarget(nodeId: string): boolean {
  const cs = connectSource.value
  if (!cs) return false
  if (nodeId === 'end') return false
  if (nodeId === 'reject') return cs.field === 'on_pass'
  if (nodeId.startsWith('step-')) {
    const j = Number(nodeId.slice(5))
    return !(j > cs.idx && steps.value[j]?.kind !== 'ai')
  }
  return true
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
  position: relative;
  height: 480px;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  background: $color-bg-page;
  overflow: hidden;
}
.vf-pill {
  padding: 5px 28px;
  border-radius: 17px;
  font-size: 13px;
  color: $color-white;
  line-height: 20px;
  text-align: center;
  white-space: nowrap;
}
.start-pill {
  background: $color-primary;
}
.end-pill {
  background: $color-text-secondary;
}
.reject-pill {
  background: $color-danger;
}
.node-card {
  width: 280px;
  border: 1px solid $color-border;
  border-radius: 8px;
  background: $color-bg-card;
  box-shadow: 0 1px 4px rgba(31, 35, 41, 0.06);
  overflow: visible;
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
.connect-blocked {
  opacity: 0.35;
  filter: grayscale(0.7);
  transition: opacity 0.15s;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px $spacing-sm;
  background: $color-table-header;
  border-bottom: 1px solid $color-border;
  border-radius: 8px 8px 0 0;
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
.branch-line {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  line-height: 26px;
  &.invalid {
    color: $color-danger;
  }
}
.branch-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.branch-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.edge-add {
  position: absolute;
  pointer-events: all;
}
.join-add {
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
.branch-label {
  position: absolute;
  pointer-events: all;
  padding: 1px 8px;
  border-radius: 10px;
  border: 1px solid;
  background: $color-white;
  font-size: 11px;
  line-height: 16px;
  white-space: nowrap;
  &.invalid {
    color: $color-danger !important;
    border-color: $color-danger !important;
  }
}
.flow-empty {
  position: absolute;
  left: 50%;
  bottom: $spacing-md;
  transform: translateX(-50%);
  font-size: 12px;
  color: $color-text-secondary;
  pointer-events: none;
}
.flow-footer {
  display: flex;
  justify-content: space-between;
  margin-top: $spacing-sm;
}
</style>

<style lang="scss">
// Vue Flow 画布内部元素的全局覆盖（scoped 无法穿透）
.flow-canvas {
  .vue-flow__node {
    // 自定义节点外观完全由模板承担
    border: none;
    background: none;
    padding: 0;
    font-size: inherit;
    color: inherit;
    text-align: initial;
  }
  .vue-flow__handle {
    &.ghost-handle {
      opacity: 0;
      pointer-events: none;
      width: 1px;
      height: 1px;
      min-width: 0;
      min-height: 0;
      border: none;
    }
    &.branch-target {
      width: 10px;
      height: 10px;
      background: $color-white;
      border: 2px solid $color-primary;
      opacity: 0.45;
      transition: opacity 0.15s;
      &:hover,
      &.connectingto,
      &.valid {
        opacity: 1;
      }
    }
    &.branch-source {
      width: 12px;
      height: 12px;
      border: 2px solid $color-white;
      box-shadow: 0 0 0 1px $color-border-dark;
      right: -7px;
      top: 50%;
      transform: translateY(-50%);
      cursor: crosshair;
    }
  }
  .vue-flow__controls {
    box-shadow: $shadow-popup;
    border-radius: $radius-small;
    overflow: hidden;
  }
}
</style>
