// 认证接口（接口文档 §2.1）
import axios from 'axios'
import { request } from '@/utils/request'
import type { LoginParams, LoginResult, UserInfo, RouteMenu } from './types'

export function login(data: LoginParams) {
  return request<LoginResult>({ url: '/auth/login', method: 'post', data })
}

export function logout() {
  return request<null>({ url: '/auth/logout', method: 'post' })
}

export function getInfo() {
  return request<UserInfo>({ url: '/auth/info', method: 'get' })
}

export function getRoutes() {
  return request<RouteMenu[]>({ url: '/auth/routes', method: 'get' })
}

// 注册开关（免登录）；后端未提供该接口时按未开放处理
export interface RegisterConfig {
  enabled: boolean
}

export async function getRegisterConfig(): Promise<RegisterConfig> {
  try {
    // 裸调：后端未提供该接口（并行开发中）时静默按未开放处理，不弹错误提示
    const resp = await axios.get('/api/admin/auth/register-config')
    return resp.data?.data ?? { enabled: false }
  } catch {
    return { enabled: false }
  }
}

// 注册可选公司列表（免登录，仅注册开关开启时可用，否则 40303）
export interface TenantOption {
  code: string
  name: string
}

export async function getRegisterTenants(): Promise<TenantOption[]> {
  try {
    // 裸调：开关关闭（40303）或接口不可用时静默按空列表处理，不弹错误提示
    const resp = await axios.get('/api/admin/auth/register-tenants')
    return resp.data?.data ?? []
  } catch {
    return []
  }
}

// 注册（免登录）；tenant_code 选填：多租户时由"所属公司"下拉传入，不传 = 默认租户
export function register(data: { username: string; password: string; name: string; phone: string; tenant_code?: string }) {
  return request<null>({ url: '/auth/register', method: 'post', data })
}
