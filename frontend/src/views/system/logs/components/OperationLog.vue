<template>
  <div class="log-panel">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="操作人">
          <el-input v-model="query.username" placeholder="操作人账号" clearable style="width: 140px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="模块">
          <el-select v-model="query.module" placeholder="全部模块" clearable style="width: 130px">
            <el-option v-for="m in moduleOptions" :key="m.value" :label="m.label" :value="m.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="动作">
          <el-select v-model="query.action" placeholder="全部动作" clearable style="width: 130px">
            <el-option v-for="a in actionOptions" :key="a.value" :label="a.label" :value="a.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部" clearable style="width: 110px">
            <el-option label="成功" :value="1" />
            <el-option label="失败" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间">
          <el-date-picker
            v-model="timeRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 340px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 只读表格：无新增/编辑/删除入口 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button :icon="Document" :loading="exporting" @click="handleExport">导出</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="created_at" label="时间" width="160" />
        <el-table-column label="操作人" width="110">
          <template #default="{ row }">{{ row.username && row.username !== '-' ? row.username : '--' }}</template>
        </el-table-column>
        <el-table-column prop="module" label="模块" width="100">
          <template #default="{ row }">
            {{ moduleOptions.find((m) => m.value === row.module)?.label || row.module }}
          </template>
        </el-table-column>
        <el-table-column label="动作" width="100" show-overflow-tooltip>
          <template #default="{ row }">{{ row.action_name || row.action }}</template>
        </el-table-column>
        <el-table-column prop="method" label="请求方式" width="90" align="center" />
        <el-table-column prop="path" label="路径" min-width="220" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="120" />
        <el-table-column prop="cost_ms" label="耗时" width="80" align="right">
          <template #default="{ row }">{{ row.cost_ms }}ms</template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该条件下暂无操作日志" />
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

    <!-- 详情对话框：请求参数 JSON 格式化展示 -->
    <el-dialog v-model="detailVisible" title="操作详情" width="560px">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="操作人">{{ current?.username && current.username !== '-' ? current.username : '--' }}</el-descriptions-item>
        <el-descriptions-item label="时间">{{ current?.created_at }}</el-descriptions-item>
        <el-descriptions-item label="模块">{{ moduleOptions.find((m) => m.value === current?.module)?.label || current?.module }}</el-descriptions-item>
        <el-descriptions-item label="动作">{{ current?.action_name || current?.action }}</el-descriptions-item>
        <el-descriptions-item label="请求">{{ current?.method }} {{ current?.path }}</el-descriptions-item>
        <el-descriptions-item label="IP">{{ current?.ip }}</el-descriptions-item>
      </el-descriptions>
      <div class="params-title">请求参数</div>
      <pre class="params-json">{{ prettyParams }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Document, RefreshRight } from '@element-plus/icons-vue'
import { listOperationLogs, type LogQuery } from '@/api/log'
import { downloadFile } from '@/utils/download'
import type { OperationLog } from '@/api/types'


// 动作枚举：登录类（login/logout/refresh/register）已由后端从操作日志剔除，见登录日志 Tab
const actionOptions = [
  { label: '新增', value: 'create' },
  { label: '修改', value: 'update' },
  { label: '删除', value: 'delete' },
  { label: '导出', value: 'export' },
  { label: '导入', value: 'import' },
  { label: '重置密码', value: 'reset_password' },
  { label: '修改密码', value: 'change_password' },
  { label: '生成任务', value: 'generate' },
  { label: '派单', value: 'dispatch' },
  { label: '验收', value: 'confirm' }
]

const moduleOptions = [
  { label: '系统管理', value: 'system' },
  { label: '小区管理', value: 'community' },
  { label: '巡检管理', value: 'inspection' },
  { label: '异常工单', value: 'workorder' },
  { label: '统计分析', value: 'stats' },
  { label: '小程序', value: 'mp' }
]

const loading = ref(false)
const list = ref<OperationLog[]>([])
const total = ref(0)
const timeRange = ref<[string, string] | null>(null)
const query = reactive<LogQuery & { module?: string; action?: string }>({
  page: 1, page_size: 20, username: '', module: '', action: '', status: ''
})

async function fetchList() {
  loading.value = true
  try {
    const data = await listOperationLogs({
      ...query,
      username: query.username || undefined,
      module: query.module || undefined,
      action: query.action || undefined,
      status: query.status === '' ? undefined : query.status,
      start_time: timeRange.value?.[0],
      end_time: timeRange.value?.[1]
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
  query.username = ''
  query.module = ''
  query.action = ''
  query.status = ''
  timeRange.value = null
  handleSearch()
}

onMounted(fetchList)


// ===== 详情 =====
const detailVisible = ref(false)
const current = ref<OperationLog | null>(null)

const prettyParams = computed(() => {
  if (!current.value?.params) return '（无）'
  try {
    return JSON.stringify(JSON.parse(current.value.params), null, 2)
  } catch {
    return current.value.params
  }
})

function openDetail(row: OperationLog) {
  current.value = row
  detailVisible.value = true
}

// ===== 导出 =====
const exporting = ref(false)

async function handleExport() {
  exporting.value = true
  try {
    await downloadFile('/stats/export', { type: 'operation_log' }, 'operation_logs.xlsx')
    ElMessage.success('导出任务已提交，请稍后在消息中心查看')
  } catch {
    // 拦截器已提示
  } finally {
    exporting.value = false
  }
}
</script>

<style scoped lang="scss">
.params-title {
  font-weight: 600;
  margin: $spacing-lg 0 $spacing-sm;
}

.params-json {
  background: $color-bg-page;
  border-radius: $radius-small;
  padding: $spacing-md;
  font-size: $font-size-aux;
  color: $color-text-regular;
  max-height: 240px;
  overflow: auto;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
