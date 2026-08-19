<template>
  <div>
    <el-tabs v-model="activeTab">
      <!-- 岗位列表 -->
      <el-tab-pane :label="isTemplate ? '模板岗位' : '岗位列表'" name="posts">
        <div class="filter-card">
          <el-form inline @submit.prevent>
            <el-form-item label="岗位名称">
              <el-input v-model="keyword" placeholder="名称 / 编码" clearable style="width: 200px" @input="filterPosts" />
            </el-form-item>
            <el-form-item label="业务线">
              <el-select v-model="lineFilter" placeholder="全部" clearable style="width: 140px" @change="filterPosts">
                <el-option v-for="l in POST_LINES" :key="l.value" :label="l.label" :value="l.value" />
              </el-select>
            </el-form-item>
          </el-form>
        </div>

        <div class="table-card">
          <div class="table-toolbar">
            <div class="table-toolbar-left">
              <el-button v-perms="`${permPrefix}:create`" type="primary" :icon="Plus" @click="openPostForm()">
                {{ isTemplate ? '新增模板岗位' : '新增岗位' }}
              </el-button>
            </div>
            <el-tooltip content="刷新" placement="top">
              <el-button :icon="RefreshRight" circle @click="fetchPosts()" />
            </el-tooltip>
          </div>

          <el-table v-loading="postLoading" :data="filteredPosts" style="width: 100%">
            <el-table-column prop="name" label="岗位名称" min-width="140">
              <template #default="{ row }">
                <span style="font-weight: 600">{{ row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="code" label="岗位编码" min-width="130" show-overflow-tooltip />
            <el-table-column label="业务线" width="90" align="center">
              <template #default="{ row }">{{ row.line_name || postLineName(row.line) }}</template>
            </el-table-column>
            <el-table-column label="绑定角色" min-width="120">
              <template #default="{ row }">{{ row.role_name || '--' }}</template>
            </el-table-column>
            <el-table-column label="主管级" width="80" align="center">
              <template #default="{ row }">
                <el-tag v-if="row.is_supervisor" type="warning" size="small">主管级</el-tag>
                <span v-else class="text-secondary">--</span>
              </template>
            </el-table-column>
            <el-table-column prop="sort" label="排序" width="70" align="center" />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.remark || '--' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="130">
              <template #default="{ row }">
                <el-button v-perms="`${permPrefix}:update`" link type="primary" @click="openPostForm(row)">编辑</el-button>
                <el-button v-perms="`${permPrefix}:delete`" link type="danger" @click="handleDeletePost(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty :description="isTemplate ? '暂无模板岗位' : '暂无岗位'">
                <el-button v-perms="`${permPrefix}:create`" type="primary" @click="openPostForm()">
                  {{ isTemplate ? '新增模板岗位' : '新增岗位' }}
                </el-button>
              </el-empty>
            </template>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- 职责槽位默认绑定 -->
      <el-tab-pane label="职责槽位默认绑定" name="duty">
        <div class="table-card">
          <el-alert
            type="info"
            :closable="false"
            :title="dutyTip"
            style="margin-bottom: 16px"
          />
          <el-table v-loading="dutyLoading" :data="dutyList" stripe style="width: 100%">
            <el-table-column prop="name" label="职责槽位" min-width="140" />
            <el-table-column label="绑定岗位" min-width="320">
              <template #default="{ row }">
                <el-select
                  v-model="dutyEdits[row.slot]"
                  multiple
                  collapse-tags
                  collapse-tags-tooltip
                  placeholder="留空 = 该环节跳过"
                  :disabled="!userStore.hasPerm(dutySavePerm)"
                  style="width: 100%"
                >
                  <el-option v-for="p in enabledPosts" :key="p.code" :label="p.name" :value="p.code" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="来源" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="row.source === 'tenant' ? 'warning' : 'info'" size="small">
                  {{ row.source === 'tenant' ? '租户默认' : '平台默认' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <div class="duty-footer">
            <el-button
              v-perms="dutySavePerm"
              type="primary"
              :loading="dutySaving"
              @click="handleDutySave"
            >保存绑定</el-button>
          </div>
          <ReviewFlowEditor :api="flowApi" :slot-options="slotOptions" :save-perm="dutySavePerm" />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 新增/编辑岗位对话框 -->
    <el-dialog
      v-model="postFormVisible"
      :title="postForm.id ? (isTemplate ? '编辑模板岗位' : '编辑岗位') : (isTemplate ? '新增模板岗位' : '新增岗位')"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-form ref="postFormRef" :model="postForm" :rules="postFormRules" label-width="88px">
        <el-form-item label="岗位名称" prop="name">
          <el-input v-model="postForm.name" placeholder="如：秩序维护员" />
        </el-form-item>
        <el-form-item label="岗位编码" prop="code">
          <el-input v-model="postForm.code" :disabled="!!postForm.id" placeholder="如：security_guard" />
        </el-form-item>
        <el-form-item label="业务线" prop="line">
          <el-select v-model="postForm.line" placeholder="请选择业务线" style="width: 100%">
            <el-option v-for="l in POST_LINES" :key="l.value" :label="l.label" :value="l.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定角色">
          <el-select v-model="postForm.role_id" clearable placeholder="选填，绑定后该岗位成员获得角色权限" style="width: 100%">
            <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="主管级">
          <el-switch v-model="postForm.is_supervisor" />
        </el-form-item>
        <el-form-item label="排序号">
          <el-input-number v-model="postForm.sort" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="postForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="postForm.remark" type="textarea" :rows="2" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="postFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="postSubmitting" @click="handleSubmitPost">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
// 岗位管理 / 岗位模板库共享组件（《管理后台信息架构与菜单归位方案》第三章）
// 两组接口完全同构，仅 api 函数、权限点前缀与文案不同，由 props 注入
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { POST_LINES, postLineName, getReviewFlow, saveReviewFlow, getPostTemplateReviewFlow, savePostTemplateReviewFlow, type PostItem, type PostLine, type PostSaveReq, type PostDutyBindingView, type ReviewFlowStep } from '@/api/post'
import { listRoles } from '@/api/role'
import { useUserStore } from '@/store/user'
import type { RoleItem } from '@/api/types'
import ReviewFlowEditor from './ReviewFlowEditor.vue'

const props = defineProps<{
  // 权限点前缀：system:post（岗位管理）/ platform:post（岗位模板库）
  permPrefix: string
  // 是否平台模板库（仅影响文案与说明；接口差异由下方 api 注入承担）
  isTemplate?: boolean
  api: {
    list: () => Promise<PostItem[]>
    create: (data: PostSaveReq) => Promise<unknown>
    update: (id: string, data: PostSaveReq) => Promise<unknown>
    remove: (id: string) => Promise<unknown>
    listDuty: () => Promise<PostDutyBindingView[]>
    saveDuty: (bindings: { slot: string; post_codes: string[] }[]) => Promise<unknown>
  }
}>()

const userStore = useUserStore()
const isTemplate = computed(() => !!props.isTemplate)

const activeTab = ref<'posts' | 'duty'>('posts')

// 槽位保存权限点：岗位管理 system:post:duty / 模板库 platform:post:update（与后端路由一致）
const dutySavePerm = computed(() => (isTemplate.value ? `${props.permPrefix}:update` : `${props.permPrefix}:duty`))

// 打卡审核链（租户级 / 平台模板两组接口同构，按 isTemplate 注入）
const flowApi = computed(() =>
  isTemplate.value
    ? { listFlow: getPostTemplateReviewFlow, saveFlow: (s: ReviewFlowStep[]) => savePostTemplateReviewFlow(s) as Promise<unknown> }
    : { listFlow: getReviewFlow, saveFlow: (s: ReviewFlowStep[]) => saveReviewFlow(s) as Promise<unknown> }
)
// 审核链环节的槽位选项（复用职责槽位列表）
const slotOptions = computed(() => dutyList.value.map((d) => ({ slot: d.slot, name: d.name })))

const dutyTip = computed(() =>
  isTemplate.value
    ? '各环节默认人选由「槽位绑定岗位 → 项目编制在职成员」推导；此处为平台默认，开通租户时随模板岗位一并复制；岗位留空 = 该环节跳过'
    : '各环节默认人选由「槽位绑定岗位 → 项目编制在职成员」推导；此处为租户级默认（未配置时回落平台默认）；岗位留空 = 该环节跳过；项目级覆盖在「小区管理 → 编制 → 职责槽位绑定」中配置'
)

// ===== 岗位列表 =====
const postLoading = ref(false)
const posts = ref<PostItem[]>([])
const keyword = ref('')
const lineFilter = ref('')
const filteredPosts = ref<PostItem[]>([])

// 仅启用岗位参与槽位绑定选择
const enabledPosts = computed(() => posts.value.filter((p) => p.status === 1))

function filterPosts() {
  let list = posts.value
  const kw = keyword.value.trim()
  if (kw) list = list.filter((p) => p.name.includes(kw) || p.code.includes(kw))
  if (lineFilter.value) list = list.filter((p) => p.line === lineFilter.value)
  filteredPosts.value = list
}

async function fetchPosts() {
  postLoading.value = true
  try {
    posts.value = await props.api.list()
    filterPosts()
  } finally {
    postLoading.value = false
  }
}

// ===== 绑定角色候选（角色列表接口，按租户上下文返回内置+本租户角色） =====
const roleOptions = ref<RoleItem[]>([])

async function fetchRoleOptions() {
  const data = await listRoles({ page: 1, page_size: 100 })
  roleOptions.value = data.list.filter((r) => r.status === 1)
}

onMounted(() => {
  fetchPosts()
  fetchRoleOptions()
  fetchDutyBindings()
})

// ===== 岗位新增/编辑/删除 =====
const postFormVisible = ref(false)
const postSubmitting = ref(false)
const postFormRef = ref<FormInstance>()
const postForm = reactive({
  id: '',
  name: '',
  code: '',
  line: '' as PostLine | '',
  role_id: '',
  is_supervisor: false,
  sort: 0,
  status: 1,
  remark: ''
})

const postFormRules: FormRules = {
  name: [{ required: true, message: '请输入岗位名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入岗位编码', trigger: 'blur' },
    { pattern: /^[a-z0-9][a-z0-9_]{1,63}$/, message: '2–64 位小写字母/数字/下划线', trigger: 'blur' }
  ],
  line: [{ required: true, message: '请选择业务线', trigger: 'change' }]
}

function openPostForm(post?: PostItem) {
  postFormRef.value?.clearValidate()
  Object.assign(postForm, post
    ? {
        id: post.id, name: post.name, code: post.code, line: post.line,
        role_id: post.role_id || '', is_supervisor: post.is_supervisor,
        sort: post.sort, status: post.status, remark: post.remark
      }
    : { id: '', name: '', code: '', line: '', role_id: '', is_supervisor: false, sort: 0, status: 1, remark: '' })
  postFormVisible.value = true
}

async function handleSubmitPost() {
  await postFormRef.value?.validate()
  postSubmitting.value = true
  try {
    const payload = {
      code: postForm.code,
      name: postForm.name,
      line: postForm.line,
      role_id: postForm.role_id,
      is_supervisor: postForm.is_supervisor,
      sort: postForm.sort,
      status: postForm.status,
      remark: postForm.remark
    }
    if (postForm.id) {
      await props.api.update(postForm.id, payload)
      ElMessage.success('岗位已更新')
    } else {
      await props.api.create(payload)
      ElMessage.success('岗位已创建')
    }
    postFormVisible.value = false
    fetchPosts()
  } finally {
    postSubmitting.value = false
  }
}

// 删除：被编制/槽位绑定引用时后端拒绝，错误信息由拦截器统一提示
async function handleDeletePost(post: PostItem) {
  const ok = await ElMessageBox.confirm(`删除后不可恢复，确定删除岗位「${post.name}」吗？`, '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'error'
  }).then(() => true).catch(() => false)
  if (!ok) return
  await props.api.remove(post.id)
  ElMessage.success('已删除')
  fetchPosts()
}

// ===== 职责槽位默认绑定 =====
const dutyLoading = ref(false)
const dutySaving = ref(false)
const dutyList = ref<PostDutyBindingView[]>([])
// 逐槽位编辑值（slot → post_codes）
const dutyEdits = reactive<Record<string, string[]>>({})

async function fetchDutyBindings() {
  dutyLoading.value = true
  try {
    const data = await props.api.listDuty()
    dutyList.value = data
    for (const b of data) {
      dutyEdits[b.slot] = [...b.post_codes]
    }
  } finally {
    dutyLoading.value = false
  }
}

// 整体保存：全量提交 7 个槽位（后端逐槽位 upsert）
async function handleDutySave() {
  dutySaving.value = true
  try {
    const bindings = dutyList.value.map((b) => ({ slot: b.slot, post_codes: dutyEdits[b.slot] || [] }))
    await props.api.saveDuty(bindings)
    ElMessage.success('绑定已保存')
    fetchDutyBindings()
  } finally {
    dutySaving.value = false
  }
}
</script>

<style scoped lang="scss">
.duty-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: $spacing-md;
}
</style>
