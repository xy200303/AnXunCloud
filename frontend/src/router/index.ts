// 路由：静态路由（登录/404）+ 登录后动态注入业务路由
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { getToken } from '@/utils/auth'
import { useUserStore } from '@/store/user'
import { usePermissionStore } from '@/store/permission'
import { useTagsViewStore } from '@/store/tagsView'

// 白名单：无需登录
const WHITE_LIST = ['/login', '/404']

export const constantRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录' }
  },
  {
    path: '/404',
    component: () => import('@/views/error/404.vue'),
    meta: { title: '页面不存在' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes: constantRoutes,
  scrollBehavior: () => ({ top: 0 })
})

// 导航守卫：登录校验 + 动态路由注入
router.beforeEach(async (to) => {
  document.title = to.meta?.title
    ? `${to.meta.title} · 安巡云`
    : '安巡云 · 管理后台'

  const hasToken = !!getToken()

  if (!hasToken) {
    if (WHITE_LIST.includes(to.path)) return true
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  // 已登录访问登录页 → 工作台
  if (to.path === '/login') {
    return { path: '/dashboard' }
  }

  const userStore = useUserStore()
  const permissionStore = usePermissionStore()

  // 动态路由未加载：拉取用户信息 + 菜单树并注入
  if (!permissionStore.loaded) {
    try {
      if (!userStore.info) {
        await userStore.fetchInfo()
      }
      const routes = await permissionStore.generateRoutes()
      routes.forEach((r) => router.addRoute(r))
      // 注入后重新导航，确保匹配到新路由
      return { ...to, replace: true }
    } catch {
      userStore.reset()
      return { path: '/login', query: { redirect: to.fullPath } }
    }
  }

  // 已加载但无匹配（刷新后未知路径）→ 404
  if (to.matched.length === 0) {
    return { path: '/404' }
  }
  return true
})

router.afterEach((to) => {
  useTagsViewStore().addTag(to)
})

// 登出/会话失效时重置（供外部调用）
export function resetRouterState() {
  usePermissionStore().reset()
  useTagsViewStore().reset()
}

export default router
