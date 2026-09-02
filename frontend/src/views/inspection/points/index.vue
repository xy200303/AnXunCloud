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
              <el-button v-perms="'inspection:point:create'" :icon="Files" @click="openBatch">批量建点</el-button>
              <el-button
                v-perms="'inspection:point:qrcode'"
                :icon="Grid"
                :loading="qrcodeLoading"
                @click="handleBatchQrcode"
              >批量生成二维码</el-button>
              <el-button v-perms="'inspection:point:import'" :icon="Upload" @click="openImport">批量导入</el-button>
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
            <el-table-column label="打卡方式" width="110" align="center">
              <template #default="{ row }">
                {{ checkinModeLabel(row) }}
              </template>
            </el-table-column>
            <el-table-column label="模板" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">{{ row.template_name || '--' }}</template>
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
        <!-- 结构化位置：仅挂楼栋时显示（车库/公园/区域类点位无需单元楼层） -->
        <el-row v-if="form.communityBuilding.length === 2 && form.communityBuilding[1]" :gutter="16">
          <el-col :span="12">
            <el-form-item label="单元号" prop="unit_no">
              <el-input-number v-model="form.unit_no" :min="1" :max="99" controls-position="right" placeholder="选填" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="楼层" prop="floor">
              <el-input-number v-model="form.floor" :min="-50" :max="200" controls-position="right" placeholder="选填，负数=地下层" style="width: 100%" />
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

        <!-- 坐标：腾讯地图 key 已配置时支持地图选点，未配置时仅手动输入 -->
        <el-form-item label="坐标">
          <div class="coord-row">
            <el-input-number v-model="form.longitude" :precision="6" :step="0.0001" :min="70" :max="140" placeholder="经度" controls-position="right" />
            <el-input-number v-model="form.latitude" :precision="6" :step="0.0001" :min="3" :max="54" placeholder="纬度" controls-position="right" />
            <el-tooltip :content="mapKey ? '打开地图点击选点' : '请先在「系统管理-参数配置」填写 map.tencent_key'" placement="top">
              <el-button :icon="MapLocation" :disabled="!mapKey" @click="mapPickerVisible = true">地图选点</el-button>
            </el-tooltip>
          </div>
          <div class="text-secondary">{{ mapKey ? 'GCJ-02 坐标系；可点击「地图选点」在地图上拾取坐标' : 'GCJ-02 坐标系；地图选点需先在「系统管理-参数配置」填写 map.tencent_key' }}</div>
        </el-form-item>

        <el-form-item label="围栏校验">
          <el-switch
            v-model="form.require_fence"
            active-text="必须在围栏内"
            inactive-text="不校验"
            @change="formRef?.validateField('credential')"
          />
          <span class="text-secondary fence-hint">开启后打卡时 GPS 距点位超过半径将被拒绝</span>
        </el-form-item>

        <el-form-item label="围栏半径">
          <el-slider v-model="form.fence_radius" :min="50" :max="1000" :step="10" :disabled="!form.require_fence" show-input />
        </el-form-item>

        <el-form-item label="点位凭证" prop="credential">
          <el-radio-group v-model="form.credential">
            <el-radio value="qrcode">扫码</el-radio>
            <el-radio value="nfc">NFC</el-radio>
            <el-radio value="any">任一</el-radio>
            <el-radio value="none">不需要</el-radio>
          </el-radio-group>
          <div class="text-secondary">凭证用于确认「到的是这个点位」；任一 = 扫码或 NFC 均可；不需要且不启用围栏时为免核验点位</div>
        </el-form-item>

        <el-form-item label="检查项模板" prop="template_id" required>
          <el-select v-model="form.template_id" placeholder="必选；必拍项/逐项判定均由模板驱动" style="width: 100%">
            <el-option v-for="t in filteredTemplates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
          <div class="text-secondary">仅显示通用模板和与所选点位类型匹配的启用模板；「现场全貌」类需求请在模板中加对应检查项</div>
        </el-form-item>

        <el-form-item label="NFC 卡号" prop="nfc_id">
          <el-input v-model="form.nfc_id" placeholder="选填；凭证为 NFC 时必填，为任一时建议填写" style="width: 260px" />
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

    <!-- 批量建点助手：楼栋 × 楼层 × 每层数量展开，同楼栋同名跳过（幂等） -->
    <el-dialog v-model="batchVisible" title="批量建点" width="640px" :close-on-click-modal="false">
      <!-- 结果反馈 -->
      <template v-if="batchResult">
        <el-alert
          :title="`建点完成：新增 ${batchResult.created} 个，跳过 ${batchResult.skipped.length} 个`"
          :type="batchResult.skipped.length ? 'warning' : 'success'"
          :closable="false"
          show-icon
        />
        <template v-if="batchResult.skipped.length">
          <div class="fail-header">
            <span class="card-title">跳过明细（同楼栋同名不重复创建）</span>
          </div>
          <el-table :data="batchResult.skipped" border size="small" max-height="260">
            <el-table-column prop="name" label="点位名称" min-width="180" show-overflow-tooltip />
            <el-table-column prop="reason" label="跳过原因" min-width="140" show-overflow-tooltip />
          </el-table>
        </template>
      </template>

      <el-form v-else ref="batchFormRef" :model="batchForm" :rules="batchRules" label-width="110px">
        <el-form-item label="所属小区" prop="community_id">
          <el-select v-model="batchForm.community_id" placeholder="选择小区" style="width: 100%" @change="batchForm.building_ids = []">
            <el-option v-for="c in treeData" :key="c.community_id" :label="c.label" :value="c.community_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="楼栋" prop="building_ids">
          <el-select v-model="batchForm.building_ids" multiple collapse-tags collapse-tags-tooltip placeholder="选择楼栋（可多选）" style="width: 100%">
            <el-option v-for="b in batchBuildings" :key="b.building_id" :label="b.label" :value="b.building_id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="batchForm.building_ids.length > 0" label="单元范围">
          <div class="floor-row">
            <el-input-number v-model="batchForm.unit_from" :min="1" :max="99" controls-position="right" placeholder="起" />
            <span class="text-secondary">至</span>
            <el-input-number v-model="batchForm.unit_to" :min="1" :max="99" controls-position="right" placeholder="止" />
            <span class="text-secondary">单元</span>
          </div>
          <div class="text-secondary">单单元楼栋保持 1 至 1 即可</div>
        </el-form-item>
        <el-form-item label="楼层范围" prop="floor_from">
          <div class="floor-row">
            <el-input-number v-model="batchForm.floor_from" :min="-50" :max="200" controls-position="right" placeholder="起" />
            <span class="text-secondary">至</span>
            <el-input-number v-model="batchForm.floor_to" :min="-50" :max="200" controls-position="right" placeholder="止" />
            <span class="text-secondary">每层</span>
            <el-input-number v-model="batchForm.per_floor" :min="1" :max="50" controls-position="right" />
            <span class="text-secondary">个</span>
          </div>
          <div class="text-secondary">负数表示地下层（-1 = B1，-2 = B2）</div>
        </el-form-item>
        <el-form-item label="命名规则" prop="name_pattern">
          <el-input v-model="batchForm.name_pattern" placeholder="如：{building}{floor}层消防箱{seq}" />
          <div class="text-secondary">占位符：{'{building}'} 楼栋名、{'{unit}'} 单元、{'{floor}'} 楼层（地下层自动渲染 B1/B2）、{'{seq}'} 每层序号</div>
        </el-form-item>
        <el-form-item label="点位类型" prop="type">
          <el-select v-model="batchForm.type" placeholder="选择类型" style="width: 100%">
            <el-option v-for="d in pointTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="点位凭证">
          <el-radio-group v-model="batchForm.credential">
            <el-radio value="qrcode">扫码</el-radio>
            <el-radio value="nfc">NFC</el-radio>
            <el-radio value="any">任一</el-radio>
            <el-radio value="none">不需要</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="检查项模板" prop="template_id" required>
          <el-select v-model="batchForm.template_id" placeholder="必选；点位强制绑定模板" style="width: 100%">
            <el-option v-for="t in batchTemplates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="坐标">
          <div class="coord-row">
            <el-input-number v-model="batchForm.longitude" :precision="6" :step="0.0001" :min="70" :max="140" placeholder="经度（选填）" controls-position="right" />
            <el-input-number v-model="batchForm.latitude" :precision="6" :step="0.0001" :min="3" :max="54" placeholder="纬度（选填）" controls-position="right" />
          </div>
          <div class="text-secondary">批量点位共用同一坐标，可留空后续逐个修正</div>
        </el-form-item>
      </el-form>

      <template #footer>
        <template v-if="batchResult">
          <el-button @click="batchResult = null">继续建点</el-button>
          <el-button type="primary" @click="batchVisible = false">完成</el-button>
        </template>
        <template v-else>
          <el-button @click="batchVisible = false">取消</el-button>
          <el-button type="primary" :loading="batchSubmitting" @click="handleBatchSubmit">生成点位</el-button>
        </template>
      </template>
    </el-dialog>

    <!-- 批量导入：三步向导对话框 -->
    <el-dialog v-model="importVisible" title="批量导入点位" width="640px" :close-on-click-modal="false" @closed="resetImport">
      <el-steps :active="importStep" align-center finish-status="success" class="import-steps">
        <el-step title="模板说明" />
        <el-step title="上传文件" />
        <el-step title="导入结果" />
      </el-steps>

      <!-- 第一步：模板说明 -->
      <div v-show="importStep === 0" class="import-pane">
        <el-button :icon="Download" @click="handleDownloadTemplate">下载导入模板 point_import_template.xlsx</el-button>
        <el-table :data="templateFields" border size="small" class="import-fields">
          <el-table-column prop="field" label="字段" width="110" />
          <el-table-column prop="required" label="必填" width="70" align="center" />
          <el-table-column prop="rule" label="填写规则" />
        </el-table>
      </div>

      <!-- 第二步：上传文件 -->
      <div v-show="importStep === 1" class="import-pane">
        <el-upload
          ref="uploadRef"
          drag
          :auto-upload="false"
          :limit="1"
          accept=".xlsx"
          :on-change="handleFileChange"
          :on-remove="() => (importFile = null)"
          :on-exceed="handleFileExceed"
        >
          <el-icon :size="40" class="upload-icon"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处，或 <em>点击选择文件</em></div>
          <template #tip>
            <div class="text-secondary">仅支持 .xlsx，单次最多 500 行，文件 ≤ 5MB</div>
          </template>
        </el-upload>
        <div v-if="importError" class="import-error">{{ importError }}</div>
      </div>

      <!-- 第三步：结果反馈 -->
      <div v-show="importStep === 2" class="import-pane">
        <el-alert
          v-if="importResult"
          :title="`导入完成：成功 ${importResult.success_count} 条，失败 ${importResult.fail_count} 条`"
          :type="importResult.fail_count > 0 ? 'warning' : 'success'"
          :closable="false"
          show-icon
        />
        <template v-if="importResult && importResult.fail_details.length">
          <div class="fail-header">
            <span class="card-title">失败明细</span>
            <el-button size="small" :icon="Download" @click="downloadFailDetails">下载失败明细</el-button>
          </div>
          <el-table :data="importResult.fail_details" border size="small" max-height="260">
            <el-table-column prop="row" label="行号" width="80" align="center" />
            <el-table-column prop="name" label="点位名称" width="180" show-overflow-tooltip />
            <el-table-column prop="reason" label="失败原因" />
          </el-table>
          <div class="text-secondary fail-tip">修正失败行后可重新上传，同楼栋下同名点位不会重复创建</div>
        </template>
      </div>

      <template #footer>
        <template v-if="importStep === 0">
          <el-button type="primary" @click="importStep = 1">下一步</el-button>
        </template>
        <template v-else-if="importStep === 1">
          <el-button @click="importStep = 0">上一步</el-button>
          <el-button type="primary" :loading="importing" :disabled="!importFile" @click="handleImport">
            开始导入
          </el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="importVisible = false">完成</el-button>
        </template>
      </template>
    </el-dialog>
  </div>

  <!-- 地图选点对话框（teleport 到 body，避免受表单弹窗层级影响） -->
  <MapPickerDialog
    v-model:visible="mapPickerVisible"
    :api-key="mapKey"
    :longitude="form.longitude"
    :latitude="form.latitude"
    @confirm="handleMapPick"
  />
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, computed } from 'vue'
import {
  ElMessage, ElMessageBox,
  type FormInstance, type FormRules, type UploadFile, type UploadInstance, type UploadRawFile
} from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Grid, MapLocation, Delete, Upload, Download, UploadFilled, Files } from '@element-plus/icons-vue'
import { listPoints, createPoint, updatePoint, deletePoint, generateQrcodes, importPoints, batchCreatePoints, type PointQuery, type PointImportResult, type PointBatchResult } from '@/api/point'
import { withFileToken } from '@/api/upload'
import { listTemplates } from '@/api/template'
import { listCommunityTree } from '@/api/community'
import { listDictOptions, type DictOption } from '@/api/dict'
import { getMapConfig } from '@/api/map'
import MapPickerDialog from '@/components/MapPickerDialog.vue'
import { downloadFile } from '@/utils/download'
import type { PointItem, TemplateItem } from '@/api/biz-types'

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

