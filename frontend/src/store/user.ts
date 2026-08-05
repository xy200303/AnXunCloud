// 当前登录用户状态：登录、用户信息、权限点、登出
import { defineStore } from 'pinia'
import { login as apiLogin, logout as apiLogout, getInfo } from '@/api/auth'
import { setToken, setRefreshToken, clearToken } from '@/utils/auth'
import type { UserInfo, LoginParams } from '@/api/types'

export const useUserStore = defineStore('user', {
  state: () => ({
    info: null as UserInfo | null,
    perms: [] as string[]
  }),
  getters: {
    name: (state) => state.info?.name || '',
    isSuperAdmin: (state) => state.info?.roles?.some((r) => r.code === 'super_admin') ?? false
  },
  actions: {
    async login(params: LoginParams) {
      const data = await apiLogin(params)
      setToken(data.access_token)
      setRefreshToken(data.refresh_token)
    },
    async fetchInfo() {
      const info = await getInfo()
      this.info = info
      this.perms = info.perms || []
    },
    async logout() {
      try {
        await apiLogout()
      } catch {
        // 登出接口失败不阻塞本地清理
      }
      this.reset()
    },
    reset() {
      clearToken()
      this.info = null
      this.perms = []
    },
    // 按钮级权限判断（超管放行）
    hasPerm(perm: string): boolean {
      if (this.isSuperAdmin) return true
      return this.perms.includes(perm)
    }
  }
})
