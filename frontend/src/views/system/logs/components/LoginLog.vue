<template>
  <div class="log-panel">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="账号">
          <el-input v-model="query.username" placeholder="登录账号" clearable style="width: 140px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="IP">
          <el-input v-model="query.ip" placeholder="登录 IP" clearable style="width: 140px" @keyup.enter="handleSearch" />
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

    <!-- 只读表格 -->
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
        <el-table-column prop="username" label="账号" min-width="130" show-overflow-tooltip />
        <el-table-column label="来源" width="90" align="center">
          <template #default="{ row }">
            <!-- 后端字段为 channel：admin 后台 / mp 小程序 -->
            <el-tag :type="(row.channel || row.source) === 'mp' || row.source === '小程序' ? 'success' : 'primary'" size="small">
              {{ (row.channel || row.source) === 'mp' ? '小程序' : row.source || '后台' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="ua" label="UA" min-width="240" show-overflow-tooltip />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <!-- 失败记录橙色警示，提示关注 -->
            <el-tag :type="row.status === 1 ? 'success' : 'warning'" size="small">
              {{ row.status === 1 ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="msg" label="提示信息" min-width="150" show-overflow-tooltip />
        <template #empty>
          <el-empty description="该条件下暂无登录日志" />
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Document, RefreshRight } from '@element-plus/icons-vue'
import { listLoginLogs, type LogQuery } from '@/api/log'
import { downloadFile } from '@/utils/download'
import type { LoginLog } from '@/api/types'


const loading = ref(false)
const list = ref<LoginLog[]>([])
const total = ref(0)
const timeRange = ref<[string, string] | null>(null)
const query = reactive<LogQuery & { ip?: string }>({
  page: 1, page_size: 20, username: '', ip: '', status: ''
})

async function fetchList() {
  loading.value = true
  try {
    const data = await listLoginLogs({
      ...query,
      username: query.username || undefined,
      ip: query.ip || undefined,
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
  query.ip = ''
  query.status = ''
  timeRange.value = null
  handleSearch()
}

onMounted(fetchList)


const exporting = ref(false)

async function handleExport() {
  exporting.value = true
  try {
    await downloadFile('/stats/export', { type: 'login_log' }, 'login_logs.xlsx')
    ElMessage.success('导出任务已提交，请稍后在消息中心查看')
  } catch {
    // 拦截器已提示
  } finally {
    exporting.value = false
  }
}
</script>
