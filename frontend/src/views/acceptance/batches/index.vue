<template>
  <div class="app-container">
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="批次名称"><el-input v-model="query.name" clearable style="width: 190px" @keyup.enter="handleSearch" /></el-form-item>
        <el-form-item label="小区"><el-select v-model="query.community_id" clearable filterable style="width: 170px"><el-option v-for="item in communities" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="状态"><el-select v-model="query.status" clearable style="width: 130px"><el-option v-for="(label, key) in statusLabels" :key="key" :label="label" :value="key" /></el-select></el-form-item>
        <el-form-item><el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button><el-button :icon="Refresh" @click="handleReset">重置</el-button></el-form-item>
      </el-form>
    </div>
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'acceptance:manage'" type="primary" :icon="Plus" :disabled="!schemas.length" @click="openCreate">新建批次</el-button>
          <el-button v-if="!schemas.length" link type="primary" @click="router.push('/templates/acceptance')">配置内容模板</el-button>
        </div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="fetchList" />
      </div>
      <el-table v-loading="loading" :data="list">
        <el-table-column prop="name" label="批次名称" min-width="190" show-overflow-tooltip />
        <el-table-column label="小区" min-width="130"><template #default="{ row }">{{ communityName(row.community_id) }}</template></el-table-column>
        <el-table-column label="台账类型" width="120"><template #default="{ row }">{{ row.ledger_type === 'institution_change' ? '机构更迭' : '分户验收' }}</template></el-table-column>
        <el-table-column label="查验进度" min-width="160"><template #default="{ row }"><el-progress :percentage="progress(row)" :format="() => `${row.covered_count}/${row.target_count}`" /></template></el-table-column>
        <el-table-column label="问题" width="80" align="right"><template #default="{ row }">{{ row.problem_count }}</template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabels[row.status] }}</el-tag></template></el-table-column>
        <el-table-column prop="manager_name" label="负责人" width="110" />
        <el-table-column label="操作" width="90" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="router.push(`/acceptance/batches/${row.id}`)">详情</el-button></template></el-table-column>
        <template #empty><el-empty description="还没有承接查验批次" /></template>
      </el-table>
      <div class="pagination-wrap"><el-pagination v-model:current-page="query.page" v-model:page-size="query.page_size" :total="total" :page-sizes="[10, 20, 50, 100]" layout="total, sizes, prev, pager, next" @change="fetchList" /></div>
    </div>

    <el-dialog v-model="createVisible" title="新建查验批次" width="760px" :close-on-click-modal="false">
      <el-form :model="form" label-width="100px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="批次名称" required><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="小区" required><el-select v-model="form.community_id" filterable style="width: 100%" @change="loadTargets"><el-option v-for="item in communities" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="台账类型" required><el-select v-model="form.ledger_type" style="width: 100%"><el-option label="机构更迭" value="institution_change" /><el-option label="分户验收" value="unit_acceptance" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="查验期间" required><el-date-picker v-model="form.period" type="daterange" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="查验快照"><el-input :model-value="`${schemas.length} 个启用专项，${schemas.reduce((sum, item) => sum + item.rows.length, 0)} 个清单行`" disabled /></el-form-item>
        <el-form-item label="查验对象" required>
          <div class="target-editor">
            <div v-for="(target, index) in form.targets" :key="index" class="target-row">
              <el-select v-model="target.target_type" style="width: 120px" @change="target.ref_id = ''; target.name = ''; target.building_id = undefined"><el-option label="资料集合" value="documents" /><el-option label="楼栋" value="building" /><el-option label="房屋" value="unit" /><el-option label="区域" value="area" /><el-option label="设备" value="equipment" /></el-select>
              <el-select v-model="target.category_code" placeholder="专项类别" style="width: 135px" @change="target.table_codes = []"><el-option v-for="(label, key) in categoryLabels" :key="key" :label="label" :value="key" /></el-select>
              <el-select v-model="target.table_codes" multiple collapse-tags placeholder="分配专项" style="min-width: 210px"><el-option v-for="schema in schemas.filter((item) => item.category === target.category_code)" :key="schema.code" :label="schema.name" :value="schema.code" /></el-select>
              <el-select v-if="target.target_type === 'unit'" v-model="target.building_id" placeholder="所属楼栋" style="width: 150px"><el-option v-for="building in buildings" :key="building.id" :label="building.name" :value="building.id" /></el-select>
              <el-select v-if="target.target_type === 'building'" v-model="target.ref_id" placeholder="选择楼栋" style="width: 180px"><el-option v-for="building in buildings" :key="building.id" :label="building.name" :value="building.id" /></el-select>
              <template v-else><el-input v-model="target.ref_id" placeholder="对象标识" style="width: 150px" /><el-input v-model="target.name" placeholder="对象名称" style="width: 170px" /></template>
              <el-button link type="danger" :disabled="form.targets.length === 1" @click="form.targets.splice(index, 1)">删除</el-button>
            </div>
            <el-button link type="primary" :icon="Plus" @click="addTarget">添加对象</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="createVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="createBatch">创建并进入分工</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Refresh, RefreshRight, Search } from '@element-plus/icons-vue'
