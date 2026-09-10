<template>
  <!-- 维保确认：pending 列表（后端已按 AI review 置顶排序），批量通过 / 驳回填理由 -->
  <div class="table-card">
    <div class="table-toolbar">
      <div class="table-toolbar-left">
        <el-button
          v-perms="'equipment:confirm'"
          type="primary"
          :icon="CircleCheck"
          :disabled="!selected.length"
          :loading="confirming"
          @click="handleBatchConfirm"
        >批量通过{{ selected.length ? `（${selected.length}）` : '' }}</el-button>
        <span class="text-secondary">AI 存疑（review）的记录已置顶标红，请重点核对照片后再通过</span>
      </div>
      <el-tooltip content="刷新" placement="top">
        <el-button :icon="RefreshRight" circle @click="fetchList" />
      </el-tooltip>
    </div>

    <el-table
      v-loading="loading"
      :data="list"
      stripe
      style="width: 100%"
      :row-class-name="rowClass"
      @selection-change="(rows: MaintenanceItem[]) => (selected = rows)"
    >
      <el-table-column type="selection" width="44" />
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-body">
            <el-descriptions :column="2" size="small" border>
              <el-descriptions-item label="维保类型">{{ maintTypeLabel(row.maintenance_type) }}</el-descriptions-item>
              <el-descriptions-item label="维保日期">{{ row.maintenance_date }}</el-descriptions-item>
              <el-descriptions-item label="维保单位">{{ row.vendor || '--' }}</el-descriptions-item>
              <el-descriptions-item label="经办人">{{ row.operator_name }}</el-descriptions-item>
              <el-descriptions-item label="登记人">{{ row.created_by_name }}（{{ row.created_at }}）</el-descriptions-item>
              <el-descriptions-item label="备注">{{ row.note || '--' }}</el-descriptions-item>
              <el-descriptions-item v-if="row.label_missing" label="标签缺失">
                <el-tag type="warning" size="small">确认后该设备退出自动到期判定，请处置（更换设备或代登记日期）</el-tag>
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
      <el-table-column label="AI" width="70" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.ai_verdict === 'review'" type="danger" size="small">存疑</el-tag>
          <el-tag v-else-if="row.ai_verdict === 'pass'" type="success" size="small">通过</el-tag>
          <span v-else class="text-secondary">--</span>
        </template>
      </el-table-column>
      <el-table-column prop="equipment_code" label="设备编号" min-width="130" show-overflow-tooltip />
      <el-table-column label="设备名称" min-width="130" show-overflow-tooltip>
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
      <el-table-column prop="operator_name" label="经办人" width="100" align="center" />
      <el-table-column prop="created_by_name" label="登记人" width="100" align="center" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button v-perms="'equipment:confirm'" link type="primary" @click="handleConfirm([row.id])">通过</el-button>
          <el-button v-perms="'equipment:confirm'" link type="danger" @click="handleReject(row)">驳回</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无待确认的维保登记" />
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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheck, RefreshRight } from '@element-plus/icons-vue'
import {
  listMaintenancePending, confirmMaintenances, rejectMaintenance,
  type MaintenanceItem, type MaintenanceType, type MaintenancePhoto
} from '@/api/equipment'
import { withFileToken } from '@/api/upload'
import { listDictOptions, type DictOption } from '@/api/dict'

const emit = defineEmits<{ changed: [] }>()

const loading = ref(false)
const confirming = ref(false)
const list = ref<MaintenanceItem[]>([])
const total = ref(0)
const selected = ref<MaintenanceItem[]>([])
const query = reactive({ page: 1, page_size: 20 })

const maintTypeOptions = ref<DictOption[]>([])

function maintTypeLabel(t: MaintenanceType) {
  return maintTypeOptions.value.find((d) => d.value === t)?.label || t
}

// AI 存疑行标红（后端已置顶，这里加强视觉提示）
function rowClass({ row }: { row: MaintenanceItem }) {
  return row.ai_verdict === 'review' ? 'row-ai-review' : ''
}

async function fetchList() {
  loading.value = true
  try {
    const d = await listMaintenancePending(query)
    list.value = d.list
    total.value = d.total
  } finally {
    loading.value = false
  }
}

async function handleConfirm(ids: string[]) {
  confirming.value = true
  try {
    const res = await confirmMaintenances(ids)
    ElMessage.success(`已确认 ${res.confirmed} 条${res.skipped ? `，跳过已处理 ${res.skipped} 条` : ''}`)
    fetchList()
    emit('changed')
  } finally {
    confirming.value = false
  }
}

async function handleBatchConfirm() {
  const ok = await ElMessageBox.confirm(
    `确认通过勾选的 ${selected.value.length} 条维保登记？确认后台账即时生效（回写最近维保日期并重算到期日）。`,
    '批量确认',
    { confirmButtonText: '通过', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  handleConfirm(selected.value.map((m) => m.id))
}

async function handleReject(row: MaintenanceItem) {
  const { value } = await ElMessageBox.prompt(
    `驳回设备「${row.equipment_name}（${row.equipment_code}）」的维保登记，登记人将收到通知并可重新登记`,
    '驳回理由',
    {
      confirmButtonText: '驳回',
      cancelButtonText: '取消',
      inputPlaceholder: '必填，如：照片为旧标签，请重新拍摄',
      inputValidator: (v: string) => (v && v.trim() ? true : '驳回必须填写理由')
    }
  ).catch(() => ({ value: '' }))
  if (!value) return
  await rejectMaintenance(row.id, value.trim())
  ElMessage.success('已驳回')
  fetchList()
  emit('changed')
}

onMounted(() => {
  fetchList()
  listDictOptions('equipment_maint_type').then((d) => {
    maintTypeOptions.value = d || []
  })
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

:deep(.row-ai-review) {
  background: var(--el-color-danger-light-9);
}
</style>
