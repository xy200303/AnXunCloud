<template>
  <div class="app-container">
    <div class="building-layout">
      <!-- 左：小区列表（单选过滤） -->
      <div class="community-panel">
        <div
          class="community-item"
          :class="{ active: !currentCommunityId }"
          @click="selectCommunity(null)"
        >
          全部小区
        </div>
        <div
          v-for="c in communities"
          :key="c.id"
          class="community-item"
          :class="{ active: currentCommunityId === c.id }"
          @click="selectCommunity(c.id)"
        >
          <el-icon><OfficeBuilding /></el-icon>
          <span>{{ c.name }}</span>
        </div>
      </div>

      <!-- 右：楼栋/区域表格 -->
      <div class="table-card building-table">
        <div class="table-toolbar">
          <div class="table-toolbar-left">
            <el-button
              v-perms="'community:building:create'"
              type="primary"
              :icon="Plus"
              :disabled="!currentCommunityId"
              @click="openForm()"
            >新增楼栋/区域</el-button>
            <span v-if="!currentCommunityId" class="text-secondary">请先选择左侧小区</span>
          </div>
          <el-tooltip content="刷新" placement="top">
            <el-button :icon="RefreshRight" circle @click="fetchList" />
          </el-tooltip>
        </div>

        <el-table v-loading="loading" :data="list" stripe style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="160" />
          <el-table-column label="类型" width="110" align="center">
            <template #default="{ row }">
              <el-tag :type="row.type === 'building' ? 'primary' : 'warning'" size="small">
                {{ row.type === 'building' ? '楼栋' : '区域' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="point_count" label="点位数" width="90" align="right" />
          <el-table-column prop="created_at" label="创建时间" width="160" />
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button v-perms="'community:building:update'" link type="primary" @click="openForm(row)">编辑</el-button>
              <el-button v-perms="'community:building:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty :description="currentCommunityId ? '该小区暂无楼栋/区域' : '请选择小区'">
              <el-button
                v-if="currentCommunityId"
                v-perms="'community:building:create'"
                type="primary"
                @click="openForm()"
              >新增楼栋/区域</el-button>
            </el-empty>
          </template>
        </el-table>

        <div class="pagination-wrap">
          <el-pagination
            v-model:current-page="query.page"
            v-model:page-size="query.page_size"
            :total="total"
            layout="total, prev, pager, next"
            @change="fetchList"
          />
        </div>
      </div>
    </div>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑楼栋/区域' : '新增楼栋/区域'" width="440px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="所属小区">
          <el-input :model-value="currentCommunityName" disabled />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="如：1号楼 / 地下车库A区" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="form.type">
            <el-radio value="building">楼栋</el-radio>
            <el-radio value="area">区域</el-radio>
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
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, RefreshRight, OfficeBuilding } from '@element-plus/icons-vue'
import { listCommunities, listBuildings, createBuilding, updateBuilding, deleteBuilding } from '@/api/community'
import type { CommunityItem, BuildingItem } from '@/api/biz-types'

const route = useRoute()
const communities = ref<CommunityItem[]>([])
const currentCommunityId = ref<string | null>(null)

const currentCommunityName = computed(
  () => communities.value.find((c) => c.id === currentCommunityId.value)?.name || ''
)

const loading = ref(false)
const list = ref<BuildingItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20 })

onMounted(async () => {
  const data = await listCommunities({ page: 1, page_size: 100, status: 1 })
  communities.value = data.list
  // 从小区管理带入的过滤
  const fromQuery = String(route.query.community_id || '')
  if (fromQuery) {
    currentCommunityId.value = fromQuery
    fetchList()
  }
})

function selectCommunity(id: string | null) {
  currentCommunityId.value = id
  query.page = 1
  fetchList()
}

async function fetchList() {
  if (!currentCommunityId.value) {
    list.value = []
    total.value = 0
    return
  }
  loading.value = true
  try {
    const data = await listBuildings({ community_id: currentCommunityId.value, ...query })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: '', name: '', type: 'building' })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }]
}

function openForm(row?: BuildingItem) {
  formRef.value?.clearValidate()
  Object.assign(form, row ? { id: row.id, name: row.name, type: row.type } : { id: '', name: '', type: 'building' })
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (form.id) {
      await updateBuilding(form.id, { name: form.name, type: form.type })
      ElMessage.success('已更新')
    } else {
      await createBuilding({ community_id: currentCommunityId.value!, name: form.name, type: form.type })
      ElMessage.success('已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// 删除：存在点位时后端拒删（42003），错误信息由拦截器展示
async function handleDelete(row: BuildingItem) {
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除「${row.name}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteBuilding(row.id)
  ElMessage.success('已删除')
  fetchList()
}
</script>

<style scoped lang="scss">
.building-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.community-panel {
  width: 220px;
  flex-shrink: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-sm;

  .community-item {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    padding: $spacing-md;
    border-radius: $radius-small;
    cursor: pointer;
    color: $color-text-regular;
    margin-bottom: $spacing-xs;

    &:hover {
      background: $color-bg-page;
    }

    &.active {
      background: $color-primary-light;
      color: $color-primary;
      font-weight: 600;
    }
  }
}

.building-table {
  flex: 1;
  min-width: 0;
}
</style>
