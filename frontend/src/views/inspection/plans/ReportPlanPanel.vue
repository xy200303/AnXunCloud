<template>
  <div>
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'report:generate'" type="primary" :icon="Plus" @click="openForm()">新增报告计划</el-button>
          <el-select
            v-model="filterCommunity" placeholder="全部小区" clearable style="width: 160px" @change="fetchList"
          >
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchList" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="name" label="计划名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="community_name" label="小区" min-width="110" />
        <el-table-column label="报告类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.patrol_type_label || '综合' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cycle_text" label="生成规则" min-width="180" />
        <el-table-column prop="gen_time" label="生成时点" width="90" align="center" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'enabled' ? 'success' : 'info'" size="small">
              {{ row.status === 'enabled' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="上次生成" min-width="130">
          <template #default="{ row }">
            <span>{{ row.last_period || '—' }}</span>
            <el-tooltip v-if="row.last_error" :content="row.last_error" placement="top">
              <el-icon color="#D54941" style="margin-left: 4px; vertical-align: -2px"><WarningFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'report:generate'" link type="success" :loading="runningId === row.id" @click="handleRun(row)">
              立即生成
            </el-button>
            <el-button v-perms="'report:generate'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'report:generate'" link :type="row.status === 'enabled' ? 'warning' : 'success'" @click="handleToggle(row)">
              {{ row.status === 'enabled' ? '停用' : '启用' }}
            </el-button>
            <el-button v-perms="'report:generate'" link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑报告计划' : '新增报告计划'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="96px">
        <el-form-item label="计划名称" prop="name">
          <el-input v-model="form.name" placeholder="如：综合月报（每月1日）" maxlength="64" />
        </el-form-item>
        <el-form-item label="小区" prop="community_id">
          <el-select
            v-model="form.community_id"
            placeholder="选择小区"
            :loading="communitiesLoading"
            no-data-text="当前租户暂无可用小区"
            style="width: 100%"
          >
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="报告类型">
          <el-select v-model="form.patrol_type" placeholder="综合（全部类型）" clearable style="width: 100%">
            <el-option-group v-for="g in patrolTypeGroups" :key="g.label" :label="g.label">
              <el-option v-for="o in g.options" :key="o.value" :label="o.label" :value="o.value" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="生成周期" prop="cycle_type">
          <el-radio-group v-model="form.cycle_type">
            <el-radio-button value="monthly">每月</el-radio-button>
            <el-radio-button value="weekly">每周</el-radio-button>
            <el-radio-button value="daily">每天</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.cycle_type === 'monthly'" label="生成日">
          <el-input-number v-model="form.day" :min="1" :max="28" />
          <span class="form-tip">每月这一天生成上月报告</span>
        </el-form-item>
        <el-form-item v-else-if="form.cycle_type === 'weekly'" label="生成日">
          <el-select v-model="form.weekday" style="width: 160px">
            <el-option v-for="(n, i) in ['一','二','三','四','五','六','日']" :key="i + 1" :label="'每周' + n" :value="i + 1" />
          </el-select>
          <span class="form-tip">这一天生成上周（周一~周日）报告</span>
        </el-form-item>
        <el-form-item v-else label="生成范围">
          <span class="form-tip">每天生成昨日报告</span>
        </el-form-item>
        <el-form-item label="生成时点" prop="gen_time">
          <el-time-picker v-model="genTimeDate" format="HH:mm" placeholder="06:00" style="width: 140px" />
        </el-form-item>
        <el-form-item label="明细范围">
          <el-radio-group v-model="form.detail_mode">
            <el-radio-button value="full">全部点位</el-radio-button>
            <el-radio-button value="abnormal">仅异常点位</el-radio-button>
          </el-radio-group>
          <span class="form-tip">点位量大时选「仅异常点位」压缩报告页数</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Plus, RefreshRight, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import {
  listReportPlans, createReportPlan, updateReportPlan, deleteReportPlan, runReportPlan,
  type ReportPlan
} from '@/api/reportPlan'
import { listCommunities } from '@/api/community'
import { usePatrolTypes } from '@/composables/usePatrolTypes'
import type { CommunityItem } from '@/api/biz-types'

const communities = ref<CommunityItem[]>([])
const communitiesLoading = ref(false)
const { patrolTypeGroups } = usePatrolTypes()

const loading = ref(false)
const list = ref<ReportPlan[]>([])
const filterCommunity = ref('')

async function fetchList() {
  loading.value = true
  try {
    list.value = await listReportPlans(filterCommunity.value || undefined)
  } finally {
    loading.value = false
  }
}

// ===== 表单 =====
const formVisible = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({
  id: '', name: '', community_id: '', patrol_type: '',
  cycle_type: 'monthly', day: 1, weekday: 1, gen_time: '06:00', status: 'enabled', remark: '',
  detail_mode: 'full'
})
const genTimeDate = computed({
  get: () => {
    const [h, m] = (form.gen_time || '06:00').split(':').map(Number)
    return new Date(2000, 0, 1, h || 6, m || 0)
  },
  set: (d: Date) => {
    form.gen_time = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
})
const formRules = {
  name: [{ required: true, message: '请输入计划名称', trigger: 'blur' }],
  community_id: [{ required: true, message: '请选择小区', trigger: 'change' }],
  cycle_type: [{ required: true }],
  gen_time: [{ required: true, message: '请选择生成时点', trigger: 'change' }]
}

function openForm(row?: ReportPlan) {
  if (row) {
    Object.assign(form, {
      id: row.id, name: row.name, community_id: row.community_id, patrol_type: row.patrol_type,
      cycle_type: row.cycle_type,
      day: Number(row.cycle_config?.day ?? 1),
      weekday: Number(row.cycle_config?.weekday ?? 1),
      gen_time: row.gen_time || '06:00', status: row.status, remark: row.remark || '',
      detail_mode: row.detail_mode || 'full'
    })
  } else {
    Object.assign(form, {
      id: '', name: '', community_id: filterCommunity.value || '', patrol_type: '',
      cycle_type: 'monthly', day: 1, weekday: 1, gen_time: '06:00', status: 'enabled', remark: '',
      detail_mode: 'full'
    })
  }
  formVisible.value = true
}

async function submitForm() {
  await formRef.value?.validate()
  saving.value = true
  try {
    const body = {
      community_id: form.community_id,
      name: form.name,
      patrol_type: form.patrol_type || undefined,
      cycle_type: form.cycle_type,
      cycle_config:
        form.cycle_type === 'monthly' ? { day: form.day }
        : form.cycle_type === 'weekly' ? { weekday: form.weekday }
        : {},
      gen_time: form.gen_time,
      status: form.status,
      remark: form.remark || undefined,
      detail_mode: form.detail_mode
    }
    if (form.id) {
      await updateReportPlan(form.id, body)
      ElMessage.success('已保存')
    } else {
      await createReportPlan(body)
      ElMessage.success('已创建')
    }
    formVisible.value = false
    fetchList()
  } finally {
    saving.value = false
  }
}

// ===== 行操作 =====
const runningId = ref('')

async function handleRun(row: ReportPlan) {
  runningId.value = row.id
  try {
    const res = await runReportPlan(row.id)
    ElMessage.success(`已生成：${res.title}`)
    fetchList()
  } catch {
    // 拦截器已提示（如该期间已有归档报告）
  } finally {
    runningId.value = ''
  }
}

async function handleToggle(row: ReportPlan) {
  const next = row.status === 'enabled' ? 'disabled' : 'enabled'
  await updateReportPlan(row.id, {
    community_id: row.community_id, name: row.name, patrol_type: row.patrol_type || undefined,
    cycle_type: row.cycle_type, cycle_config: row.cycle_config, gen_time: row.gen_time,
    status: next, remark: row.remark || undefined, detail_mode: row.detail_mode || 'full'
  })
  ElMessage.success(next === 'enabled' ? '已启用' : '已停用')
  fetchList()
}

async function handleDelete(row: ReportPlan) {
  const ok = await ElMessageBox.confirm(`删除报告计划「${row.name}」？已生成的报告不受影响。`, '删除确认', {
    confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await deleteReportPlan(row.id)
  ElMessage.success('已删除')
  fetchList()
}

async function loadCommunities() {
  communitiesLoading.value = true
  try {
    const data = await listCommunities({ page: 1, page_size: 100, status: 1 })
    communities.value = data.list
  } finally {
    communitiesLoading.value = false
  }
}

onMounted(async () => {
  await loadCommunities()
  fetchList()
})
</script>

<style scoped>
.form-tip {
  margin-left: 12px;
  color: #86909c;
  font-size: 12px;
}
</style>
