<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="标题">
          <el-input v-model="query.title" placeholder="公告标题" clearable style="width: 180px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 130px">
            <el-option label="草稿" :value="0" />
            <el-option label="已发布" :value="1" />
            <el-option label="已下线" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'system:notice:create'" type="primary" :icon="Plus" @click="openForm()">发布公告</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column label="内容摘要" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">{{ row.content }}</template>
        </el-table-column>
        <el-table-column prop="created_by" label="发布人" width="110" />
        <el-table-column prop="publish_at" label="发布时间" width="160">
          <template #default="{ row }">{{ row.publish_at || '--' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : row.status === 0 ? 'info' : 'warning'" size="small">
              {{ { 0: '草稿', 1: '已发布', 2: '已下线' }[row.status as number] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right" align="center">
          <template #default="{ row }">
            <div class="action-cell">
              <el-button link type="primary" @click="openPreview(row)">预览</el-button>
              <el-button v-perms="'system:notice:update'" link type="primary" @click="openForm(row)">编辑</el-button>
              <el-dropdown v-if="userStore.hasPerm('system:notice:update') || userStore.hasPerm('system:notice:delete')" trigger="click" @command="(cmd: string) => handleRowCommand(cmd, row)">
                <el-button link type="primary">更多<el-icon><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="row.status !== 1 && userStore.hasPerm('system:notice:update')" command="publish">发布</el-dropdown-item>
                    <el-dropdown-item v-if="row.status === 1 && userStore.hasPerm('system:notice:update')" command="offline">下线</el-dropdown-item>
                    <el-dropdown-item v-if="userStore.hasPerm('system:notice:delete')" command="delete">
                    <span class="danger-text">删除</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无公告">
            <el-button v-perms="'system:notice:create'" type="primary" @click="openForm()">发布公告</el-button>
          </el-empty>
        </template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.page_size"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 发布/编辑对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑公告' : '发布公告'" width="600px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" maxlength="64" show-word-limit placeholder="公告标题" />
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <el-input v-model="form.content" type="textarea" :rows="6" placeholder="公告正文（纯文本）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :loading="submitting" @click="handleSubmit(0)">存草稿</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit(1)">发布并推送</el-button>
      </template>
    </el-dialog>

    <!-- 预览：模拟小程序消息页样式 -->
    <el-dialog v-model="previewVisible" title="公告预览（小程序消息页效果）" width="400px">
      <div class="notice-preview">
        <div class="preview-title">{{ previewItem?.title }}</div>
        <div class="preview-meta">{{ previewItem?.created_by }} · {{ previewItem?.publish_at || '未发布' }}</div>
        <div class="preview-content">{{ previewItem?.content }}</div>
      </div>
      <template #footer>
        <el-button type="primary" @click="previewVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, ArrowDown } from '@element-plus/icons-vue'
import { listNotices, createNotice, updateNotice, deleteNotice } from '@/api/notice'
import { useUserStore } from '@/store/user'
import type { NoticeItem } from '@/api/biz-types'

const userStore = useUserStore()
const loading = ref(false)
const list = ref<NoticeItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, title: '', status: '' as number | '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listNotices({
      ...query,
      title: query.title || undefined,
      status: query.status === '' ? undefined : query.status
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.title = ''
  query.status = ''
  handleSearch()
}

onMounted(fetchList)

// ===== 发布/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', title: '', content: '' })

const formRules: FormRules = {
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  content: [{ required: true, message: '请输入内容', trigger: 'blur' }]
}

function openForm(row?: NoticeItem) {
  formRef.value?.clearValidate()
  Object.assign(form, row ? { id: row.id, title: row.title, content: row.content } : { id: '', title: '', content: '' })
  formVisible.value = true
}

async function handleSubmit(status: 0 | 1) {
  await formRef.value?.validate()
  // 发布动作二次确认
  if (status === 1) {
    const ok = await ElMessageBox.confirm('发布后将推送至巡检员小程序消息页，确定发布吗？', '发布确认', {
      confirmButtonText: '发布',
      cancelButtonText: '取消',
      type: 'warning'
    }).then(() => true).catch(() => false)
    if (!ok) return
  }
  submitting.value = true
  try {
    if (form.id) {
      await updateNotice(form.id, { title: form.title, content: form.content, status })
      ElMessage.success(status === 1 ? '已发布并推送' : '草稿已保存')
    } else {
      await createNotice({ title: form.title, content: form.content, status })
      ElMessage.success(status === 1 ? '已发布并推送' : '草稿已保存')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// ===== 行内操作：发布/下线/删除 =====
async function handleRowCommand(cmd: string, row: NoticeItem) {
  if (cmd === 'publish') {
    const ok = await ElMessageBox.confirm('发布后将推送至巡检员小程序消息页，确定发布吗？', '发布确认', {
      confirmButtonText: '发布', cancelButtonText: '取消', type: 'warning'
    }).then(() => true).catch(() => false)
    if (!ok) return
    await updateNotice(row.id, { status: 1 })
    ElMessage.success('已发布')
    fetchList()
  } else if (cmd === 'offline') {
    const ok = await ElMessageBox.confirm('下线后小程序端将不再展示该公告，确定下线吗？', '下线确认', {
      confirmButtonText: '下线', cancelButtonText: '取消', type: 'warning'
    }).then(() => true).catch(() => false)
    if (!ok) return
    await updateNotice(row.id, { status: 2 })
    ElMessage.success('已下线')
    fetchList()
  } else if (cmd === 'delete') {
    const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除公告「${row.title}」吗？`, '删除确认', {
      confirmButtonText: '删除', cancelButtonText: '取消', type: 'error'
    }).then(() => true).catch(() => false)
    if (!ok) return
    await deleteNotice(row.id)
    ElMessage.success('已删除')
    fetchList()
  }
}

// ===== 预览 =====
const previewVisible = ref(false)
const previewItem = ref<NoticeItem | null>(null)

function openPreview(row: NoticeItem) {
  previewItem.value = row
  previewVisible.value = true
}
</script>

<style scoped lang="scss">
.danger-text {
  color: $color-danger;
}

/* 操作列：链接按钮与下拉按钮纵向居中对齐 */
.action-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: $spacing-sm;

  :deep(.el-button + .el-button) {
    margin-left: 0;
  }

  :deep(.el-dropdown) {
    line-height: 1;
  }
}

// 模拟小程序消息页的公告卡片
.notice-preview {
  background: $color-bg-page;
  border-radius: $radius-card;
  padding: $spacing-lg;

  .preview-title {
    font-size: $font-size-card-title;
    font-weight: 600;
    color: $color-text-primary;
  }

  .preview-meta {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin: $spacing-xs 0 $spacing-md;
  }

  .preview-content {
    font-size: $font-size-body;
    color: $color-text-regular;
    line-height: 1.6;
    white-space: pre-wrap;
  }
}
</style>
