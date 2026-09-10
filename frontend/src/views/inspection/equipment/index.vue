<template>
  <div class="app-container">
    <el-tabs v-model="mainTab" class="main-tabs">
      <el-tab-pane label="设备列表" name="list" />
      <el-tab-pane name="confirm">
        <template #label>
          <el-badge :value="pendingTotal" :hidden="pendingTotal === 0" :max="99">维保确认</el-badge>
        </template>
      </el-tab-pane>
    </el-tabs>

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
          <el-table-column label="状态灯" width="70" align="center">
            <template #default="{ row }">
              <el-tooltip :content="dueStateLabel(row.due_state)" placement="top">
                <span class="due-dot" :class="`due-${row.due_state}`" />
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
    <ConfirmList v-else ref="confirmRef" @changed="fetchPendingTotal" />

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
              <el-select v-model="form.type" placeholder="选择类型" filterable style="width: 100%">
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
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 维保登记对话框 -->
    <el-dialog v-model="registerVisible" title="维保登记" width="560px" :close-on-click-modal="false">
      <el-alert
        v-if="registerTarget"
        :title="`${registerTarget.name}（${registerTarget.code}）`"
        :closable="false"
        class="register-target"
      />
      <el-form ref="registerFormRef" :model="registerForm" :rules="registerRules" label-width="96px">
        <el-form-item label="维保类型" prop="maintenance_type">
          <el-select v-model="registerForm.maintenance_type" style="width: 100%">
            <el-option v-for="d in maintTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="标签缺失">
          <el-switch v-model="registerForm.label_missing" active-text="钢印磨损/铭牌缺失" />
          <div class="text-secondary">勾选后日期免填；照片仍必传（拍设备本体作证）；经理确认后该设备退出自动到期判定</div>
        </el-form-item>
        <!-- 台账补录：允许随单提交出厂/最近维保日期，经理确认后一并回写台账 -->
        <template v-if="registerForm.maintenance_type === 'ledger_fix' && !registerForm.label_missing">
          <el-form-item label="出厂日期">
            <el-date-picker v-model="registerForm.manufacture_date" type="date" value-format="YYYY-MM-DD" placeholder="瓶体钢印日期" style="width: 100%" />
          </el-form-item>
          <el-form-item label="最近维保日">
            <el-date-picker v-model="registerForm.last_maintenance_date" type="date" value-format="YYYY-MM-DD" placeholder="维修贴纸日期" style="width: 100%" />
          </el-form-item>
        </template>
        <el-form-item label="维保日期" prop="maintenance_date">
          <el-date-picker v-model="registerForm.maintenance_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="维保单位">
          <el-input v-model="registerForm.vendor" placeholder="选填" maxlength="128" />
        </el-form-item>
        <el-form-item label="经办人">
          <el-input v-model="registerForm.operator_name" placeholder="默认当前用户" maxlength="64" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="registerForm.note" placeholder="选填" maxlength="255" />
        </el-form-item>
        <el-form-item label="标签照片" prop="photos">
          <div class="photo-upload">
            <div v-for="(p, i) in registerForm.photos" :key="p.file_id" class="photo-item">
              <el-image :src="p.url" fit="cover" class="photo-thumb" :preview-src-list="registerForm.photos.map((x) => x.url)" :initial-index="i" preview-teleported />
              <el-button link type="danger" :icon="Delete" @click="registerForm.photos.splice(i, 1)" />
            </div>
            <el-upload
              v-if="registerForm.photos.length < 9"
              :show-file-list="false"
              accept="image/*"
              :http-request="handlePhotoUpload"
            >
              <el-button :icon="Plus" :loading="photoUploading">上传照片</el-button>
            </el-upload>
          </div>
          <div class="text-secondary">必传至少 1 张新维修标签照片；提交后进经理确认，确认后台账才更新</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="registerVisible = false">取消</el-button>
        <el-button type="primary" :loading="registering" @click="handleRegister">提交登记</el-button>
      </template>
    </el-dialog>

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
    <el-dialog v-model="importVisible" title="批量导入设备台账" width="640px" :close-on-click-modal="false" @closed="resetImport">
      <el-steps :active="importStep" align-center finish-status="success" class="import-steps">
        <el-step title="模板说明" />
        <el-step title="上传文件" />
        <el-step title="导入结果" />
      </el-steps>

      <div v-show="importStep === 0" class="import-pane">
        <el-button :icon="Download" @click="handleDownloadTemplate">下载导入模板 equipment_import_template.xlsx</el-button>
        <el-alert
          title="模板列头与甲方现有台账（雄楚春天设施设备台帐）一致，原表可直接导入：第 1 行大标题自动跳过，按列头名识别（顺序可调）"
          type="info"
          :closable="false"
          class="import-tip"
        />
        <el-table :data="templateFields" border size="small" class="import-fields">
          <el-table-column prop="field" label="字段" width="110" />
          <el-table-column prop="required" label="必填" width="70" align="center" />
          <el-table-column prop="rule" label="填写规则" />
        </el-table>
      </div>

      <div v-show="importStep === 1" class="import-pane">
        <el-form label-width="96px">
          <el-form-item label="导入到小区" required>
            <el-select v-model="importCommunityId" placeholder="整个文件导入到该小区" style="width: 100%">
              <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
          </el-form-item>
        </el-form>
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
            <div class="text-secondary">仅支持 .xlsx，单次最多 2000 行，文件 ≤ 5MB；同编号按更新处理（可重复导入）</div>
          </template>
        </el-upload>
        <div v-if="importError" class="import-error">{{ importError }}</div>
      </div>

      <div v-show="importStep === 2" class="import-pane">
        <el-alert
          v-if="importResult"
          :title="`导入完成：新增 ${importResult.created_count} 条，更新 ${importResult.updated_count} 条，失败 ${importResult.fail_count} 条，自动绑定点位 ${importResult.auto_bound} 条`"
          :type="importResult.fail_count > 0 ? 'warning' : 'success'"
          :closable="false"
          show-icon
        />
        <template v-if="importResult && importResult.fail_details.length">
          <div class="fail-header">
            <span class="card-title">失败明细</span>
          </div>
          <el-table :data="importResult.fail_details" border size="small" max-height="260">
            <el-table-column prop="row" label="行号" width="80" align="center" />
            <el-table-column prop="code" label="设备编号" width="160" show-overflow-tooltip />
            <el-table-column prop="reason" label="失败原因" />
          </el-table>
          <div class="text-secondary fail-tip">修正失败行后可重新上传，同编号设备不会重复创建</div>
        </template>
      </div>

      <template #footer>
        <template v-if="importStep === 0">
          <el-button type="primary" @click="importStep = 1">下一步</el-button>
        </template>
        <template v-else-if="importStep === 1">
          <el-button @click="importStep = 0">上一步</el-button>
          <el-button type="primary" :loading="importing" :disabled="!importFile || !importCommunityId" @click="handleImport">
            开始导入
          </el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="importVisible = false">完成</el-button>
        </template>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  ElMessage, ElMessageBox,
  type FormInstance, type FormRules, type UploadFile, type UploadInstance, type UploadRawFile, type UploadRequestOptions
} from 'element-plus'
import { Search, Refresh, Plus, RefreshRight, Upload, Download, UploadFilled, Delete } from '@element-plus/icons-vue'
import {
  listEquipment, createEquipment, updateEquipment, deleteEquipment, batchDeleteEquipment,
  registerMaintenance, listMaintenancePending, listMaintenanceHistory,
  importEquipment,
  type EquipmentItem, type EquipmentQuery, type EquipmentStatus, type DueState,
  type MaintenanceItem, type MaintenanceType, type ConfirmStatus, type EquipmentImportResult
} from '@/api/equipment'
import { uploadImage, withFileToken } from '@/api/upload'
import { listCommunities, listCommunityTree } from '@/api/community'
import { listPoints } from '@/api/point'
import { listDictOptions, type DictOption } from '@/api/dict'
import { downloadFile, downloadFilePost } from '@/utils/download'
import type { CommunityItem } from '@/api/biz-types'
import ConfirmList from './ConfirmList.vue'

