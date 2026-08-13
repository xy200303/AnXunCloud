/**
 * uni.request 统一封装（四端一致）。
 *
 * 能力：
 * - baseURL 按端/环境条件编译（dev 默认指向本机 8090；发版前切换 PROD，见 README「环境切换」）；
 * - 请求拦截：自动携带 Authorization: Bearer <access_token>；
 * - 响应统一解 {code, message, data} 信封（接口文档 §1.3）；
 * - code === 40102（access token 过期）时用 refresh_token 静默刷新并重放原请求，
 *   单飞锁（refreshing）防并发重复刷新；刷新失败清空登录态并回登录页；
 * - 其余业务错误 reject 带 message 的 Error，页面直接展示。
 */

import {
  getAccessToken,
  getRefreshToken,
  saveTokens,
  clearAuthStorage
} from '@/utils/storage'

// ---- baseURL：按端 + 环境切换 ------------------------------------------------
// 发版前：将 ACTIVE_ENV 切到 'prod'（HTTPS 强制）。
type EnvName = 'dev' | 'prod'
const ACTIVE_ENV: EnvName = 'prod'

// #ifndef MP-WEIXIN
const BASE_URL_DEV = 'http://10.172.17.43:8090/api/app' // 内网调试：PC 局域网 IP（手机与电脑同一内网；IP 变了改这里）
const BASE_URL_PROD = 'https://pi.hbuer.com/api/app'
// #endif
// #ifdef MP-WEIXIN
const BASE_URL_DEV = 'http://10.172.17.43:8090/api/mp' // 内网调试：PC 局域网 IP
const BASE_URL_PROD = 'https://pi.hbuer.com/api/mp'
// #endif

const BASE_URL = ACTIVE_ENV == 'prod' ? BASE_URL_PROD : BASE_URL_DEV

const CODE_TOKEN_EXPIRED = 40102 // access token 已过期，触发静默刷新
// 40101 未登录或 token 无效 / 40103 refresh 无效 / 40104 账号已停用 → 直接踢回登录页
const CODE_AUTH_FATAL = [40101, 40103, 40104]

/** 统一响应信封（接口文档 §1.3） */
export type ApiEnvelope<T = any> = {
  code: number
  message: string
  data: T
}

// ---- 底层请求 ----------------------------------------------------------------

/**
 * 发起一次原始请求，resolve 响应信封。
 * uni.request 的 success 在任意 HTTP 状态码下均回调，业务码在信封里判断。
 */
function httpRaw(
  url: string,
  method: 'GET' | 'POST' | 'PUT',
  data: Record<string, any> | null,
  token: string
): Promise<ApiEnvelope> {
  return new Promise<ApiEnvelope>((resolve, reject) => {
    const header: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token != '') {
      header['Authorization'] = 'Bearer ' + token
    }
    uni.request({
      url: url,
      method: method,
      data: method != 'GET' ? data ?? undefined : undefined,
      header: header,
      success: (res) => {
        const body = res.data as ApiEnvelope | null
        if (body == null || typeof body.code != 'number') {
          reject(new Error('服务响应格式异常'))
        } else {
          resolve(body)
        }
      },
      fail: () => {
        reject(new Error('网络异常，请检查网络后重试'))
      }
    })
  })
}

// ---- 静默刷新（单飞锁） --------------------------------------------------------

let refreshing: Promise<boolean> | null = null

/** 用 refresh_token 换新双令牌；成功落盘并 resolve(true)，失败 resolve(false)。并发调用共享同一次刷新。 */
export function refreshSession(): Promise<boolean> {
  if (refreshing != null) {
    return refreshing
  }
  refreshing = new Promise<boolean>((resolve) => {
    const done = (ok: boolean) => {
      refreshing = null
      resolve(ok)
    }
    const rt = getRefreshToken()
    if (rt == '') {
      done(false)
      return
    }
    httpRaw(BASE_URL + '/refresh', 'POST', { refresh_token: rt }, '')
      .then((env) => {
        const at = env.data?.access_token ?? ''
        const newRt = env.data?.refresh_token ?? ''
        if (env.code == 0 && at != '' && newRt != '') {
          saveTokens(at, newRt)
          done(true)
        } else {
          done(false)
        }
      })
      .catch(() => {
        done(false)
      })
  })
  return refreshing
}

/** 刷新失败/登录态失效：清空登录态并回登录页（并发请求只跳一次） */
let loggingOut = false
export function forceLogoutToLogin(msg?: string): void {
  if (loggingOut) return
  loggingOut = true
  clearAuthStorage()
  uni.showToast({ title: msg != null && msg != '' ? msg : '登录已失效，请重新登录', icon: 'none' })
  uni.reLaunch({ url: '/pages/login/index' })
  // reLaunch 完成后解锁，避免并发请求重复跳转/连弹 toast
  setTimeout(() => {
    loggingOut = false
  }, 3000)
}

// ---- 信封解析 + 40102 重放 ------------------------------------------------------

/**
 * 统一请求入口。
 * @param path    以 / 开头的接口路径（不含 baseURL）
 * @param method  'GET' | 'POST' | 'PUT'
 * @param data    POST body（GET 忽略）
 * @param withAuth 是否携带 access_token（登录/注册/刷新等免鉴权接口传 false）
 * @param isRetry 内部参数：40102 刷新后的重放，防止循环
 * @returns 信封中的 data 字段（类型由调用方断言；data 为 null 时 resolve(null)）
 */
export function request<T = any>(
  path: string,
  method: 'GET' | 'POST' | 'PUT',
  data: Record<string, any> | null = null,
  withAuth: boolean = true,
  isRetry: boolean = false
): Promise<T | null> {
  return new Promise<T | null>((resolve, reject) => {
    const token = withAuth ? getAccessToken() : ''
    httpRaw(BASE_URL + path, method, data, token)
      .then((env) => {
        if (env.code == 0) {
          resolve(env.data as T)
        } else if (env.code == CODE_TOKEN_EXPIRED && withAuth && !isRetry) {
          refreshSession()
            .then((ok) => {
              if (ok) {
                request<T>(path, method, data, withAuth, true).then(resolve).catch(reject)
              } else {
                forceLogoutToLogin()
                reject(new Error('登录已失效，请重新登录'))
              }
            })
            .catch(() => {
              forceLogoutToLogin()
              reject(new Error('登录已失效，请重新登录'))
            })
        } else if (withAuth && CODE_AUTH_FATAL.indexOf(env.code) >= 0) {
          // token 无效 / refresh 失效 / 账号停用：直接清登录态回登录页
          forceLogoutToLogin(env.message)
          reject(new Error(env.message || '登录已失效，请重新登录'))
        } else {
          reject(new Error(env.message || '请求失败'))
        }
      })
      .catch(reject)
  })
}

export function httpGet<T = any>(path: string, withAuth: boolean = true): Promise<T | null> {
  return request<T>(path, 'GET', null, withAuth)
}

export function httpPost<T = any>(path: string, data: Record<string, any> | null, withAuth: boolean = true): Promise<T | null> {
  return request<T>(path, 'POST', data, withAuth)
}

export function httpPut<T = any>(path: string, data: Record<string, any> | null, withAuth: boolean = true): Promise<T | null> {
  return request<T>(path, 'PUT', data, withAuth)
}

/** 当前 baseURL（调试用） */
export function getBaseUrl(): string {
  return BASE_URL
}

/** 站点公开访问源（去掉 /api/app|mp 后缀）：用于生成 NFC 短链接等对外 URL */
export function getPublicOrigin(): string {
  return BASE_URL.replace(/\/api\/(app|mp)$/, '')
}
