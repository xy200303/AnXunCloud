<template>
  <!-- 维保记录：全状态流水总表（只读；待确认的处理动作在「维保确认」tab） -->
  <div class="filter-card">
    <el-form :model="query" inline>
      <el-form-item label="小区">
        <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 150px">
          <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="确认状态">
        <el-select v-model="query.confirm_status" placeholder="全部" clearable style="width: 120px">
          <el-option label="待确认" value="pending" />
          <el-option label="已确认" value="confirmed" />
          <el-option label="已驳回" value="rejected" />
        </el-select>
      </el-form-item>
      <el-form-item label="维保日期">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          start-placeholder="开始"
          end-placeholder="结束"
          style="width: 240px"
        />
      </el-form-item>
      <el-form-item label="标签缺失">
        <el-checkbox v-model="onlyLabelMissing">仅看标签缺失登记</el-checkbox>
      </el-form-item>
      <el-form-item label="关键字">
        <el-input v-model="query.keyword" placeholder="设备编号或名称" clearable style="width: 160px" @keyup.enter="handleSearch" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
        <el-button :icon="Refresh" @click="handleReset">重置</el-button>
      </el-form-item>
    </el-form>
  </div>

  <div class="table-card">
    <el-table v-loading="loading" :data="list" stripe style="width: 100%" row-key="id">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-body">
            <el-descriptions :column="2" size="small" border>
              <el-descriptions-item label="维保类型">{{ maintTypeLabel(row.maintenance_type) }}</el-descriptions-item>
              <el-descriptions-item label="维保日期">{{ row.maintenance_date }}</el-descriptions-item>
              <el-descriptions-item label="维保单位">{{ row.vendor || '--' }}</el-descriptions-item>
              <el-descriptions-item label="经办人">{{ row.operator_name }}</el-descriptions-item>
              <el-descriptions-item label="登记人">{{ row.created_by_name }}（{{ row.created_at }}）</el-descriptions-item>
              <el-descriptions-item label="确认人">
                <template v-if="row.confirmed_by_name">{{ row.confirmed_by_name }}（{{ row.confirmed_at }}）</template>
                <span v-else class="text-secondary">--</span>
              </el-descriptions-item>
              <el-descriptions-item label="备注">{{ row.note || '--' }}</el-descriptions-item>
              <el-descriptions-item v-if="row.reject_reason" label="驳回理由">
                <span class="reject-reason">{{ row.reject_reason }}</span>
              </el-descriptions-item>
              <el-descriptions-item v-if="row.maintenance_type === 'ledger_fix'" label="补录出厂日期">{{ row.fix_manufacture_date || '--' }}</el-descriptions-item>
              <el-descriptions-item v-if="row.maintenance_type === 'ledger_fix'" label="补录维保日期">{{ row.fix_last_maintenance_date || '--' }}</el-descriptions-item>
              <el-descriptions-item label="AI 预检">
                <template v-if="row.ai_verdict === 'review'">
                  <el-tag type="danger" size="small">存疑</el-tag> {{ row.ai_reason }}
                </template>
                <el-tag v-else-if="row.ai_verdict === 'pass'" type="success" size="small">通过</el-tag>
                <span v-else class="text-secondary">未预检</span>
              </el-descriptions-item>
            </el-descriptions>
            <div class="expand-photos">
              <el-image
                v-for="(p, i) in row.photos"
                :key="p.file_id"
                :src="withFileToken(p.url)"
                fit="cover"
                class="expand-thumb"
                :preview-src-list="row.photos.map((x: MaintenancePhoto) => withFileToken(x.url))"
                :initial-index="i"
                preview-teleported
              />
              <span v-if="!row.photos.length" class="text-secondary">无照片</span>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="equipment_code" label="设备编号" min-width="130" show-overflow-tooltip />
      <el-table-column label="设备名称" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.equipment_name }}
          <el-tag v-if="row.label_missing" type="info" size="small">标签缺失</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="点位" min-width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.point_name || '--' }}</template>
      </el-table-column>
      <el-table-column label="维保类型" width="100" align="center">
        <template #default="{ row }">{{ maintTypeLabel(row.maintenance_type) }}</template>
      </el-table-column>
      <el-table-column prop="maintenance_date" label="维保日期" width="100" align="center" />
      <el-table-column prop="created_by_name" label="登记人" width="100" align="center" />
      <el-table-column label="确认状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.confirm_status === 'confirmed'" type="success" size="small">已确认</el-tag>
          <el-tag v-else-if="row.confirm_status === 'rejected'" type="danger" size="small">已驳回</el-tag>
          <el-tag v-else type="warning" size="small">待确认</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="AI" width="70" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.ai_verdict === 'review'" type="danger" size="small">存疑</el-tag>
          <el-tag v-else-if="row.ai_verdict === 'pass'" type="success" size="small">通过</el-tag>
          <span v-else class="text-secondary">--</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="登记时间" width="160" align="center" />
      <template #empty>
        <el-empty description="暂无维保记录" />
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
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import {
  listMaintenanceRecords,
  type MaintenanceItem, type MaintenancePhoto, type MaintenanceType
} from '@/api/equipment'
import { withFileToken } from '@/api/upload'
import { useCommunities } from '@/composables/useCommunities'
import { useDictOptions } from '@/composables/useDictOptions'
import { usePagedList } from '@/composables/usePagedList'

const { communities } = useCommunities()
const dateRange = ref<[string, string] | null>(null)
const onlyLabelMissing = ref(false)

const { loading, list, total, query, fetchList } = usePagedList(
  (q) =>
    listMaintenanceRecords({
      ...q,
      start_date: dateRange.value?.[0] || undefined,
      end_date: dateRange.value?.[1] || undefined,
      label_missing: onlyLabelMissing.value ? '1' : undefined
    }),
  () => ({ page: 1, page_size: 20, community_id: '', confirm_status: '', keyword: '' })
)

const { options: maintTypeOptions } = useDictOptions('equipment_maint_type')

function maintTypeLabel(t: MaintenanceType) {
  return maintTypeOptions.value.find((d) => d.value === t)?.label || t
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.community_id = ''
  query.confirm_status = ''
  query.keyword = ''
  dateRange.value = null
  onlyLabelMissing.value = false
  handleSearch()
}

onMounted(() => {
  fetchList()
})

defineExpose({ fetchList })
</script>

<style scoped lang="scss">
.expand-body {
  padding: $spacing-md $spacing-xl;
}

.expand-photos {
  display: flex;
  gap: $spacing-sm;
  margin-top: $spacing-md;

  .expand-thumb {
    width: 96px;
    height: 96px;
    border-radius: $radius-small;
  }
}

.reject-reason {
  color: var(--el-color-danger);
}
</style>
