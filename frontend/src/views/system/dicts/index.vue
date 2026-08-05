<template>
  <div class="app-container">
    <div class="dict-layout">
      <!-- 左：字典类型 -->
      <div class="dict-card">
        <div class="dict-card-header">
          <el-input v-model="typeKeyword" placeholder="编码/名称" clearable :prefix-icon="Search" style="width: 150px" @keyup.enter="fetchTypes" />
          <el-button v-perms="'system:dict:create'" type="primary" :icon="Plus" @click="openTypeForm()">新增</el-button>
        </div>
        <el-table
          v-loading="typeLoading"
          :data="typeList"
          highlight-current-row
          size="small"
          @current-change="handleTypeSelect"
        >
          <el-table-column prop="name" label="类型名称" min-width="100" show-overflow-tooltip />
          <el-table-column prop="code" label="编码" min-width="110" show-overflow-tooltip />
          <el-table-column label="操作" width="100" align="center">
            <template #default="{ row }">
              <el-button v-perms="'system:dict:update'" link type="primary" size="small" @click.stop="openTypeForm(row)">编辑</el-button>
              <el-button
                v-perms="'system:dict:delete'"
                link
                type="danger"
                size="small"
                @click.stop="handleDeleteType(row)"
              >删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无字典类型" :image-size="60">
              <el-button v-perms="'system:dict:create'" type="primary" size="small" @click="openTypeForm()">新增类型</el-button>
            </el-empty>
          </template>
        </el-table>
        <div class="pagination-wrap">
          <el-pagination
            v-model:current-page="typeQuery.page"
            v-model:page-size="typeQuery.page_size"
            :total="typeTotal"
            layout="total, prev, pager, next"
            small
            @change="fetchTypes"
          />
        </div>
      </div>

      <!-- 右：字典数据 -->
      <div class="dict-card dict-data-card">
        <template v-if="currentType">
          <div class="dict-card-header">
            <span class="card-title">{{ currentType.name }}（{{ currentType.code }}）</span>
            <el-button v-perms="'system:dict:create'" type="primary" :icon="Plus" @click="openDataForm()">新增数据</el-button>
          </div>
          <el-table v-loading="dataLoading" :data="dataList" stripe>
            <el-table-column prop="label" label="标签" min-width="110" />
            <el-table-column prop="value" label="存储值" min-width="120" show-overflow-tooltip />
            <el-table-column prop="sort" label="排序" width="70" align="center" />
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120" align="center">
              <template #default="{ row }">
                <el-button v-perms="'system:dict:update'" link type="primary" size="small" @click="openDataForm(row)">编辑</el-button>
                <el-button v-perms="'system:dict:delete'" link type="danger" size="small" @click="handleDeleteData(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="该类型暂无数据项" :image-size="60">
                <el-button v-perms="'system:dict:create'" type="primary" size="small" @click="openDataForm()">新增数据</el-button>
              </el-empty>
            </template>
          </el-table>
          <div class="text-secondary dict-tip">两端字典有缓存，修改后将在下次刷新后生效</div>
        </template>
        <el-empty v-else description="请选择左侧字典类型" />
      </div>
    </div>

    <!-- 类型编辑对话框 -->
    <el-dialog v-model="typeFormVisible" :title="typeForm.id ? '编辑字典类型' : '新增字典类型'" width="440px" :close-on-click-modal="false">
      <el-form ref="typeFormRef" :model="typeForm" :rules="typeFormRules" label-width="88px">
        <el-form-item label="类型名称" prop="name">
          <el-input v-model="typeForm.name" placeholder="如：点位类型" />
        </el-form-item>
        <el-form-item label="类型编码" prop="code">
          <el-input v-model="typeForm.code" :disabled="!!typeForm.id" placeholder="如：point_type" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="typeFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="typeSubmitting" @click="handleSubmitType">保存</el-button>
      </template>
    </el-dialog>

    <!-- 数据编辑对话框 -->
    <el-dialog v-model="dataFormVisible" :title="dataForm.id ? '编辑字典数据' : '新增字典数据'" width="440px" :close-on-click-modal="false">
      <el-form ref="dataFormRef" :model="dataForm" :rules="dataFormRules" label-width="88px">
        <el-form-item label="所属类型">
          <el-input :model-value="currentType?.name" disabled />
        </el-form-item>
        <el-form-item label="标签" prop="label">
          <el-input v-model="dataForm.label" placeholder="显示名，如：配电房" />
        </el-form-item>
        <el-form-item label="存储值" prop="value">
          <el-input v-model="dataForm.value" placeholder="如：power_room，同类型下唯一" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="dataForm.sort" :min="0" :max="999" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="dataForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dataFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="dataSubmitting" @click="handleSubmitData">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Plus } from '@element-plus/icons-vue'
import {
  listDictTypes, createDictType, updateDictType, deleteDictType,
  listDictData, createDictData, updateDictData, deleteDictData
} from '@/api/dict'
import type { DictType, DictData } from '@/api/types'

