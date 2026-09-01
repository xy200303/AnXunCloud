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
        <el-form-item label="巡查类型">
          <el-select v-model="query.patrol_type" placeholder="全部" clearable style="width: 160px">
            <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
              <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
            </el-option-group>
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
        <el-table-column label="巡查类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ patrolTypeLabel(row.patrol_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="周期" width="130">
          <template #default="{ row }">{{ cycleLabel(row) }}</template>
        </el-table-column>
        <el-table-column label="巡检员" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.inspector_names?.join('、') || '--' }}</template>
        </el-table-column>
        <!-- 点位：手动名单显示点位数；按类型圈选显示圈选的类型标签 -->
        <el-table-column label="点位" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.selection_mode === 'by_point_types'" class="selection-cell">
              <el-tag size="small" type="warning" effect="plain">按类型圈选</el-tag>
              {{ pointTypeNames(row.point_types) }}
            </span>
            <span v-else>{{ row.point_count }} 个</span>
          </template>
        </el-table-column>
        <el-table-column label="执行时段" width="120" align="center">
          <template #default="{ row }">
            <el-tooltip v-if="row.cycle_config?.rounds?.length" placement="top">
              <template #content>
                <div v-for="(r, i) in row.cycle_config.rounds" :key="i">{{ r.name }} {{ r.window || '自由时段' }}</div>
              </template>
              <span>{{ row.cycle_config.rounds.length }} 轮次</span>
            </el-tooltip>
            <span v-else>{{ row.time_window || '--' }}</span>
          </template>
        </el-table-column>
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
    <el-dialog v-model="formVisible" :title="form.id ? '编辑巡检计划' : '新增巡检计划'" width="760px" :close-on-click-modal="false">
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

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="巡查类型" prop="patrol_type">
              <el-select v-model="form.patrol_type" style="width: 100%">
                <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
                  <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
                </el-option-group>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="选点方式">
              <el-radio-group v-model="form.selection_mode">
                <el-radio value="explicit">手动选择点位</el-radio>
                <el-radio value="by_point_types">按点位类型圈选</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 按点位类型圈选：point_type 字典多选 + 实时命中预览 -->
        <el-form-item v-if="form.selection_mode === 'by_point_types'" label="点位类型" prop="point_types">
          <el-select
            v-model="form.point_types"
            multiple
            collapse-tags
            collapse-tags-tooltip
            placeholder="选择要圈选的点位类型"
            style="width: 100%"
            :disabled="!form.community_id"
            @change="refreshPreview"
          >
            <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
          </el-select>
          <div class="preview-line">
            <template v-if="form.community_id && form.point_types.length">
              <span :class="{ 'danger-text': !previewLoading && previewCount === 0 }">
                当前命中 {{ previewCount }} 个点位
              </span>
              <el-button link type="primary" size="small" :disabled="!previewCount" @click="previewVisible = true">
                查看点位清单
              </el-button>
            </template>
            <span v-else class="text-secondary">任务生成时按类型实时展开命中点位，新增同类型点位自动进入下期任务</span>
          </div>
        </el-form-item>

        <!-- 巡检路线：可选点位 + 已选点位（有序，可上下移动） -->
        <el-form-item v-else label="巡检路线" prop="point_ids">
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
          <div v-if="form.cycle_type === 'weekly'" class="cycle-picker">
            <el-select
              v-model="form.cycle_config.weekdays"
              multiple
              placeholder="选择星期"
              style="width: 320px"
            >
              <el-option v-for="(w, i) in ['周一', '周二', '周三', '周四', '周五', '周六', '周日']" :key="i + 1" :label="w" :value="i + 1" />
            </el-select>
            <div class="cycle-quick">
              <el-button link size="small" type="primary" @click="quickWeekdays([1, 2, 3, 4, 5])">工作日</el-button>
              <el-button link size="small" type="primary" @click="quickWeekdays([6, 7])">非工作日</el-button>
              <el-button link size="small" type="primary" @click="quickWeekdays([1, 2, 3, 4, 5, 6, 7])">全选</el-button>
              <el-button link size="small" @click="quickWeekdays([])">清空</el-button>
            </div>
          </div>
          <div v-if="form.cycle_type === 'monthly'" class="cycle-picker">
            <el-select
              v-model="form.cycle_config.days"
              multiple
              placeholder="选择日期"
              style="width: 320px"
            >
              <el-option label="月末" :value="-1" />
              <el-option v-for="d in 31" :key="d" :label="`${d} 日`" :value="d" />
            </el-select>
            <div class="cycle-quick">
              <el-button link size="small" type="primary" @click="quickMonthDays('workday')">本月工作日</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('weekend')">本月非工作日</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('all')">全月(1-28)</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('odd')">单号日</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('even')">双号日</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('firstHalf')">上半月</el-button>
              <el-button link size="small" type="primary" @click="quickMonthDays('secondHalf')">下半月</el-button>
              <el-button link size="small" @click="quickMonthDays('clear')">清空</el-button>
            </div>
            <div class="text-secondary cycle-hint">「本月工作日/非工作日」按当前月份日历填入具体日期，跨月后星期会漂移，长期计划建议用「全月/单双号/上下半月」或每周周期</div>
          </div>
        </el-form-item>

        <!-- 分配方式（每周/每月）：总量按 执行日数×巡检员数 连续均分 -->
        <el-form-item v-if="form.cycle_type !== 'daily'" label="分配方式">
          <el-radio-group v-model="form.assign_mode">
            <el-radio value="split">按执行日均分（推荐）</el-radio>
            <el-radio value="all">每个执行日巡全部点位</el-radio>
          </el-radio-group>
          <div v-if="form.assign_mode === 'split'" class="text-secondary cycle-hint">
            点位总量按 执行日数 × 巡检员数 连续切块，同一巡检员同一天分到相邻的一片区域，跑动距离最小。
            <template v-if="splitEstimate > 0">预计每人每日约 <b>{{ splitEstimate }}</b> 个点位。</template>
          </div>
        </el-form-item>

        <!-- 轮次设置（仅每天/每周）：配了轮次后计划级执行时段忽略 -->
        <el-form-item v-if="form.cycle_type !== 'monthly'" label="轮次设置">
          <div class="rounds-editor">
            <div class="rounds-toolbar">
              <el-button size="small" :icon="Plus" @click="addRound()">添加轮次</el-button>
              <el-popover v-model:visible="quickGenVisible" placement="bottom-start" width="320" trigger="click">
                <template #reference>
                  <el-button size="small" plain>快捷生成</el-button>
                </template>
                <div class="quick-gen">
                  <div class="quick-gen-row">
                    <span class="quick-gen-label">开始时刻</span>
                    <el-time-picker v-model="quickGen.start" format="HH:mm" value-format="HH:mm" placeholder="如 07:00" style="width: 130px" />
                  </div>
                  <div class="quick-gen-row">
                    <span class="quick-gen-label">结束时刻</span>
                    <el-time-picker v-model="quickGen.end" format="HH:mm" value-format="HH:mm" placeholder="如 19:00" style="width: 130px" />
                  </div>
                  <div class="quick-gen-row">
                    <span class="quick-gen-label">每隔</span>
                    <el-input-number v-model="quickGen.everyHours" :min="1" :max="12" size="small" style="width: 110px" />
                    <span class="text-secondary">小时一轮</span>
                  </div>
                  <div class="quick-gen-row">
                    <span class="quick-gen-label">每轮时长</span>
                    <el-input-number v-model="quickGen.durationMin" :min="15" :max="720" :step="15" size="small" style="width: 110px" />
                    <span class="text-secondary">分钟</span>
                  </div>
                  <el-button size="small" type="primary" style="width: 100%" @click="applyQuickGen">生成并替换轮次</el-button>
                </div>
              </el-popover>
              <el-dropdown trigger="click" @command="applyShiftTemplate">
                <el-button size="small" plain>班次模板<el-icon><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="two">两班倒（07:00-19:00 / 19:00-07:00）</el-dropdown-item>
                    <el-dropdown-item command="three">三班倒（8 小时制）</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <div v-if="form.rounds.length" class="rounds-list">
              <div v-for="(r, i) in form.rounds" :key="i" class="round-row">
                <el-input v-model="r.name" placeholder="轮次名" style="width: 130px" />
                <el-time-picker v-model="r.start" format="HH:mm" value-format="HH:mm" placeholder="开始" style="width: 110px" />
                <span class="text-secondary">至</span>
                <el-time-picker v-model="r.end" format="HH:mm" value-format="HH:mm" placeholder="结束" style="width: 110px" />
                <el-button link size="small" :icon="Top" :disabled="i === 0" @click="moveRound(i, -1)" />
                <el-button link size="small" :icon="Bottom" :disabled="i === form.rounds.length - 1" @click="moveRound(i, 1)" />
                <el-button link size="small" type="danger" :icon="Close" @click="form.rounds.splice(i, 1)" />
              </div>
            </div>
            <div class="text-secondary rounds-hint">
              留空则按下方「执行时段」每天一轮；起止留空 = 自由时段（只考核轮次次数）；窗口允许跨零点（如 19:00-07:00），起止相等非法
            </div>
            <div v-if="roundsError" class="route-error">{{ roundsError }}</div>
          </div>
        </el-form-item>

        <!-- 每日达标轮次（仅每天）：可空 = 不设达标线 -->
        <el-form-item v-if="form.cycle_type === 'daily'" label="达标轮次">
          <el-input-number v-model="form.daily_min_rounds" :min="0" :max="50" placeholder="不设达标线" controls-position="right" style="width: 160px" />
          <span class="text-secondary rounds-hint">每日达标轮次线（履约考核用），留空不考核；巡更达成率报表按此判定标红</span>
        </el-form-item>

        <!-- 计划级执行时段：配了轮次则忽略 -->
        <el-form-item v-if="!roundsEnabled" label="执行时段" prop="timeRange">
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

    <!-- 圈选命中点位清单 -->
    <el-dialog v-model="previewVisible" title="圈选命中点位" width="560px">
      <el-table v-loading="previewLoading" :data="previewPoints" border size="small" max-height="380" style="width: 100%">
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="name" label="点位名称" min-width="150" show-overflow-tooltip />
        <el-table-column label="楼栋/区域" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.building_name || '--' }}</template>
        </el-table-column>
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ pointTypeName(row.type) }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Search, Refresh, Plus, RefreshRight, ArrowDown, Top, Bottom, Close, VideoPlay
} from '@element-plus/icons-vue'
import { listPlans, getPlan, createPlan, updatePlan, deletePlan, updatePlanStatus, previewPlanPoints } from '@/api/plan'
import { generateTasks } from '@/api/task'
import { listCommunities } from '@/api/community'
import { listPoints } from '@/api/point'
import { listUsers } from '@/api/user'
import { listDictOptions, type DictOption } from '@/api/dict'
import { useUserStore } from '@/store/user'
import { usePatrolTypes } from '@/composables/usePatrolTypes'
import type { PlanItem, PlanCycleConfig, PlanSelectionMode, PlanAssignMode, CommunityItem, PointItem, PatrolType } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const userStore = useUserStore()
// 巡查类型字典（按大类分组：日常巡逻/专项检查）
const { patrolTypeGroups, patrolTypeLabel } = usePatrolTypes()

