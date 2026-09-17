<template>
  <div class="app-container">
    <div class="table-card main-tabs-card">
      <el-tabs v-model="mainTab">
        <el-tab-pane label="设备列表" name="list" />
        <el-tab-pane name="confirm">
          <template #label>
            <el-badge :value="pendingTotal" :hidden="pendingTotal === 0" :max="99">维保确认</el-badge>
          </template>
        </el-tab-pane>
        <el-tab-pane label="维保记录" name="records" />
      </el-tabs>
    </div>

    <!-- ========== 设备列表 ========== -->
    <template v-if="mainTab === 'list'">
      <div class="filter-card">
        <el-form :model="query" inline>
          <el-form-item label="小区">
            <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 150px">
              <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="设备类型">
            <el-select v-model="query.type" placeholder="全部类型" clearable filterable style="width: 150px">
              <el-option v-for="d in typeOptions" :key="d.value" :label="d.label" :value="d.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="到期状态">
            <el-select v-model="query.due_state" placeholder="全部" clearable style="width: 120px">
              <el-option label="正常" value="normal" />
              <el-option label="临期" value="warning" />
              <el-option label="已逾期" value="overdue" />
              <el-option label="无到期日" value="none" />
              <el-option label="应报废" value="scrap" />
              <el-option label="标签缺失" value="label_missing" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" placeholder="全部" clearable style="width: 110px">
              <el-option v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="点位绑定">
            <el-select v-model="query.bind_state" placeholder="全部" clearable style="width: 110px">
              <el-option label="已绑定" value="bound" />
              <el-option label="未绑定" value="unbound" />
            </el-select>
          </el-form-item>
          <el-form-item label="关键字">
            <el-input v-model="query.keyword" placeholder="编号或名称" clearable style="width: 150px" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
            <el-button :icon="Refresh" @click="handleReset">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="table-card">
        <!-- 台账类型页签：「全部」+ 设备类型字典项；切换即设置 query.type 重新查询，有方案时表格按方案动态列渲染 -->
        <el-tabs v-model="query.type" class="type-tabs" @tab-change="handleSearch">
          <el-tab-pane label="全部" name="" />
          <el-tab-pane v-for="d in typeOptions" :key="d.value" :label="d.label" :name="d.value" />
        </el-tabs>
        <div class="table-toolbar">
          <div class="table-toolbar-left">
            <el-button v-perms="'equipment:create'" type="primary" :icon="Plus" @click="openForm()">新增设备</el-button>
            <el-button v-perms="'equipment:import'" :icon="Upload" @click="openImport">批量导入</el-button>
            <el-button v-perms="'equipment:export'" :icon="Download" :disabled="selectedRows.length === 0" @click="handleExportSelected">
              导出选中{{ selectedRows.length > 0 ? `（${selectedRows.length}）` : '' }}
            </el-button>
            <el-button v-perms="'equipment:export'" :icon="Download" @click="handleExport">导出全部</el-button>
            <el-button v-perms="'equipment:delete'" type="danger" plain :icon="Delete" :disabled="selectedCount === 0" @click="handleBatchDelete">
              批量删除{{ selectedCount > 0 ? `（${selectedCount}）` : '' }}
            </el-button>
            <el-button v-if="!selectAllFiltered" link type="primary" :disabled="total === 0" @click="selectAllFiltered = true">
              全选筛选结果（{{ total }} 条）
            </el-button>
            <el-tag v-else type="warning" closable @close="cancelSelectAll">已全选 {{ total }} 条（跨页）</el-tag>
          </div>
          <el-tooltip content="刷新" placement="top">
            <el-button :icon="RefreshRight" circle @click="fetchList" />
          </el-tooltip>
        </div>

        <el-table ref="tableRef" v-loading="loading" :data="list" stripe style="width: 100%" row-key="id" @selection-change="handleSelectionChange">
          <el-table-column type="selection" width="45" reserve-selection />
          <!-- 方案动态列：选中类型且方案 list_columns 非空时按方案渲染，纯文本直出 -->
          <template v-if="dynamicColumns">
            <el-table-column
              v-for="col in dynamicColumns"
              :key="col.key"
              :label="col.label"
              :width="col.width || undefined"
              :min-width="col.width ? undefined : 130"
              show-overflow-tooltip
            >
              <template #default="{ row }">{{ cellValue(row, col.key) }}</template>
            </el-table-column>
          </template>
          <!-- 通用默认列集：「全部」或该类型无方案时保持现有结构 -->
          <template v-else>
          <el-table-column label="状态灯" width="70" align="center">
            <template #default="{ row }">
              <el-tooltip :content="dueStateLabel(row.due_state)" placement="top">
                <StatusDot :state="row.due_state" />
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column prop="code" label="设备编号" min-width="130" show-overflow-tooltip />
          <el-table-column prop="name" label="设备名称" min-width="130" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.name }}
              <el-tag v-if="row.label_missing" type="info" size="small" class="eq-flag">标签缺失</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="110">
            <template #default="{ row }">{{ row.type_label || row.type }}</template>
          </el-table-column>
          <el-table-column label="位置" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ locationText(row) }}</template>
          </el-table-column>
          <el-table-column label="点位" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.point_name">{{ row.point_name }}</span>
              <el-tag v-else type="warning" size="small" effect="plain">未绑定</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="last_maintenance_date" label="最近维保" width="100" align="center">
            <template #default="{ row }">{{ row.last_maintenance_date || '--' }}</template>
          </el-table-column>
          <el-table-column prop="next_due_date" label="下次到期" width="100" align="center">
            <template #default="{ row }">
              <span :class="{ 'text-danger': row.due_state === 'overdue' }">{{ row.next_due_date || '--' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="scrap_date" label="报废日期" width="100" align="center">
            <template #default="{ row }">{{ row.scrap_date || '--' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" size="small">{{ row.status_label || row.status }}</el-tag>
            </template>
          </el-table-column>
          </template>
          <el-table-column label="操作" width="240" fixed="right">
            <template #default="{ row }">
              <el-button v-perms="'equipment:list'" link type="primary" @click="openDetail(row)">详情</el-button>
              <el-button v-perms="'equipment:maintenance'" link type="primary" @click="openRegister(row)">维保登记</el-button>
              <el-button v-perms="'equipment:list'" link type="primary" @click="openHistory(row)">维保历史</el-button>
              <el-button v-perms="'equipment:update'" link type="primary" @click="openForm(row)">编辑</el-button>
              <el-button v-perms="'equipment:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="该条件下暂无设备">
              <el-button v-perms="'equipment:create'" type="primary" @click="openForm()">新增设备</el-button>
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
    </template>

    <!-- ========== 维保确认 ========== -->
    <ConfirmList v-else-if="mainTab === 'confirm'" ref="confirmRef" @changed="fetchPendingTotal" />

    <!-- ========== 维保记录（全状态流水总表） ========== -->
    <MaintRecordList v-else ref="recordRef" />

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑设备' : '新增设备'" width="640px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="96px">
        <el-form-item label="位置绑定" prop="path">
          <el-cascader
            v-model="form.path"
            :options="cascaderOptions"
            :props="cascaderProps"
            placeholder="选择 小区 / 楼栋 / 点位（点位可空）"
            style="width: 100%"
            clearable
            filterable
            @expand-change="handleCascaderExpand"
          />
          <div class="text-secondary">小区必选；楼栋/点位选填（设备挪位置时重新选择即改绑，点位类型不匹配仅提示不拦截）</div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="设备编号" prop="code">
              <el-input v-model="form.code" placeholder="租户内唯一，如 XCCT-MH-0001" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="设备名称" prop="name">
              <el-input v-model="form.name" placeholder="如：1栋3楼灭火器" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="设备类型" prop="type">
              <el-select v-model="form.type" placeholder="选择类型" filterable style="width: 100%" @change="syncFormSchema">
                <el-option v-for="d in typeOptions" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio value="in_service">在用</el-radio>
                <el-radio value="stopped">停用</el-radio>
                <el-radio value="scrapped">报废</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="出厂日期">
              <el-date-picker v-model="form.manufacture_date" type="date" value-format="YYYY-MM-DD" placeholder="选填" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="下次到期日">
              <el-date-picker v-model="form.next_due_date" type="date" value-format="YYYY-MM-DD" placeholder="留空按类型规则自动算" style="width: 100%" />
              <div class="text-secondary">人工覆盖用；留空{{ form.id ? '保持原值' : '按类型规则计算' }}</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注">
          <el-input v-model="form.remark" placeholder="选填" maxlength="255" />
        </el-form-item>
        <!-- 类型方案表单专属字段：值存 extra 口袋随提交 -->
        <template v-if="formFields.length">
          <el-form-item v-for="f in formFields" :key="f.key" :label="f.label">
            <el-date-picker
              v-if="f.type === 'date'"
              v-model="form.extra[f.key]"
              type="date"
              value-format="YYYY-MM-DD"
              placeholder="选填"
              style="width: 100%"
            />
            <el-input v-else v-model="form.extra[f.key]" placeholder="选填" maxlength="128" />
          </el-form-item>
        </template>
        <!-- 其他档案字段：extra 中已有但不在当前类型方案 form_fields 里的历史键（如导入/换类型遗留），可编辑并随 extra 一起提交 -->
        <el-collapse v-if="otherExtraFields.length">
          <el-collapse-item :title="`其他档案字段（${otherExtraFields.length}）`" name="extra">
            <el-form-item v-for="e in otherExtraFields" :key="e.key" :label="e.label">
              <el-input v-model="form.extra[e.key]" placeholder="选填" maxlength="128" />
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 维保登记对话框 -->
    <MaintenanceFormDialog v-model:visible="registerVisible" :equipment="registerTarget" @saved="fetchPendingTotal" />

    <!-- 设备详情抽屉：基础字段 + extra 档案信息（空值不显示） -->
    <el-drawer v-model="detailVisible" :title="detailRow ? `设备详情：${detailRow.name}（${detailRow.code}）` : '设备详情'" size="480px">
      <template v-if="detailRow">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="设备编号">{{ detailRow.code }}</el-descriptions-item>
          <el-descriptions-item label="设备名称">{{ detailRow.name }}</el-descriptions-item>
          <el-descriptions-item label="设备类型">{{ detailRow.type_label || detailRow.type }}</el-descriptions-item>
          <el-descriptions-item label="位置">{{ locationText(detailRow) }}</el-descriptions-item>
          <el-descriptions-item label="出厂日期">{{ detailRow.manufacture_date || '--' }}</el-descriptions-item>
          <el-descriptions-item label="最近维保">{{ detailRow.last_maintenance_date || '--' }}</el-descriptions-item>
          <el-descriptions-item label="下次到期">{{ detailRow.next_due_date || '--' }}</el-descriptions-item>
          <el-descriptions-item label="报废日期">{{ detailRow.scrap_date || '--' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagType(detailRow.status)" size="small">{{ detailRow.status_label || detailRow.status }}</el-tag>
            <el-tag v-if="detailRow.label_missing" type="info" size="small" style="margin-left: 8px">标签缺失</el-tag>
          </el-descriptions-item>
          <el-descriptions-item v-if="detailRow.remark" label="备注">{{ detailRow.remark }}</el-descriptions-item>
        </el-descriptions>
        <template v-if="detailExtras.length">
          <div class="archive-title">档案信息</div>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item v-for="e in detailExtras" :key="e.key" :label="e.label">{{ e.value }}</el-descriptions-item>
          </el-descriptions>
        </template>
      </template>
    </el-drawer>

    <!-- 维保历史抽屉 -->
    <el-drawer v-model="historyVisible" :title="`维保历史${historyTarget ? `：${historyTarget.name}（${historyTarget.code}）` : ''}`" size="520px">
      <el-timeline v-loading="historyLoading" class="history-timeline">
        <el-timeline-item
          v-for="m in historyList"
          :key="m.id"
          :timestamp="m.maintenance_date"
          :type="confirmStatusType(m.confirm_status)"
          placement="top"
        >
          <div class="history-item">
            <div class="history-head">
              <span class="history-type">{{ maintTypeLabel(m.maintenance_type) }}</span>
              <el-tag :type="confirmStatusType(m.confirm_status)" size="small">{{ confirmStatusLabel(m.confirm_status) }}</el-tag>
              <el-tag v-if="m.ai_verdict === 'review'" type="danger" size="small">AI 存疑</el-tag>
              <el-tag v-else-if="m.ai_verdict === 'pass'" type="success" size="small">AI 通过</el-tag>
            </div>
            <div class="history-line">经办人：{{ m.operator_name }}<template v-if="m.vendor">　单位：{{ m.vendor }}</template></div>
            <div v-if="m.note" class="history-line">备注：{{ m.note }}</div>
            <div v-if="m.reject_reason" class="history-line text-danger">驳回理由：{{ m.reject_reason }}</div>
            <div v-if="m.ai_reason" class="history-line text-secondary">AI 说明：{{ m.ai_reason }}</div>
            <div v-if="m.photos.length" class="history-photos">
              <el-image
                v-for="(p, i) in m.photos"
                :key="p.file_id"
                :src="withFileToken(p.url)"
                fit="cover"
                class="photo-thumb"
                :preview-src-list="m.photos.map((x) => withFileToken(x.url))"
                :initial-index="i"
                preview-teleported
              />
            </div>
            <div class="history-line text-secondary">
              登记：{{ m.created_by_name }} {{ m.created_at }}
              <template v-if="m.confirmed_at">　确认：{{ m.confirmed_by_name }} {{ m.confirmed_at }}</template>
            </div>
          </div>
        </el-timeline-item>
      </el-timeline>
      <el-empty v-if="!historyLoading && !historyList.length" description="暂无维保记录" />
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="historyQuery.page"
          v-model:page-size="historyQuery.page_size"
          :total="historyTotal"
          layout="total, prev, pager, next"
          @change="fetchHistory"
        />
      </div>
    </el-drawer>

    <!-- 批量导入：三步向导（列头直接兼容甲方台账结构） -->
    <EquipmentImportDialog v-model:visible="importVisible" :initial-community-id="query.community_id" @done="fetchList" />
  </div>
</template>

<script setup lang="ts">
import { computed, onActivated, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  ElMessage, ElMessageBox,
  type FormInstance, type FormRules
} from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Upload, Download, Delete } from '@element-plus/icons-vue'
import {
  listEquipment, createEquipment, updateEquipment, deleteEquipment, batchDeleteEquipment,
  listMaintenancePending, listMaintenanceHistory,
  getTypeSchema,
  type EquipmentItem, type EquipmentStatus, type DueState, type BindState,
  type MaintenanceItem, type MaintenanceType, type ConfirmStatus,
  type EquipmentTypeSchema
} from '@/api/equipment'
import { withFileToken } from '@/api/upload'
import { listCommunityTree } from '@/api/community'
import { listPoints } from '@/api/point'
import { downloadFile, downloadFilePost } from '@/utils/download'
import { useDictOptions } from '@/composables/useDictOptions'
import { useCommunities } from '@/composables/useCommunities'
import { usePagedList } from '@/composables/usePagedList'
import ConfirmList from './ConfirmList.vue'
import MaintRecordList from './MaintRecordList.vue'
import StatusDot from '@/components/StatusDot.vue'
import MaintenanceFormDialog from './MaintenanceFormDialog.vue'
import EquipmentImportDialog from './EquipmentImportDialog.vue'

// ===== 主 tab =====
const route = useRoute()
const mainTab = ref<'list' | 'confirm' | 'records'>('list')
const confirmRef = ref<InstanceType<typeof ConfirmList>>()
const recordRef = ref<InstanceType<typeof MaintRecordList>>()

// ===== 字典与小区（共享缓存 composable） =====
const { options: typeOptions } = useDictOptions('equipment_type')
const { options: maintTypeOptions } = useDictOptions('equipment_maint_type')
const { communities } = useCommunities()

const statusOptions: { label: string; value: EquipmentStatus }[] = [
  { label: '在用', value: 'in_service' },
  { label: '维保中', value: 'maintaining' },
  { label: '停用', value: 'stopped' },
  { label: '报废', value: 'scrapped' }
]

// ===== 列表 =====
const { loading, list, total, query, fetchList, handleSearch: searchList } = usePagedList(
  (q) => listEquipment({
    page: q.page,
    page_size: q.page_size,
    type: q.type || undefined,
    community_id: q.community_id || undefined,
    status: (q.status || undefined) as EquipmentStatus | undefined,
    due_state: (q.due_state || undefined) as DueState | undefined,
    bind_state: (q.bind_state || undefined) as BindState | undefined,
    keyword: q.keyword || undefined
  }),
  () => ({ page: 1, page_size: 20, type: '', community_id: '', status: '', due_state: '', bind_state: '', keyword: '' })
)

function handleSearch() {
  selectAllFiltered.value = false // 筛选条件变化后跨页全选失效（目标集已变）
  syncTypeSchema()
  searchList()
}

// ===== 类型字段方案（页签动态列 + 表单专属字段）：按类型缓存，无方案记 null 走通用默认 =====
const schemaCache = new Map<string, EquipmentTypeSchema | null>()
const schemaInflight = new Map<string, Promise<EquipmentTypeSchema | null>>()

function ensureSchema(type: string): Promise<EquipmentTypeSchema | null> {
  if (schemaCache.has(type)) return Promise.resolve(schemaCache.get(type) ?? null)
  let p = schemaInflight.get(type)
  if (!p) {
    p = getTypeSchema(type)
      .then((s) => {
        schemaCache.set(type, s)
        return s
      })
      .catch(() => {
        schemaCache.set(type, null) // 拉取失败按无方案处理，不阻断列表
        return null
      })
      .finally(() => {
        schemaInflight.delete(type)
      })
    schemaInflight.set(type, p)
  }
  return p
}

// 当前列表类型（页签/筛选 select 同一 query.type）的方案
const currentSchema = ref<EquipmentTypeSchema | null>(null)
const dynamicColumns = computed(() => {
  const cols = currentSchema.value?.config?.list_columns
  return cols && cols.length ? cols : null
})

async function syncTypeSchema() {
  const t = query.type
  if (!t) {
    currentSchema.value = null
    return
  }
  const s = await ensureSchema(t)
  if (query.type === t) currentSchema.value = s // 防止快速切换时旧响应覆盖新页签
}

// 方案列取值：key 优先取行顶层字段，取不到回退 extra 口袋
function cellValue(row: EquipmentItem, key: string) {
  const v = (row as unknown as Record<string, unknown>)[key]
  if (v != null && String(v) !== '') return String(v)
  const ev = row.extra?.[key]
  return ev != null && String(ev) !== '' ? String(ev) : '--'
}

function handleReset() {
  query.type = ''
  query.community_id = ''
  query.status = ''
  query.due_state = ''
  query.bind_state = ''
  query.keyword = ''
  handleSearch()
}

// 维保确认 tab 角标（pending 总数）
const pendingTotal = ref(0)

async function fetchPendingTotal() {
  try {
    const d = await listMaintenancePending({ page: 1, page_size: 1 })
    pendingTotal.value = d.total
  } catch {
    // 无 equipment:confirm 权限时静默（角标不显示）
  }
}

// 位置展示：小区 · 楼栋 · 点位（remark 中含安装位置拼接，截断展示）
function locationText(row: EquipmentItem) {
  const parts = [row.community_name, row.building_name, row.point_name].filter(Boolean)
  return parts.length ? parts.join(' · ') : (row.remark || '--')
}

function dueStateLabel(s: DueState) {
  return { normal: '正常', warning: '临期', overdue: '已逾期', none: '无到期日', scrap: '应报废', label_missing: '标签缺失' }[s] || s
}

function statusTagType(s: EquipmentStatus) {
  return { in_service: 'success', maintaining: 'warning', stopped: 'info', scrapped: 'danger' }[s] as 'success' | 'warning' | 'info' | 'danger'
}

function maintTypeLabel(t: MaintenanceType) {
  return maintTypeOptions.value.find((d) => d.value === t)?.label || t
}

function confirmStatusLabel(s: ConfirmStatus) {
  return { pending: '待确认', confirmed: '已确认', rejected: '已驳回' }[s] || s
}

function confirmStatusType(s: ConfirmStatus) {
  return { pending: 'warning', confirmed: 'success', rejected: 'danger' }[s] as 'warning' | 'success' | 'danger'
}

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  path: [] as string[], // ['c:{小区id}', 'b:{楼栋id}'?, 'p:{点位id}'?]
  code: '',
  name: '',
  type: '',
  manufacture_date: '',
  next_due_date: '',
  status: 'in_service' as EquipmentStatus,
  remark: '',
  extra: {} as Record<string, string> // 类型方案专属字段值口袋（编辑时先回填整份 extra，避免后端整体替换丢历史键）
})

// 当前 form.type 方案的表单专属字段
const formSchema = ref<EquipmentTypeSchema | null>(null)
const formFields = computed(() => formSchema.value?.config?.form_fields ?? [])

async function syncFormSchema() {
  const t = form.type
  if (!t) {
    formSchema.value = null
    return
  }
  const s = await ensureSchema(t)
  if (form.type === t) formSchema.value = s
}

const formRules: FormRules = {
  path: [{ required: true, type: 'array', min: 1, message: '请选择所属小区', trigger: 'change' }],
  code: [{ required: true, message: '请输入设备编号', trigger: 'blur' }],
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择设备类型', trigger: 'change' }]
}

// 级联选择器：小区 → 楼栋 → 点位（点位节点 leaf，小区/楼栋可直接选中表示不绑楼栋/点位）
interface CascaderNode {
  value: string
  label: string
  children?: CascaderNode[]
}
const cascaderOptions = ref<CascaderNode[]>([])
const cascaderProps = { value: 'value', label: 'label', children: 'children', checkStrictly: true }
const pointsLoaded = new Set<string>() // 已加载点位的小区 id

async function buildCascader() {
  const tree = await listCommunityTree()
  cascaderOptions.value = tree.map((c) => ({
    value: `c:${c.id}`,
    label: c.name,
    children: c.buildings.map((b) => ({ value: `b:${b.id}`, label: b.name, children: [] }))
  }))
}

// 展开小区时加载其点位（page_size 上限 100，超出部分不可选——点位绑定主路径是 App 扫码）
async function loadCommunityPoints(communityId: string) {
  if (pointsLoaded.has(communityId)) return
  pointsLoaded.add(communityId)
  try {
    const d = await listPoints({ community_id: communityId, page: 1, page_size: 100 })
    const cNode = cascaderOptions.value.find((n) => n.value === `c:${communityId}`)
    if (!cNode) return
    const communityPoints: CascaderNode[] = []
    for (const p of d.list) {
      const pNode: CascaderNode = { value: `p:${p.id}`, label: p.name }
      if (p.building_id) {
        const bNode = cNode.children?.find((n) => n.value === `b:${p.building_id}`)
        if (bNode) {
          bNode.children = [...(bNode.children || []), pNode]
          continue
        }
      }
      communityPoints.push(pNode)
    }
    cNode.children = [...(cNode.children || []), ...communityPoints]
  } catch {
    pointsLoaded.delete(communityId) // 失败允许重试
  }
}

function handleCascaderExpand(path: (string | number)[]) {
  const first = String(path[0] || '')
  if (first.startsWith('c:')) loadCommunityPoints(first.slice(2))
}

async function openForm(row?: EquipmentItem) {
  formRef.value?.clearValidate()
  if (cascaderOptions.value.length === 0) await buildCascader()
  if (row) {
    // 编辑回填：先加载该小区点位，再还原级联路径
    await loadCommunityPoints(row.community_id)
    const path = [`c:${row.community_id}`]
    if (row.building_id) path.push(`b:${row.building_id}`)
    if (row.point_id) path.push(`p:${row.point_id}`)
    Object.assign(form, {
      id: row.id, path, code: row.code, name: row.name, type: row.type,
      manufacture_date: row.manufacture_date || '', next_due_date: row.next_due_date || '',
      status: row.status, remark: row.remark || '', extra: { ...(row.extra || {}) }
    })
  } else {
    Object.assign(form, {
      id: '', path: [], code: '', name: '', type: '',
      manufacture_date: '', next_due_date: '', status: 'in_service', remark: '', extra: {}
    })
  }
  syncFormSchema()
  formVisible.value = true
}

// 解析级联路径：末段决定绑定粒度（点位/楼栋/小区）
function parsePath(path: string[]) {
  let communityId = ''
  let buildingId: string | null = null
  let pointId: string | null = null
  for (const seg of path) {
    if (seg.startsWith('c:')) communityId = seg.slice(2)
    else if (seg.startsWith('b:')) buildingId = seg.slice(2)
    else if (seg.startsWith('p:')) pointId = seg.slice(2)
  }
  return { communityId, buildingId, pointId }
}

async function handleSubmit() {
  await formRef.value?.validate()
  const { communityId, buildingId, pointId } = parsePath(form.path)
  if (!communityId) {
    ElMessage.warning('请选择所属小区')
    return
  }
  // extra 口袋：剔除空值；编辑时携带整份（后端 SaveReq.Extra 为整体替换）
  const extraEntries = Object.entries(form.extra)
    .filter(([, v]) => v != null && String(v).trim() !== '')
    .map(([k, v]) => [k, String(v)] as [string, string])
  const payload = {
    community_id: communityId,
    building_id: buildingId,
    point_id: pointId,
    type: form.type,
    code: form.code.trim(),
    name: form.name.trim(),
    manufacture_date: form.manufacture_date || '',
    next_due_date: form.next_due_date || '',
    status: form.status,
    remark: form.remark,
    extra: extraEntries.length ? Object.fromEntries(extraEntries) : undefined
  }
  submitting.value = true
  try {
    const res = form.id ? await updateEquipment(form.id, payload) : await createEquipment(payload)
    if (res.warning) {
      ElMessage.warning(res.warning)
    } else {
      ElMessage.success(form.id ? '设备已更新' : '设备已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: EquipmentItem) {
  const ok = await ElMessageBox.confirm(
    `删除后维保历史保留但设备不可再用，确定删除设备「${row.name}（${row.code}）」吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteEquipment(row.id)
  ElMessage.success('已删除')
  fetchList()
}

// ===== 批量删除 / 勾选导出（跨页选择：reserve-selection 保持勾选，全选走筛选条件模式） =====
const tableRef = ref()
const selectedRows = ref<EquipmentItem[]>([])
const selectAllFiltered = ref(false)
const selectedCount = computed(() => (selectAllFiltered.value ? total.value : selectedRows.value.length))

function handleSelectionChange(rows: EquipmentItem[]) {
  selectedRows.value = rows
}

function cancelSelectAll() {
  selectAllFiltered.value = false
  selectedRows.value = []
  tableRef.value?.clearSelection()
}

async function handleBatchDelete() {
  const n = selectedCount.value
  if (n === 0) return
  const scopeText = selectAllFiltered.value ? `当前筛选结果的全部 ${n} 台设备` : `选中的 ${n} 台设备`
  const ok = await ElMessageBox.confirm(
    `删除后维保历史保留但设备不可再用，确定删除${scopeText}吗？`,
    '批量删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  const res = await batchDeleteEquipment(
    selectAllFiltered.value
      ? {
          all: true,
          type: query.type || undefined,
          community_id: query.community_id || undefined,
          status: query.status || undefined,
          due_state: query.due_state || undefined,
          bind_state: query.bind_state || undefined,
          keyword: query.keyword || undefined
        }
      : { ids: selectedRows.value.map((r) => r.id) }
  )
  ElMessage.success(`已删除 ${res.deleted} 台设备`)
  cancelSelectAll()
  fetchList()
}

function handleExportSelected() {
  if (selectedRows.value.length === 0) return
  downloadFilePost('/equipment/export', { ids: selectedRows.value.map((r) => r.id) }, '设备台账_选中.xlsx')
}

// ===== 维保登记（表单与提交在 MaintenanceFormDialog 内，父组件只持有打开入口） =====
const registerVisible = ref(false)
const registerTarget = ref<EquipmentItem | null>(null)

function openRegister(row: EquipmentItem) {
  registerTarget.value = row
  registerVisible.value = true
}

// ===== 设备详情抽屉（extra 档案信息键值中文对照，空值不显示） =====
const detailVisible = ref(false)
const detailRow = ref<EquipmentItem | null>(null)

// extra 键 → 中文标签（与导入映射一致）
const extraLabels: [string, string][] = [
  ['project_name', '项目名称'], ['room', '机房名称'], ['level', '设备等级'], ['dept', '责任部门'],
  ['system', '所属设备系统'], ['brand', '品牌'], ['spec', '规格型号'], ['original_value', '设备原值'],
  ['quantity', '数量'], ['put_into_service', '投运日期'], ['maint_status', '维保状态'], ['run_status', '运行状态'],
  ['origin', '产地'], ['manufacturer_contact', '厂家联系人'], ['installer_contact', '安装单位联系人电话'],
  ['vendor', '维保单位'], ['vendor_contact', '维保单位联系人电话'], ['other_info', '其他信息']
]

// extra 中已有、但不在当前类型方案 form_fields 里的键（标签复用 extraLabels 映射，未收录的键回退原始键名）
const otherExtraFields = computed(() => {
  const schemaKeys = new Set(formFields.value.map((f) => f.key))
  const labelMap = new Map(extraLabels)
  return Object.keys(form.extra)
    .filter((k) => !schemaKeys.has(k))
    .map((k) => ({ key: k, label: labelMap.get(k) || k }))
})

const detailExtras = computed(() => {
  const extra = detailRow.value?.extra
  if (!extra) return [] as { key: string; label: string; value: string }[]
  const out: { key: string; label: string; value: string }[] = []
  for (const [key, label] of extraLabels) {
    const v = extra[key]
    if (v != null && String(v).trim() !== '') {
      out.push({ key, label, value: String(v) })
    }
  }
  return out
})

function openDetail(row: EquipmentItem) {
  detailRow.value = row
  detailVisible.value = true
}

// ===== 维保历史抽屉 =====
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyTarget = ref<EquipmentItem | null>(null)
const historyList = ref<MaintenanceItem[]>([])
const historyTotal = ref(0)
const historyQuery = reactive({ page: 1, page_size: 20 })

function openHistory(row: EquipmentItem) {
  historyTarget.value = row
  historyQuery.page = 1
  historyVisible.value = true
  fetchHistory()
}

async function fetchHistory() {
  if (!historyTarget.value) return
  historyLoading.value = true
  try {
    const d = await listMaintenanceHistory(historyTarget.value.id, historyQuery)
    historyList.value = d.list
    historyTotal.value = d.total
  } finally {
    historyLoading.value = false
  }
}

// ===== 导出 =====
function handleExport() {
  downloadFile('/equipment/export', {
    type: query.type || undefined,
    community_id: query.community_id || undefined,
    status: query.status || undefined,
    due_state: query.due_state || undefined,
    bind_state: query.bind_state || undefined,
    keyword: query.keyword || undefined
  }, '设备台账.xlsx')
}

// ===== 批量导入：三步向导（流程在 EquipmentImportDialog 内，父组件只持有打开入口） =====
const importVisible = ref(false)

function openImport() {
  importVisible.value = true
}

onMounted(() => {
  // 点位页「关联设备」跳转带 keyword 定位
  const kw = route.query.keyword
  if (typeof kw === 'string' && kw) {
    query.keyword = kw
  }
  fetchList()
  fetchPendingTotal()
})

// keep-alive 缓存页：再次激活（非首次）时重拉当前 tab 数据，分页/筛选状态保持不变
let activated = false
onActivated(() => {
  if (!activated) {
    activated = true
    return
  }
  fetchList()
  fetchPendingTotal()
  confirmRef.value?.fetchList()
  recordRef.value?.fetchList()
})
</script>

<style scoped lang="scss">
.main-tabs-card {
  padding-bottom: 0;
  margin-bottom: $spacing-lg;

  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }
}

.type-tabs {
  margin-bottom: $spacing-md;
}

.eq-flag {
  margin-left: $spacing-sm;
}

.text-danger {
  color: $color-danger;
}

.photo-thumb {
  width: 64px;
  height: 64px;
  border-radius: $radius-small;
}

.archive-title {
  font-weight: 600;
  margin: $spacing-lg 0 $spacing-md;
  color: $color-text-primary;
}

.history-timeline {
  padding-left: $spacing-xs;
}

.history-item {
  .history-head {
    display: flex;
    gap: $spacing-sm;
    align-items: center;
    margin-bottom: $spacing-xs;

    .history-type {
      font-weight: 600;
    }
  }

  .history-line {
    font-size: $font-size-aux;
    margin-top: 2px;
  }

  .history-photos {
    display: flex;
    gap: $spacing-sm;
    margin-top: $spacing-xs;
  }
}
</style>
