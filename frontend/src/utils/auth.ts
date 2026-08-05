// 登录态令牌存取（localStorage）
const TOKEN_KEY = 'Admin-Token'
const REFRESH_KEY = 'Admin-Refresh-Token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_KEY) || ''
}

export function setRefreshToken(token: string) {
  localStorage.setItem(REFRESH_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_KEY)
}
