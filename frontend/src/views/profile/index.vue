<template>
  <div class="app-container">
    <el-row :gutter="16">
      <!-- 左：个人信息卡 -->
      <el-col :xs="24" :md="8">
        <div class="profile-card">
          <el-avatar :size="72" class="profile-avatar">{{ avatarText }}</el-avatar>
          <div class="profile-name">{{ userStore.name }}</div>
          <div class="profile-role">{{ roleNames }}</div>
          <el-descriptions :column="1" border class="profile-desc">
            <el-descriptions-item label="登录账号">{{ info?.username || '--' }}</el-descriptions-item>
            <el-descriptions-item label="手机号">{{ info?.phone || '--' }}</el-descriptions-item>
            <el-descriptions-item label="所属小区">{{ communityText }}</el-descriptions-item>
            <el-descriptions-item label="数据范围">
              {{ info?.data_scope === 'all' ? '全部数据' : '按小区' }}
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </el-col>

      <!-- 右：基本资料 / 修改密码 -->
      <el-col :xs="24" :md="16">
        <div class="profile-card">
          <el-tabs v-model="activeTab">
            <el-tab-pane label="基本资料" name="profile">
              <el-form ref="profileFormRef" :model="profileForm" :rules="profileRules" label-width="88px" class="tab-form">
                <el-form-item label="登录账号">
                  <el-input :model-value="info?.username" disabled />
                </el-form-item>
                <el-form-item label="姓名" prop="name">
                  <el-input v-model="profileForm.name" placeholder="请输入姓名" />
                </el-form-item>
                <el-form-item label="手机号" prop="phone">
                  <el-input v-model="profileForm.phone" placeholder="请输入手机号" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="savingProfile" @click="handleSaveProfile">
                    保存修改
                  </el-button>
                </el-form-item>
              </el-form>
            </el-tab-pane>

            <el-tab-pane label="修改密码" name="password">
              <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="88px" class="tab-form">
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
                  <el-button type="primary" :loading="savingPwd" @click="handleSavePassword">
                    保存修改
                  </el-button>
                  <span class="text-secondary pwd-tip">修改成功后需重新登录</span>
                </el-form-item>
              </el-form>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useUserStore } from '@/store/user'
import { updateProfile, updatePassword } from '@/api/user'
import { resetRouterState } from '@/router'

const router = useRouter()
const userStore = useUserStore()

const activeTab = ref('profile')
const info = computed(() => userStore.info)

const avatarText = computed(() => userStore.name.slice(0, 1) || '用')
const roleNames = computed(() => userStore.info?.roles?.map((r) => r.name).join(' / ') || '--')
const communityText = computed(() => {
  const ids = userStore.info?.community_ids || []
  return ids.length ? `${ids.length} 个小区` : '全部小区'
})

// ===== 基本资料 =====
const profileFormRef = ref<FormInstance>()
const savingProfile = ref(false)
const profileForm = reactive({ name: '', phone: '' })

const profileRules: FormRules = {
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1\d{10}$/, message: '手机号格式不正确（1 开头 11 位）', trigger: 'blur' }
  ]
}

onMounted(() => {
  profileForm.name = userStore.info?.name || ''
  profileForm.phone = userStore.info?.phone || ''
})

async function handleSaveProfile() {
  await profileFormRef.value?.validate()
  savingProfile.value = true
  try {
    await updateProfile(profileForm)
    ElMessage.success('资料已保存')
    await userStore.fetchInfo()
  } finally {
    savingProfile.value = false
  }
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
</script>

<style scoped lang="scss">
.profile-card {
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-xl;
  text-align: center;

  .profile-avatar {
    background-color: $color-primary;
    font-size: $font-size-data;
  }

  .profile-name {
    font-size: $font-size-card-title;
    font-weight: 600;
    margin-top: $spacing-md;
  }

  .profile-role {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin: $spacing-xs 0 $spacing-lg;
  }

  .profile-desc {
    text-align: left;
  }
}

.tab-form {
  max-width: 460px;
  text-align: left;
  margin-top: $spacing-lg;
}

.pwd-tip {
  margin-left: $spacing-md;
}
</style>