// ===== 字典类型（左栏） =====
const typeLoading = ref(false)
const typeList = ref<DictType[]>([])
const typeTotal = ref(0)
const typeKeyword = ref('')
const typeQuery = reactive({ page: 1, page_size: 10 })
const currentType = ref<DictType | null>(null)

async function fetchTypes() {
  typeLoading.value = true
  try {
    const data = await listDictTypes({ ...typeQuery, name: typeKeyword.value || undefined })
    typeList.value = data.list
    typeTotal.value = data.total
  } finally {
    typeLoading.value = false
  }
}

onMounted(async () => {
  await fetchTypes()
  if (typeList.value.length) {
    currentType.value = typeList.value[0]
    fetchData()
  }
})

function handleTypeSelect(row: DictType | null) {
  if (!row) return
  currentType.value = row
  fetchData()
}

// ===== 字典数据（右栏） =====
const dataLoading = ref(false)
const dataList = ref<DictData[]>([])

async function fetchData() {
  if (!currentType.value) return
  dataLoading.value = true
  try {
    const data = await listDictData({ type_code: currentType.value.code, page: 1, page_size: 100 })
    dataList.value = data.list
  } finally {
    dataLoading.value = false
  }
}

// ===== 类型表单 =====
const typeFormVisible = ref(false)
const typeSubmitting = ref(false)
const typeFormRef = ref<FormInstance>()
const typeForm = reactive({ id: '', name: '', code: '' })

const typeFormRules: FormRules = {
  name: [{ required: true, message: '请输入类型名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入类型编码', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]{1,63}$/, message: '小写字母开头，仅含小写字母数字下划线', trigger: 'blur' }
  ]
}

function openTypeForm(row?: DictType) {
  typeFormRef.value?.clearValidate()
  Object.assign(typeForm, row ? { id: row.id, name: row.name, code: row.code } : { id: '', name: '', code: '' })
  typeFormVisible.value = true
}

async function handleSubmitType() {
  await typeFormRef.value?.validate()
  typeSubmitting.value = true
  try {
    if (typeForm.id) {
      await updateDictType(typeForm.id, { name: typeForm.name })
      ElMessage.success('字典类型已更新')
    } else {
      await createDictType({ name: typeForm.name, code: typeForm.code })
      ElMessage.success('字典类型已创建')
    }
    typeFormVisible.value = false
    fetchTypes()
  } finally {
    typeSubmitting.value = false
  }
}

async function handleDeleteType(row: DictType) {
  const ok = await ElMessageBox.confirm(
    `删除类型「${row.name}」将级联删除其下全部字典数据，删除后不可恢复，确定删除吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteDictType(row.id)
  ElMessage.success('已删除')
  if (currentType.value?.id === row.id) {
    currentType.value = null
    dataList.value = []
  }
  fetchTypes()
}

// ===== 数据表单 =====
const dataFormVisible = ref(false)
const dataSubmitting = ref(false)
const dataFormRef = ref<FormInstance>()
const dataForm = reactive({ id: '', label: '', value: '', sort: 0, status: 1 })

const dataFormRules: FormRules = {
  label: [{ required: true, message: '请输入标签', trigger: 'blur' }],
  value: [{ required: true, message: '请输入存储值', trigger: 'blur' }]
}

function openDataForm(row?: DictData) {
  dataFormRef.value?.clearValidate()
  Object.assign(dataForm, row
    ? { id: row.id, label: row.label, value: row.value, sort: row.sort, status: row.status }
    : { id: '', label: '', value: '', sort: 0, status: 1 })
  dataFormVisible.value = true
}

async function handleSubmitData() {
  await dataFormRef.value?.validate()
  dataSubmitting.value = true
  try {
    if (dataForm.id) {
      await updateDictData(dataForm.id, dataForm)
      ElMessage.success('字典数据已更新')
    } else {
      await createDictData({
        type_code: currentType.value!.code,
        label: dataForm.label, value: dataForm.value,
        sort: dataForm.sort, status: dataForm.status
      })
      ElMessage.success('字典数据已创建')
    }
    dataFormVisible.value = false
    fetchData()
    fetchTypes()
  } finally {
    dataSubmitting.value = false
  }
}

async function handleDeleteData(row: DictData) {
  const ok = await ElMessageBox.confirm(
    `若已有业务数据引用「${row.label}」，删除后历史数据将显示原值，确定删除吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteDictData(row.id)
  ElMessage.success('已删除')
  fetchData()
  fetchTypes()
}
</script>

<style scoped lang="scss">
.dict-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.dict-card {
  width: 420px;
  flex-shrink: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg;

  .dict-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: $spacing-sm;
    margin-bottom: $spacing-md;
  }
}

.dict-data-card {
  flex: 1;
  min-width: 0;
}

.dict-tip {
  margin-top: $spacing-md;
}
</style>
