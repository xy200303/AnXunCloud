/**
 * 本地存储 key 与登录态存储操作。
 * request 层与 pinia store 共用，避免循环依赖。
 */

export const KEY_ACCESS_TOKEN = 'access_token'
export const KEY_REFRESH_TOKEN = 'refresh_token'
export const KEY_USER_INFO = 'user_info'

/** 清空登录态（token / refresh_token / 用户缓存） */
export function clearAuthStorage(): void {
  uni.removeStorageSync(KEY_ACCESS_TOKEN)
  uni.removeStorageSync(KEY_REFRESH_TOKEN)
  uni.removeStorageSync(KEY_USER_INFO)
}

export function getAccessToken(): string {
  return uni.getStorageSync(KEY_ACCESS_TOKEN) as string
}

export function getRefreshToken(): string {
  return uni.getStorageSync(KEY_REFRESH_TOKEN) as string
}

export function saveTokens(accessToken: string, refreshToken: string): void {
  uni.setStorageSync(KEY_ACCESS_TOKEN, accessToken)
  uni.setStorageSync(KEY_REFRESH_TOKEN, refreshToken)
}
