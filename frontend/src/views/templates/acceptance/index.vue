<template>
  <div class="app-container">
    <div class="table-card">
      <el-tabs v-model="category">
        <el-tab-pane v-for="(label, key) in categoryLabels" :key="key" :label="label" :name="key" />
      </el-tabs>
      <div class="table-toolbar">
        <div class="table-toolbar-left"><el-button v-perms="'acceptance:schema:update'" type="primary" :icon="Plus" @click="openSchema()">新增专项</el-button></div>
        <el-button :icon="RefreshRight" circle title="刷新" @click="load" />
      </div>
      <el-table v-loading="loading" :data="filteredSchemas" row-key="code">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="row-list">
              <div class="row-toolbar"><strong>清单行</strong><el-button v-perms="'acceptance:schema:update'" link type="primary" @click="openRow(row.code)">新增行</el-button></div>
              <el-table :data="row.rows" size="small">
                <el-table-column prop="code" label="行编码" min-width="120" />
                <el-table-column prop="name" label="名称" min-width="150" />
                <el-table-column prop="specification" label="规格" min-width="130" />
                <el-table-column prop="unit" label="单位" width="80" />
                <el-table-column label="必填" width="80"><template #default="{ row: item }">{{ item.required ? '必填' : '选填' }}</template></el-table-column>
                <el-table-column label="状态" width="80"><template #default="{ row: item }">{{ item.status === 'enabled' ? '启用' : '停用' }}</template></el-table-column>
                <el-table-column label="操作" width="80"><template #default="{ row: item }"><el-button v-perms="'acceptance:schema:update'" link type="primary" @click="openRow(row.code, item)">编辑</el-button></template></el-table-column>
              </el-table>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="专项" min-width="180" />
        <el-table-column label="表型" width="130"><template #default="{ row }">{{ kindLabels[row.table_kind] }}</template></el-table-column>
        <el-table-column label="关联检查模板" min-width="180"><template #default="{ row }">{{ row.template_name || '—' }}</template></el-table-column>
        <el-table-column label="行数" width="90" align="right"><template #default="{ row }">{{ row.rows.length }}</template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ statusLabels[row.status] }}</el-tag></template></el-table-column>
        <el-table-column label="来源" width="90"><template #default="{ row }">{{ row.source === 'platform' ? '平台' : '租户' }}</template></el-table-column>
        <el-table-column label="操作" width="90" fixed="right"><template #default="{ row }"><el-button v-perms="'acceptance:schema:update'" link type="primary" @click="openSchema(row)">编辑</el-button></template></el-table-column>
        <template #empty><el-empty description="该类别还没有专项" /></template>
      </el-table>
    </div>

    <el-drawer v-model="schemaVisible" :title="schemaForm.existing ? '编辑专项' : '新增专项'" size="480px">
      <el-form :model="schemaForm" label-width="100px">
        <el-form-item label="专项编码" required><el-input v-model="schemaForm.code" :disabled="schemaForm.existing" placeholder="如 outdoor_garden" /></el-form-item>
        <el-form-item label="专项名称" required><el-input v-model="schemaForm.name" /></el-form-item>
        <el-form-item label="类别" required><el-select v-model="schemaForm.category" style="width: 100%"><el-option v-for="(label, key) in categoryLabels" :key="key" :label="label" :value="key" /></el-select></el-form-item>
        <el-form-item label="表型" required><el-select v-model="schemaForm.table_kind" style="width: 100%"><el-option v-for="(label, key) in kindLabels" :key="key" :label="label" :value="key" /></el-select></el-form-item>
        <el-form-item v-if="schemaForm.table_kind === 'matrix'" label="检查模板" required>
          <el-select v-model="schemaForm.template_id" filterable style="width: 100%"><el-option v-for="template in verifyTemplates" :key="template.id" :label="template.name" :value="template.id" /></el-select>
        </el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="schemaForm.status"><el-radio value="draft">草稿</el-radio><el-radio value="enabled">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="schemaVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitSchema">保存</el-button></template>
    </el-drawer>

    <el-dialog v-model="rowVisible" :title="rowForm.existing ? '编辑清单行' : '新增清单行'" width="520px">
      <el-form :model="rowForm" label-width="90px">
        <el-form-item label="行编码" required><el-input v-model="rowForm.code" :disabled="rowForm.existing" placeholder="如 item_001" /></el-form-item>
        <el-form-item label="名称" required><el-input v-model="rowForm.name" /></el-form-item>
        <el-form-item label="规格"><el-input v-model="rowForm.specification" /></el-form-item>
        <el-form-item label="单位"><el-input v-model="rowForm.unit" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="rowForm.sort" :min="0" controls-position="right" /></el-form-item>
        <el-form-item label="必填"><el-switch v-model="rowForm.required" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="rowForm.status"><el-radio value="enabled">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="rowVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitRow">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { listAcceptanceSchemas, saveAcceptanceSchema, saveAcceptanceRow, type AcceptanceRow, type AcceptanceSchema } from '@/api/acceptance'
