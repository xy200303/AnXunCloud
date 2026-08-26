<template>
  <div class="app-container">
    <!-- 页头：返回 + 所属模板 -->
    <div class="table-card items-head">
      <el-button :icon="ArrowLeft" @click="goBack">返回模板列表</el-button>
      <h2 class="page-title items-title">
        检查项配置<template v-if="tplName"> · {{ tplName }}</template>
      </h2>
    </div>

    <!-- 检查项表格 -->
    <div class="table-card">
      <div class="table-toolbar">
        <div class="table-toolbar-left">
          <el-button v-perms="'inspection:template:update'" type="primary" :icon="Plus" @click="openForm()">新增检查项</el-button>
        </div>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="RefreshRight" circle @click="fetchItems" />
        </el-tooltip>
      </div>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="sort" label="排序号" width="80" align="center" />
        <el-table-column prop="name" label="检查项名称" min-width="180" show-overflow-tooltip />
        <el-table-column label="检查要求" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">{{ row.requirement || '—' }}</template>
        </el-table-column>
        <el-table-column label="是否必检" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.required ? 'success' : 'info'" size="small">{{ row.required ? '必检' : '选检' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="拍照要求" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="photoTagType(row.photo_required)" size="small" effect="plain">{{ photoLabel(row.photo_required) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="AI 判定类型" width="110" align="center">
          <template #default="{ row }">{{ judgeTypeLabel(row.judge_type) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button v-perms="'inspection:template:update'" link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button v-perms="'inspection:template:update'" link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无检查项">
            <el-button v-perms="'inspection:template:update'" type="primary" @click="openForm()">新增检查项</el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <!-- 新增/编辑检查项对话框 -->
    <el-dialog v-model="formVisible" :title="form.id ? '编辑检查项' : '新增检查项'" width="560px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="88px">
        <el-form-item label="项名" prop="name">
          <el-input v-model="form.name" placeholder="如：灭火器压力正常" maxlength="128" show-word-limit />
        </el-form-item>
        <el-form-item label="检查要求">
          <el-input
            v-model="form.requirement"
            type="textarea"
            :rows="3"
            placeholder="检查标准（可选），如：指针处于绿色区域为正常，红区需充装、黄区超压"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="是否必检">
          <el-switch v-model="form.required" inline-prompt active-text="必检" inactive-text="选检" />
        </el-form-item>
        <el-form-item label="拍照要求">
          <el-select v-model="form.photo_required" style="width: 100%">
            <el-option label="无需拍照" value="none" />
            <el-option label="选拍" value="optional" />
            <el-option label="必拍" value="required" />
          </el-select>
        </el-form-item>
        <el-form-item label="AI 判定类型">
          <el-select v-model="form.judge_type" style="width: 100%">
            <el-option v-for="jt in JUDGE_TYPES" :key="jt.value" :label="jt.label" :value="jt.value">
              <div class="judge-option">
                <span>{{ jt.label }}</span>
                <span class="judge-option-desc">{{ jt.desc }}</span>
              </div>
            </el-option>
          </el-select>
          <div class="text-secondary judge-desc">{{ judgeTypeDesc(form.judge_type) }}</div>
        </el-form-item>
        <!-- metric：指标区间子表单 → judge_config {metric,unit,min,max} -->
        <template v-if="form.judge_type === 'metric'">
          <el-form-item label="指标名称">
            <el-input v-model="form.cfg_metric" placeholder="如：温度" maxlength="32" />
          </el-form-item>
          <el-form-item label="单位">
            <el-input v-model="form.cfg_unit" placeholder="如：℃" maxlength="16" />
          </el-form-item>
          <el-form-item label="允许区间">
            <el-input-number v-model="form.cfg_min" controls-position="right" placeholder="下限" />
            <span class="range-sep">至</span>
            <el-input-number v-model="form.cfg_max" controls-position="right" placeholder="上限" />
          </el-form-item>
        </template>
        <!-- state/indicator：期望状态 → judge_config {expected} -->
        <el-form-item v-if="form.judge_type === 'state' || form.judge_type === 'indicator'" label="期望状态">
          <el-input v-model="form.cfg_expected" placeholder="如：阀门处于开启状态 / 指示灯为绿色常亮" maxlength="64" />
        </el-form-item>
        <el-form-item label="排序号">
          <el-input-number v-model="form.sort" :min="0" :max="9999" controls-position="right" />
          <span class="text-secondary sort-hint">新增时留空则追加到末尾</span>
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
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { ArrowLeft, Plus, RefreshRight } from '@element-plus/icons-vue'
import {
  getTemplate,
  listTemplateItems,
  createTemplateItem,
  updateTemplateItem,
  deleteTemplateItem,
  type TemplateItemRow,
  type JudgeConfig
} from '@/api/template'
import type { PhotoRequired } from '@/api/biz-types'

// AI 判定类型选项（与后端 ai.NormalizeJudgeType 对齐；非法/空值后端兜底为 general）
const JUDGE_TYPES: { value: string; label: string; desc: string }[] = [
  { value: 'general', label: '通用综合判定', desc: '默认：结合检查要求综合判断照片是否合格' },
  { value: 'presence', label: '有无', desc: '判断照片中是否存在所述设施/物品' },
  { value: 'damage', label: '是否损坏', desc: '判断设施外观是否破损、锈蚀、变形' },
  { value: 'metric', label: '指标区间', desc: '读取表计数值并判断是否处于允许区间（需配置指标与上下限）' },
  { value: 'state', label: '状态位置', desc: '判断实际状态是否符合期望状态描述' },
  { value: 'label', label: '有效期标识', desc: '识别检验日期/合格证/铅封，过期或缺失判异常' },
  { value: 'passage', label: '通道遮挡', desc: '判断通道是否被杂物占用、堵塞' },
  { value: 'leak', label: '渗漏痕迹', desc: '判断是否存在水渍、渗漏痕迹' },
  { value: 'indicator', label: '指示灯状态', desc: '判断指示灯颜色/亮灭是否符合期望' },
  { value: 'tidiness', label: '环境整洁', desc: '判断环境是否整洁、有无垃圾堆放' },
  { value: 'manual', label: '手动确认', desc: 'AI 无法判定的感官项（噪音/气味等）：巡检员手选正常/异常，默认正常，不调 AI' }
]