// ===== 列表 =====
const loading = ref(false)
const list = ref<PlanItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectorOptions = ref<UserItem[]>([])
const pointTypeOptions = ref<DictOption[]>([])
const query = reactive({ page: 1, page_size: 20, community_id: undefined as string | undefined, name: '', cycle_type: '', patrol_type: '', status: '' as number | '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listPlans({
      ...query,
      name: query.name || undefined,
      cycle_type: query.cycle_type || undefined,
      patrol_type: query.patrol_type || undefined,
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
  query.patrol_type = ''
  query.status = ''
  handleSearch()
}

onMounted(async () => {
  fetchList()
  const [cData, uData, ptData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 }),
    listDictOptions('point_type')
  ])
  communities.value = cData.list
  inspectorOptions.value = uData.list
  pointTypeOptions.value = ptData || []
})

function cycleLabel(row: PlanItem) {
  if (row.cycle_type === 'daily') return '每天'
  if (row.cycle_type === 'weekly') {
    const names = ['', '一', '二', '三', '四', '五', '六', '日']
    return `每周${(row.cycle_config?.weekdays || []).map((w) => names[w]).join('、')}${row.assign_mode === 'split' ? ' · 均分' : ''}`
  }
  if (row.cycle_type === 'monthly') {
    const days = (row.cycle_config?.days || []).map((d) => (d === -1 ? '月末' : `${d}`))
    return `每月 ${days.join('、')} 日${row.assign_mode === 'split' ? ' · 均分' : ''}`
  }
  return row.cycle_type
}

