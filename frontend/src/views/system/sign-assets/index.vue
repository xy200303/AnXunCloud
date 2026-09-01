<template>
  <div class="app-container">

    <!-- 顶部 Tab：公章 / 用户签名 -->
    <div class="table-card sign-tabs-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="公章" name="company_seal" />
        <el-tab-pane label="用户签名" name="user_signature" />
      </el-tabs>
    </div>

    <!-- 搜索区：仅用户签名 Tab 显示 -->
    <div v-if="activeTab === 'user_signature'" class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="用户">
          <el-select v-model="query.owner_id" placeholder="全部用户" clearable filterable style="width: 160px">
            <el-option v-for="u in users" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 130px">
            <el-option label="生效中" value="active" />
            <el-option label="已替换" value="replaced" />
            <el-option label="已废止" value="revoked" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 公章 Tab：当前生效公章卡片 -->
    <div v-if="activeTab === 'company_seal'" class="table-card seal-current-card">
      <div class="seal-current-header">
        <span class="seal-current-title">当前生效公章</span>
        <el-button v-perms="'system:signasset:create'" type="primary" :icon="Upload" @click="openSealDialog">更换公章</el-button>
      </div>
      <div v-if="activeSeal" class="seal-current-body">
        <el-image
          :src="assetUrl(activeSeal)"
          fit="contain"
          class="seal-current-img"
          :preview-src-list="[assetUrl(activeSeal)]"
          preview-teleported
        />
        <div class="seal-current-meta">
          <div class="meta-row"><span class="meta-label">版本</span><span>v{{ activeSeal.version }}</span></div>
          <div class="meta-row">
            <span class="meta-label">SHA256</span>
            <span>{{ sha256Short(activeSeal.sha256) }}</span>
          </div>
          <div class="meta-row"><span class="meta-label">生效时间</span><span>{{ activeSeal.created_at || '--' }}</span></div>
          <div class="meta-row"><span class="meta-label">备注</span><span>{{ activeSeal.remark || '--' }}</span></div>
        </div>
      </div>
      <el-empty v-else description="暂无生效公章，请上传" :image-size="80" />
    </div>

    <!-- 表格：公章历史版本 / 用户签名列表 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <span v-if="activeTab === 'company_seal'" class="toolbar-title">历史版本</span>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column v-if="activeTab === 'user_signature'" label="用户" min-width="120">
          <template #default="{ row }">{{ row.owner_name || row.owner_id || '--' }}</template>
        </el-table-column>
        <el-table-column label="签名/公章" width="110" align="center">
          <template #default="{ row }">
            <el-image
              v-if="assetUrl(row)"
              :src="assetUrl(row)"
              fit="contain"
              class="asset-thumb"
              :preview-src-list="[assetUrl(row)]"
              preview-teleported
            />
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column label="版本" width="80" align="center">
          <template #default="{ row }">v{{ row.version }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status).type" size="small">{{ statusTag(row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="SHA256" min-width="110">
          <template #default="{ row }">{{ sha256Short(row.sha256) }}</template>
        </el-table-column>
        <el-table-column label="上传人" width="110">
          <template #default="{ row }">{{ row.created_by || '--' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="上传时间" width="160">
          <template #default="{ row }">{{ row.created_at || '--' }}</template>
        </el-table-column>
        <el-table-column label="废止信息" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.status === 'revoked'">{{ row.revoked_reason || '--' }}（{{ row.revoked_at || '--' }}）</span>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || '--' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'active'"
              v-perms="'system:signasset:revoke'"
              link
              type="danger"
              @click="handleRevoke(row)"
            >废止</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty :description="activeTab === 'company_seal' ? '暂无公章版本记录' : '暂无签名资产'" />
        </template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.page_size"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 更换公章对话框：上传图片拿 file_id 后创建资产 -->
    <el-dialog v-model="sealDialogVisible" title="更换公章" width="480px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
        上传后原公章自动转为「已替换」，全局仅保留一条生效公章
      </el-alert>
      <el-form label-width="88px">
        <el-form-item label="公章图片" required>
          <div class="seal-uploader">
            <div v-if="sealForm.file_id" class="seal-preview">
              <el-image :src="withFileToken(sealForm.url)" fit="contain" class="seal-img" />
              <el-button type="danger" link @click="sealForm.file_id = ''">移除</el-button>
            </div>
            <el-upload
              :auto-upload="false"
              :show-file-list="false"
              accept=".png,.jpg,.jpeg"
              :on-change="handleSealChange"
            >
              <el-button :loading="sealUploading">{{ sealForm.file_id ? '重新选择' : '选择图片' }}</el-button>
            </el-upload>
            <div class="text-secondary">PNG/JPG，不超过 2MB</div>
          </div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="sealForm.remark" type="textarea" :rows="2" placeholder="选填，如：2026 年度公章" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sealDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="sealSubmitting" @click="submitSeal">确定更换</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, RefreshRight, Upload } from '@element-plus/icons-vue'
import { listSignAssets, createSignAsset, revokeSignAsset, type SignAssetItem, type SignAssetType, type SignAssetStatus } from '@/api/sign-asset'
import { listUsers } from '@/api/user'
import { uploadImage, fileUrl, withFileToken } from '@/api/upload'
import type { UserItem } from '@/api/types'
import type { UploadFile } from 'element-plus'


const activeTab = ref<SignAssetType>('company_seal')
const loading = ref(false)
const list = ref<SignAssetItem[]>([])
const total = ref(0)
const users = ref<UserItem[]>([])

const query = reactive({
  page: 1,
  page_size: 20,
  owner_id: undefined as string | undefined,
  status: '' as SignAssetStatus | ''
})

