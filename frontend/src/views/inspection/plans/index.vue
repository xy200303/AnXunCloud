<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 150px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="计划名称">
          <el-input v-model="query.name" placeholder="计划名称" clearable style="width: 150px" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="周期">
          <el-select v-model="query.cycle_type" placeholder="全部" clearable style="width: 110px">
            <el-option label="每天" value="daily" />
            <el-option label="每周" value="weekly" />
            <el-option label="每月" value="monthly" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部" clearable style="width: 110px">
            <el-option label="启用" :value="1" />
            <el-option label="停用" :value="0" />
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
          <el-button v-perms="'inspection:plan:create'" type="primary" :icon="Plus" @click="openForm()">新增计划</el-button>
          <el-button v-perms="'inspection:task:generate'" :icon="VideoPlay" :loading="generating" @click="handleGenerate">
            生成今日任务
          </el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="计划名称" min-width="140" show-overflow-tooltip />
        <el-table-column prop="community_name" label="小区" min-width="120" />
        <el-table-column label="周期" width="130">
          <template #default="{ row }">{{ cycleLabel(row) }}</template>
        </el-table-column>
        <el-table-column label="巡检员" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.inspector_names?.join('、') || '--' }}</template>
        </el-table-column>
        <el-table-column prop="point_count" label="点位数" width="80" align="right" />
        <el-table-column prop="time_window" label="执行时段" width="120" align="center" />
        <el-table-column label="有效期" width="200" align="center">
          <template #default="{ row }">{{ row.start_date }} ~ {{ row.end_date }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'inspection:plan:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleRowCommand(cmd, row)">
              <el-button link type="primary">更多<el-icon><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="userStore.hasPerm('inspection:plan:disable')" command="status">
                    {{ row.status === 1 ? '停用' : '启用' }}
                  </el-dropdown-item>
                  <el-dropdown-item v-if="userStore.hasPerm('inspection:plan:delete')" command="delete">
                    <span class="danger-text">删除</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无巡检计划">
            <el-button v-perms="'inspection:plan:create'" type="primary" @click="openForm()">新增计划</el-button>
          </el-empty>
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

    <!-- 新增/编辑对话框（字段多，用对话框） -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑巡检计划' : '新增巡检计划'" width="720px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="96px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="计划名称" prop="name">
              <el-input v-model="form.name" placeholder="如：翡翠湾日常巡检" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属小区" prop="community_id">
              <el-select v-model="form.community_id" placeholder="选择小区" style="width: 100%" :disabled="!!form.id" @change="handleFormCommunityChange">
                <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 巡检路线：可选点位 + 已选点位（有序，可上下移动） -->
        <el-form-item label="巡检路线" prop="point_ids">
          <div class="route-picker">
            <div class="route-col">
              <div class="route-col-title">已选点位（按顺序）</div>
              <div class="route-list">
                <div v-for="(p, i) in selectedPoints" :key="p.id" class="route-item">
                  <span class="route-order">{{ i + 1 }}.</span>
                  <span class="route-name" :title="`${p.building_name} / ${p.name}`">{{ p.building_name }} / {{ p.name }}</span>
                  <el-button link size="small" :icon="Top" :disabled="i === 0" @click="movePoint(i, -1)" />
                  <el-button link size="small" :icon="Bottom" :disabled="i === selectedPoints.length - 1" @click="movePoint(i, 1)" />
                  <el-button link size="small" type="danger" :icon="Close" @click="removePoint(i)" />
                </div>
                <el-empty v-if="!selectedPoints.length" description="尚未选择点位" :image-size="48" />
              </div>
              <div v-if="routeError" class="route-error">{{ routeError }}</div>
            </div>
            <div class="route-col">
              <div class="route-col-title">可选点位（{{ candidatePoints.length }}）</div>
              <div class="route-list">
                <el-checkbox-group v-model="checkedCandidateIds">
                  <div v-for="p in candidatePoints" :key="p.id" class="route-item">
                    <el-checkbox :value="p.id" :disabled="form.point_ids.includes(p.id)">
                      <span class="route-name" :title="`${p.building_name} / ${p.name}`">{{ p.building_name }} / {{ p.name }}</span>
                    </el-checkbox>
                  </div>
                </el-checkbox-group>
                <el-empty v-if="!form.community_id" description="请先选择小区" :image-size="48" />
              </div>
              <el-button size="small" type="primary" plain :disabled="!checkedCandidateIds.length" @click="addPoints">
                加入路线
              </el-button>
            </div>
          </div>
        </el-form-item>

        <!-- 周期联动 -->
        <el-form-item label="周期" prop="cycle_type">
          <el-radio-group v-model="form.cycle_type">
            <el-radio value="daily">每天</el-radio>
            <el-radio value="weekly">每周</el-radio>
            <el-radio value="monthly">每月</el-radio>
          </el-radio-group>
          <el-select
            v-if="form.cycle_type === 'weekly'"
            v-model="form.cycle_config.weekdays"
            multiple
            placeholder="选择星期"
            style="width: 320px; margin-left: 12px"
          >
            <el-option v-for="(w, i) in ['周一', '周二', '周三', '周四', '周五', '周六', '周日']" :key="i + 1" :label="w" :value="i + 1" />
          </el-select>
          <el-select
            v-if="form.cycle_type === 'monthly'"
            v-model="form.cycle_config.days"
            multiple
            placeholder="选择日期"
            style="width: 320px; margin-left: 12px"
          >
            <el-option v-for="d in 31" :key="d" :label="`${d} 日`" :value="d" />
          </el-select>
        </el-form-item>

        <el-form-item label="执行时段" prop="timeRange">
          <el-time-picker
            v-model="form.timeRange"
            is-range
            format="HH:mm"
            value-format="HH:mm"
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
          />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="巡检员" prop="inspector_ids">
              <el-select v-model="form.inspector_ids" multiple placeholder="多人按日轮转" style="width: 100%">
                <el-option v-for="u in inspectorOptions" :key="u.id" :label="u.name" :value="u.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="有效期" prop="dateRange">
              <el-date-picker
                v-model="form.dateRange"
                type="daterange"
                value-format="YYYY-MM-DD"
                range-separator="至"
                start-placeholder="生效日期"
                end-placeholder="截止日期"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Search, Refresh, Plus, RefreshRight, ArrowDown, Top, Bottom, Close, VideoPlay
} from '@element-plus/icons-vue'
import { listPlans, getPlan, createPlan, updatePlan, deletePlan, updatePlanStatus } from '@/api/plan'
import { generateTasks } from '@/api/task'
import { listCommunities } from '@/api/community'
import { listPoints } from '@/api/point'
import { listUsers } from '@/api/user'
import { useUserStore } from '@/store/user'
import type { PlanItem, CommunityItem, PointItem } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const userStore = useUserStore()