function judgeTypeLabel(jt?: string) {
  return JUDGE_TYPES.find((o) => o.value === jt)?.label || JUDGE_TYPES[0].label
}

function judgeTypeDesc(jt: string) {
  return JUDGE_TYPES.find((o) => o.value === jt)?.desc || ''
}

const route = useRoute()
const router = useRouter()
const templateId = route.params.id as string

const loading = ref(false)
const list = ref<TemplateItemRow[]>([])
const tplName = ref('')

function goBack() {
  router.push('/inspection/templates')
}

function photoLabel(p: PhotoRequired) {
  return { none: '无需拍照', optional: '选拍', required: '必拍' }[p] || p
}

function photoTagType(p: PhotoRequired) {
  return ({ none: 'info', optional: 'warning', required: 'danger' } as const)[p] || 'info'
}

async function fetchItems() {
  loading.value = true
  try {
    list.value = await listTemplateItems(templateId)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchItems()
  getTemplate(templateId).then((t) => {
    tplName.value = t.name
  })
})

// ===== 新增/编辑 =====
const formVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: '',
  name: '',
  requirement: '',
  required: true,
  photo_required: 'none' as PhotoRequired,
  judge_type: 'general',
  // judge_config 子表单字段（按判定类型取用）
  cfg_metric: '',
  cfg_unit: '',
  cfg_min: null as number | null,
  cfg_max: null as number | null,
  cfg_expected: '',
  // null 表示缺省：新增追加到末尾
  sort: null as number | null
})

// 组装 judge_config：metric → {metric,unit,min,max}；state/indicator → {expected}；其余置空
function buildJudgeConfig(): JudgeConfig {
  if (form.judge_type === 'metric') {
    if (!form.cfg_metric.trim()) return null
    return {
      metric: form.cfg_metric.trim(),
      unit: form.cfg_unit.trim(),
      min: form.cfg_min,
      max: form.cfg_max
    }
  }
  if (form.judge_type === 'state' || form.judge_type === 'indicator') {
    return form.cfg_expected.trim() ? { expected: form.cfg_expected.trim() } : null
  }
  return null
}

function fillJudgeConfig(cfg: JudgeConfig) {
  form.cfg_metric = (cfg?.metric as string) || ''
  form.cfg_unit = (cfg?.unit as string) || ''
  form.cfg_min = typeof cfg?.min === 'number' ? (cfg.min as number) : null
  form.cfg_max = typeof cfg?.max === 'number' ? (cfg.max as number) : null
  form.cfg_expected = (cfg?.expected as string) || ''
}

const formRules: FormRules = {
  name: [{ required: true, message: '请输入检查项名称', trigger: 'blur' }]
}

function openForm(row?: TemplateItemRow) {
  formRef.value?.clearValidate()
  if (row) {
    Object.assign(form, {
      id: row.id,
      name: row.name,
      requirement: row.requirement || '',
      required: row.required,
      photo_required: row.photo_required || 'none',
      judge_type: row.judge_type || 'general',
      sort: row.sort
    })
    fillJudgeConfig(row.judge_config ?? null)
  } else {
    Object.assign(form, { id: '', name: '', requirement: '', required: true, photo_required: 'none', judge_type: 'general', sort: null })
    fillJudgeConfig(null)
  }
  formVisible.value = true
}

async function handleSubmit() {
  await formRef.value?.validate()
  const payload = {
    name: form.name.trim(),
    requirement: form.requirement.trim(),
    required: form.required,
    photo_required: form.photo_required,
    judge_type: form.judge_type,
    judge_config: buildJudgeConfig(),
    ...(form.sort === null ? {} : { sort: form.sort })
  }
  submitting.value = true
  try {
    if (form.id) {
      await updateTemplateItem(templateId, form.id, payload)
      ElMessage.success('检查项已更新')
    } else {
      await createTemplateItem(templateId, payload)
      ElMessage.success('检查项已新增')
    }
    formVisible.value = false
    fetchItems()
  } finally {
    submitting.value = false
  }
}

// 删除：历史打卡记录已快照，删项不影响记录
async function handleDelete(row: TemplateItemRow) {
  const ok = await ElMessageBox.confirm(
    `确定删除检查项「${row.name}」吗？历史打卡记录不受影响。`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'error' }
  ).then(() => true).catch(() => false)
  if (!ok) return
  await deleteTemplateItem(templateId, row.id)
  ElMessage.success('已删除')
  fetchItems()
}
</script>

<style scoped lang="scss">
.items-head {
  display: flex;
  align-items: center;
  gap: $spacing-md;

  .items-title {
    margin: 0;
  }
}

.sort-hint {
  margin-left: $spacing-sm;
  font-size: 12px;
}

.judge-option {
  display: flex;
  justify-content: space-between;
  gap: $spacing-md;

  .judge-option-desc {
    font-size: 12px;
    color: $color-text-placeholder;
  }
}

.judge-desc {
  font-size: 12px;
  line-height: 1.4;
}

.range-sep {
  margin: 0 $spacing-sm;
  color: $color-text-secondary;
}
</style>
