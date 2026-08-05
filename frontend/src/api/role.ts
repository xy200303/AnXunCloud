// 角色管理接口（接口文档 §2.4）
import { request, type PageResult } from '@/utils/request'
import type { RoleItem, RoleDetail } from './types'

export function listRoles(params: { page?: number; page_size?: number; name?: string; status?: number | '' }) {
  return request<PageResult<RoleItem>>({ url: '/system/roles', method: 'get', params })
}

export function getRole(id: string) {
  return request<RoleDetail>({ url: `/system/roles/${id}`, method: 'get' })
}

export function createRole(data: Partial<RoleItem> & { menu_ids?: string[] }) {
  return request<{ id: string }>({ url: '/system/roles', method: 'post', data })
}

export function updateRole(id: string, data: Partial<RoleItem> & { menu_ids?: string[] }) {
  return request<null>({ url: `/system/roles/${id}`, method: 'put', data })
}

export function deleteRole(id: string) {
  return request<null>({ url: `/system/roles/${id}`, method: 'delete' })
}

// 分配权限：菜单树勾选 + 数据范围
export function assignRoleMenus(id: string, data: { menu_ids: string[]; data_scope: string }) {
  return request<null>({ url: `/system/roles/${id}/menus`, method: 'put', data })
}
