// 认证接口（接口文档 §2.1）
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
