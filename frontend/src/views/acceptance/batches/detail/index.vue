<template>
  <div v-loading="loading" class="app-container">
    <template v-if="detail">
      <div class="table-card batch-header">
        <div>
          <h2>{{ detail.batch.name }}</h2>
          <div class="batch-period">{{ detail.batch.period_start?.slice(0, 10) }} 至 {{ detail.batch.period_end?.slice(0, 10) }}</div>
        </div>
        <div class="batch-actions">
          <el-tag :type="detail.batch.status === 'done' ? 'success' : 'warning'">{{ statusLabels[detail.batch.status] }}</el-tag>
          <el-button v-if="detail.batch.status === 'draft' && isManager" v-perms="'acceptance:manage'" type="primary" @click="start">开始查验</el-button>
          <el-button v-if="detail.batch.status === 'inspecting' && isManager" v-perms="'acceptance:submit'" type="primary" @click="submit">送审</el-button>
        </div>
      </div>
      <div class="table-card">
        <el-progress :percentage="progress" :format="() => `${covered}/${detail?.targets.length || 0} 个对象已确认`" />
      </div>
      <div class="table-card">
        <el-tabs v-model="tab">
          <el-tab-pane label="查验对象" name="targets">
            <el-tabs v-model="category" type="card">
              <el-tab-pane label="全部" name="" />
              <el-tab-pane v-for="(label, key) in categoryLabels" :key="key" :label="label" :name="key" />
            </el-tabs>
            <div class="table-toolbar"><div class="table-toolbar-left"><el-button v-if="canRecord" v-perms="'acceptance:submit'" type="primary" :icon="Plus" @click="openRecord()">登记问题</el-button></div></div>
            <el-table :data="filteredTargets">
              <el-table-column prop="name" label="查验对象" min-width="170" />
              <el-table-column label="类别" width="120"><template #default="{ row }">{{ categoryLabels[row.category_code] || row.category_code }}</template></el-table-column>
              <el-table-column prop="location" label="位置" min-width="150" />
              <el-table-column label="覆盖确认" width="110"><template #default="{ row }"><el-tag :type="row.coverage_confirmed ? 'success' : 'info'">{{ row.coverage_confirmed ? '已确认' : '待查验' }}</el-tag></template></el-table-column>
              <el-table-column label="问题" width="80" align="right"><template #default="{ row }">{{ problemCount(row.id) }}</template></el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button v-if="canRecord" link type="primary" @click="openRecord(row.id)">登记问题</el-button>
                  <el-button v-if="canRecord && !row.coverage_confirmed" link type="success" @click="confirmCoverage(row.id)">确认已逐项查验</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="问题清单" name="records">
            <el-table :data="detail.records">
              <el-table-column label="对象" min-width="130"><template #default="{ row }">{{ targetName(row.target_id) }}</template></el-table-column>
              <el-table-column prop="table_code" label="专项" min-width="130" />
              <el-table-column prop="item_code" label="检查项" min-width="130" />
              <el-table-column label="结果" width="90"><template #default="{ row }"><el-tag type="danger">{{ row.result === 'missing' ? '缺项' : '问题' }}</el-tag></template></el-table-column>
              <el-table-column prop="note" label="问题事实" min-width="200" show-overflow-tooltip />
              <el-table-column label="照片" width="80" align="right"><template #default="{ row }">{{ row.photos.length }}</template></el-table-column>
              <el-table-column prop="revision" label="修订" width="70" align="right" />
              <el-table-column label="遗留接收" width="110" fixed="right"><template #default="{ row }"><el-button v-if="detail.batch.status === 'handover' && isManager" link type="primary" @click="openAcceptance(problemFor(row.id))">{{ problemFor(row.id)?.accepted ? '查看/修改' : '登记接收' }}</el-button><el-tag v-else-if="problemFor(row.id)?.accepted" type="success">已接收</el-tag></template></el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="报告与签署" name="report">
            <div class="report-toolbar">
              <el-steps :active="detail.batch.review_step" finish-status="success">
                <el-step v-for="step in detail.batch.review_flow" :key="step.role_code" :title="step.name" />
              </el-steps>
              <div v-if="canReview" class="review-actions"><el-button type="primary" @click="review('approve')">审核通过</el-button><el-button type="danger" @click="review('reject')">驳回</el-button></div>
              <el-button v-if="detail.batch.status === 'draft' && isManager" @click="openSubjects">调整签署位</el-button>
            </div>
            <div class="section-title">签署状态</div>
            <el-table :data="detail.subjects">
              <el-table-column prop="name" label="签字位" min-width="130" />
              <el-table-column label="必需" width="80"><template #default="{ row }">{{ row.required ? '必需' : '可留空' }}</template></el-table-column>
              <el-table-column label="代表人" min-width="130"><template #default="{ row }">{{ signatureFor(row.code)?.representative || '待签署' }}</template></el-table-column>
              <el-table-column label="报告摘要" min-width="150"><template #default="{ row }">{{ signatureFor(row.code)?.report_sha256 || '—' }}</template></el-table-column>
              <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="signatureFor(row.code) ? 'success' : 'info'">{{ signatureFor(row.code) ? '已签署' : '待签署' }}</el-tag></template></el-table-column>
              <el-table-column label="操作" width="110"><template #default="{ row }"><el-button v-if="signableSubject(row)" link type="primary" @click="openSign(row)">上传签署</el-button></template></el-table-column>
            </el-table>
            <div class="section-title">报告文件</div>
            <el-table :data="detail.files">
              <el-table-column prop="volume_name" label="文件" min-width="160" />
              <el-table-column label="阶段" width="110"><template #default="{ row }">{{ row.stage === 'frozen' ? '冻结待签' : '最终归档' }}</template></el-table-column>
              <el-table-column prop="format" label="格式" width="80" />
              <el-table-column prop="sha256" label="摘要" min-width="150" />
              <el-table-column label="操作" width="100"><template #default="{ row }"><el-button v-if="row.status === 'ready'" link type="primary" @click="downloadReport(row.id, row.volume_name, row.stage, row.format)">下载</el-button></template></el-table-column>
            </el-table>
            <div class="section-title">审核记录</div>
            <el-table :data="detail.votes">
              <el-table-column prop="step_index" label="节点" width="80"><template #default="{ row }">{{ row.step_index + 1 }}</template></el-table-column>
              <el-table-column prop="action" label="动作" width="100" />
              <el-table-column prop="reason" label="意见" min-width="200" />
              <el-table-column prop="created_at" label="时间" width="170" />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </template>

    <el-dialog v-model="recordVisible" title="登记问题事实" width="600px" :close-on-click-modal="false">
      <el-form :model="recordForm" label-width="100px">
        <el-form-item label="查验对象" required><el-select v-model="recordForm.target_id" style="width: 100%"><el-option v-for="target in detail?.targets || []" :key="target.id" :label="target.name" :value="target.id" /></el-select></el-form-item>
        <el-form-item label="专项" required><el-select v-model="recordForm.table_code" style="width: 100%" @change="recordForm.item_code = ''"><el-option v-for="schema in recordSchemas" :key="schema.code" :label="schema.name" :value="schema.code" /></el-select></el-form-item>
        <el-form-item label="检查项" required>
          <el-select v-model="recordForm.item_code" filterable style="width: 100%"><el-option v-for="item in recordItems" :key="item.code" :label="item.name" :value="item.code" /></el-select>
        </el-form-item>
        <el-form-item label="结果" required><el-radio-group v-model="recordForm.result"><el-radio value="problem">问题</el-radio><el-radio value="missing">缺项</el-radio></el-radio-group></el-form-item>
        <el-form-item label="事实值"><el-input v-model="recordForm.value_text" placeholder="数量、读数或资料编号" /></el-form-item>
        <template v-if="recordForm.table_code === 'unit_meter'">
          <el-form-item label="水表表号"><el-input v-model="recordForm.values.water_meter_number" /></el-form-item>
          <el-form-item label="水表底数"><el-input v-model="recordForm.values.water_reading" /></el-form-item>
          <el-form-item label="电表表号"><el-input v-model="recordForm.values.electric_meter_number" /></el-form-item>
          <el-form-item label="电表底数"><el-input v-model="recordForm.values.electric_reading" /></el-form-item>
        </template>
        <el-form-item label="问题事实" required><el-input v-model="recordForm.note" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="问题照片">
          <el-upload :show-file-list="false" :http-request="uploadPhoto"><el-button :loading="uploading">上传照片</el-button></el-upload>
          <div v-for="(photo, index) in recordForm.photos" :key="photo" class="photo-row"><span>照片 {{ index + 1 }}</span><el-button link type="danger" @click="recordForm.photos.splice(index, 1)">移除</el-button></div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="recordVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveRecord">保存问题</el-button></template>
    </el-dialog>
    <el-dialog v-model="acceptVisible" title="遗留问题接收" width="560px" :close-on-click-modal="false">
      <el-form :model="acceptForm" label-width="110px">
        <el-form-item label="问题事实"><el-input :model-value="acceptForm.fact" disabled /></el-form-item>
        <el-form-item label="风险等级" required><el-radio-group v-model="acceptForm.risk_level"><el-radio value="normal">一般问题</el-radio><el-radio value="major">重大安全风险</el-radio></el-radio-group></el-form-item>
        <el-form-item label="安全/风险措施" required><el-input v-model="acceptForm.safety_measure" type="textarea" :rows="3" /></el-form-item>
        <el-form-item v-if="acceptForm.risk_level === 'major'" label="措施已获批" required><el-switch v-model="acceptForm.measure_approved" /></el-form-item>
        <el-form-item label="接收方接受"><el-switch v-model="acceptForm.accepted" /></el-form-item>
        <template v-if="acceptForm.accepted">
          <el-form-item label="接收方代表" required><el-input v-model="acceptForm.accepted_by" /></el-form-item>
          <el-form-item label="接收签署文件" required><el-upload :show-file-list="false" :http-request="uploadAcceptance"><el-button :loading="uploading">上传文件</el-button></el-upload><span v-if="acceptForm.acceptance_file_id" class="sign-file">已上传</span></el-form-item>
        </template>
      </el-form>
      <template #footer><el-button @click="acceptVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveAcceptance">保存并重新冻结</el-button></template>
    </el-dialog>
    <el-dialog v-model="signVisible" :title="`上传签署 · ${signForm.subjectName}`" width="520px">
      <el-form :model="signForm" label-width="90px">
        <el-form-item label="代表人" required><el-input v-model="signForm.representative" :disabled="!signForm.external" /></el-form-item>
        <el-form-item label="签署证据" required><el-upload :show-file-list="false" :http-request="uploadSignature"><el-button :loading="uploading">上传签字扫描件</el-button></el-upload><span v-if="signForm.file_id" class="sign-file">已上传</span></el-form-item>
      </el-form>
      <template #footer><el-button @click="signVisible = false">取消</el-button><el-button type="primary" :disabled="!signForm.file_id || !signForm.representative" :loading="saving" @click="saveSignature">确认签署</el-button></template>
    </el-dialog>
    <el-dialog v-model="subjectsVisible" title="调整签署位" width="680px">
      <div v-for="(subject, index) in subjectDraft" :key="index" class="subject-row">
        <el-input v-model="subject.code" placeholder="编码" />
        <el-input v-model="subject.name" placeholder="签署位名称" />
        <el-checkbox v-model="subject.external">外部签署</el-checkbox>
        <el-checkbox v-model="subject.required">必需</el-checkbox>
        <el-button link type="danger" @click="subjectDraft.splice(index, 1)">删除</el-button>
      </div>
      <el-button link type="primary" @click="subjectDraft.push({ code: '', name: '', role_code: '', external: true, required: true, sort: subjectDraft.length + 1 })">添加签署位</el-button>
      <template #footer><el-button @click="subjectsVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveSubjects">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getAcceptanceBatch, startAcceptanceBatch, submitAcceptanceBatch, saveAcceptanceRecord, confirmAcceptanceCoverage, reviewAcceptanceBatch, signAcceptanceBatch, updateAcceptanceSubjects, acceptAcceptanceProblem, type AcceptanceBatchDetail, type AcceptanceSignSubject, type AcceptanceProblem } from '@/api/acceptance'
