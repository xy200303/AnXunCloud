<template>
  <div class="login-page">
    <!-- 左侧品牌区（窄屏隐藏） -->
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

    <!-- 右侧登录表单 -->
    <div class="form-panel">
      <div class="form-wrap">
        <div class="form-header">
          <h2 class="form-title">欢迎登录</h2>
          <p class="form-sub">请使用管理员分配的账号登录管理后台</p>
        </div>

        <el-form ref="formRef" :model="form" :rules="rules" size="large" @keyup.enter="handleLogin">
          <el-form-item prop="username">
            <el-input v-model="form.username" placeholder="请输入账号" :prefix-icon="User" autocomplete="username" />
          </el-form-item>
          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              show-password
              placeholder="请输入密码"
              :prefix-icon="Lock"
              autocomplete="current-password"
            />
          </el-form-item>
          <el-form-item>
            <el-button class="login-btn" type="primary" :loading="loading" @click="handleLogin">
              登 录
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 注册入口：仅当后端开关开启时显示 -->
        <div v-if="registerEnabled" class="to-register">
          没有账号？<router-link to="/register">注册</router-link>
        </div>
        <p class="form-footer">账号由管理员统一开通，如有问题请联系系统管理员</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, OfficeBuilding } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { getRegisterConfig } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const formRef = ref<FormInstance>()
const loading = ref(false)
const registerEnabled = ref(false)

const form = reactive({
  username: '',
  password: ''
})

onMounted(async () => {
  // 注册成功跳转回填账号
  if (route.query.username) {
    form.username = String(route.query.username)
  }
  // 注册开关：接口不可用时按关闭处理
  const cfg = await getRegisterConfig()
  registerEnabled.value = !!cfg.enabled
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function handleLogin() {
  await formRef.value?.validate()
  loading.value = true
  try {
    await userStore.login(form)
    ElMessage.success('登录成功')
    // 失败时拦截器已提示具体原因，且不清空账号；默认落 '/' 由布局重定向到首个可用菜单
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.login-page {
  min-height: 100%;
  display: flex;
}

// ===== 左侧品牌区 =====
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
  width: 360px;

  .form-title {
    font-size: 24px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0;
  }

  .form-sub {
    font-size: $font-size-body;
    color: $color-text-secondary;
    margin: $spacing-sm 0 $spacing-xxl;
  }

  .login-btn {
    width: 100%;
  }

  .to-register {
    text-align: center;
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }

  .form-footer {
    margin-top: 48px;
    text-align: center;
    font-size: $font-size-aux;
    color: $color-text-placeholder;
  }
}

// 窄屏（平板/手机）隐藏品牌区，表单居中
@media (max-width: 768px) {
  .brand-panel {
    display: none;
  }
}
</style>
