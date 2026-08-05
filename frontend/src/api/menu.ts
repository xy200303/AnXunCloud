// 菜单管理接口（接口文档 §2.5）
import { request } from '@/utils/request'
import type { MenuNode } from './types'

export function listMenus(params?: { title?: string; status?: number | '' }) {
  return request<MenuNode[]>({ url: '/system/menus', method: 'get', params })
}

export function getMenu(id: string) {
  return request<MenuNode>({ url: `/system/menus/${id}`, method: 'get' })
}

export function createMenu(data: Partial<MenuNode>) {
  return request<{ id: string }>({ url: '/system/menus', method: 'post', data })
}

export function updateMenu(id: string, data: Partial<MenuNode>) {
  return request<null>({ url: `/system/menus/${id}`, method: 'put', data })
}

export function deleteMenu(id: string) {
  return request<null>({ url: `/system/menus/${id}`, method: 'delete' })
}
