// 动态路由：登录后拉取菜单树，按 views 目录约定映射组件并注入路由
import { defineStore } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'
import { getRoutes } from '@/api/auth'
import type { RouteMenu } from '@/api/types'
import Layout from '@/layout/index.vue'

// views 下全部页面组件，按后端 path 约定解析：
// 例 /system/user -> views/system/user/index.vue
const viewModules = import.meta.glob('../views/**/*.vue')

function resolveComponent(path: string) {
  const comp = viewModules[`../views${path}/index.vue`]
  if (!comp) {
    console.warn(`[路由] 未找到页面组件：views${path}/index.vue`)
    return () => import('@/views/error/404.vue')
  }
  return comp
}

function buildRoutes(menus: RouteMenu[], parentPath = ''): RouteRecordRaw[] {
  return menus
    .filter((m) => m.type === 'dir' || m.type === 'menu')
    .sort((a, b) => a.sort - b.sort)
    .map((m) => {
      const fullPath = m.path.startsWith('/') ? m.path : `${parentPath}/${m.path}`
      if (m.type === 'dir') {
        // 目录：仅作分组，子菜单拍平挂到 Layout 下
        return buildRoutes(m.children || [], fullPath)
      }
      return {
        path: fullPath,
        component: resolveComponent(fullPath),
        name: fullPath,
        meta: { title: m.title, icon: m.icon }
      } as RouteRecordRaw
    })
    .flat()
}

export const usePermissionStore = defineStore('permission', {
  state: () => ({
    // 侧边栏菜单树（原始结构）
    menus: [] as RouteMenu[],
    loaded: false
  }),
  actions: {
    async generateRoutes(): Promise<RouteRecordRaw[]> {
      const menus = await getRoutes()
      this.menus = menus
      const children = buildRoutes(menus)
      // 首页落点：第一个可用菜单页（新注册账号无菜单时落个人中心）
      const firstPath = children[0]?.path || '/profile'
      // 隐藏路由：不进侧边菜单，由列表页跳转进入
      const hiddenRoutes: RouteRecordRaw[] = [
        {
          path: '/community/building',
          component: viewModules['../views/community/building/index.vue'],
          name: '/community/building',
          meta: { title: '楼栋管理' }
        },
        {
          path: '/inspection/tasks/detail/:id',
          component: viewModules['../views/inspection/tasks/detail/index.vue'],
          name: '/inspection/tasks/detail',
          meta: { title: '任务明细' }
        },
        {
          path: '/workorders/detail/:id',
          component: viewModules['../views/workorders/detail/index.vue'],
          name: '/workorders/detail',
          meta: { title: '工单详情' }
        }
      ]
      children.push(...hiddenRoutes)
      // 个人中心：任何登录用户可访问（仅操作本人数据）；菜单未下发时兜底注册
      if (!children.some((r) => r.path === '/profile')) {
        children.push({
          path: '/profile',
          component: viewModules['../views/profile/index.vue'],
          name: '/profile',
          meta: { title: '个人中心' }
        } as RouteRecordRaw)
      }
      const layoutRoute: RouteRecordRaw = {
        path: '/',
        component: Layout,
        redirect: firstPath,
        children
      }
      this.loaded = true
      return [layoutRoute]
    },
    reset() {
      this.menus = []
      this.loaded = false
    }
  }
})
