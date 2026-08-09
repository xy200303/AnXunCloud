<template>
  <div class="app-container">
    <div class="point-layout">
      <!-- 左：小区/楼栋树 -->
      <div class="tree-card">
        <div class="tree-title">小区/楼栋</div>
        <el-tree
          :data="treeData"
          node-key="treeKey"
          default-expand-all
          highlight-current
          :props="{ label: 'label', children: 'children' }"
          @node-click="handleNodeClick"
        />
      </div>

      <!-- 右：搜索 + 工具栏 + 表格 + 分页 -->
      <div class="point-main">
        <div class="filter-card">
          <el-form :model="query" inline>
            <el-form-item label="点位名称">
              <el-input v-model="query.name" placeholder="点位名称" clearable style="width: 150px" @keyup.enter="handleSearch" />
            </el-form-item>
            <el-form-item label="类型">
              <el-select v-model="query.type" placeholder="全部类型" clearable style="width: 130px">
                <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
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

        <div class="table-card">
          <div class="table-toolbar">
            <div class="table-toolbar-left">
              <el-button v-perms="'inspection:point:create'" type="primary" :icon="Plus" @click="openForm()">新增点位</el-button>
              <el-button
                v-perms="'inspection:point:qrcode'"
                :icon="Grid"
                :loading="qrcodeLoading"
                @click="handleBatchQrcode"
              >批量生成二维码</el-button>
            </div>
            <el-tooltip content="刷新" placement="top">
              <el-button :icon="RefreshRight" circle @click="fetchList" />
            </el-tooltip>
          </div>

          <el-table v-loading="loading" :data="list" stripe style="width: 100%" @selection-change="(rows: PointItem[]) => (selected = rows)">
            <el-table-column type="selection" width="44" />
            <el-table-column prop="name" label="点位名称" min-width="130" show-overflow-tooltip />
            <el-table-column prop="building_name" label="楼栋/区域" min-width="120" show-overflow-tooltip />
            <el-table-column prop="type_label" label="类型" width="100">
              <template #default="{ row }">{{ row.type_label || row.type }}</template>
            </el-table-column>
            <el-table-column prop="qrcode_no" label="二维码编号" width="120" />
            <el-table-column label="打卡方式" width="100" align="center">
              <template #default="{ row }">
                {{ checkinModeLabel(row.checkin_mode) }}
              </template>
            </el-table-column>
            <el-table-column prop="fence_radius" label="围栏半径" width="90" align="right">
              <template #default="{ row }">{{ row.fence_radius }}m</template>
            </el-table-column>
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <el-button v-perms="'inspection:point:update'" link type="primary" @click="openForm(row)">编辑</el-button>
                <el-button v-perms="'inspection:point:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="该条件下暂无点位">
                <el-button v-perms="'inspection:point:create'" type="primary" @click="openForm()">新增点位</el-button>
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
      </div>
    </div>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑点位' : '新增点位'" width="640px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="96px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="所属楼栋" prop="building_id">
              <el-cascader
                v-model="form.communityBuilding"
                :options="cascaderOptions"
                :props="{ value: 'id', label: 'label', children: 'children' }"
                placeholder="选择小区 / 楼栋"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="点位名称" prop="name">
              <el-input v-model="form.name" placeholder="如：1号楼大堂" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="点位类型" prop="type">
              <el-select v-model="form.type" placeholder="选择类型" style="width: 100%">
                <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="二维码编号">
              <el-input :model-value="form.id ? form.qrcode_no : '保存后自动生成'" disabled />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 坐标：腾讯地图 key 未配置，先手动输入；此处预留地图选点组件位置 -->
        <el-form-item label="坐标">
          <div class="coord-row">
            <el-input-number v-model="form.longitude" :precision="6" :step="0.0001" :min="70" :max="140" placeholder="经度" controls-position="right" />
            <el-input-number v-model="form.latitude" :precision="6" :step="0.0001" :min="3" :max="54" placeholder="纬度" controls-position="right" />
            <el-tooltip content="地图选点组件将在腾讯地图 key 配置后接入" placement="top">
              <el-button :icon="MapLocation" disabled>地图选点</el-button>
            </el-tooltip>
          </div>
          <div class="text-secondary">GCJ-02 坐标系；地图选点待腾讯地图 key 配置后开放</div>
        </el-form-item>

        <el-form-item label="围栏半径">
          <el-slider v-model="form.fence_radius" :min="50" :max="500" :step="10" show-input />
        </el-form-item>

        <el-form-item label="打卡方式" prop="checkin_mode">
          <el-radio-group v-model="form.checkin_mode">
            <el-radio value="qrcode">扫码</el-radio>
            <el-radio value="fence">围栏</el-radio>
            <el-radio value="either">任一（默认）</el-radio>
            <el-radio value="both">两者</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="必拍项">
          <div class="photo-items">
            <div v-for="(item, i) in form.required_photo_items" :key="i" class="photo-item">
              <el-input v-model="form.required_photo_items[i]" placeholder="如：配电箱" style="width: 200px" />
              <el-button link type="danger" :icon="Delete" @click="form.required_photo_items.splice(i, 1)" />
            </div>
            <el-button :icon="Plus" size="small" @click="form.required_photo_items.push('')">添加必拍项</el-button>
            <div class="text-secondary">为空表示仅要求现场照片</div>
          </div>
        </el-form-item>

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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Grid, MapLocation, Delete } from '@element-plus/icons-vue'
import { listPoints, createPoint, updatePoint, deletePoint, generateQrcodes, type PointQuery } from '@/api/point'
import { listCommunityTree } from '@/api/community'
import { listDictData } from '@/api/dict'
import type { PointItem } from '@/api/biz-types'
import type { DictData } from '@/api/types'