// ===== 主 tab =====
const route = useRoute()
const mainTab = ref<'list' | 'confirm'>('list')
const confirmRef = ref<InstanceType<typeof ConfirmList>>()

// ===== 字典与小区 =====
const typeOptions = ref<DictOption[]>([])
const maintTypeOptions = ref<DictOption[]>([])
const communities = ref<CommunityItem[]>([])

const statusOptions: { label: string; value: EquipmentStatus }[] = [
  { label: '在用', value: 'in_service' },
  { label: '维保中', value: 'maintaining' },
  { label: '停用', value: 'stopped' },
  { label: '报废', value: 'scrapped' }
]

// ===== 列表 =====
const loading = ref(false)
const list = ref<EquipmentItem[]>([])
const total = ref(0)
const query = reactive<EquipmentQuery>({ page: 1, page_size: 20, type: '', community_id: '', status: '', due_state: '', keyword: '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listEquipment({
      page: query.page,
      page_size: query.page_size,
      type: query.type || undefined,
      community_id: query.community_id || undefined,
      status: (query.status || undefined) as EquipmentStatus | undefined,
      due_state: (query.due_state || undefined) as DueState | undefined,
      keyword: query.keyword || undefined
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  selectAllFiltered.value = false // 筛选条件变化后跨页全选失效（目标集已变）
  fetchList()
}

function handleReset() {
  query.type = ''
  query.community_id = ''
  query.status = ''
  query.due_state = ''
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
  remark: ''
})

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
      status: row.status, remark: row.remark || ''
    })
  } else {
    Object.assign(form, {
      id: '', path: [], code: '', name: '', type: '',
      manufacture_date: '', next_due_date: '', status: 'in_service', remark: ''
    })
  }
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
    remark: form.remark
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

