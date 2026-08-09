// 用户管理接口（接口文档 §2.3；profile/password 为接口文档补充的 system 子动作）
import axios from 'axios'
import { request, type PageResult } from '@/utils/request'
import { getToken } from '@/utils/auth'
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
// signature_file_key：手写签名图片 file_key（先经上传接口获取）；传空串表示删除签名
export function updateProfile(data: { name: string; phone: string; email?: string; signature_file_key?: string }) {
  return request<null>({ url: '/system/users/profile', method: 'put', data })
}

export function updatePassword(data: { old_password: string; new_password: string }) {
  return request<null>({ url: '/system/users/password', method: 'put', data })
}

// 最近登录记录（仅本人，登录即可）
export interface MyLoginLog {
  ip: string
  ua: string
  status: number
  msg: string
  created_at: string
}

// 裸调：后端接口并行开发中，未上线/报错时由调用方静默降级，不弹错误提示
export async function getMyLoginLogs(limit = 5): Promise<MyLoginLog[]> {
  const resp = await axios.get('/api/admin/system/users/my-login-logs', {
    params: { limit },
    headers: { Authorization: `Bearer ${getToken()}` }
  })
  if (resp.data?.code !== 0) throw new Error(resp.data?.message || '获取登录记录失败')
  return resp.data.data || []
}
