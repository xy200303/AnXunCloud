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
      <!-- 全局租户切换器：仅超管且租户数 > 1 时显示 -->
      <tenant-context-bar />

      <!-- 全屏 -->
      <el-tooltip content="全屏" placement="bottom">
        <el-icon class="action-icon" :size="18" @click="toggleFullscreen">
          <FullScreen />
        </el-icon>
      </el-tooltip>

      <!-- 消息通知：真实未读角标 + 弹出面板 -->
      <el-popover v-model:visible="messageVisible" trigger="click" :width="360" placement="bottom-end" @show="handlePopoverShow">
        <template #reference>
          <el-badge :value="unreadCount" :max="99" :hidden="unreadCount === 0" class="message-badge">
            <el-icon class="action-icon" :size="18"><Bell /></el-icon>
          </el-badge>
        </template>
        <div class="message-panel">
          <div class="message-panel-head">
            <span class="panel-title">未读消息 {{ unreadCount }} 条</span>
            <el-button
              link
              type="primary"
              size="small"
              :disabled="unreadCount === 0"
              @click="handleReadAll"
            >全部已读</el-button>
          </div>
          <div v-loading="messagesLoading" class="message-list">
            <template v-if="messages.length">
              <div
                v-for="msg in messages"
                :key="msg.id"
                class="message-item"
                :class="{ unread: !msg.is_read }"
                @click="handleMessageClick(msg)"
              >
                <span class="unread-dot" />
                <div class="message-main">
                  <div class="message-title">{{ msg.title }}</div>
                  <div class="message-time">{{ msg.created_at }}</div>
                </div>
              </div>
            </template>
            <div v-else-if="!messagesLoading" class="message-empty">{{ messagesNote }}</div>
          </div>
        </div>
      </el-popover>

      <!-- 用户下拉 -->
      <el-dropdown trigger="click" @command="handleCommand">
        <span class="user-entry">
          <el-avatar :size="28" class="user-avatar" :src="avatarUrl">{{ avatarText }}</el-avatar>
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { Expand, Fold, FullScreen, Bell, ArrowDown, User, Lock, SwitchButton } from '@element-plus/icons-vue'
import { useAppStore } from '@/store/app'
import { useUserStore } from '@/store/user'
import { usePermissionStore } from '@/store/permission'
import { resetRouterState } from '@/router'
import type { RouteMenu } from '@/api/types'
import { listMessages, markMessageRead, type MessageItem } from '@/api/message'
import { fileUrl } from '@/api/upload'
import PasswordDialog from '@/components/PasswordDialog.vue'
import TenantContextBar from '@/components/TenantContextBar.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const userStore = useUserStore()
const permissionStore = usePermissionStore()

const passwordVisible = ref(false)

const avatarText = computed(() => userStore.name.slice(0, 1) || '用')
// 头像 URL：avatar 存 file_key，拼 /uploads 静态路由；无头像回退姓氏文字
const avatarUrl = computed(() => (userStore.info?.avatar ? fileUrl(userStore.info.avatar) : ''))

// ===== 消息通知 =====
const messageVisible = ref(false)
const messagesLoading = ref(false)
const messages = ref<MessageItem[]>([])
const unreadCount = ref(0)
const messagesNote = ref('暂无消息')

// 拉取消息与未读数；后端接口未上线时静默降级（不弹错、角标隐藏）
async function fetchMessages() {
  try {
    const data = await listMessages({ page: 1, page_size: 10 })
    messages.value = data.list || []
    unreadCount.value = data.unread_count ?? messages.value.filter((m) => !m.is_read).length
  } catch {
    messages.value = []
    unreadCount.value = 0
    messagesNote.value = '消息服务暂未开放'
  }
}

// 打开面板时刷新一次（带 loading）
async function handlePopoverShow() {
  messagesLoading.value = true
  try {
    await fetchMessages()
  } finally {
    messagesLoading.value = false
  }
}

// 60s 轮询未读角标
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchMessages()
  pollTimer = setInterval(fetchMessages, 60000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

// 点击消息：标记已读（局部更新角标），不再跳转业务详情
async function handleMessageClick(msg: MessageItem) {
  if (!msg.is_read) {
    msg.is_read = 1
    unreadCount.value = Math.max(0, unreadCount.value - 1)
    markMessageRead(msg.id).catch(() => {})
  }
}

// 全部已读（id=0 约定）
async function handleReadAll() {
  try {
    await markMessageRead('0')
    messages.value.forEach((m) => (m.is_read = 1))
    unreadCount.value = 0
  } catch {
    // 静默：接口未上线时不打扰
  }
}


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
  cursor: pointer;

  .action-icon:hover {
    color: $color-primary;
  }
}

// 消息面板（popover 虽 teleport 到 body，但 scoped 样式按属性选择器仍生效）
.message-panel {
  .message-panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: $spacing-sm;
    border-bottom: 1px solid $color-border;
    margin-bottom: $spacing-xs;

    .panel-title {
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  .message-list {
    max-height: 360px;
    overflow-y: auto;
    min-height: 60px;
  }

  .message-empty {
    text-align: center;
    color: $color-text-secondary;
    font-size: $font-size-aux;
    padding: $spacing-xl 0;
  }

  .message-item {
    display: flex;
    align-items: flex-start;
    gap: $spacing-sm;
    padding: $spacing-sm $spacing-xs;
    border-radius: $radius-small;
    cursor: pointer;

    &:hover {
      background: $color-bg-page;
    }

    .unread-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: transparent;
      margin-top: 6px;
      flex-shrink: 0;
    }

    &.unread {
      .unread-dot {
        background: $color-primary;
      }

      .message-title {
        font-weight: 600;
        color: $color-text-primary;
      }
    }

    .message-main {
      flex: 1;
      min-width: 0;
    }

    .message-title {
      color: $color-text-regular;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .message-time {
      font-size: $font-size-aux;
      color: $color-text-secondary;
      margin-top: 2px;
    }
  }
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
