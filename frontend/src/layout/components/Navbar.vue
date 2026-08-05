<template>
  <header class="navbar">
    <div class="navbar-left">
      <el-icon class="action-icon" :size="18" @click="appStore.toggleSidebar()">
        <Expand v-if="appStore.sidebarCollapsed" />
        <Fold v-else />
      </el-icon>
      <el-breadcrumb separator="/">
        <el-breadcrumb-item v-for="(item, index) in breadcrumbs" :key="index">
          {{ item }}
        </el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="navbar-right">
      <!-- 全屏 -->
      <el-tooltip content="全屏" placement="bottom">
        <el-icon class="action-icon" :size="18" @click="toggleFullscreen">
          <FullScreen />
        </el-icon>
      </el-tooltip>

      <!-- 消息通知（消息中心后续迭代，先给入口与角标） -->
      <el-tooltip content="消息通知" placement="bottom">
        <el-badge :value="3" :max="99" class="message-badge">
          <el-icon class="action-icon" :size="18"><Bell /></el-icon>
        </el-badge>
      </el-tooltip>

      <!-- 用户下拉 -->
      <el-dropdown trigger="click" @command="handleCommand">
        <span class="user-entry">
          <el-avatar :size="28" class="user-avatar">{{ avatarText }}</el-avatar>
          <span class="user-name">{{ userStore.name }}</span>
          <el-icon :size="14"><ArrowDown /></el-icon>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="profile">
              <el-icon><User /></el-icon>个人中心
            </el-dropdown-item>
            <el-dropdown-item command="password">
              <el-icon><Lock /></el-icon>修改密码
            </el-dropdown-item>
            <el-dropdown-item divided command="logout">
              <el-icon><SwitchButton /></el-icon>退出登录
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <!-- 修改密码对话框（顶栏直达，与个人中心共用表单逻辑） -->
    <password-dialog v-model="passwordVisible" @success="handlePasswordChanged" />
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAppStore } from '@/store/app'
import { useUserStore } from '@/store/user'
import { usePermissionStore } from '@/store/permission'
import { resetRouterState } from '@/router'
import type { RouteMenu } from '@/api/types'
import PasswordDialog from '@/components/PasswordDialog.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const userStore = useUserStore()
const permissionStore = usePermissionStore()

const passwordVisible = ref(false)

const avatarText = computed(() => userStore.name.slice(0, 1) || '用')

// 面包屑：从菜单树回溯当前路径的祖先链
const breadcrumbs = computed(() => {
  const chain: string[] = []
  const walk = (menus: RouteMenu[], trail: string[]): boolean => {
    for (const m of menus) {
      const next = [...trail, m.title]
      if (m.path === route.path) {
        chain.push(...next)
        return true
      }
      if (m.children?.length && walk(m.children, next)) return true
    }
    return false
  }
  walk(permissionStore.menus, [])
  return chain.length ? chain : [(route.meta?.title as string) || '']
})

function toggleFullscreen() {
  if (document.fullscreenElement) {
    document.exitFullscreen()
  } else {
    document.documentElement.requestFullscreen()
  }
}

async function handleCommand(command: string) {
  if (command === 'profile') {
    router.push('/profile')
  } else if (command === 'password') {
    passwordVisible.value = true
  } else if (command === 'logout') {
    const ok = await ElMessageBox.confirm('确定退出登录吗？', '退出登录', {
      confirmButtonText: '退出',
      cancelButtonText: '取消',
      type: 'warning'
    }).then(() => true).catch(() => false)
    if (!ok) return
    await userStore.logout()
    resetRouterState()
    router.push('/login')
  }
}

// 修改密码成功后强制重新登录
function handlePasswordChanged() {
  userStore.reset()
  resetRouterState()
  router.push('/login')
}
</script>

<style scoped lang="scss">
.navbar {
  height: $navbar-height;
  background: $color-bg-card;
  border-bottom: 1px solid $color-border;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 $spacing-lg;
  flex-shrink: 0;
}

.navbar-left {
  display: flex;
  align-items: center;
  gap: $spacing-lg;
}

.navbar-right {
  display: flex;
  align-items: center;
  gap: $spacing-lg;
}

.action-icon {
  cursor: pointer;
  color: $color-text-regular;

  &:hover {
    color: $color-primary;
  }
}

.message-badge {
  display: flex;
}

.user-entry {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  cursor: pointer;
  color: $color-text-primary;

  .user-avatar {
    background-color: $color-primary;
    font-size: $font-size-aux;
  }

  .user-name {
    font-size: $font-size-body;
  }
}
</style>
