<template>
  <div class="login-page">
    <!-- 左侧品牌区（窄屏隐藏） -->
    <div class="brand-panel">
      <div class="brand-deco deco-tr"></div>
      <div class="brand-deco deco-bl"></div>
      <div class="brand-inner">
        <div class="brand-logo">
          <img :src="brandMark" alt="安巡云" />
        </div>
        <h1 class="brand-name">安巡云</h1>
        <p class="brand-slogan">ANXUNCLOUD · 物业巡检管理平台</p>
        <div class="brand-divider"></div>
        <ul class="brand-points">
          <li><el-icon :size="15"><CircleCheck /></el-icon><span>巡检计划与任务执行监控</span></li>
          <li><el-icon :size="15"><CircleCheck /></el-icon><span>扫码 / NFC / GPS 围栏打卡</span></li>
          <li><el-icon :size="15"><CircleCheck /></el-icon><span>异常工单闭环处理</span></li>
        </ul>
      </div>
      <p class="brand-footer">安巡云 AnxunCloud · 物业巡检管理平台</p>
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
import { User, Lock, CircleCheck } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { getRegisterConfig } from '@/api/auth'

// 品牌资源走 public 目录，需带上部署子路径（/admin/）
const brandMark = `${import.meta.env.BASE_URL}brand/anxuncloud-mark-reverse.svg`

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
  width: 440px;
  flex-shrink: 0;
  position: relative;
  overflow: hidden;
  background: linear-gradient(155deg, #14263d 0%, $color-sidebar 54%, $color-primary-active 100%);
  color: $color-white;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 0 56px;

  // 装饰光圈（右上 / 左下，纯装饰不响应事件）
  .brand-deco {
    position: absolute;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.14);
    pointer-events: none;
  }

  .deco-tr {
    width: 340px;
    height: 340px;
    top: -120px;
    right: -120px;
    box-shadow: 0 0 0 40px rgba(255, 255, 255, 0.04), 0 0 0 90px rgba(255, 255, 255, 0.03);
  }

  .deco-bl {
    width: 260px;
    height: 260px;
    bottom: -100px;
    left: -100px;
    box-shadow: 0 0 0 36px rgba(255, 255, 255, 0.04);
  }

  .brand-inner {
    position: relative;
    z-index: 1;
  }

  .brand-logo {
    width: 76px;
    height: 76px;

    img {
      display: block;
      width: 100%;
      height: 100%;
    }
  }

  .brand-name {
    margin: 18px 0 4px;
    font-size: 34px;
    font-weight: 600;
    letter-spacing: 8px;
  }

  .brand-slogan {
    margin: 0;
    color: rgba(247, 250, 255, 0.72);
    font-size: 13px;
    letter-spacing: 1.5px;
  }

  .brand-divider {
    width: 40px;
    height: 3px;
    border-radius: 2px;
    background: #c8a56b;
    margin: $spacing-xxl 0;
  }

  .brand-points {
    margin: 0;
    padding: 0;
    list-style: none;

    li {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: $font-size-body;
      opacity: 0.9;
      line-height: 2.4;
    }
  }

  .brand-footer {
    position: absolute;
    left: 56px;
    bottom: 28px;
    font-size: $font-size-aux;
    opacity: 0.55;
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