import { listTemplates } from '@/api/template'
import type { TemplateItem } from '@/api/biz-types'

const categoryLabels: Record<string, string> = { documents: '物业资料', building: '房屋本体', equipment: '设施设备', outdoor: '室外场地', office: '物业用房', garage: '地下车库', unit: '分户验收' }
const kindLabels: Record<string, string> = { matrix: '矩阵检查项', documents: '资料清单', keys: '钥匙清单', assets: '固定资产' }
const statusLabels: Record<string, string> = { draft: '草稿', enabled: '启用', disabled: '停用' }
const category = ref('documents')
const loading = ref(false)
const saving = ref(false)
const schemas = ref<AcceptanceSchema[]>([])
const verifyTemplates = ref<TemplateItem[]>([])
const filteredSchemas = computed(() => schemas.value.filter((schema) => schema.category === category.value))
async function load() {
  loading.value = true
  try { schemas.value = await listAcceptanceSchemas() } finally { loading.value = false }
}
const schemaVisible = ref(false)
const schemaForm = ref({ existing: false, code: '', category: 'documents', name: '', table_kind: 'documents' as AcceptanceSchema['table_kind'], template_id: null as string | null, status: 'draft' as AcceptanceSchema['status'] })
function openSchema(schema?: AcceptanceSchema) {
  schemaForm.value = schema ? { ...schema, existing: true } : { existing: false, code: '', category: category.value, name: '', table_kind: 'documents', template_id: null, status: 'draft' }
  schemaVisible.value = true
}
async function submitSchema() {
  saving.value = true
  try {
    const { code, category: schemaCategory, name, table_kind, template_id, status } = schemaForm.value
    await saveAcceptanceSchema(code, { code, category: schemaCategory, name, table_kind, template_id, status })
    schemaVisible.value = false
    ElMessage.success('专项已保存')
    await load()
  } finally { saving.value = false }
}
const rowVisible = ref(false)
const rowForm = ref({ existing: false, tableCode: '', code: '', name: '', specification: '', unit: '', required: false, sort: 0, status: 'enabled' as AcceptanceRow['status'] })
function openRow(tableCode: string, row?: AcceptanceRow) {
  rowForm.value = row ? { ...row, tableCode, existing: true } : { existing: false, tableCode, code: '', name: '', specification: '', unit: '', required: false, sort: 0, status: 'enabled' }
  rowVisible.value = true
}
async function submitRow() {
  saving.value = true
  try {
    const { tableCode, code, name, specification, unit, required, sort, status } = rowForm.value
    await saveAcceptanceRow(tableCode, code, { name, specification, unit, required, sort, status })
    rowVisible.value = false
    ElMessage.success('清单行已保存')
    await load()
  } finally { saving.value = false }
}
onMounted(async () => {
  await Promise.all([load(), listTemplates({ page: 1, page_size: 100, purpose: 'verify', status: 'enabled' }).then((result) => { verifyTemplates.value = result.list })])
})
</script>

<style scoped>
.row-list { padding: 8px 24px 16px; }
.row-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
</style>
