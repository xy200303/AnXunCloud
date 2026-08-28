import { ref } from 'vue'
import type { RouteMenu } from '@/api/types'

/**
 * 简洁模式（面向低配置使用方）：隐藏角色/岗位/审批流等高级菜单，只留日常业务功能。
 * 顶栏右上角开关控制，选择持久化在浏览器 localStorage（纯展示层过滤，不拦截路由直达）。
 */
const STORAGE_KEY = 'anxun.simple-mode'

/** 简洁模式下隐藏的菜单路径（目录前缀整体隐藏） */
const ADVANCED_PATHS = new Set([
  '/system/roles',        // 角色管理（内置角色已够用）
  '/system/posts',        // 岗位管理（预置岗位模板已够用）
  '/system/sign-assets',  // 签章管理
  '/system/review-flow',  // 审批流程（默认链已够用）
  '/system/brand',        // 企业品牌
  '/platform'             // 平台管理整组（租户/菜单/字典/系统配置等，仅部署运维用）
])

/** 全局单例：默认开启 */
const simple = ref(localStorage.getItem(STORAGE_KEY) !== '0')

export function useSimpleMode() {
  function setSimple(v: boolean) {
    simple.value = v
    localStorage.setItem(STORAGE_KEY, v ? '1' : '0')
  }
  return { simple, setSimple }
}

/** 简洁模式菜单过滤：命中高级路径的隐藏；目录的子项全被隐藏时目录一并隐藏 */
export function filterSimpleMenus(menus: RouteMenu[]): RouteMenu[] {
  if (!simple.value) return menus
  const walk = (list: RouteMenu[]): RouteMenu[] => {
    const out: RouteMenu[] = []
    for (const m of list) {
      if (ADVANCED_PATHS.has(m.path)) continue
      const children = m.children ? walk(m.children) : m.children
      if ((m.children?.length ?? 0) > 0 && (children?.length ?? 0) === 0) continue
      out.push({ ...m, children })
    }
    return out
  }
  return walk(menus)
}
