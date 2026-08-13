/**
 * 登录态 store（pinia options store）。
 * token / refreshToken / userInfo 持久化到 uni storage（request 层直接从 storage 读 token，
 * 与本 store 以 storage 为单一事实来源保持同步）。
 */

import { defineStore } from 'pinia'
import { apiLogin, apiLogout, apiProfile, UserInfo } from '@/services/api'
import {
  KEY_USER_INFO,
  getAccessToken,
  getRefreshToken,
  saveTokens,
  clearAuthStorage
} from '@/utils/storage'

function loadCachedUser(): UserInfo | null {
  const raw = uni.getStorageSync(KEY_USER_INFO) as string
  if (raw == '') return null
  try {
    return JSON.parse(raw) as UserInfo
  } catch (e) {
    return null
  }
}

export const useAuthStore = defineStore('auth', {
  state() {
    return {
      token: getAccessToken(),
      refreshToken: getRefreshToken(),
      userInfo: null as UserInfo | null
    }
  },
  getters: {
    isLoggedIn(): boolean {
      return this.token != ''
    },
    /** 工单 tab 是否可见：巡检员（我上报的）与维修工（待我处理）可见，主管/经理走 PC */
    canSeeWorkorders(): boolean {
      const u = this.userInfo
      return u != null && (u.roles.indexOf('inspector') >= 0 || u.roles.indexOf('repair') >= 0)
    },
    /**
     * 权限点判断：传入单个 code 或数组，任一命中即 true；userInfo 为空返回 false。
     * 超管以 super_admin 角色兜底（与 api.ts hasPerm 同口径）。
     */
    hasPerm(): (codes: string | string[]) => boolean {
      const u = this.userInfo
      return (codes: string | string[]): boolean => {
        if (u == null) return false
        if ((u.roles ?? []).indexOf('super_admin') >= 0) return true
        const list = typeof codes == 'string' ? [codes] : codes
        const perms = u.perms ?? []
        for (let i = 0; i < list.length; i++) {
          if (perms.indexOf(list[i]) >= 0) return true
        }
        return false
      }
    }
  },
  actions: {
    /** 启动时从 storage 恢复用户缓存（App.vue onLaunch 调用） */
    restore() {
      this.userInfo = loadCachedUser()
    },
    login(username: string, password: string): Promise<void> {
      return new Promise<void>((resolve, reject) => {
        apiLogin(username, password)
          .then((res) => {
            this.token = res.access_token
            this.refreshToken = res.refresh_token
            saveTokens(res.access_token, res.refresh_token)
            if (res.user != null) {
              this.setUser(res.user)
              resolve()
            } else {
              // 登录回包不含 user 时补拉个人信息
              this.fetchProfile().then(() => resolve()).catch(() => resolve())
            }
          })
          .catch(reject)
      })
    },
    fetchProfile(): Promise<void> {
      return new Promise<void>((resolve, reject) => {
        apiProfile()
          .then((u) => {
            this.setUser(u)
            resolve()
          })
          .catch(reject)
      })
    },
    setUser(u: UserInfo) {
      this.userInfo = u
      uni.setStorageSync(KEY_USER_INFO, JSON.stringify(u))
    },
    /** 登出：先调后端注销（失败也继续本地清理），再清登录态回登录页 */
    logout() {
      apiLogout()
        .then(() => { this.resetToLogin() })
        .catch((_e: any) => { this.resetToLogin() })
    },
    resetToLogin() {
      this.token = ''
      this.refreshToken = ''
      this.userInfo = null
      clearAuthStorage()
      uni.reLaunch({ url: '/pages/login/index' })
    }
  }
})