// ===== 左树 =====
interface TreeNode {
  treeKey: string
  label: string
  community_id: string
  building_id?: string
  children?: TreeNode[]
}

const treeData = ref<TreeNode[]>([])

// 一次请求拉取小区/楼栋树（后端 /communities/tree）
async function fetchTree() {
  const nodes = await listCommunityTree()
  treeData.value = nodes.map((c) => ({
    treeKey: `c-${c.id}`,
    label: c.name,
    community_id: c.id,
    children: c.buildings.map((b) => ({
      treeKey: `b-${b.id}`,
      label: b.name,
      community_id: c.id,
      building_id: b.id
    }))
  }))
}

function handleNodeClick(node: TreeNode) {
  query.community_id = node.community_id
  query.building_id = node.building_id
  query.page = 1
  fetchList()
}

// ===== 列表 =====
const loading = ref(false)
const list = ref<PointItem[]>([])
const total = ref(0)
const selected = ref<PointItem[]>([])
const query = reactive<PointQuery>({ page: 1, page_size: 20, name: '', type: '', status: '' })

const pointTypeOptions = ref<DictData[]>([])

async function fetchList() {
  loading.value = true
  try {
    const data = await listPoints({
      ...query,
      name: query.name || undefined,
      type: query.type || undefined,
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
  query.name = ''
  query.type = ''
  query.status = ''
  query.community_id = undefined
  query.building_id = undefined
  handleSearch()
}

onMounted(() => {
  fetchTree()
  fetchList()
  // 点位类型字典
  listDictData({ type_code: 'point_type', page: 1, page_size: 100 }).then((d) => {
    pointTypeOptions.value = d.list.filter((x) => x.status === 1)
  })
})

function checkinModeLabel(mode: string) {
  return { qrcode: '扫码', fence: '围栏', either: '任一', both: '两者' }[mode] || mode
}

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  qrcode_no: '',
  communityBuilding: [] as string[],
  name: '',
  type: '',
  longitude: null as number | null,
  latitude: null as number | null,
  fence_radius: 100,
  checkin_mode: 'either',
  required_photo_items: [] as string[],
  sort: 0,
  status: 1
})

const formRules: FormRules = {
  communityBuilding: [{ required: true, type: 'array', min: 2, message: '请选择所属楼栋', trigger: 'change' }],
  name: [{ required: true, message: '请输入点位名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择点位类型', trigger: 'change' }],
  checkin_mode: [{ required: true, message: '请选择打卡方式', trigger: 'change' }]
}

// 级联选择：小区 → 楼栋
const cascaderOptions = ref<any[]>([])

function buildCascader() {
  cascaderOptions.value = treeData.value.map((c) => ({
    id: c.community_id,
    label: c.label,
    children: (c.children || []).map((b) => ({ id: b.building_id, label: b.label }))
  }))
}

function openForm(row?: PointItem) {
  buildCascader()
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id,
      qrcode_no: row.qrcode_no,
      communityBuilding: [row.community_id, row.building_id],
      name: row.name,
      type: row.type,
      longitude: row.longitude,
      latitude: row.latitude,
      fence_radius: row.fence_radius,
      checkin_mode: row.checkin_mode,
      required_photo_items: [...(row.required_photo_items || [])],
      sort: row.sort,
      status: row.status
    })
  } else {
    Object.assign(form, {
      id: '', qrcode_no: '', communityBuilding: [], name: '', type: '',
      longitude: null, latitude: null, fence_radius: 100, checkin_mode: 'either',
      required_photo_items: [], sort: 0, status: 1
    })
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  if (form.longitude == null || form.latitude == null) {
    ElMessage.warning('请填写点位坐标（经度/纬度）')
    return
  }
  // 必拍项去除空项
  const items = form.required_photo_items.map((s) => s.trim()).filter(Boolean)
  const payload = {
    community_id: form.communityBuilding[0],
    building_id: form.communityBuilding[1],
    name: form.name,
    type: form.type,
    longitude: form.longitude,
    latitude: form.latitude,
    fence_radius: form.fence_radius,
    checkin_mode: form.checkin_mode,
    required_photo_items: items,
    sort: form.sort,
    status: form.status
  }
  submitting.value = true
  try {
    if (form.id) {
      await updatePoint(form.id, payload as any)
      ElMessage.success('点位已更新')
    } else {
      const res = await createPoint(payload as any)
      ElMessage.success(`点位已创建，二维码编号 ${res.qrcode_no}`)
    }
    formVisible.value = false
    fetchList()
  } finally {
    submitting.value = false
  }
}

