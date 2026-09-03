<template>
  <div class="app-container">
    <!-- 发布物管理（App 安装包 / 小程序码；官网下载页与 App 检查更新共用此列表） -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <span class="card-title" style="margin-right: 12px">发布物管理</span>
          <el-button v-perms="'system:site:upload'" type="primary" :icon="Plus" @click="openUpload">上传发布物</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchReleases" />
        </el-tooltip>
      </div>

      <el-table v-loading="releaseLoading" :data="releases" stripe style="width: 100%">
        <el-table-column label="平台" width="140">
          <template #default="{ row }">
            <el-tag :type="platformTag(row.platform)">{{ platformLabel(row.platform) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="版本" width="150">
          <template #default="{ row }">
            {{ row.version || '--' }}
            <el-tag v-if="row.force_update" type="danger" size="small" effect="plain" style="margin-left: 6px">强制</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="文件名" min-width="200" show-overflow-tooltip />
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="备注" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.note || '--' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="上传时间" width="160" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" @click="previewRelease(row)">{{ row.platform === 'wechat_mp' ? '查看' : '下载' }}</el-button>
            <el-button v-perms="'system:site:delete'" text type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无发布物，上传后官网下载页自动开放对应渠道" />
        </template>
      </el-table>
      <div class="table-tip">每个平台只生效最新上传的一条：官网下载页展示它，App 启动检查更新也对齐它（强制更新不可跳过）。</div>
    </div>

    <!-- 上传对话框 -->
    <el-dialog v-model="uploadVisible" title="上传发布物" width="520px" :close-on-click-modal="false">
      <el-form :model="uploadForm" label-width="90px">
        <el-form-item label="平台" required>
          <el-radio-group v-model="uploadForm.platform">
            <el-radio-button value="android">Android</el-radio-button>
            <el-radio-button value="harmony">HarmonyOS</el-radio-button>
            <el-radio-button value="ios">iOS</el-radio-button>
            <el-radio-button value="wechat_mp">小程序码</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="uploadForm.platform !== 'wechat_mp'" label="版本号" required>
          <el-input v-model="uploadForm.version" placeholder="如 1.0.0" style="width: 200px" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="uploadForm.note" placeholder="选填，如更新内容说明" />
        </el-form-item>
        <el-form-item v-if="uploadForm.platform !== 'wechat_mp'" label="更新方式">
          <el-radio-group v-model="uploadForm.force">
            <el-radio :value="false">弱更新</el-radio>
            <el-radio :value="true">强制更新</el-radio>
          </el-radio-group>
          <div class="form-tip">弱更新：App 弹窗可「以后再说」；强制更新：弹窗不可跳过，必须更新后才能使用</div>
        </el-form-item>
        <el-form-item label="文件" required>
          <el-upload
            ref="uploadRef"
            drag
            :auto-upload="false"
            :limit="1"
            :accept="acceptExt"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
            :file-list="fileList"
          >
            <el-icon :size="36" style="color: var(--el-color-primary)"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽文件到此处，或 <em>点击选择</em></div>
            <template #tip>
              <div class="el-upload__tip">{{ uploadTip }}</div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item v-if="uploading">
          <el-progress :percentage="uploadPercent" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false" :disabled="uploading">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="handleUpload">开始上传</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type UploadFile, type UploadFiles, type UploadInstance } from 'element-plus'
import { Plus, RefreshRight, UploadFilled } from '@element-plus/icons-vue'
import { listReleases, uploadRelease, deleteRelease, type AppRelease } from '@/api/site'

// ========== 发布物列表 ==========
const releases = ref<AppRelease[]>([])
const releaseLoading = ref(false)

async function fetchReleases() {
  releaseLoading.value = true
  try {
    releases.value = (await listReleases()) || []
  } finally {
    releaseLoading.value = false
  }
}

function platformLabel(p: string) {
  return { android: 'Android', harmony: 'HarmonyOS', ios: 'iOS', wechat_mp: '微信小程序码' }[p] || p
}
function platformTag(p: string) {
  return p === 'wechat_mp' ? 'success' : 'primary'
}
function formatSize(bytes: number) {
  if (!bytes) return '--'
  return bytes >= 1048576 ? (bytes / 1048576).toFixed(1) + ' MB' : Math.round(bytes / 1024) + ' KB'
}
function previewRelease(row: AppRelease) {
  window.open(`/api/public/download/app/${row.id}`, '_blank')
}

async function handleDelete(row: AppRelease) {
  await ElMessageBox.confirm(
    `确定删除「${platformLabel(row.platform)} ${row.version || row.name}」吗？删除后官网下载页将回退到上一条记录。`,
    '删除确认',
    { type: 'warning' }
  )
  await deleteRelease(row.id)
  ElMessage.success('已删除')
  fetchReleases()
}

// ========== 上传 ==========
const uploadVisible = ref(false)
const uploading = ref(false)
const uploadPercent = ref(0)
const uploadRef = ref<UploadInstance>()
const fileList = ref<UploadFile[]>([])
const chosenFile = ref<File | null>(null)
const uploadForm = reactive({ platform: 'android', version: '', note: '', force: false })

const acceptExt = computed(() => {
  return { android: '.apk', harmony: '.hap', ios: '.ipa', wechat_mp: '.png,.jpg,.jpeg' }[uploadForm.platform] || ''
})
const uploadTip = computed(() => {
  return uploadForm.platform === 'wechat_mp'
    ? '支持 PNG / JPG 格式的小程序码图片'
    : `仅支持 ${acceptExt.value} 格式，不超过 512MB`
})

function openUpload() {
  uploadForm.platform = 'android'
  uploadForm.version = ''
  uploadForm.note = ''
  uploadForm.force = false
  fileList.value = []
  chosenFile.value = null
  uploadPercent.value = 0
  uploadVisible.value = true
}

function handleFileChange(file: UploadFile, files: UploadFiles) {
  // 只保留最新选择的一个
  fileList.value = files.slice(-1)
  chosenFile.value = file.raw || null
}
function handleFileRemove() {
  fileList.value = []
  chosenFile.value = null
}

async function handleUpload() {
  if (!chosenFile.value) {
    ElMessage.warning('请选择文件')
    return
  }
  if (uploadForm.platform !== 'wechat_mp' && !uploadForm.version.trim()) {
    ElMessage.warning('请填写版本号')
    return
  }
  const form = new FormData()
  form.append('platform', uploadForm.platform)
  form.append('version', uploadForm.version.trim())
  form.append('note', uploadForm.note.trim())
  form.append('force', uploadForm.force ? 'true' : 'false')
  form.append('file', chosenFile.value)
  uploading.value = true
  uploadPercent.value = 0
  try {
    await uploadRelease(form, (p) => (uploadPercent.value = p))
    ElMessage.success('上传成功，官网下载页已更新')
    uploadVisible.value = false
    fetchReleases()
  } finally {
    uploading.value = false
  }
}

onMounted(() => {
  fetchReleases()
})
</script>

<style scoped lang="scss">
.card-title {
  font-size: $font-size-card-title;
  font-weight: 600;
  color: $color-text-primary;
}
.table-tip {
  margin-top: 10px;
  font-size: $font-size-aux;
  color: $color-text-secondary;
}
.form-tip {
  font-size: $font-size-aux;
  color: $color-text-secondary;
  line-height: 1.5;
}
</style>
