<template>
  <div class="login-page">
    <div class="login-header">
      <el-icon :size="32" class="logo-icon"><OfficeBuilding /></el-icon>
      <h1 class="system-name">物业巡检管理系统</h1>
      <p class="system-sub">管理后台</p>
    </div>

    <div class="login-card">
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
    </div>

    <p class="login-footer">账号由管理员统一开通，如有问题请联系系统管理员</p>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  password: ''
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
    // 失败时拦截器已提示具体原因，且不清空账号
    const redirect = (route.query.redirect as string) || '/dashboard'
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
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: $color-bg-page;
  padding: $spacing-xl;
}

.login-header {
  text-align: center;
  margin-bottom: $spacing-xl;

  .logo-icon {
    color: $color-primary;
  }

  .system-name {
    font-size: 24px;
    font-weight: 600;
    color: $color-text-primary;
    margin: $spacing-md 0 $spacing-xs;
  }

  .system-sub {
    font-size: $font-size-body;
    color: $color-text-secondary;
    margin: 0;
  }
}

.login-card {
  width: 400px;
  background: $color-bg-card;
  border-radius: $radius-card;
  padding: $spacing-xxl $spacing-xl $spacing-xl;
  box-shadow: $shadow-popup;

  .login-btn {
    width: 100%;
  }
}

.login-footer {
  margin-top: $spacing-xl;
  font-size: $font-size-aux;
  color: $color-text-secondary;
}
</style>