import { uploadImage } from '@/api/upload'
import { downloadFile } from '@/utils/download'
import { useUserStore } from '@/store/user'

const route = useRoute()
const userStore = useUserStore()
const batchId = route.params.id as string
const statusLabels: Record<string, string> = { draft: '草稿', inspecting: '进行中', reviewing: '审核中', handover: '待签署', done: '已归档' }
const categoryLabels: Record<string, string> = { documents: '物业资料', building: '房屋本体', equipment: '设施设备', outdoor: '室外场地', office: '物业用房', garage: '地下车库', unit: '分户验收' }
const detail = ref<AcceptanceBatchDetail>()
const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const tab = ref('targets')
const category = ref('')
const isManager = computed(() => detail.value?.batch.manager_id === userStore.info?.id)
const canRecord = computed(() => detail.value?.batch.status === 'inspecting' && detail.value.members.some((member) => member.user_id === userStore.info?.id))
const covered = computed(() => detail.value?.targets.filter((target) => target.coverage_confirmed).length || 0)
const progress = computed(() => detail.value?.targets.length ? Math.round(covered.value * 100 / detail.value.targets.length) : 0)
const filteredTargets = computed(() => detail.value?.targets.filter((target) => !category.value || target.category_code === category.value) || [])
const schemas = computed(() => detail.value?.batch.template_snapshot.schemas || [])
const problemCount = (targetId: string) => detail.value?.records.filter((record) => record.target_id === targetId).length || 0
const targetName = (targetId: string) => detail.value?.targets.find((target) => target.id === targetId)?.name || targetId
async function load() {
  loading.value = true
  try { detail.value = await getAcceptanceBatch(batchId) } finally { loading.value = false }
}
async function start() { await startAcceptanceBatch(batchId); await load() }
async function submit() {
  await ElMessageBox.confirm('送审后，问题事实将冻结进入审核。确认全部对象已完成查验？', '送审确认', { type: 'warning' })
  await submitAcceptanceBatch(batchId)
  ElMessage.success('批次已送审')
  await load()
}
async function confirmCoverage(targetId: string) {
  await ElMessageBox.confirm('确认该对象已逐项查验？未登记问题的项目将按已覆盖处理。', '覆盖确认', { type: 'warning' })
  await confirmAcceptanceCoverage(batchId, targetId)
  await load()
}
const recordVisible = ref(false)
const recordSchemas = computed(() => {
  const target = detail.value?.targets.find((item) => item.id === recordForm.value.target_id)
  return schemas.value.filter((schema) => target?.table_codes.includes(schema.code))
})
const recordForm = ref({ target_id: '', table_code: '', item_code: '', result: 'problem', value_text: '', values: { water_meter_number: '', water_reading: '', electric_meter_number: '', electric_reading: '' }, note: '', photos: [] as string[] })
function openRecord(targetId = '') {
  recordForm.value = { target_id: targetId || detail.value?.targets[0]?.id || '', table_code: recordSchemas.value[0]?.code || '', item_code: '', result: 'problem', value_text: '', values: { water_meter_number: '', water_reading: '', electric_meter_number: '', electric_reading: '' }, note: '', photos: [] }
  recordVisible.value = true
}
const recordItems = computed(() => {
  const schema = schemas.value.find((item) => item.code === recordForm.value.table_code)
  if (!schema) return []
  if (schema.table_kind !== 'matrix') return schema.rows.map((row) => ({ code: row.code, name: row.name }))
  return (detail.value?.batch.template_snapshot.verify_templates || []).flatMap((template) => (template.Items || []).map((item) => ({ code: item.Code, name: item.Name })))
})
async function uploadPhoto(options: { file: File }) {
  uploading.value = true
  try {
    const result = await uploadImage(options.file, 'acceptance')
    recordForm.value.photos.push(result.file_id)
  } finally { uploading.value = false }
}
async function saveRecord() {
  saving.value = true
  try {
    await saveAcceptanceRecord(batchId, recordForm.value)
    recordVisible.value = false
    ElMessage.success('问题事实已保存')
    await load()
  } finally { saving.value = false }
}
const canReview = computed(() => {
  if (!detail.value || detail.value.batch.status !== 'reviewing' || isManager.value) return false
  const step = detail.value.batch.review_flow?.[detail.value.batch.review_step]
  if (!step || !userStore.info?.roles?.some((role) => role.code === step.role_code)) return false
  return !detail.value.records.some((record) => record.created_by === userStore.info?.id)
})
async function review(action: 'approve' | 'reject') {
  let reason = ''
  if (action === 'reject') {
    const result = await ElMessageBox.prompt('请输入驳回原因', '驳回查验', { inputValidator: (value) => !!value?.trim() || '驳回原因必填' }).catch(() => null)
    if (!result) return
    reason = result.value.trim()
  } else {
    await ElMessageBox.confirm('确认当前审核节点通过？', '审核确认', { type: 'warning' })
  }
  await reviewAcceptanceBatch(batchId, { action, reason })
  ElMessage.success('审核已提交')
  await load()
}
const signatureFor = (code: string) => detail.value?.signatures.find((signature) => signature.subject_code === code && signature.status === 'valid')
const signVisible = ref(false)
const signForm = ref({ code: '', subjectName: '', external: false, representative: '', file_id: '' })
const signableSubject = (subject: AcceptanceSignSubject) => detail.value?.batch.status === 'handover' && !signatureFor(subject.code) && (subject.external ? isManager.value : subject.role_code ? userStore.info?.roles?.some((role) => role.code === subject.role_code) : detail.value.members.some((member) => member.user_id === userStore.info?.id))
function openSign(subject: AcceptanceSignSubject) {
  signForm.value = { code: subject.code, subjectName: subject.name, external: subject.external, representative: subject.external ? '' : userStore.name, file_id: '' }
  signVisible.value = true
}
async function uploadSignature(options: { file: File }) {
  uploading.value = true
  try {
    const result = await uploadImage(options.file, 'acceptance')
    signForm.value.file_id = result.file_id
  } finally { uploading.value = false }
}
async function saveSignature() {
  saving.value = true
  try {
    await signAcceptanceBatch(batchId, { subject_code: signForm.value.code, representative: signForm.value.representative, file_id: signForm.value.file_id })
    signVisible.value = false
    ElMessage.success('签署已保存')
    await load()
  } finally { saving.value = false }
}
const acceptVisible = ref(false)
const acceptForm = ref({ id: '', fact: '', risk_level: 'normal' as 'normal' | 'major', safety_measure: '', measure_approved: false, accepted: false, accepted_by: '', acceptance_file_id: null as string | null })
const problemFor = (recordId: string) => detail.value?.problems.find((problem) => problem.record_id === recordId)
function openAcceptance(problem?: AcceptanceProblem) {
  if (!problem) return
  acceptForm.value = { id: problem.id, fact: problem.fact, risk_level: problem.risk_level, safety_measure: problem.safety_measure, measure_approved: problem.measure_approved, accepted: problem.accepted, accepted_by: problem.accepted_by, acceptance_file_id: problem.acceptance_file_id || null }
  acceptVisible.value = true
}
async function uploadAcceptance(options: { file: File }) {
  uploading.value = true
  try {
    const result = await uploadImage(options.file, 'acceptance')
    acceptForm.value.acceptance_file_id = result.file_id
  } finally { uploading.value = false }
}
async function saveAcceptance() {
  saving.value = true
  try {
    const { id, fact: _, ...payload } = acceptForm.value
    await acceptAcceptanceProblem(batchId, id, payload)
    acceptVisible.value = false
    ElMessage.success('遗留接收已保存，报告正在重新冻结')
    await load()
  } finally { saving.value = false }
}
const subjectsVisible = ref(false)
const subjectDraft = ref<AcceptanceSignSubject[]>([])
function openSubjects() {
  subjectDraft.value = (detail.value?.subjects || []).map((subject) => ({ ...subject }))
  subjectsVisible.value = true
}
async function saveSubjects() {
  saving.value = true
  try {
    await updateAcceptanceSubjects(batchId, subjectDraft.value)
    subjectsVisible.value = false
    await load()
  } finally { saving.value = false }
}
async function downloadReport(fileId: string, name: string, stage: string, format: string) {
  await downloadFile(`/acceptance/batches/${batchId}/files/${fileId}/download`, undefined, `${name}-${stage}.${format}`)
}
onMounted(load)
</script>

<style scoped>
.batch-header { display: flex; justify-content: space-between; align-items: center; }
.batch-header h2 { margin: 0 0 8px; font-size: 20px; }
.batch-period { color: #606266; }
.batch-actions { display: flex; align-items: center; gap: 12px; }
.section-title { margin: 20px 0 12px; font-weight: 600; }
.review-actions { display: flex; gap: 8px; margin-top: 16px; }
.subject-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; }
.sign-file { margin-left: 12px; color: #2ba471; }
.photo-row { display: flex; gap: 12px; align-items: center; }
</style>
