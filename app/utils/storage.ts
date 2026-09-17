/**
 * 本地存储 key 与登录态存储操作。
 * request 层与 pinia store 共用，避免循环依赖。
 */

export const KEY_ACCESS_TOKEN = 'access_token'
export const KEY_REFRESH_TOKEN = 'refresh_token'
export const KEY_USER_INFO = 'user_info'

/** 多账号切换：本机保存的账号凭据列表；设备级数据，登出不清理 */
export const KEY_SWITCH_ACCOUNTS = 'switch_accounts'

/** 保存的账号凭据（本机 storage；password 明文仅供本机一键登录回填） */
export type SwitchAccount = {
  username: string
  password: string
  /** 公司编码：用户名跨租户重名时消歧，空 = 不传 */
  tenant_code: string
  /** 展示名（登录成功时取用户姓名），空 = 显示用户名 */
  remark: string
}

export function loadSwitchAccounts(): SwitchAccount[] {
  const raw = uni.getStorageSync(KEY_SWITCH_ACCOUNTS) as string
  if (raw == '') return []
  try {
    const list = JSON.parse(raw) as SwitchAccount[]
    return Array.isArray(list) ? list : []
  } catch (e) {
    return []
  }
}

export function saveSwitchAccounts(list: SwitchAccount[]): void {
  uni.setStorageSync(KEY_SWITCH_ACCOUNTS, JSON.stringify(list))
}

/** 登录成功后保存/覆盖账号（同账号同公司为同一条）；置顶为最近使用 */
export function upsertSwitchAccount(entry: SwitchAccount): void {
  const list = loadSwitchAccounts().filter((a) => !(a.username == entry.username && a.tenant_code == entry.tenant_code))
  list.unshift(entry)
  saveSwitchAccounts(list)
}

/** 会话性数据 storage key 注册表（登出/强制登出随登录态一并清理，防共用设备串户） */
export const KEY_OFFLINE_QUEUE = 'offline_checkins'
export const KEY_CHECKIN_DRAFT_PREFIX = 'checkin_draft:'
export const MAINTAIN_EDIT_KEY = 'maintain_edit_draft'
export const KEY_UPDATE_PKG_CACHE = 'update_pkg_cache'

/** 清空登录态（token / refresh_token / 用户缓存） */
export function clearAuthStorage(): void {
  uni.removeStorageSync(KEY_ACCESS_TOKEN)
  uni.removeStorageSync(KEY_REFRESH_TOKEN)
  uni.removeStorageSync(KEY_USER_INFO)
}

/** 清空会话性数据（离线打卡队列 / 打卡草稿前缀 key / 维保编辑草稿 / 安装包缓存）；不碰登录态 */
export function clearSessionStorage(): void {
  uni.removeStorageSync(KEY_OFFLINE_QUEUE)
  uni.removeStorageSync(MAINTAIN_EDIT_KEY)
  uni.removeStorageSync(KEY_UPDATE_PKG_CACHE)
  try {
    const info = uni.getStorageInfoSync()
    const keys = (info && info.keys) || []
    for (let i = 0; i < keys.length; i++) {
      if (keys[i].indexOf(KEY_CHECKIN_DRAFT_PREFIX) == 0) {
        uni.removeStorageSync(keys[i])
      }
    }
  } catch (e) {
    // getStorageInfoSync 异常不阻断登出主流程
  }
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
