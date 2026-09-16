<template>
  <!-- 批量导入：三步向导（列头直接兼容甲方台账结构） -->
  <el-dialog v-model="visible" title="批量导入设备台账" width="640px" :close-on-click-modal="false" @closed="resetImport">
    <el-steps :active="importStep" align-center finish-status="success" class="import-steps">
      <el-step title="模板说明" />
      <el-step title="上传文件" />
      <el-step title="导入结果" />
    </el-steps>

    <div v-show="importStep === 0" class="import-pane">
      <el-button :icon="Download" @click="handleDownloadTemplate">下载导入模板 equipment_import_template.xlsx</el-button>
      <el-alert
        title="模板列头与甲方现有台账（雄楚春天设施设备台帐）一致，原表可直接导入：第 1 行大标题自动跳过，按列头名识别（顺序可调）"
        type="info"
        :closable="false"
        class="import-tip"
      />
      <el-table :data="templateFields" border size="small" class="import-fields">
        <el-table-column prop="field" label="字段" width="110" />
        <el-table-column prop="required" label="必填" width="70" align="center" />
        <el-table-column prop="rule" label="填写规则" />
      </el-table>
    </div>

    <div v-show="importStep === 1" class="import-pane">
      <el-form label-width="96px">
        <el-form-item label="导入到小区" required>
          <el-select v-model="importCommunityId" placeholder="整个文件导入到该小区" style="width: 100%">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-upload
        ref="uploadRef"
        drag
        :auto-upload="false"
        :limit="1"
        accept=".xlsx"
        :on-change="handleFileChange"
        :on-remove="() => (importFile = null)"
        :on-exceed="handleFileExceed"
      >
        <el-icon :size="40" class="upload-icon"><UploadFilled /></el-icon>
        <div class="el-upload__text">拖拽文件到此处，或 <em>点击选择文件</em></div>
        <template #tip>
          <div class="text-secondary">仅支持 .xlsx，单次最多 2000 行，文件 ≤ 5MB；同编号按更新处理（可重复导入）</div>
        </template>
      </el-upload>
      <div v-if="importError" class="import-error">{{ importError }}</div>
    </div>

    <div v-show="importStep === 2" class="import-pane">
      <el-alert
        v-if="importResult"
        :title="`导入完成：新增 ${importResult.created_count} 条，更新 ${importResult.updated_count} 条，失败 ${importResult.fail_count} 条，自动绑定点位 ${importResult.auto_bound} 条`"
        :type="importResult.fail_count > 0 ? 'warning' : 'success'"
        :closable="false"
        show-icon
      />
      <template v-if="importResult && importResult.fail_details.length">
        <div class="fail-header">
          <span class="card-title">失败明细</span>
        </div>
        <el-table :data="importResult.fail_details" border size="small" max-height="260">
          <el-table-column prop="row" label="行号" width="80" align="center" />
          <el-table-column prop="code" label="设备编号" width="160" show-overflow-tooltip />
          <el-table-column prop="reason" label="失败原因" />
        </el-table>
        <div class="text-secondary fail-tip">修正失败行后可重新上传，同编号设备不会重复创建</div>
      </template>
    </div>

    <template #footer>
      <template v-if="importStep === 0">
        <el-button type="primary" @click="importStep = 1">下一步</el-button>
      </template>
      <template v-else-if="importStep === 1">
        <el-button @click="importStep = 0">上一步</el-button>
        <el-button type="primary" :loading="importing" :disabled="!importFile || !importCommunityId" @click="handleImport">
          开始导入
        </el-button>
      </template>
      <template v-else>
        <el-button type="primary" @click="visible = false">完成</el-button>
      </template>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { type UploadFile, type UploadInstance, type UploadRawFile } from 'element-plus'
import { Download, UploadFilled } from '@element-plus/icons-vue'
import { importEquipment, type EquipmentImportResult } from '@/api/equipment'
import { downloadFile } from '@/utils/download'
import { useCommunities } from '@/composables/useCommunities'