// ===== 维保登记 =====
const registerVisible = ref(false)
const registering = ref(false)
const photoUploading = ref(false)
const registerFormRef = ref<FormInstance>()
const registerTarget = ref<EquipmentItem | null>(null)

const registerForm = reactive({
  maintenance_type: 'repair' as MaintenanceType,
  label_missing: false,
  maintenance_date: '',
  manufacture_date: '',
  last_maintenance_date: '',
  vendor: '',
  operator_name: '',
  note: '',
  photos: [] as { file_id: string; url: string }[]
})

const registerRules: FormRules = {
  maintenance_type: [{ required: true, message: '请选择维保类型', trigger: 'change' }],
  maintenance_date: [{ required: true, message: '请选择维保日期', trigger: 'change' }],
  photos: [
    {
      validator: (_r, _v, cb) =>
        registerForm.photos.length === 0 ? cb(new Error('请至少上传 1 张新维修标签照片')) : cb(),
      trigger: 'change'
    }
  ]
}

function openRegister(row: EquipmentItem) {
  registerTarget.value = row
  registerFormRef.value?.clearValidate()
  Object.assign(registerForm, {
    maintenance_type: 'repair',
    label_missing: false,
    maintenance_date: new Date().toLocaleDateString('sv-SE'), // 本地今天，YYYY-MM-DD
    manufacture_date: '', last_maintenance_date: '',
    vendor: '', operator_name: '', note: '', photos: []
  })
  registerVisible.value = true
}

async function handlePhotoUpload(opt: UploadRequestOptions) {
  photoUploading.value = true
  try {
    // 管理端维保登记照片上传（scene=equipment，仅图片）；登记接口校验文件归属本人
    const res = await uploadImage(opt.file, 'equipment')
    registerForm.photos.push({ file_id: res.file_id, url: withFileToken(res.url) })
    registerFormRef.value?.validateField('photos')
  } catch {
    // 拦截器已提示
  } finally {
    photoUploading.value = false
  }
}

async function handleRegister() {
  await registerFormRef.value?.validate()
  if (!registerTarget.value) return
  registering.value = true
  try {
    await registerMaintenance({
      equipment_id: registerTarget.value.id,
      maintenance_type: registerForm.maintenance_type,
      maintenance_date: registerForm.maintenance_date || undefined,
      manufacture_date: registerForm.maintenance_type === 'ledger_fix' ? registerForm.manufacture_date || undefined : undefined,
      last_maintenance_date: registerForm.maintenance_type === 'ledger_fix' ? registerForm.last_maintenance_date || undefined : undefined,
      vendor: registerForm.vendor || undefined,
      operator_name: registerForm.operator_name || undefined,
      note: registerForm.note || undefined,
      file_ids: registerForm.photos.map((p) => p.file_id),
      label_missing: registerForm.label_missing || undefined
    })
    ElMessage.success('登记已提交，待经理确认后生效')
    registerVisible.value = false
    fetchPendingTotal()
  } finally {
    registering.value = false
  }
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
    keyword: query.keyword || undefined
  }, '设备台账.xlsx')
}

