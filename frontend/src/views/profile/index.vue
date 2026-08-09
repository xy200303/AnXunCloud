<template>
  <div class="app-container">
    <el-row :gutter="16">
      <!-- 左：账号概览卡 -->
      <el-col :xs="24" :md="8">
        <div class="profile-card overview-card">
          <el-avatar :size="72" class="profile-avatar">{{ avatarText }}</el-avatar>
          <div class="profile-name">
            {{ userStore.name }}
            <el-tag v-if="info?.is_builtin" type="warning" size="small" class="builtin-tag">内置账号</el-tag>
          </div>
          <div class="profile-roles">
            <el-tag v-for="r in info?.roles || []" :key="r.id" size="small" effect="plain">{{ r.name }}</el-tag>
            <span v-if="!info?.roles?.length" class="text-secondary">未分配角色</span>
          </div>

          <el-descriptions :column="1" border class="profile-desc">
            <el-descriptions-item label="最近登录">
              <template v-if="info?.last_login_at">
                {{ info.last_login_at }}
                <span v-if="info.last_login_ip" class="text-secondary">（{{ info.last_login_ip }}）</span>
              </template>
              <span v-else>--</span>
            </el-descriptions-item>
            <el-descriptions-item label="账号创建时间">{{ info?.created_at || '--' }}</el-descriptions-item>
            <el-descriptions-item label="小程序绑定">
              <el-tag v-if="info?.openid" type="success" size="small">已绑定</el-tag>
              <el-tag v-else type="info" size="small">未绑定</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="手机号">
              <template v-if="info?.phone">{{ info.phone }}</template>
              <!-- 未填写：warning 色提示，点击跳到右侧基本资料表单 -->
              <el-link v-else type="warning" @click="focusPhone">未填写，去补充</el-link>
            </el-descriptions-item>
            <el-descriptions-item label="数据范围">
              {{ info?.data_scope === 'all' ? '全部数据' : '按小区' }}
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </el-col>

      <!-- 右：基本资料 / 账号安全（Tab 切换） -->
      <el-col :xs="24" :md="16">
        <div class="profile-card side-card">
          <el-tabs v-model="activeTab">
            <el-tab-pane label="基本资料" name="profile">
              <el-form ref="profileFormRef" :model="profileForm" :rules="profileRules" label-width="88px" class="side-form">
                <el-form-item label="登录账号">
                  <el-input :model-value="info?.username" disabled />
                </el-form-item>
                <el-form-item label="姓名" prop="name">
                  <el-input v-model="profileForm.name" placeholder="请输入姓名" />
                </el-form-item>
                <el-form-item label="手机号" prop="phone">
                  <el-input ref="phoneInputRef" v-model="profileForm.phone" placeholder="请输入手机号" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="savingProfile" :disabled="!profileDirty" @click="handleSaveProfile">
                    保存修改
                  </el-button>
                </el-form-item>
              </el-form>

              <!-- 手写签名：用于月报签字栏，独立于基本资料表单保存 -->
              <el-divider content-position="left">手写签名</el-divider>
              <div class="signature-block">
                <div v-if="signatureUrl" class="signature-preview">
                  <el-image
                    :src="signatureUrl"
                    fit="contain"
                    class="signature-img"
                    :preview-src-list="[signatureUrl]"
                    preview-teleported
                  />
                </div>
                <div class="signature-actions">
                  <el-upload
                    :auto-upload="false"
                    :show-file-list="false"
                    accept=".png,.jpg,.jpeg"
                    :on-change="handleSignatureChange"
                  >
                    <el-button :loading="savingSignature">{{ signatureUrl ? '更换签名' : '上传签名' }}</el-button>
                  </el-upload>
                  <el-button
                    v-if="signatureUrl"
                    type="danger"
                    link
                    :loading="savingSignature"
                    @click="handleRemoveSignature"
                  >删除</el-button>
                </div>
                <div class="text-secondary signature-tip">
                  手写签名将显示在月度巡检报告的签字栏中，建议使用白底黑字的 PNG/JPG 图片，大小不超过 2MB。
                </div>
              </div>
            </el-tab-pane>

            <el-tab-pane label="账号安全" name="security">
              <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="88px" class="side-form">
                <el-form-item label="旧密码" prop="old_password">
                  <el-input v-model="pwdForm.old_password" type="password" show-password placeholder="请输入旧密码" />
                </el-form-item>
                <el-form-item label="新密码" prop="new_password">
                  <el-input v-model="pwdForm.new_password" type="password" show-password placeholder="8-32 位，须含字母与数字" />
                </el-form-item>
                <el-form-item label="确认密码" prop="confirm_password">
                  <el-input v-model="pwdForm.confirm_password" type="password" show-password placeholder="请再次输入新密码" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="savingPwd" @click="handleSavePassword">修改密码</el-button>
                  <span class="text-secondary pwd-tip">修改成功后需重新登录</span>
                </el-form-item>
              </el-form>
            </el-tab-pane>

            <el-tab-pane label="登录记录" name="logs">
              <!-- 最近登录记录（最多 5 条） -->
              <div class="login-logs">
                <template v-if="loginLogs.length">
                  <div v-for="(log, i) in loginLogs" :key="i" class="login-log-item">
                    <span class="log-time">{{ log.created_at }}</span>
                    <span class="log-ip">{{ log.ip }}</span>
                    <el-tooltip :content="log.ua" placement="top" :disabled="!log.ua">
                      <span class="log-ua">{{ log.ua || '--' }}</span>
                    </el-tooltip>
                    <el-tag :type="log.status === 1 ? 'success' : 'warning'" size="small">
                      {{ log.status === 1 ? '成功' : '失败' }}
                    </el-tag>
                  </div>
                </template>
                <div v-else class="text-secondary">{{ logsNote }}</div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type InputInstance, type UploadFile } from 'element-plus'