import { listAcceptanceBatches, listAcceptanceSchemas, createAcceptanceBatch, type AcceptanceBatch, type AcceptanceSchema } from '@/api/acceptance'
import { listBuildings } from '@/api/community'
import { useCommunities } from '@/composables/useCommunities'
import { usePagedList } from '@/composables/usePagedList'
import type { BuildingItem } from '@/api/biz-types'

const router = useRouter()
const { communities } = useCommunities()
const statusLabels: Record<string, string> = { draft: '草稿', inspecting: '进行中', reviewing: '审核中', handover: '待签署', done: '已归档' }
const categoryLabels: Record<string, string> = { documents: '物业资料', building: '房屋本体', equipment: '设施设备', outdoor: '室外场地', office: '物业用房', garage: '地下车库', unit: '分户验收' }
const { loading, list, total, query, fetchList, handleSearch, handleReset } = usePagedList((q) => listAcceptanceBatches({ ...q, name: q.name || undefined, community_id: q.community_id || undefined, status: q.status || undefined }), () => ({ page: 1, page_size: 20, name: '', community_id: '', status: '' }))
const schemas = ref<AcceptanceSchema[]>([])
const buildings = ref<BuildingItem[]>([])
const communityName = (id: string) => communities.value.find((item) => item.id === id)?.name || id
const progress = (row: AcceptanceBatch) => row.target_count ? Math.round(row.covered_count * 100 / row.target_count) : 0
const statusType = (status: string) => status === 'done' ? 'success' : status === 'reviewing' || status === 'handover' ? 'warning' : 'info'
const createVisible = ref(false)
const saving = ref(false)
type TargetDraft = { target_type: string; ref_id: string; building_id?: string; category_code: string; table_codes: string[]; name: string }
const form = ref({ name: '', community_id: '', ledger_type: 'institution_change', period: [] as string[], targets: [{ target_type: 'documents', ref_id: 'documents', category_code: 'documents', table_codes: [], name: '物业资料' }] as TargetDraft[] })
function openCreate() {
  form.value.name = ''
  form.value.targets = [{ target_type: 'documents', ref_id: 'documents', category_code: 'documents', table_codes: [], name: '物业资料' }]
  createVisible.value = true
}
async function loadTargets() {
  buildings.value = form.value.community_id ? (await listBuildings({ community_id: form.value.community_id, page_size: 100 })).list : []
}
function addTarget() {
  form.value.targets.push({ target_type: 'building', ref_id: '', category_code: 'building', table_codes: [], name: '' })
}
async function createBatch() {
  saving.value = true
  try {
    const created = await createAcceptanceBatch({ community_id: form.value.community_id, name: form.value.name, ledger_type: form.value.ledger_type, period_start: form.value.period[0], period_end: form.value.period[1], targets: form.value.targets })
    createVisible.value = false
    ElMessage.success('批次已创建')
    router.push(`/acceptance/batches/${created.id}`)
  } finally { saving.value = false }
}
onMounted(async () => {
  await Promise.all([fetchList(), listAcceptanceSchemas().then((items) => { schemas.value = items.filter((item) => item.status === 'enabled') })])
})
</script>

<style scoped>
.target-editor { width: 100%; }
.target-row { display: flex; gap: 8px; margin-bottom: 8px; }
</style>
