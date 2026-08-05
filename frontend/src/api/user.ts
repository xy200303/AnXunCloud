// 用户管理接口（接口文档 §2.3；profile/password 为接口文档补充的 system 子动作）
import { request, type PageResult } from '@/utils/request'
import type { UserItem, UserForm, ImportResult } from './types'

export interface UserQuery {
  page?: number
  page_size?: number
  username?: string
  phone?: string
  role_id?: string
  community_id?: string
  status?: number | ''
}

export function listUsers(params: UserQuery) {
  return request<PageResult<UserItem>>({ url: '/system/users', method: 'get', params })
}

export function getUser(id: string) {
  return request<UserItem>({ url: `/system/users/${id}`, method: 'get' })
}

export function createUser(data: UserForm) {
  return request<{ id: string }>({ url: '/system/users', method: 'post', data })
}

export function updateUser(id: string, data: Partial<UserForm>) {
  return request<null>({ url: `/system/users/${id}`, method: 'put', data })
}

export function deleteUser(id: string) {
  return request<null>({ url: `/system/users/${id}`, method: 'delete' })
}

export function resetUserPassword(id: string, newPassword: string) {
  return request<null>({ url: `/system/users/${id}/password/reset`, method: 'put', data: { new_password: newPassword } })
}

export function updateUserStatus(id: string, status: number) {
  return request<null>({ url: `/system/users/${id}/status`, method: 'put', data: { status } })
}

export function importUsers(file: File) {
  const form = new FormData()
  form.append('file', file)
  return request<ImportResult>({
    url: '/system/users/import',
    method: 'post',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 60000
  })
}

// 个人中心：资料修改与改密（接口文档补充）
export function updateProfile(data: { name: string; phone: string; email?: string }) {
  return request<null>({ url: '/system/users/profile', method: 'put', data })
}

export function updatePassword(data: { old_password: string; new_password: string }) {
  return request<null>({ url: '/system/users/password', method: 'put', data })
}