import { useUserStore } from '@/store/user'
import { updateProfile, updatePassword, getMyLoginLogs, type MyLoginLog } from '@/api/user'
import { uploadImage } from '@/api/upload'
import { resetRouterState } from '@/router'

const router = useRouter()
const userStore = useUserStore()

const info = computed(() => userStore.info)
const avatarText = computed(() => userStore.name.slice(0, 1) || '用')
const activeTab = ref('profile')

// ===== 基本资料 =====
const profileFormRef = ref<FormInstance>()
const phoneInputRef = ref<InputInstance>()
const savingProfile = ref(false)
const profileForm = reactive({ name: '', phone: '' })

const profileRules: FormRules = {
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1\d{10}$/, message: '手机号格式不正确（1 开头 11 位）', trigger: 'blur' }
  ]
}

// 未保存修改跟踪（离开提示用）
const profileDirty = computed(
  () => profileForm.name !== (info.value?.name || '') || profileForm.phone !== (info.value?.phone || '')
)
const pwdDirty = computed(() => !!(pwdForm.old_password || pwdForm.new_password || pwdForm.confirm_password))

onMounted(() => {
  profileForm.name = userStore.info?.name || ''
  profileForm.phone = userStore.info?.phone || ''
  fetchLoginLogs()
})

// 手机号"未填写"点击：切到基本资料 Tab 并聚焦手机号输入框
async function focusPhone() {
  activeTab.value = 'profile'
  await nextTick()
  phoneInputRef.value?.focus()
}

async function handleSaveProfile() {
  await profileFormRef.value?.validate()
  savingProfile.value = true
  try {
    await updateProfile(profileForm)
    ElMessage.success('资料已保存')
    // 刷新 store，顶栏头像区姓名即时更新
    await userStore.fetchInfo()
  } finally {
    savingProfile.value = false
  }
}

// ===== 手写签名（月报签字栏用）=====
const signatureUrl = computed(() => info.value?.signature_url || '')
const savingSignature = ref(false)

// 签名随 profile 保存：name/phone 传当前已保存值，避免覆盖基本资料表单
async function saveSignature(fileKey: string) {
  savingSignature.value = true
  try {
    await updateProfile({
      name: info.value?.name || '',
      phone: info.value?.phone || '',
      signature_file_key: fileKey
    })
    await userStore.fetchInfo()
    return true
  } catch {
    // 统一错误提示由 request 封装处理；上传接口未就绪（并行开发中）时静默失败
    return false
  } finally {
    savingSignature.value = false
  }
}