// 点位类型值 → 字典标签
function pointTypeName(v: string) {
  return pointTypeOptions.value.find((d) => d.value === v)?.label || v
}

function pointTypeNames(types?: string[] | null) {
  if (!types?.length) return '--'
  return types.map(pointTypeName).join('、')
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
const roundsError = ref('')

// 轮次编辑行：start/end 拆开编辑，提交时拼成 window（HH:MM-HH:MM）
interface RoundRow {
  name: string
  start: string
  end: string
}

const form = reactive({
  id: '',
  name: '',
  community_id: null as string | null,
  patrol_type: 'safety' as PatrolType,
  selection_mode: 'explicit' as PlanSelectionMode,
  point_ids: [] as string[],
  point_types: [] as string[],
  cycle_type: 'daily',
  cycle_config: {} as PlanCycleConfig,
  assign_mode: 'split' as PlanAssignMode,
  rounds: [] as RoundRow[],
  daily_min_rounds: null as number | null,
  timeRange: null as [string, string] | null,
  inspector_ids: [] as string[],
  dateRange: null as [string, string] | null,
  status: 1
})

// 配了轮次（每天/每周）时忽略计划级执行时段
const roundsEnabled = computed(() => form.cycle_type !== 'monthly' && form.rounds.length > 0)

// ===== 周期快捷选择 =====
function quickWeekdays(days: number[]) {
  form.cycle_config.weekdays = days
}

// 每月快捷选择：workday/weekend 按当前月份日历展开为具体日期（跨月星期漂移，提示文案已说明）
function quickMonthDays(kind: string) {
  const range = (a: number, b: number) => Array.from({ length: b - a + 1 }, (_, i) => a + i)
  switch (kind) {
    case 'workday':
    case 'weekend': {
      const now = new Date()
      const last = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
      const days: number[] = []
      for (let d = 1; d <= last; d++) {
        const wd = new Date(now.getFullYear(), now.getMonth(), d).getDay()
        if ((kind === 'workday') === (wd >= 1 && wd <= 5)) days.push(d)
      }
      form.cycle_config.days = days
      break
    }
    case 'all':
      form.cycle_config.days = range(1, 28)
      break
    case 'odd':
      form.cycle_config.days = range(1, 31).filter((d) => d % 2 === 1)
      break
    case 'even':
      form.cycle_config.days = range(2, 30).filter((d) => d % 2 === 0)
      break
    case 'firstHalf':
      form.cycle_config.days = range(1, 15)
      break
    case 'secondHalf':
      form.cycle_config.days = range(16, 28)
      break
    default:
      form.cycle_config.days = []
  }
}

// 执行日数（weekly=星期数，monthly=日期数）
const execDayCount = computed(() => {
  if (form.cycle_type === 'weekly') return form.cycle_config.weekdays?.length || 0
  if (form.cycle_type === 'monthly') return form.cycle_config.days?.length || 0
  return 0
})

// 均分预估：每人每日点位数 = 总量 / (执行日数 × 巡检员数)
const splitEstimate = computed(() => {
  if (form.cycle_type === 'daily' || form.assign_mode !== 'split') return 0
  if (!execDayCount.value || !form.inspector_ids.length) return 0
  const total = form.selection_mode === 'explicit' ? form.point_ids.length : previewCount.value
  if (!total) return 0
  return Math.ceil(total / (execDayCount.value * form.inspector_ids.length))
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入计划名称', trigger: 'blur' }],
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  patrol_type: [{ required: true, message: '请选择巡查类型', trigger: 'change' }],
  point_types: [
    {
      validator: (_r, v: string[], cb) =>
        form.selection_mode === 'by_point_types' && !v?.length ? cb(new Error('请选择要圈选的点位类型')) : cb(),
      trigger: 'change'
    }
  ],
  cycle_type: [{ required: true, message: '请选择周期', trigger: 'change' }],
  timeRange: [
    {
      validator: (_r, v, cb) => (roundsEnabled.value || v ? cb() : cb(new Error('请选择执行时段'))),
      trigger: 'change'
    }
  ],
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
  refreshPreview()
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

// ===== 圈选命中预览 =====
const previewLoading = ref(false)
const previewCount = ref(0)
const previewPoints = ref<{ id: string; name: string; type: string; building_name: string }[]>([])
const previewVisible = ref(false)

async function refreshPreview() {
  if (form.selection_mode !== 'by_point_types' || !form.community_id || !form.point_types.length) {
    previewCount.value = 0
    previewPoints.value = []
    return
  }
  previewLoading.value = true
  try {
    const data = await previewPlanPoints({ community_id: form.community_id, point_types: form.point_types.join(',') })
    previewCount.value = data.count
    previewPoints.value = data.points
  } finally {
    previewLoading.value = false
  }
}

// ===== 轮次编辑 =====
function addRound(name = '', start = '', end = '') {
  form.rounds.push({ name: name || `第${form.rounds.length + 1}轮`, start, end })
  roundsError.value = ''
}

function moveRound(i: number, dir: number) {
  const j = i + dir
  const arr = form.rounds
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

// 快捷生成：开始/结束时刻 + 每隔 N 小时 + 每轮时长 → 展开轮次（结束早于开始按跨零点处理）
const quickGenVisible = ref(false)
const quickGen = reactive({ start: '07:00', end: '19:00', everyHours: 2, durationMin: 120 })

function toMinutes(t: string) {
  const [h, m] = t.split(':').map(Number)
  return h * 60 + m
}

function fmtMinutes(min: number) {
  const v = ((min % 1440) + 1440) % 1440
  return `${String(Math.floor(v / 60)).padStart(2, '0')}:${String(v % 60).padStart(2, '0')}`
}

function applyQuickGen() {
  if (!quickGen.start || !quickGen.end) {
    ElMessage.warning('请先选择开始/结束时刻')
    return
  }
  const s = toMinutes(quickGen.start)
  let e = toMinutes(quickGen.end)
  if (e <= s) e += 1440 // 跨零点：结束归次日
  const step = quickGen.everyHours * 60
  const rounds: RoundRow[] = []
  for (let t = s; t < e && rounds.length < 48; t += step) {
    rounds.push({ name: `第${rounds.length + 1}轮`, start: fmtMinutes(t), end: fmtMinutes(t + quickGen.durationMin) })
  }
  if (!rounds.length) {
    ElMessage.warning('该时间段内无法展开轮次')
    return
  }
  form.rounds = rounds
  roundsError.value = ''
  quickGenVisible.value = false
  ElMessage.success(`已生成 ${rounds.length} 个轮次，可继续微调`)
}

// 班次模板一键填充
function applyShiftTemplate(cmd: string) {
  if (cmd === 'two') {
    form.rounds = [
      { name: '白班', start: '07:00', end: '19:00' },
      { name: '夜班', start: '19:00', end: '07:00' }
    ]
  } else {
    form.rounds = [
      { name: '早班', start: '08:00', end: '16:00' },
      { name: '中班', start: '16:00', end: '00:00' },
      { name: '夜班', start: '00:00', end: '08:00' }
    ]
  }
  roundsError.value = ''
  ElMessage.success('已按班次模板填充轮次')
}

// 轮次合法性：名称必填；起止同时留空 = 自由时段（window 空），只填一端非法；起止相等非法
function validateRounds(): boolean {
  for (const r of form.rounds) {
    if (!r.name.trim()) {
      roundsError.value = '轮次名称不能为空'
      return false
    }
    if (!r.start && !r.end) continue // 自由时段
    if (!r.start || !r.end) {
      roundsError.value = `轮次「${r.name}」起止时间须同时填写或同时留空（自由时段）`
      return false
    }
    if (r.start === r.end) {
      roundsError.value = `轮次「${r.name}」起止时间相等非法（全天请配 00:00-23:59）`
      return false
    }
  }
  roundsError.value = ''
  return true
}

async function openForm(row?: PlanItem) {
  formRef.value?.clearValidate()
  routeError.value = ''
  roundsError.value = ''
  checkedCandidateIds.value = []
  previewVisible.value = false
  if (row) {
    const detail = await getPlan(row.id)
    Object.assign(form, {
      id: detail.id,
      name: detail.name,
      community_id: detail.community_id,
      patrol_type: detail.patrol_type || 'safety',
      selection_mode: detail.selection_mode || 'explicit',
      point_ids: (detail.points || []).sort((a, b) => a.sort - b.sort).map((p) => p.id),
      point_types: [...(detail.point_types || [])],
      cycle_type: detail.cycle_type,
      cycle_config: { ...detail.cycle_config },
      assign_mode: detail.assign_mode || 'all',
      // window 拆开为起止两个时刻便于编辑
      rounds: (detail.cycle_config?.rounds || []).map((r) => {
        const [start = '', end = ''] = (r.window || '').split('-')
        return { name: r.name, start, end }
      }),
      daily_min_rounds: detail.cycle_config?.daily_min_rounds ?? null,
      timeRange: detail.time_window ? (detail.time_window.split('-') as [string, string]) : null,
      inspector_ids: [...detail.inspector_ids],
      dateRange: [detail.start_date, detail.end_date],
      status: detail.status
    })
    const data = await listPoints({ community_id: detail.community_id, status: 1, page: 1, page_size: 100 })
    candidatePoints.value = data.list
    refreshPreview()
  } else {
    Object.assign(form, {
      id: '', name: '', community_id: null, patrol_type: 'safety', selection_mode: 'explicit' as PlanSelectionMode,
      point_ids: [], point_types: [], cycle_type: 'daily', cycle_config: {}, assign_mode: 'split' as PlanAssignMode, rounds: [], daily_min_rounds: null,
      timeRange: null, inspector_ids: [], dateRange: null, status: 1
    })
    candidatePoints.value = []
    previewCount.value = 0
    previewPoints.value = []
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  // 选点校验：手动名单 / 类型圈选二选一
  if (form.selection_mode === 'explicit' && !form.point_ids.length) {
    routeError.value = '巡检路线为空，请从可选点位中加入至少 1 个点位'
    return
  }
  if (form.selection_mode === 'by_point_types' && !form.point_types.length) {
    ElMessage.warning('请选择要圈选的点位类型')
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
  // 轮次校验（仅每天/每周）
  if (form.cycle_type !== 'monthly' && form.rounds.length && !validateRounds()) return
  // 组装 cycle_config：每周/每月保留原有明细；每天/每周附加轮次与达标线
  const cycle_config: PlanCycleConfig = {}
  if (form.cycle_type === 'weekly') cycle_config.weekdays = form.cycle_config.weekdays
  if (form.cycle_type === 'monthly') cycle_config.days = form.cycle_config.days
  if (form.cycle_type !== 'monthly' && form.rounds.length) {
    // 起止留空 = 自由时段（window 空串）
    cycle_config.rounds = form.rounds.map((r) => ({
      name: r.name.trim(),
      window: r.start && r.end ? `${r.start}-${r.end}` : ''
    }))
  }
  if (form.cycle_type === 'daily' && form.daily_min_rounds != null) {
    cycle_config.daily_min_rounds = form.daily_min_rounds
  }
  const payload = {
    community_id: form.community_id!,
    name: form.name,
    patrol_type: form.patrol_type,
    selection_mode: form.selection_mode,
    point_ids: form.selection_mode === 'explicit' ? form.point_ids : [],
    point_types: form.selection_mode === 'by_point_types' ? form.point_types : [],
    cycle_type: form.cycle_type,
    cycle_config,
    assign_mode: form.cycle_type === 'daily' ? 'all' : form.assign_mode,
    inspector_ids: form.inspector_ids,
    start_date: form.dateRange![0],
    end_date: form.dateRange![1],
    // 配了轮次顶层 time_window 留空
    time_window: roundsEnabled.value ? '' : `${form.timeRange![0]}-${form.timeRange![1]}`,
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

.cycle-picker {
  margin-left: 12px;

  .cycle-quick {
    margin-top: 4px;
  }
}

.cycle-hint {
  font-size: $font-size-aux;
  margin-top: 2px;
  width: 100%;
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

.selection-cell {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
}

.preview-line {
  width: 100%;
  font-size: $font-size-aux;
  display: flex;
  align-items: center;
  gap: $spacing-xs;
}

.rounds-editor {
  width: 100%;

  .rounds-toolbar {
    display: flex;
    gap: $spacing-sm;
    margin-bottom: $spacing-sm;
  }

  .rounds-list {
    display: flex;
    flex-direction: column;
    gap: $spacing-sm;
    margin-bottom: $spacing-xs;
  }

  .round-row {
    display: flex;
    align-items: center;
    gap: $spacing-xs;
  }

  .rounds-hint {
    font-size: $font-size-aux;
    margin-left: $spacing-sm;
  }
}

.quick-gen {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;

  .quick-gen-row {
    display: flex;
    align-items: center;
    gap: $spacing-sm;

    .quick-gen-label {
      width: 60px;
      font-size: $font-size-aux;
      color: $color-text-secondary;
    }
  }
}
</style>
