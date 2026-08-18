<template>
  <div class="app-container">
    <!-- 筛选区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="任务日期">
          <el-date-picker v-model="query.task_date" type="date" value-format="YYYY-MM-DD" placeholder="默认今天" style="width: 150px" @change="fetchList" />
        </el-form-item>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 150px" @change="fetchList">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡检员">
          <el-select v-model="query.inspector_id" placeholder="全部" clearable filterable style="width: 130px" @change="fetchList">
            <el-option v-for="u in inspectors" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡查类型">
          <el-select v-model="query.patrol_type" placeholder="全部" clearable style="width: 140px" @change="fetchList">
            <el-option v-for="o in PATROL_TYPE_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="fetchList">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <!-- 状态筛选 Tab -->
      <div class="monitor-toolbar">
        <el-radio-group v-model="activeTab" @change="fetchList">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="doing">进行中</el-radio-button>
          <el-radio-button value="done">已完成</el-radio-button>
          <el-radio-button value="missing">有缺卡</el-radio-button>
          <el-radio-button value="abnormal">有异常</el-radio-button>
          <el-radio-button value="suspect">疑似作弊</el-radio-button>
        </el-radio-group>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <!-- 任务卡片列表 -->
      <div v-loading="loading" class="task-cards">
        <template v-if="list.length">
          <div v-for="task in list" :key="task.id" class="task-card" @click="goDetail(task)">
            <div class="task-card-head">
              <span class="task-name">{{ task.community_name }} · {{ task.plan_name }}</span>
              <el-tag size="small" effect="plain">{{ patrolTypeLabel(task.patrol_type) }}</el-tag>
              <span class="task-inspector">{{ task.inspector_name }}</span>
              <el-tag :type="statusType(task)" size="small">
                <el-icon class="status-icon"><component :is="statusIcon(task)" /></el-icon>
                {{ statusLabel(task) }}
              </el-tag>
            </div>
            <div class="task-card-body">
              <el-progress
                :percentage="task.progress"
                :stroke-width="10"
                :status="task.progress >= 100 ? 'success' : undefined"
                class="task-progress"
              />
              <span class="task-fraction">{{ task.done_points }}/{{ task.total_points }}</span>
            </div>
            <div class="task-card-foot">
              <span class="text-secondary">时段 {{ task.time_window }}</span>
              <span v-if="task.missing_count > 0" class="warn-flag">
                <el-icon><Warning /></el-icon>缺卡 {{ task.missing_count }}
              </span>
              <span v-if="task.abnormal_count > 0" class="danger-flag">
                <el-icon><CircleClose /></el-icon>异常 {{ task.abnormal_count }}
              </span>
              <span v-if="task.suspect_count > 0" class="danger-flag">
                <el-icon><Flag /></el-icon>疑似作弊 {{ task.suspect_count }}
              </span>
              <el-button link type="primary" class="detail-link">查看明细</el-button>
            </div>
          </div>
        </template>
        <el-empty v-else-if="!loading" description="该条件下暂无任务">
          <el-button v-perms="'inspection:task:generate'" type="primary" @click="handleGenerate">生成今日任务</el-button>
        </el-empty>
      </div>

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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search, Refresh, RefreshRight, Warning, CircleClose, Flag, Loading, CircleCheck, Clock } from '@element-plus/icons-vue'
import { listTasks, generateTasks, type TaskQuery } from '@/api/task'
import { listCommunities } from '@/api/community'
import { listUsers } from '@/api/user'
import type { TaskItem, CommunityItem } from '@/api/biz-types'
import { PATROL_TYPE_OPTIONS, patrolTypeLabel } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const router = useRouter()
const loading = ref(false)
const list = ref<TaskItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectors = ref<UserItem[]>([])
const activeTab = ref('all')

const query = reactive<TaskQuery>({ page: 1, page_size: 20, task_date: '', community_id: undefined, inspector_id: undefined, patrol_type: '' })

async function fetchList() {
  loading.value = true
  try {
    // Tab 与 status/filter 参数映射（接口文档 §2.13.1）
    const params: TaskQuery = { ...query, task_date: query.task_date || undefined, patrol_type: query.patrol_type || undefined }
    if (activeTab.value === 'doing') params.status = 'doing'
    else if (activeTab.value === 'done') params.status = 'done'
    else if (activeTab.value !== 'all') params.filter = activeTab.value as TaskQuery['filter']
    const data = await listTasks(params)
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleReset() {
  query.task_date = ''
  query.community_id = undefined
  query.inspector_id = undefined
  query.patrol_type = ''
  activeTab.value = 'all'
  query.page = 1
  fetchList()
}

onMounted(async () => {
  fetchList()
  const [cData, uData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 })
  ])
  communities.value = cData.list
  inspectors.value = uData.list
})

// 状态色 + 图标双编码
function statusLabel(task: TaskItem) {
  if (task.suspect_count > 0) return '疑似作弊'
  if (task.abnormal_count > 0) return '有异常'
  if (task.status === 'done') return '已完成'
  if (task.missing_count > 0) return '有缺卡'
  return { doing: '进行中', pending: '待开始', overdue: '已逾期' }[task.status] || task.status
}

function statusType(task: TaskItem) {
  if (task.suspect_count > 0 || task.abnormal_count > 0) return 'danger'
  if (task.status === 'done') return 'success'
  if (task.missing_count > 0 || task.status === 'overdue') return 'warning'
  return 'primary'
}

function statusIcon(task: TaskItem) {
  if (task.suspect_count > 0) return Flag
  if (task.abnormal_count > 0) return CircleClose
  if (task.status === 'done') return CircleCheck
  if (task.status === 'doing') return Loading
  return Clock
}

function goDetail(task: TaskItem) {
  router.push(`/inspection/tasks/detail/${task.id}`)
}

// 空态主操作：手动生成今日任务
async function handleGenerate() {
  const res = await generateTasks()
  ElMessage.success(res.created > 0 ? `已生成 ${res.created} 个任务` : '今日任务已存在，无需重复生成')
  fetchList()
}
</script>

<style scoped lang="scss">
.monitor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: $spacing-lg;
}

.task-cards {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
  min-height: 200px;
}

.task-card {
  border: 1px solid $color-border;
  border-radius: $radius-card;
  padding: $spacing-lg;
  cursor: pointer;
  transition: border-color 0.2s;

  &:hover {
    border-color: $color-primary-hover;
  }

  .task-card-head {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    margin-bottom: $spacing-md;

    .task-name {
      font-weight: 600;
      color: $color-text-primary;
    }

    .task-inspector {
      color: $color-text-secondary;
      font-size: $font-size-aux;
    }

    .el-tag {
      margin-left: auto;
    }

    .status-icon {
      vertical-align: -2px;
      margin-right: 2px;
    }
  }

  .task-card-body {
    display: flex;
    align-items: center;
    gap: $spacing-md;

    .task-progress {
      flex: 1;
    }

    .task-fraction {
      color: $color-text-regular;
      font-size: $font-size-aux;
    }
  }

  .task-card-foot {
    display: flex;
    align-items: center;
    gap: $spacing-lg;
    margin-top: $spacing-sm;

    .warn-flag {
      display: inline-flex;
      align-items: center;
      gap: $spacing-xs;
      color: $color-warning;
      font-size: $font-size-aux;
    }

    .danger-flag {
      display: inline-flex;
      align-items: center;
      gap: $spacing-xs;
      color: $color-danger;
      font-size: $font-size-aux;
    }

    .detail-link {
      margin-left: auto;
    }
  }
}
</style>