async function handleSignatureChange(uploadFile: UploadFile) {
  const raw = uploadFile.raw
  if (!raw) return
  if (!['image/png', 'image/jpeg'].includes(raw.type)) {
    ElMessage.warning('仅支持 PNG/JPG 格式图片')
    return
  }
  if (raw.size > 2 * 1024 * 1024) {
    ElMessage.warning('图片大小不能超过 2MB')
    return
  }
  savingSignature.value = true
  try {
    const { file_key } = await uploadImage(raw, 'signature')
    savingSignature.value = false
    if (await saveSignature(file_key)) ElMessage.success('签名已保存')
  } catch {
    savingSignature.value = false
    ElMessage.warning('签名上传失败，上传接口可能尚未就绪')
  }
}

async function handleRemoveSignature() {
  const ok = await ElMessageBox.confirm('删除后月报签字栏将不再显示您的手写签名，确定删除吗？', '删除签名', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => true).catch(() => false)
  if (!ok) return
  if (await saveSignature('')) ElMessage.success('签名已删除')
}

// ===== 修改密码 =====
const pwdFormRef = ref<FormInstance>()
const savingPwd = ref(false)
const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })

const pwdRules: FormRules = {
  old_password: [{ required: true, message: '请输入旧密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      pattern: /^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/,
      message: '新密码需 8-32 位，且同时包含字母与数字',
      trigger: 'blur'
    }
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_r: any, value: string, cb: (e?: Error) => void) =>
        value === pwdForm.new_password ? cb() : cb(new Error('两次输入的密码不一致')),
      trigger: 'blur'
    }
  ]
}

async function handleSavePassword() {
  await pwdFormRef.value?.validate()
  savingPwd.value = true
  try {
    await updatePassword({ old_password: pwdForm.old_password, new_password: pwdForm.new_password })
    ElMessage.success('密码已修改，请使用新密码重新登录')
    // 改密成功强制重新登录
    userStore.reset()
    resetRouterState()
    router.push('/login')
  } finally {
    savingPwd.value = false
  }
}

// ===== 最近登录记录 =====
const loginLogs = ref<MyLoginLog[]>([])
const logsNote = ref('暂无登录记录')

async function fetchLoginLogs() {
  try {
    const data = await getMyLoginLogs(5)
    loginLogs.value = (data || []).slice(0, 5)
  } catch {
    // 后端接口并行开发中，未上线时静默降级
    logsNote.value = '登录记录接口暂未开放'
  }
}

// ===== 未保存离开提示 =====
onBeforeRouteLeave(async () => {
  if (!profileDirty.value && !pwdDirty.value) return true
  const ok = await ElMessageBox.confirm('有未保存的修改，离开后内容将丢失，确定离开吗？', '未保存的修改', {
    confirmButtonText: '离开',
    cancelButtonText: '继续编辑',
    type: 'warning'
  }).then(() => true).catch(() => false)
  return ok
})
</script>

<style scoped lang="scss">
.profile-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-xl;
}

.overview-card {
  text-align: center;

  .profile-avatar {
    background-color: $color-primary;
    font-size: $font-size-data;
  }

  .profile-name {
    font-size: $font-size-card-title;
    font-weight: 600;
    margin-top: $spacing-md;

    .builtin-tag {
      margin-left: $spacing-sm;
      vertical-align: 2px;
    }
  }

  .profile-roles {
    display: flex;
    justify-content: center;
    gap: $spacing-sm;
    margin: $spacing-sm 0 $spacing-lg;
    min-height: 24px;
  }

  .profile-desc {
    text-align: left;
  }
}

.side-card {
  .side-form {
    max-width: 480px;
  }

  .pwd-tip {
    margin-left: $spacing-md;
  }
}

.signature-block {
  .signature-preview {
    width: 240px;
    height: 80px;
    background: $color-white;
    border: 1px solid $color-border;
    border-radius: $radius-card;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: $spacing-md;

    .signature-img {
      max-width: 232px;
      max-height: 72px;
      cursor: pointer;
    }
  }

  .signature-actions {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    margin-bottom: $spacing-sm;
  }

  .signature-tip {
    font-size: $font-size-aux;
  }
}

.login-logs {
  .login-log-item {
    display: flex;
    align-items: center;
    gap: $spacing-lg;
    padding: $spacing-sm 0;
    font-size: $font-size-aux;
    color: $color-text-regular;

    .log-time {
      width: 150px;
      flex-shrink: 0;
    }

    .log-ip {
      width: 110px;
      flex-shrink: 0;
    }

    .log-ua {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: $color-text-secondary;
    }
  }
}
</style>