// 当前生效公章：从列表结果中推导（status=active 的 company_seal 全局至多一条）
const activeSeal = computed(() =>
  activeTab.value === 'company_seal' ? list.value.find((a) => a.status === 'active') ?? null : null
)

function statusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      active: { label: '生效中', type: 'success' },
      replaced: { label: '已替换', type: 'info' },
      revoked: { label: '已废止', type: 'danger' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

// 图片地址：优先用接口返回的 url（敏感场景为 /api/files 鉴权地址，附 ?token=），缺省时按资产记录的 file_key（存储路径）拼静态路径
function assetUrl(row: SignAssetItem) {
  return withFileToken(row.url || fileUrl(row.file_key || ''))
}

function sha256Short(v?: string | null) {
  return v ? `${v.slice(0, 8)}…` : '--'
}

async function fetchList() {
  loading.value = true
  try {
    const data = await listSignAssets({
      page: query.page,
      page_size: query.page_size,
      asset_type: activeTab.value,
      owner_id: activeTab.value === 'user_signature' ? query.owner_id : undefined,
      status: activeTab.value === 'user_signature' ? query.status || undefined : undefined
    })
    list.value = data?.list ?? []
    total.value = data?.total ?? 0
  } catch {
    // 接口未就绪（并行开发中）或异常时置空，错误提示由请求拦截器统一弹出
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleTabChange() {
  query.page = 1
  fetchList()
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.owner_id = undefined
  query.status = ''
  handleSearch()
}

onMounted(async () => {
  fetchList()
  fetchUsers()
})

// 用户候选（签名属主筛选，随租户上下文刷新）
async function fetchUsers() {
  try {
    const data = await listUsers({ page: 1, page_size: 100, status: 1 })
    users.value = data.list
  } catch {
    users.value = []
  }
}

// ===== 废止（reason 必填） =====
async function handleRevoke(row: SignAssetItem) {
  let reason = ''
  try {
    const res = await ElMessageBox.prompt(
      `废止后不可恢复，确定废止该${activeTab.value === 'company_seal' ? '公章' : '签名'}（v${row.version}）吗？`,
      '废止确认',
      {
        confirmButtonText: '废止',
        cancelButtonText: '取消',
        inputPlaceholder: '请输入废止原因',
        inputValidator: (v: string) => (v && v.trim() ? true : '废止原因不能为空')
      }
    )
    reason = res.value.trim()
  } catch {
    return
  }
  await revokeSignAsset(row.id, reason)
  ElMessage.success('已废止')
  fetchList()
}

// ===== 更换公章 =====
const sealDialogVisible = ref(false)
const sealUploading = ref(false)
const sealSubmitting = ref(false)
const sealForm = reactive({ file_id: '', url: '', remark: '' })

function openSealDialog() {
  sealForm.file_id = ''
  sealForm.url = ''
  sealForm.remark = ''
  sealDialogVisible.value = true
}

async function handleSealChange(uploadFile: UploadFile) {
  const raw = uploadFile.raw
  if (!raw) return
  if (!['image/png', 'image/jpeg'].includes(raw.type)) {
    ElMessage.warning('仅支持 PNG/JPG 格式图片')
    return
  }
  if (raw.size > 2 * 1024 * 1024) {
    ElMessage.warning('图片大小不能超过 2MB')
    return
  }
  sealUploading.value = true
  try {
    const { file_id, url } = await uploadImage(raw, 'seal')
    sealForm.file_id = file_id
    sealForm.url = url
    ElMessage.success('图片已上传')
  } catch {
    // 错误提示由请求拦截器统一弹出
  } finally {
    sealUploading.value = false
  }
}

async function submitSeal() {
  if (!sealForm.file_id) {
    ElMessage.warning('请先上传公章图片')
    return
  }
  sealSubmitting.value = true
  try {
    await createSignAsset({
      asset_type: 'company_seal',
      file_id: sealForm.file_id,
      remark: sealForm.remark.trim() || undefined
    })
    ElMessage.success('公章已更换')
    sealDialogVisible.value = false
    query.page = 1
    fetchList()
  } catch {
    // 错误提示由请求拦截器统一弹出
  } finally {
    sealSubmitting.value = false
  }
}
</script>

<style lang="scss" scoped>
.sign-tabs-card {
  padding-bottom: 0;
  margin-bottom: $spacing-md;

  :deep(.el-tabs) {
    padding: 0 $spacing-md;
  }

  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }
}

.seal-current-card {
  margin-bottom: $spacing-lg;

  .seal-current-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: $spacing-md;

    .seal-current-title {
      font-weight: 600;
    }
  }

  .seal-current-body {
    display: flex;
    gap: $spacing-lg;

    .seal-current-img {
      width: 160px;
      height: 160px;
      background: $color-white;
      border: 1px solid $color-border;
      border-radius: $radius-card;
      cursor: pointer;
      flex-shrink: 0;
    }

    .seal-current-meta {
      display: flex;
      flex-direction: column;
      gap: $spacing-sm;

      .meta-row {
        display: flex;
        gap: $spacing-md;

        .meta-label {
          color: $color-text-secondary;
          width: 64px;
          flex-shrink: 0;
        }
      }
    }
  }
}

.toolbar-title {
  font-weight: 600;
}

.asset-thumb {
  width: 48px;
  height: 48px;
  background: $color-white;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  cursor: pointer;
  vertical-align: middle;
}

.seal-uploader {
  width: 100%;

  .seal-preview {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    margin-bottom: $spacing-sm;

    .seal-img {
      width: 120px;
      height: 120px;
      background: $color-white;
      border: 1px solid $color-border;
      border-radius: $radius-card;
    }
  }
}
</style>