const pointTypeOptions = ref<DictOption[]>([])

// 启用中的检查项模板（表单下拉用，按点位类型过滤）
const templates = ref<TemplateItem[]>([])
const filteredTemplates = computed(() =>
  templates.value.filter((t) => !t.point_type || t.point_type === form.type)
)

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

// ===== 地图选点：进入页面拉取地图配置，key 非空才开放选点按钮 =====
const mapKey = ref('')
const mapPickerVisible = ref(false)

function handleMapPick(pos: { lng: number; lat: number }) {
  form.longitude = pos.lng
  form.latitude = pos.lat
}

// 拉取地图服务配置（key 为空时「地图选点」保持禁用）；打开表单时也会重新拉，避免配完 key 必须刷新整页
function refreshMapKey() {
  getMapConfig().then((d) => {
    mapKey.value = d.provider === 'tencent' ? d.key : ''
  }).catch(() => {})
}

onMounted(() => {
  fetchTree()
  fetchList()
  refreshMapKey()
  // 点位类型字典
  listDictOptions('point_type').then((d) => {
    pointTypeOptions.value = d || []
  })
  // 启用中的检查项模板
  listTemplates({ page: 1, page_size: 100, status: 1 }).then((d) => {
    templates.value = d.list
  })
})

// 打卡方式展示：凭证 + 围栏两个维度组合成一句话
function checkinModeLabel(p: { credential: string; require_fence: boolean }) {
  const cred = { qrcode: '扫码', nfc: 'NFC', any: '任一', none: '' }[p.credential] ?? p.credential
  if (p.credential === 'none') return p.require_fence ? '围栏' : '免核验'
  return p.require_fence ? `${cred}+围栏` : cred
}

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  qrcode_no: '',
  communityBuilding: [] as string[],
  unit_no: null as number | null,
  floor: null as number | null,
  name: '',
  type: '',
  longitude: null as number | null,
  latitude: null as number | null,
  fence_radius: 100,
  credential: 'qrcode',
  require_fence: true,
  template_id: null as string | null,
  nfc_id: '',
  sort: 0,
  status: 1
})