// ===== 列表 =====
const loading = ref(false)
const list = ref<PlanItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectorOptions = ref<UserItem[]>([])
const query = reactive({ page: 1, page_size: 20, community_id: undefined as string | undefined, name: '', cycle_type: '', status: '' as number | '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listPlans({
      ...query,
      name: query.name || undefined,
      cycle_type: query.cycle_type || undefined,
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
  query.community_id = undefined
  query.name = ''
  query.cycle_type = ''
  query.status = ''
  handleSearch()
}

onMounted(async () => {
  fetchList()
  const [cData, uData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 })
  ])
  communities.value = cData.list
  inspectorOptions.value = uData.list
})

function cycleLabel(row: PlanItem) {
  if (row.cycle_type === 'daily') return '每天'
  if (row.cycle_type === 'weekly') {
    const names = ['', '一', '二', '三', '四', '五', '六', '日']
    return `每周${(row.cycle_config?.weekdays || []).map((w) => names[w]).join('、')}`
  }
  if (row.cycle_type === 'monthly') {
    return `每月 ${(row.cycle_config?.days || []).join('、')} 日`
  }
  return row.cycle_type
}

// ===== 行内操作 =====
async function handleRowCommand(cmd: string, row: PlanItem) {
  if (cmd === 'status') {
    const target = row.status === 1 ? 0 : 1
    const action = target === 0 ? '停用' : '启用'
    const ok = await ElMessageBox.confirm(
      target === 0
        ? '停用后次日起不再生成任务，已生成任务不受影响，确定停用吗？'
        : '启用后将按周期生成任务，确定启用吗？',
      `${action}确认`,
      { confirmButtonText: action, cancelButtonText: '取消', type: 'warning' }
    ).then(() => true).catch(() => false)
    if (!ok) return
    await updatePlanStatus(row.id, target)
    ElMessage.success(`已${action}`)
    fetchList()
  } else if (cmd === 'delete') {
    const ok = await ElMessageBox.confirm(
      `删除计划「${row.name}」将同时取消未开始的任务，确定删除吗？`,
      '删除确认',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
    ).then(() => true).catch(() => false)
    if (!ok) return
    await deletePlan(row.id)
    ElMessage.success('已删除')
    fetchList()
  }
}

// ===== 手动生成今日任务 =====
const generating = ref(false)

async function handleGenerate() {
  generating.value = true
  try {
    const res = await generateTasks()
    if (res.created > 0) {
      ElMessage.success(`已生成 ${res.created} 个任务（${res.date}）`)
    } else if (res.eligible_plans === 0) {
      ElMessage.warning(`${res.date} 没有需要执行的启用计划，请先在上方新增计划`)
    } else {
      ElMessage.info(`${res.date} 任务已存在，无需重复生成`)
    }
  } finally {
    generating.value = false
  }
}

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const routeError = ref('')

