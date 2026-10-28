<template>
  <div class="app-container import-page">
    <div class="table-card page-head">
      <el-button :icon="ArrowLeft" @click="router.push('/templates/check')">返回模板列表</el-button>
      <div>
        <h2 class="page-title">检查项 Excel 批量录入</h2>
        <div class="page-subtitle">先校验预览，再生成草稿模板</div>
      </div>
      <el-button :icon="Download" @click="downloadTemplate">下载导入模板</el-button>
    </div>

    <el-steps :active="step" align-center finish-status="success" class="import-steps">
      <el-step title="选择文件" />
      <el-step title="校验预览" />
      <el-step title="生成结果" />
    </el-steps>

    <div v-if="step === 0" class="table-card upload-panel">
      <el-upload
        drag
        accept=".xlsx,.csv"
        :auto-upload="false"
        :limit="1"
        :on-change="selectFile"
        :on-remove="clearFile"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">拖拽 xlsx / csv 文件到这里，或 <em>点击选择</em></div>
        <template #tip><div class="el-upload__tip">单个文件不超过 2 MiB、500 行，表头须与导入模板一致</div></template>
      </el-upload>
      <div v-if="file" class="file-row">
        <span>{{ file.name }} · {{ file.size ? (file.size / 1024).toFixed(1) : 0 }} KB</span>
        <el-button link type="danger" @click="clearFile">重新选择</el-button>
      </div>
      <el-button type="primary" :disabled="!file" :loading="parsing" @click="parseFile">解析文件</el-button>
    </div>

    <template v-if="step === 1">
      <div class="table-card">
        <div class="preview-toolbar">
          <el-tag type="success">可导入 {{ validCount }} 行</el-tag>
          <el-tag :type="invalidCount ? 'danger' : 'info'">失败 {{ invalidCount }} 行</el-tag>
          <el-button v-if="invalidCount" :icon="Download" @click="downloadFailures(rows)">导出失败行</el-button>
        </div>
        <el-table :data="rows" border max-height="500" @row-click="showErrors">
          <el-table-column prop="row" label="行" width="65" align="center" />
          <el-table-column prop="point_type" label="点位类型" min-width="120" />
          <el-table-column prop="template_name" label="模板名" min-width="150" />
          <el-table-column prop="item_code" label="项编码" min-width="130" />
          <el-table-column prop="item_name" label="项名" min-width="150" />
          <el-table-column label="结果" min-width="230">
            <template #default="{ row }">
              <el-tag v-if="row.valid" type="success">可导入</el-tag>
              <span v-else class="error-text">{{ row.errors.map((error: { cell: string; message: string }) => `${error.cell}列：${error.message}`).join('；') }}</span>
            </template>
          </el-table-column>
        </el-table>
        <el-alert v-if="selectedErrors.length" class="error-detail" type="error" :closable="false" :title="`第 ${selectedRow} 行：${selectedErrors.join('；')}`" />
        <div class="form-actions">
          <el-button @click="step = 0">上一步</el-button>
          <el-button type="primary" :disabled="!validCount" :loading="applying" @click="applyRows(rows)">生成模板与检查项</el-button>
        </div>
      </div>
    </template>

    <template v-if="step === 2">
      <div class="table-card">
        <el-result :icon="createdItems > 0 ? 'success' : 'warning'" :title="createdItems > 0 ? '批量录入完成' : '没有检查项写入'" :sub-title="`新建模板 ${createdTemplates} 套，写入检查项 ${createdItems} 条`" />
        <el-alert v-if="failedRows.length" type="warning" :closable="false" :title="`${failedRows.length} 行未写入，可下载修正或单独重试`" />
        <el-table v-if="failedRows.length" :data="failedRows" border max-height="360">
          <el-table-column prop="row" label="行" width="70" align="center" />
          <el-table-column prop="item_name" label="项名" min-width="160" />
          <el-table-column label="失败原因" min-width="300">
            <template #default="{ row }">{{ row.errors.map((error: { cell: string; message: string }) => error.message).join('；') }}</template>
          </el-table-column>
        </el-table>
        <div class="form-actions">
          <el-button v-if="failedRows.length" :icon="Download" @click="downloadFailures(failedRows)">导出失败行</el-button>
          <el-button v-if="failedRows.length" :loading="applying" @click="retryFailed">仅重试失败行</el-button>
          <el-button type="primary" :disabled="!createdTemplateId" @click="openCreatedTemplate">进入检查项核对</el-button>
          <el-button @click="reset">继续导入</el-button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type UploadFile } from 'element-plus'
