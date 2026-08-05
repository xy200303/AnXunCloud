// Tags View：已打开页面标签，支持关闭其他/全部
import { defineStore } from 'pinia'
import type { RouteLocationNormalized } from 'vue-router'

export interface TagView {
  path: string
  title: string
  // 工作台固定不可关闭
  affix?: boolean
}

export const useTagsViewStore = defineStore('tagsView', {
  state: () => ({
    tags: [{ path: '/dashboard', title: '工作台', affix: true }] as TagView[]
  }),
  actions: {
    addTag(route: RouteLocationNormalized) {
      const title = (route.meta?.title as string) || ''
      if (!title || route.path === '/login') return
      if (this.tags.some((t) => t.path === route.path)) return
      this.tags.push({ path: route.path, title })
    },
    closeTag(path: string) {
      const idx = this.tags.findIndex((t) => t.path === path)
      if (idx >= 0 && !this.tags[idx].affix) this.tags.splice(idx, 1)
      return this.tags[idx - 1]?.path || this.tags[this.tags.length - 1]?.path || '/dashboard'
    },
    closeOthers(path: string) {
      this.tags = this.tags.filter((t) => t.affix || t.path === path)
    },
    closeAll() {
      this.tags = this.tags.filter((t) => t.affix)
    },
    reset() {
      this.tags = [{ path: '/dashboard', title: '工作台', affix: true }]
    }
  }
})
