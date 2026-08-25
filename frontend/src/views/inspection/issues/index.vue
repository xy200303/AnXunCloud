<template>
  <div class="app-container">
    <!-- 搜索区 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 140px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡查类型">
          <el-select v-model="query.patrol_type" placeholder="全部类型" clearable style="width: 140px">
            <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
              <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="点位类型">
          <el-select v-model="query.point_type" placeholder="全部类型" clearable style="width: 140px">
            <el-option v-for="o in pointTypes" :key="o.value" :label="o.label" :value="o.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="复核状态">
          <el-select v-model="query.audit_status" placeholder="全部" clearable style="width: 120px">
            <el-option label="待复核" value="pending" />
            <el-option label="复核通过" value="pass" />
            <el-option label="已驳回" value="rejected" />
            <el-option label="自动通过" value="auto_pass" />
          </el-select>
        </el-form-item>
        <el-form-item label="强制提交">
          <el-checkbox v-model="onlyForceSubmit" label="仅看强制提交" />
        </el-form-item>
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            placeholder="点位名 / 异常描述"
            clearable
            style="width: 180px"
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item label="打卡日期">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 260px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
          <el-button v-perms="'inspection:record:list'" type="success" :icon="Download" :loading="exporting" @click="handleExport">
            导出
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 问题列表 -->
    <div class="table-card">
      <el-table
        v-loading="loading"
        :data="list"
        stripe
        style="width: 100%"
        row-class-name="clickable-row"
        @row-click="openDetail"
      >
        <el-table-column prop="checkin_time" label="打卡时间" width="160" />
        <el-table-column prop="community_name" label="小区" min-width="110" show-overflow-tooltip />
        <el-table-column label="楼栋/区域" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.building_name || '--' }}</template>
        </el-table-column>
        <el-table-column prop="point_name" label="点位名称" min-width="120" show-overflow-tooltip />
        <el-table-column prop="inspector_name" label="巡检员" width="100" />
        <el-table-column prop="remark" label="异常描述" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || '--' }}</template>
        </el-table-column>
        <el-table-column label="AI 结论" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="aiVerdictTag(row.ai_verdict).type" size="small">
              {{ aiVerdictTag(row.ai_verdict).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="复核状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="auditStatusTag(row.audit_status).type" size="small">
              {{ auditStatusTag(row.audit_status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="标记" width="130" align="center">
          <template #default="{ row }">
            <div v-if="row.force_submit || row.is_suspect" class="flag-tags">
              <el-tag v-if="row.force_submit" type="warning" size="small" effect="dark">强制提交</el-tag>
              <el-tag v-if="row.is_suspect" type="danger" size="small">疑似作弊</el-tag>
            </div>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetail(row)">查看详情</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该条件下暂无问题记录" />
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

    <!-- 打卡详情抽屉（公共组件） -->
    <checkin-detail-drawer
      v-model="detailVisible"
      :row="currentRow"
      :detail="detail"
      :loading="detailLoading"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Search, Refresh, Download } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listIssues, type IssueItem, type IssueQuery } from '@/api/issue'
import { getCheckin } from '@/api/checkin'
import { listCommunities } from '@/api/community'
import { listDictOptions, type DictOption } from '@/api/dict'
import { usePatrolTypes } from '@/composables/usePatrolTypes'
import { downloadFile } from '@/utils/download'
import CheckinDetailDrawer from '@/components/CheckinDetailDrawer.vue'
import type { CheckinItem, CheckinDetail, CommunityItem } from '@/api/biz-types'

const loading = ref(false)
const list = ref<IssueItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const pointTypes = ref<DictOption[]>([])
const onlyForceSubmit = ref(false)
const dateRange = ref<[string, string] | null>(null)
const query = reactive<IssueQuery>({ page: 1, page_size: 20, keyword: '' })

const { patrolTypeGroups } = usePatrolTypes()

// 复核状态标签：pending 待复核 / pass 复核通过 / rejected 已驳回 / auto_pass 自动通过
function auditStatusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pending: { label: '待复核', type: 'warning' },
      pass: { label: '复核通过', type: 'success' },
      rejected: { label: '已驳回', type: 'danger' },
      auto_pass: { label: '自动通过', type: 'info' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

// AI 结论标签：pass 通过-绿 / review 存疑-黄 / error 失败-红 / 空 无-灰
function aiVerdictTag(v: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pass: { label: '通过', type: 'success' },
      review: { label: '存疑', type: 'warning' },
      error: { label: '失败', type: 'danger' }
    }[v] || { label: '无', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

// 列表与导出共用的筛选参数
function filterParams() {
  return {
    community_id: query.community_id,
    patrol_type: query.patrol_type || undefined,
    point_type: query.point_type || undefined,
    audit_status: query.audit_status || undefined,
    force_submit: onlyForceSubmit.value || undefined,
    keyword: query.keyword?.trim() || undefined,
    date_from: dateRange.value?.[0],
    date_to: dateRange.value?.[1]
  }
}

async function fetchList() {
  loading.value = true
  try {
    const data = await listIssues({ ...query, ...filterParams() })
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
  query.patrol_type = undefined
  query.point_type = undefined
  query.audit_status = undefined
  query.keyword = ''
  onlyForceSubmit.value = false
  dateRange.value = null
  handleSearch()
}

// ===== 导出（按当前筛选条件，文件流下载） =====
const exporting = ref(false)

async function handleExport() {
  try {
    await ElMessageBox.confirm('将按当前筛选条件导出问题清单', '导出确认', {
      confirmButtonText: '导出',
      cancelButtonText: '取消',
      type: 'info'
    })
  } catch {
    return
  }
  exporting.value = true
  try {
    await downloadFile('/inspection/issues/export', filterParams(), '问题清单.xlsx')
    ElMessage.success('导出成功')
  } catch {
    // 失败提示由请求拦截器统一弹出
  } finally {
    exporting.value = false
  }
}

// ===== 详情抽屉 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<CheckinDetail | null>(null)
const currentRow = ref<CheckinItem | null>(null)

// 问题行适配为打卡行：补详情抽屉概要区用到的 checkin_type / photo_count 字段
function toCheckinRow(issue: IssueItem): CheckinItem {
  return {
    ...issue,
    checkin_type: '',
    photo_count: issue.photos?.length ?? 0
  } as unknown as CheckinItem
}

async function openDetail(row: IssueItem) {
  currentRow.value = toCheckinRow(row)
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await getCheckin(row.id)
  } finally {
    detailLoading.value = false
  }
}

onMounted(async () => {
  fetchList()
  const [cData, pData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listDictOptions('point_type', true)
  ])
  communities.value = cData.list
  pointTypes.value = pData || []
})
</script>

<style lang="scss">
// 行可点击提示（非 scoped，作用于 el-table 生成的行）
.clickable-row {
  cursor: pointer;
}
</style>

<style lang="scss" scoped>
.flag-tags {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
</style>
