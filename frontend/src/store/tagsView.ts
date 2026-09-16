// Tags View：已打开页面标签，支持关闭其他/全部；cachedViews 为 keep-alive include 白名单
import { defineStore } from 'pinia'
import type { RouteLocationNormalized } from 'vue-router'

export interface TagView {
  path: string
  title: string
  // 路由 name（keep-alive 缓存驱逐锚点）
  name?: string
  // 工作台固定不可关闭
  affix?: boolean
}

export const useTagsViewStore = defineStore('tagsView', {
  state: () => ({
    tags: [{ path: '/dashboard', title: '工作台', name: '/dashboard', affix: true }] as TagView[],
    // keep-alive 缓存的组件名白名单（meta.noCache 页面不入名单）
    cachedViews: [] as string[]
  }),
  actions: {
    addTag(route: RouteLocationNormalized) {
      const title = (route.meta?.title as string) || ''
      if (!title || route.path === '/login') return
      const name = typeof route.name === 'string' ? route.name : undefined
      if (name && !route.meta?.noCache && !this.cachedViews.includes(name)) {
        this.cachedViews.push(name)
      }
      if (this.tags.some((t) => t.path === route.path)) return
      this.tags.push({ path: route.path, title, name })
    },
    closeTag(path: string) {
      const idx = this.tags.findIndex((t) => t.path === path)
      if (idx >= 0 && !this.tags[idx].affix) {
        const [closed] = this.tags.splice(idx, 1)
        this.evictView(closed.name)
      }
      return this.tags[idx - 1]?.path || this.tags[this.tags.length - 1]?.path || '/dashboard'
    },
    // 同名路由的其他标签仍打开时保留缓存（如多个字典数据页共用组件名）
    evictView(name?: string) {
      if (!name) return
      if (this.tags.some((t) => t.name === name)) return
      this.cachedViews = this.cachedViews.filter((n) => n !== name)
    },
    // 详情类页面加载数据后更新标签标题（如"字典数据-通用状态"）
    updateTagTitle(path: string, title: string) {
      const tag = this.tags.find((t) => t.path === path)
      if (tag) tag.title = title
    },
    closeOthers(path: string) {
      this.tags = this.tags.filter((t) => t.affix || t.path === path)
      this.pruneViews()
    },
    closeAll() {
      this.tags = this.tags.filter((t) => t.affix)
      this.pruneViews()
    },
    pruneViews() {
      const alive = new Set(this.tags.map((t) => t.name).filter(Boolean))
      this.cachedViews = this.cachedViews.filter((n) => alive.has(n))
    },
    reset() {
      this.tags = [{ path: '/dashboard', title: '工作台', name: '/dashboard', affix: true }]
      this.cachedViews = []
    }
  }
})
