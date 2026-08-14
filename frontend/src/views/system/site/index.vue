<template>
  <div class="app-container">
    <!-- 页面配置 -->
    <div class="table-card">
      <div class="card-header">
        <span class="card-title">页面配置</span>
        <div class="card-header-extra">
          <el-button text type="primary" :icon="View" @click="openSite('/')">查看官网首页</el-button>
          <el-button text type="primary" :icon="Download" @click="openSite('/download')">查看下载页</el-button>
        </div>
      </div>
      <el-form v-loading="configLoading" :model="configForm" label-width="110px" style="max-width: 640px; margin-top: 16px">
        <el-form-item v-for="item in configItems" :key="item.key" :label="item.name">
          <div v-if="item.key === 'site.theme_color'" class="theme-field">
            <el-color-picker v-model="configForm[item.key]" />
            <el-input v-model="configForm[item.key]" style="width: 140px" placeholder="#2b5aed" />
            <span class="field-remark">{{ item.remark }}</span>
          </div>
          <div v-else-if="item.key === 'site.show_admin_entry'">
            <el-switch
              v-model="configForm[item.key]"
              active-value="true"
              inactive-value="false"
              active-text="显示"
              inactive-text="隐藏"
            />
            <div class="field-remark">{{ item.remark }}</div>
          </div>
          <el-input
            v-else-if="item.key === 'site.slogan' || item.key === 'site.footer_note'"
            v-model="configForm[item.key]"
            type="textarea"
            :rows="2"
            :placeholder="item.remark"
          />
          <el-input v-else v-model="configForm[item.key]" :placeholder="item.remark" />
        </el-form-item>
        <el-form-item>
          <el-button v-perms="'system:site:update'" type="primary" :loading="saving" @click="handleSaveConfig">保存配置</el-button>
          <span class="field-remark" style="margin-left: 12px">保存后官网约 1 分钟内生效</span>
        </el-form-item>
      </el-form>
    </div>

    <!-- 下载渠道 -->
    <div class="table-card" style="margin-top: 16px">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <span class="card-title" style="margin-right: 12px">下载渠道</span>
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
        <el-table-column label="版本" width="110">
          <template #default="{ row }">{{ row.version || '--' }}</template>
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
      <div class="table-tip">官网下载页每个平台只展示最新上传的一条记录。</div>
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
import { Plus, RefreshRight, View, Download, UploadFilled } from '@element-plus/icons-vue'
import {
  getSiteConfig, saveSiteConfig, listReleases, uploadRelease, deleteRelease,
  type SiteConfigItem, type AppRelease
} from '@/api/site'

// ========== 页面配置 ==========
const configItems = ref<SiteConfigItem[]>([])
const configForm = reactive<Record<string, string>>({})
const configLoading = ref(false)
const saving = ref(false)

async function fetchConfig() {
  configLoading.value = true
  try {
    const list = await getSiteConfig()
    configItems.value = list || []
    for (const item of configItems.value) configForm[item.key] = item.value
  } finally {
    configLoading.value = false
  }
}

async function handleSaveConfig() {
  saving.value = true
  try {
    await saveSiteConfig({ ...configForm })
    ElMessage.success('已保存，官网约 1 分钟内生效')
  } finally {
    saving.value = false
  }
}

function openSite(path: string) {
  window.open(path, '_blank')
}

// ========== 下载渠道 ==========
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
const uploadForm = reactive({ platform: 'android', version: '', note: '' })

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
  fetchConfig()
  fetchReleases()
})
</script>

<style scoped lang="scss">
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-title {
  font-size: $font-size-card-title;
  font-weight: 600;
  color: $color-text-primary;
}
.theme-field {
  display: flex;
  align-items: center;
  gap: 12px;
}
.field-remark {
  font-size: $font-size-aux;
  color: $color-text-secondary;
}
.table-tip {
  margin-top: 10px;
  font-size: $font-size-aux;
  color: $color-text-secondary;
}
</style>