const formRules: FormRules = {
  communityBuilding: [{ required: true, type: 'array', min: 2, message: '请选择所属楼栋', trigger: 'change' }],
  name: [{ required: true, message: '请输入点位名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择点位类型', trigger: 'change' }],
  template_id: [{ required: true, message: '点位必须绑定检查项模板', trigger: 'change' }],
  nfc_id: [
    {
      validator: (_r, v: string, cb) =>
        form.credential === 'nfc' && !v?.trim() ? cb(new Error('点位凭证为 NFC 时必填卡号')) : cb(),
      trigger: 'blur'
    }
  ]
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
  refreshMapKey() // 每次打开表单都取最新 key（参数配置里刚填的立即生效）
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id,
      qrcode_no: row.qrcode_no,
      communityBuilding: [row.community_id, row.building_id],
      unit_no: row.unit_no ?? null,
      floor: row.floor ?? null,
      name: row.name,
      type: row.type,
      longitude: row.longitude,
      latitude: row.latitude,
      fence_radius: row.fence_radius,
      credential: row.credential,
      require_fence: row.require_fence,
      template_id: row.template_id || null,
      nfc_id: row.nfc_id || '',
      sort: row.sort,
      status: row.status
    })
  } else {
    Object.assign(form, {
      id: '', qrcode_no: '', communityBuilding: [], unit_no: null, floor: null, name: '', type: '',
      longitude: null, latitude: null, fence_radius: 100, credential: 'qrcode', require_fence: true,
      template_id: null, nfc_id: '', sort: 0, status: 1
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
  const payload = {
    community_id: form.communityBuilding[0],
    building_id: form.communityBuilding[1],
    unit_no: form.communityBuilding[1] ? form.unit_no : null,
    floor: form.communityBuilding[1] ? form.floor : null,
    name: form.name,
    type: form.type,
    longitude: form.longitude,
    latitude: form.latitude,
    fence_radius: form.fence_radius,
    credential: form.credential,
    require_fence: form.require_fence,
    template_id: form.template_id || null,
    nfc_id: form.nfc_id.trim(),
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
    // file_url 为服务端生成的统一下载地址（/api/files 需登录态，拼 ?token= 供直链下载）
    const a = document.createElement('a')
    a.href = withFileToken(res.file_url)
    a.download = res.file_name
    a.target = '_blank'
    a.click()
    ElMessage.success('二维码文件已生成，开始下载')
  } finally {
    qrcodeLoading.value = false
  }
}

// ===== 批量建点助手 =====
const batchVisible = ref(false)
const batchSubmitting = ref(false)
const batchFormRef = ref<FormInstance>()
const batchResult = ref<PointBatchResult | null>(null)

const batchForm = reactive({
  community_id: undefined as string | undefined,
  building_ids: [] as string[],
  unit_from: 1,
  unit_to: 1,
  floor_from: 1,
  floor_to: 3,
  per_floor: 1,
  name_pattern: '',
  type: '',
  credential: 'qrcode',
  template_id: null as string | null,
  longitude: null as number | null,
  latitude: null as number | null
})

const batchRules: FormRules = {
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  name_pattern: [{ required: true, message: '请填写命名规则', trigger: 'blur' }],
  type: [{ required: true, message: '请选择点位类型', trigger: 'change' }],
  template_id: [{ required: true, message: '点位必须绑定检查项模板', trigger: 'change' }]
}

// 所选小区下的楼栋选项（复用左树数据）
const batchBuildings = computed(
  () => treeData.value.find((c) => c.community_id === batchForm.community_id)?.children || []
)

// 模板同样按点位类型过滤（与单点表单同口径）
const batchTemplates = computed(() =>
  templates.value.filter((t) => !t.point_type || t.point_type === batchForm.type)
)

function openBatch() {
  batchResult.value = null
  batchFormRef.value?.clearValidate()
  batchVisible.value = true
}

async function handleBatchSubmit() {
  await batchFormRef.value?.validate()
  if (batchForm.floor_from > batchForm.floor_to) {
    ElMessage.warning('楼层范围起点不能大于终点')
    return
  }
  batchSubmitting.value = true
  try {
    batchResult.value = await batchCreatePoints({
      community_id: batchForm.community_id!,
      building_ids: batchForm.building_ids.length ? batchForm.building_ids : undefined,
      unit_from: batchForm.unit_from,
      unit_to: batchForm.unit_to,
      floor_from: batchForm.floor_from,
      floor_to: batchForm.floor_to,
      per_floor: batchForm.per_floor,
      name_pattern: batchForm.name_pattern,
      type: batchForm.type,
      credential: batchForm.credential,
      template_id: batchForm.template_id || undefined,
      longitude: batchForm.longitude ?? undefined,
      latitude: batchForm.latitude ?? undefined
    })
    if (batchResult.value.created > 0) fetchList()
  } catch {
    // 拦截器已提示
  } finally {
    batchSubmitting.value = false
  }
}

// ===== 批量导入：三步向导 =====
const importVisible = ref(false)
const importStep = ref(0)
const importFile = ref<File | null>(null)
const importError = ref('')
const importing = ref(false)
const importResult = ref<PointImportResult | null>(null)
const uploadRef = ref<UploadInstance>()

const templateFields = [
  { field: '小区', required: '*', rule: '须为已存在的小区名称' },
  { field: '楼栋', required: '—', rule: '选填；填写时须为该小区下已存在的楼栋，留空为小区级点位' },
  { field: '点位名称', required: '*', rule: '同楼栋下不可重名' },
  { field: '点位类型', required: '*', rule: '须为字典 point_type 中的类型（标签或值均可）' },
  { field: '检查项模板', required: '*', rule: '必填，须为启用中的模板名称；逐项判定与必拍项均由模板驱动' },
  { field: 'NFC卡号', required: '—', rule: '选填；打卡方式为 NFC 时必填，为任一时建议填写' },
  { field: '经度/纬度', required: '—', rule: 'GCJ-02 坐标系，如 120.212001 / 30.208112；打卡方式含围栏时必填，否则可留空（记 0,0，现场用手机「获取当前位置」刷新补齐）' },
  { field: '围栏半径(米)', required: '—', rule: '选填，10–2000 整数，默认取系统参数' },
  { field: '打卡方式', required: '—', rule: '扫码 / NFC / 任一 / 围栏 / 扫码+围栏 / NFC+围栏 / 任一+围栏，默认扫码+围栏' },
  { field: '必拍项', required: '—', rule: '已废弃：必拍项由检查项模板逐项配置，该列导入时忽略' },
  { field: '状态', required: '—', rule: '启用 / 停用，默认启用' },
  { field: '备注', required: '—', rule: '选填' }
]

function openImport() {
  importStep.value = 0
  importFile.value = null
  importError.value = ''
  importResult.value = null
  importVisible.value = true
}

function resetImport() {
  importFile.value = null
  importError.value = ''
  importResult.value = null
  uploadRef.value?.clearFiles()
}

function handleDownloadTemplate() {
  downloadFile('/inspection/points/import-template', undefined, 'point_import_template.xlsx')
}

// 前置校验：非 .xlsx 或超限直接红字拒绝，不发起请求
function validateFile(file: File): boolean {
  if (!file.name.endsWith('.xlsx')) {
    importError.value = '文件格式错误：仅支持 .xlsx 文件'
    return false
  }
  if (file.size > 5 * 1024 * 1024) {
    importError.value = '文件大小超限：请控制在 5MB 以内（约 500 行）'
    return false
  }
  importError.value = ''
  return true
}

function handleFileChange(uploadFile: UploadFile) {
  const raw = uploadFile.raw
  if (!raw) return
  if (!validateFile(raw)) {
    uploadRef.value?.clearFiles()
    importFile.value = null
    return
  }
  importFile.value = raw
}

function handleFileExceed(files: File[]) {
  uploadRef.value?.clearFiles()
  const raw = files[0] as UploadRawFile
  if (validateFile(raw)) {
    uploadRef.value?.handleStart(raw)
    importFile.value = raw
  }
}

async function handleImport() {
  if (!importFile.value) return
  importing.value = true
  try {
    importResult.value = await importPoints(importFile.value)
    importStep.value = 2
    if (importResult.value.success_count > 0) fetchList()
  } catch {
    // 拦截器已提示；文件级错误（格式/空文件/超 500 行）停留在当前步可重新选择
  } finally {
    importing.value = false
  }
}

// 失败明细本地导出为 CSV，供修正后重新导入
function downloadFailDetails() {
  if (!importResult.value) return
  const rows = importResult.value.fail_details
    .map((d) => `${d.row},${d.name},${d.reason}`)
    .join('\n')
  const blob = new Blob([`﻿行号,点位名称,失败原因\n${rows}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = '点位导入失败明细.csv'
  a.click()
  URL.revokeObjectURL(url)
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

.floor-row {
  display: flex;
  gap: $spacing-sm;
  align-items: center;
}

.fence-hint {
  margin-left: $spacing-md;
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

.import-steps {
  margin-bottom: $spacing-xl;
}

.import-pane {
  min-height: 280px;
}

.import-fields {
  margin-top: $spacing-lg;
}

.upload-icon {
  color: $color-text-secondary;
}

.import-error {
  margin-top: $spacing-sm;
  color: $color-danger;
  font-size: $font-size-aux;
}

.fail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: $spacing-lg 0 $spacing-sm;
}

.fail-tip {
  margin-top: $spacing-sm;
}
</style>