const form = reactive({
  id: '',
  name: '',
  community_id: null as string | null,
  point_ids: [] as string[],
  cycle_type: 'daily',
  cycle_config: {} as { weekdays?: number[]; days?: number[] },
  timeRange: null as [string, string] | null,
  inspector_ids: [] as string[],
  dateRange: null as [string, string] | null,
  status: 1
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入计划名称', trigger: 'blur' }],
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  cycle_type: [{ required: true, message: '请选择周期', trigger: 'change' }],
  timeRange: [{ required: true, message: '请选择执行时段', trigger: 'change' }],
  inspector_ids: [{ required: true, type: 'array', min: 1, message: '请选择巡检员', trigger: 'change' }],
  dateRange: [{ required: true, message: '请选择有效期', trigger: 'change' }]
}

// 路线选择器
const candidatePoints = ref<PointItem[]>([])
const checkedCandidateIds = ref<string[]>([])

const selectedPoints = computed(() =>
  form.point_ids
    .map((id) => candidatePoints.value.find((p) => p.id === id))
    .filter(Boolean) as PointItem[]
)

async function handleFormCommunityChange() {
  form.point_ids = []
  checkedCandidateIds.value = []
  routeError.value = ''
  if (!form.community_id) {
    candidatePoints.value = []
    return
  }
  const data = await listPoints({ community_id: form.community_id, status: 1, page: 1, page_size: 100 })
  candidatePoints.value = data.list
}

function addPoints() {
  for (const id of checkedCandidateIds.value) {
    if (!form.point_ids.includes(id)) form.point_ids.push(id)
  }
  checkedCandidateIds.value = []
  routeError.value = ''
}

function removePoint(i: number) {
  form.point_ids.splice(i, 1)
}

function movePoint(i: number, dir: number) {
  const j = i + dir
  const arr = form.point_ids
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

async function openForm(row?: PlanItem) {
  formRef.value?.clearValidate()
  routeError.value = ''
  checkedCandidateIds.value = []
  if (row) {
    const detail = await getPlan(row.id)
    Object.assign(form, {
      id: detail.id,
      name: detail.name,
      community_id: detail.community_id,
      point_ids: (detail.points || []).sort((a, b) => a.sort - b.sort).map((p) => p.id),
      cycle_type: detail.cycle_type,
      cycle_config: { ...detail.cycle_config },
      timeRange: detail.time_window ? (detail.time_window.split('-') as [string, string]) : null,
      inspector_ids: [...detail.inspector_ids],
      dateRange: [detail.start_date, detail.end_date],
      status: detail.status
    })
    const data = await listPoints({ community_id: detail.community_id, status: 1, page: 1, page_size: 100 })
    candidatePoints.value = data.list
  } else {
    Object.assign(form, {
      id: '', name: '', community_id: null, point_ids: [], cycle_type: 'daily',
      cycle_config: {}, timeRange: null, inspector_ids: [], dateRange: null, status: 1
    })
    candidatePoints.value = []
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  // 路线为空前置校验
  if (!form.point_ids.length) {
    routeError.value = '巡检路线为空，请从可选点位中加入至少 1 个点位'
    return
  }
  // 周期明细校验
  if (form.cycle_type === 'weekly' && !form.cycle_config.weekdays?.length) {
    ElMessage.warning('请选择每周的执行日')
    return
  }
  if (form.cycle_type === 'monthly' && !form.cycle_config.days?.length) {
    ElMessage.warning('请选择每月的执行日')
    return
  }
  const payload = {
    community_id: form.community_id!,
    name: form.name,
    point_ids: form.point_ids,
    cycle_type: form.cycle_type,
    cycle_config: form.cycle_type === 'daily' ? {} : form.cycle_config,
    inspector_ids: form.inspector_ids,
    start_date: form.dateRange![0],
    end_date: form.dateRange![1],
    time_window: `${form.timeRange![0]}-${form.timeRange![1]}`,
    status: form.status
  }
  submitting.value = true
  try {
    if (form.id) {
      await updatePlan(form.id, payload)
      ElMessage.success('计划已更新，对之后生成的任务生效')
    } else {
      await createPlan(payload)
      ElMessage.success('计划已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="scss">
.danger-text {
  color: $color-danger;
}

.route-picker {
  width: 100%;
  display: flex;
  gap: $spacing-md;
}

.route-col {
  flex: 1;
  min-width: 0;
  border: 1px solid $color-border;
  border-radius: $radius-small;
  padding: $spacing-md;

  .route-col-title {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin-bottom: $spacing-sm;
  }

  .route-list {
    max-height: 200px;
    overflow-y: auto;
    margin-bottom: $spacing-sm;
  }

  .route-item {
    display: flex;
    align-items: center;
    gap: $spacing-xs;
    padding: $spacing-xs 0;

    .route-order {
      color: $color-text-secondary;
      width: 20px;
    }

    .route-name {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
}

.route-error {
  color: $color-danger;
  font-size: $font-size-aux;
  margin-top: $spacing-xs;
}
</style>
