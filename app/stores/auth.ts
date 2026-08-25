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
import { bindPushDevice, unbindPushDevice, syncBadge, setAppBadge } from '@/utils/push'

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
    login(username: string, password: string, tenantCode?: string): Promise<void> {
      return new Promise<void>((resolve, reject) => {
        apiLogin(username, password, tenantCode)
          .then((res) => {
            this.token = res.access_token
            this.refreshToken = res.refresh_token
            saveTokens(res.access_token, res.refresh_token)
            // uniPush：登录成功上报 CID 绑定（拿不到 cid/接口失败静默跳过，不阻塞登录）
            bindPushDevice()
            // 图标角标同步（拉取未读数 setBadgeNumber，失败静默）
            syncBadge()
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
    /** 登出：先解绑推送设备（须在清 token 前），再调后端注销（失败也继续本地清理），最后清登录态回登录页 */
    logout() {
      unbindPushDevice().then(() => {
        apiLogout()
          .then(() => { this.resetToLogin() })
          .catch((_e: any) => { this.resetToLogin() })
      })
    },
    resetToLogin() {
      this.token = ''
      this.refreshToken = ''
      this.userInfo = null
      clearAuthStorage()
      // 登出清零图标角标（App 端生效，其他端静默跳过）
      setAppBadge(0)
      uni.reLaunch({ url: '/pages/login/index' })
    }
  }
})