import { ArrowLeft, Download, UploadFilled } from '@element-plus/icons-vue'
import { applyTemplateImport, previewTemplateImport, type TemplateImportResultRow, type TemplateImportRow } from '@/api/template'
import { downloadFile, downloadFilePost } from '@/utils/download'

const router = useRouter()
const step = ref(0)
const file = ref<File | null>(null)
const parsing = ref(false)
const applying = ref(false)
const rows = ref<TemplateImportResultRow[]>([])
const failedRows = ref<TemplateImportResultRow[]>([])
const createdTemplates = ref(0)
const createdItems = ref(0)
const createdTemplateId = ref('')
const selectedRow = ref(0)
const selectedErrors = ref<string[]>([])
const validCount = computed(() => rows.value.filter((row) => row.valid).length)
const invalidCount = computed(() => rows.value.length - validCount.value)

function selectFile(uploadFile: UploadFile) {
  if (!uploadFile.raw) return
  if ((uploadFile.size ?? 0) > 2 * 1024 * 1024) { ElMessage.warning('文件不能超过 2 MiB'); return }
  file.value = uploadFile.raw
}
function clearFile() { file.value = null }
async function parseFile() {
  if (!file.value) return
  parsing.value = true
  try {
    const result = await previewTemplateImport(file.value)
    rows.value = result.rows
    failedRows.value = []
    step.value = 1
  } finally { parsing.value = false }
}
function showErrors(row: TemplateImportResultRow) {
  selectedRow.value = row.row
  selectedErrors.value = row.errors.map((error) => `${error.cell}列：${error.message}`)
}
async function applyRows(input: TemplateImportRow[]) {
  applying.value = true
  try {
    const result = await applyTemplateImport(input)
    createdTemplates.value = result.created_templates
    createdItems.value = result.created_items
    createdTemplateId.value = result.template_id
    failedRows.value = result.failed
    step.value = 2
  } finally { applying.value = false }
}
async function retryFailed() {
  const previousCreated = createdItems.value
  const previousTemplates = createdTemplates.value
  await applyRows(failedRows.value)
  createdItems.value += previousCreated
  createdTemplates.value += previousTemplates
}
function failurePayload(input: TemplateImportResultRow[]) {
  return input.map(({ valid: _valid, template_id: _templateID, errors, ...row }) => ({ ...row, errors: errors.map((error) => `${error.cell}列：${error.message}`) }))
}
function downloadFailures(input: TemplateImportResultRow[]) {
  downloadFilePost('/inspection/templates/import/failure-export', { rows: failurePayload(input) }, 'check-item-import-failures.xlsx')
}
function downloadTemplate() { downloadFile('/inspection/templates/import/template', undefined, 'check-item-import.xlsx') }
function openCreatedTemplate() { router.push(`/templates/check/${createdTemplateId.value}/items`) }
function reset() {
  step.value = 0
  file.value = null
  rows.value = []
  failedRows.value = []
  createdTemplates.value = 0
  createdItems.value = 0
  createdTemplateId.value = ''
}
</script>

<style scoped lang="scss">
.import-page { max-width: 1320px; margin: 0 auto; }
.page-head { display: flex; align-items: center; justify-content: space-between; gap: $spacing-md; }
.page-title { margin: 0; }
.page-subtitle { color: $color-text-secondary; margin-top: $spacing-xs; }
.import-steps { margin: $spacing-xl 12%; }
.upload-panel { max-width: 760px; margin: 0 auto; text-align: center; padding: $spacing-xl; }
.file-row { display: flex; justify-content: center; gap: $spacing-md; margin: $spacing-lg 0; }
.preview-toolbar { display: flex; align-items: center; gap: $spacing-md; margin-bottom: $spacing-md; }
.error-text, .error-detail { color: $color-danger; }
.error-detail { margin-top: $spacing-md; }
.form-actions { display: flex; justify-content: center; gap: $spacing-md; margin-top: $spacing-xl; }
</style>