const props = defineProps<{ initialCommunityId?: string }>()
const emit = defineEmits<{ done: [] }>()
const visible = defineModel<boolean>('visible', { required: true })

const { communities } = useCommunities()

const importStep = ref(0)
const importFile = ref<File | null>(null)
const importError = ref('')
const importing = ref(false)
const importResult = ref<EquipmentImportResult | null>(null)
const importCommunityId = ref('')
const uploadRef = ref<UploadInstance>()

const templateFields = [
  { field: '设备编号', required: '*', rule: '租户内唯一；同编号按更新处理（可重复导入）' },
  { field: '设备名称', required: '—', rule: '留空时以编号兜底' },
  { field: '设备分类', required: '*', rule: '优先匹配设备类型字典；灭火器设施/消火栓设施/电梯轿厢/电梯机房按别名映射；未识别自动扩充字典（无规则参数，不自动判到期）' },
  { field: '出厂日期', required: '—', rule: '2018-05-01 / 2018/5/1 / 2018年5月 均可；留空用投运日期兜底' },
  { field: '投运日期', required: '—', rule: '原值保留在扩展属性，同时作为出厂日期兜底' },
  { field: '设备等级/品牌/规格型号/维保状态/维保单位/产地', required: '—', rule: '原样保留到扩展属性' },
  { field: '安装位置/管控区域', required: '—', rule: '拼接进备注；点位绑定待 App 扫码' },
  { field: '异动状态', required: '—', rule: '启用/停用/报废，默认在用' }
]

// 打开时回到第一步并预填当前筛选小区
watch(visible, (v) => {
  if (!v) return
  importStep.value = 0
  importFile.value = null
  importError.value = ''
  importResult.value = null
  importCommunityId.value = props.initialCommunityId || ''
})

function resetImport() {
  importFile.value = null
  importError.value = ''
  importResult.value = null
  uploadRef.value?.clearFiles()
}

function handleDownloadTemplate() {
  downloadFile('/equipment/import-template', undefined, 'equipment_import_template.xlsx')
}

// 前置校验：非 .xlsx 或超限直接红字拒绝，不发起请求
function validateFile(file: File): boolean {
  if (!file.name.endsWith('.xlsx')) {
    importError.value = '文件格式错误：仅支持 .xlsx 文件'
    return false
  }
  if (file.size > 5 * 1024 * 1024) {
    importError.value = '文件大小超限：请控制在 5MB 以内'
    return false
  }
  importError.value = ''
  return true
}

function handleFileChange(uploadFile: UploadFile) {
  const raw = uploadFile.raw
  if (!raw) return
  if (!validateFile(raw)) {
    uploadRef.value?.clearFiles()
    importFile.value = null
    return
  }
  importFile.value = raw
}

function handleFileExceed(files: File[]) {
  uploadRef.value?.clearFiles()
  const raw = files[0] as UploadRawFile
  if (validateFile(raw)) {
    uploadRef.value?.handleStart(raw)
    importFile.value = raw
  }
}

async function handleImport() {
  if (!importFile.value || !importCommunityId.value) return
  importing.value = true
  try {
    importResult.value = await importEquipment(importCommunityId.value, importFile.value)
    importStep.value = 2
    if (importResult.value.created_count > 0 || importResult.value.updated_count > 0) emit('done')
  } catch {
    // 拦截器已提示；文件级错误停留在当前步可重新选择
  } finally {
    importing.value = false
  }
}
</script>

<style scoped lang="scss">
.import-steps {
  margin-bottom: $spacing-xl;
}

.import-pane {
  min-height: 280px;
}

.import-tip {
  margin-top: $spacing-md;
}

.import-fields {
  margin-top: $spacing-lg;
}

.upload-icon {
  color: $color-text-secondary;
}

.import-error {
  margin-top: $spacing-sm;
  color: $color-danger;
  font-size: $font-size-aux;
}

.fail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: $spacing-lg 0 $spacing-sm;
}

.fail-tip {
  margin-top: $spacing-sm;
}
</style>