// ===== 批量导入：三步向导 =====
const importVisible = ref(false)
const importStep = ref(0)
const importFile = ref<File | null>(null)
const importError = ref('')
const importing = ref(false)
const importResult = ref<EquipmentImportResult | null>(null)
const importCommunityId = ref('')
const uploadRef = ref<UploadInstance>()

const templateFields = [
  { field: '设备编号', required: '*', rule: '租户内唯一；同编号按更新处理（可重复导入）' },
  { field: '设备名称', required: '—', rule: '留空时以编号兜底' },
  { field: '设备分类', required: '*', rule: '优先匹配设备类型字典；灭火器设施/消火栓设施/电梯轿厢/电梯机房按别名映射；未识别自动扩充字典（无规则参数，不自动判到期）' },
  { field: '出厂日期', required: '—', rule: '2018-05-01 / 2018/5/1 / 2018年5月 均可；留空用投运日期兜底' },
  { field: '投运日期', required: '—', rule: '原值保留在扩展属性，同时作为出厂日期兜底' },
  { field: '设备等级/品牌/规格型号/维保状态/维保单位/产地', required: '—', rule: '原样保留到扩展属性' },
  { field: '安装位置/管控区域', required: '—', rule: '拼接进备注；点位绑定待 App 扫码' },
  { field: '异动状态', required: '—', rule: '启用/停用/报废，默认在用' }
]

function openImport() {
  importStep.value = 0
  importFile.value = null
  importError.value = ''
  importResult.value = null
  importCommunityId.value = query.community_id || ''
  importVisible.value = true
}

function resetImport() {
  importFile.value = null
  importError.value = ''
  importResult.value = null
  uploadRef.value?.clearFiles()
}

function handleDownloadTemplate() {
  downloadFile('/equipment/import-template', undefined, 'equipment_import_template.xlsx')
}

// 前置校验：非 .xlsx 或超限直接红字拒绝，不发起请求
function validateFile(file: File): boolean {
  if (!file.name.endsWith('.xlsx')) {
    importError.value = '文件格式错误：仅支持 .xlsx 文件'
    return false
  }
  if (file.size > 5 * 1024 * 1024) {
    importError.value = '文件大小超限：请控制在 5MB 以内'
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
  if (!importFile.value || !importCommunityId.value) return
  importing.value = true
  try {
    importResult.value = await importEquipment(importCommunityId.value, importFile.value)
    importStep.value = 2
    if (importResult.value.created_count > 0 || importResult.value.updated_count > 0) fetchList()
  } catch {
    // 拦截器已提示；文件级错误停留在当前步可重新选择
  } finally {
    importing.value = false
  }
}

onMounted(() => {
  // 点位页「关联设备」跳转带 keyword 定位
  const kw = route.query.keyword
  if (typeof kw === 'string' && kw) {
    query.keyword = kw
  }
  fetchList()
  fetchPendingTotal()
  listDictOptions('equipment_type').then((d) => {
    typeOptions.value = d || []
  })
  listDictOptions('equipment_maint_type').then((d) => {
    maintTypeOptions.value = d || []
  })
  listCommunities({ page: 1, page_size: 100, status: 1 }).then((d) => {
    communities.value = d.list
  })
})
</script>

<style scoped lang="scss">
.main-tabs {
  margin-bottom: $spacing-md;
}

// 红黄绿状态灯：normal 绿 / warning 黄 / overdue 红 / none 灰
.due-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;

  &.due-normal {
    background: $color-success;
  }

  &.due-warning {
    background: $color-warning;
  }

  &.due-overdue {
    background: $color-danger;
  }

  &.due-none {
    background: $color-text-placeholder;
  }

  &.due-scrap {
    background: $color-danger;
    box-shadow: 0 0 0 4rpx rgba(213, 73, 65, 0.25);
  }

  &.due-label_missing {
    background: $color-text-placeholder;
    border: 2px dashed $color-text-secondary;
    box-sizing: border-box;
  }
}

.eq-flag {
  margin-left: $spacing-sm;
}

.text-danger {
  color: $color-danger;
}

.register-target {
  margin-bottom: $spacing-lg;
}

.photo-upload {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  align-items: center;
}

.photo-item {
  display: flex;
  align-items: center;
  gap: 2px;
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

.import-steps {
  margin-bottom: $spacing-xl;
}

.import-pane {
  min-height: 280px;
}

.import-tip {
  margin-top: $spacing-md;
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
