<template>
  <div class="register-page">
    <!-- 左侧品牌区（与登录页一致，窄屏隐藏） -->
    <div class="brand-panel">
      <div class="brand-inner">
        <div class="brand-logo">
          <el-icon :size="30"><OfficeBuilding /></el-icon>
        </div>
        <h1 class="brand-name">安巡云</h1>
        <p class="brand-slogan">物业巡检管理平台</p>
        <ul class="brand-points">
          <li>巡检计划与任务执行监控</li>
          <li>扫码 / NFC / GPS 围栏打卡</li>
          <li>异常工单闭环处理</li>
        </ul>
      </div>
      <p class="brand-footer">安巡云 AnxunCloud</p>
    </div>

    <!-- 右侧注册表单 -->
    <div class="form-panel">
      <div class="form-wrap">
        <div class="form-header">
          <h2 class="form-title">账号注册</h2>
          <p class="form-sub">注册成功后由管理员分配角色与小区权限</p>
        </div>

        <!-- 开关关闭时的友好空态（直接访问 /register 的场景） -->
        <div v-if="!configLoading && !enabled" class="disabled-card">
          <el-result icon="info" title="注册功能未开放" sub-title="当前未开放自助注册，账号请联系管理员开通">
            <template #extra>
              <el-button type="primary" @click="$router.push('/login')">返回登录</el-button>
            </template>
          </el-result>
        </div>

        <template v-else>
          <el-form ref="formRef" :model="form" :rules="rules" size="large" label-position="top" v-loading="configLoading">
            <el-form-item label="账号" prop="username">
              <el-input v-model="form.username" placeholder="4-20 位字母、数字或下划线" :prefix-icon="User" />
            </el-form-item>
            <el-form-item label="姓名" prop="name">
              <el-input v-model="form.name" placeholder="真实姓名" :prefix-icon="Postcard" />
            </el-form-item>
            <el-form-item label="手机号" prop="phone">
              <el-input v-model="form.phone" placeholder="11 位手机号" :prefix-icon="Iphone" />
            </el-form-item>
            <el-form-item label="密码" prop="password">
              <el-input v-model="form.password" type="password" show-password placeholder="8-32 位，须含字母与数字" :prefix-icon="Lock" />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirm_password">
              <el-input v-model="form.confirm_password" type="password" show-password placeholder="请再次输入密码" :prefix-icon="Lock" />
            </el-form-item>
            <el-form-item>
              <el-button class="register-btn" type="primary" :loading="submitting" @click="handleRegister">
                注 册
              </el-button>
            </el-form-item>
          </el-form>
          <div class="to-login">
            已有账号？<router-link to="/login">返回登录</router-link>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Postcard, Iphone, OfficeBuilding } from '@element-plus/icons-vue'
import { getRegisterConfig, register } from '@/api/auth'

const router = useRouter()
const formRef = ref<FormInstance>()
const submitting = ref(false)
const configLoading = ref(true)
const enabled = ref(false)

const form = reactive({
  username: '',
  name: '',
  phone: '',
  password: '',
  confirm_password: ''
})

// 进入页面时拉取开关；接口不可用按未开放处理
onMounted(async () => {
  try {
    const cfg = await getRegisterConfig()
    enabled.value = !!cfg.enabled
  } finally {
    configLoading.value = false
  }
})

const validateConfirm = (_rule: any, value: string, callback: (e?: Error) => void) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules: FormRules = {
  username: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { pattern: /^\w{4,20}$/, message: '4-20 位字母、数字或下划线', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1\d{10}$/, message: '手机号格式不正确（1 开头 11 位）', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { pattern: /^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/, message: '密码需 8-32 位，且同时包含字母与数字', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' }
  ]
}

async function handleRegister() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    await register({
      username: form.username,
      password: form.password,
      name: form.name,
      phone: form.phone
    })
    ElMessage.success('注册成功，请登录')
    // 跳登录页并回填账号
    router.push({ path: '/login', query: { username: form.username } })
  } catch {
    // 拦截器已展示后端 message（如"用户名已存在"），表单内容保留可修正重试
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="scss">
.register-page {
  min-height: 100%;
  display: flex;
}

// ===== 左侧品牌区（与登录页一致） =====
.brand-panel {
  width: 420px;
  flex-shrink: 0;
  background: linear-gradient(160deg, $color-primary-active 0%, $color-primary 60%, $color-primary-hover 100%);
  color: $color-white;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 64px 48px 32px;

  .brand-logo {
    width: 56px;
    height: 56px;
    border-radius: $radius-card;
    background: rgba(255, 255, 255, 0.16);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .brand-name {
    font-size: 30px;
    font-weight: 600;
    margin: $spacing-lg 0 $spacing-xs;
    letter-spacing: 2px;
  }

  .brand-slogan {
    font-size: $font-size-card-title;
    opacity: 0.85;
    margin: 0;
    letter-spacing: 1px;
  }

  .brand-points {
    margin: 48px 0 0;
    padding: 0;
    list-style: none;

    li {
      font-size: $font-size-body;
      opacity: 0.85;
      line-height: 2.2;
      padding-left: 20px;
      position: relative;

      &::before {
        content: '';
        position: absolute;
        left: 0;
        top: 14px;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: rgba(255, 255, 255, 0.7);
      }
    }
  }

  .brand-footer {
    font-size: $font-size-aux;
    opacity: 0.6;
    margin: 0;
  }
}

// ===== 右侧表单区 =====
.form-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: $color-bg-page;
  padding: $spacing-xl;
}

.form-wrap {
  width: 400px;

  .form-title {
    font-size: 24px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0;
  }

  .form-sub {
    font-size: $font-size-body;
    color: $color-text-secondary;
    margin: $spacing-sm 0 $spacing-xl;
  }

  .register-btn {
    width: 100%;
  }

  .to-login {
    text-align: center;
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }
}

// 窄屏（平板/手机）隐藏品牌区，表单居中
@media (max-width: 768px) {
  .brand-panel {
    display: none;
  }
}
</style>