// 删除：被启用中计划引用时后端拒删（43002），错误信息由拦截器展示
async function handleDelete(row: PointItem) {
  const ok = await ElMessageBox.confirm(
    `删除后打卡记录保留但点位不可再用，确定删除点位「${row.name}」吗？`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deletePoint(row.id)
  ElMessage.success('已删除')
  fetchList()
}

// ===== 批量生成二维码：调用接口后下载排版文件 =====
const qrcodeLoading = ref(false)

async function handleBatchQrcode() {
  if (!selected.value.length) {
    ElMessage.warning('请先勾选要生成二维码的点位')
    return
  }
  if (selected.value.length > 200) {
    ElMessage.warning('单次最多生成 200 个点位')
    return
  }
  qrcodeLoading.value = true
  try {
    const res = await generateQrcodes(selected.value.map((p) => p.id))
    // file_url 为服务端生成的下载地址（本地存储或 OSS 签名 URL）
    const a = document.createElement('a')
    a.href = res.file_url
    a.download = res.file_name
    a.target = '_blank'
    a.click()
    ElMessage.success('二维码文件已生成，开始下载')
  } finally {
    qrcodeLoading.value = false
  }
}
</script>

<style scoped lang="scss">
.point-layout {
  display: flex;
  gap: $spacing-lg;
  align-items: flex-start;
}

.tree-card {
  width: 220px;
  flex-shrink: 0;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-lg;

  .tree-title {
    font-weight: 600;
    margin-bottom: $spacing-md;
    color: $color-text-primary;
  }
}

.point-main {
  flex: 1;
  min-width: 0;
}

.coord-row {
  display: flex;
  gap: $spacing-sm;
  align-items: center;
}

.photo-items {
  width: 100%;

  .photo-item {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    margin-bottom: $spacing-sm;
  }
}
</style>
