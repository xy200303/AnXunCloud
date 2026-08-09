// 小区/楼栋接口（接口文档 §2.10）
import { request, type PageResult } from '@/utils/request'
import type { CommunityItem, BuildingItem } from './biz-types'

export function listCommunities(params?: { page?: number; page_size?: number; name?: string; status?: number | '' }) {
  return request<PageResult<CommunityItem>>({ url: '/communities', method: 'get', params })
}

export function getCommunity(id: string) {
  return request<CommunityItem & { buildings: BuildingItem[] }>({ url: `/communities/${id}`, method: 'get' })
}

export function createCommunity(data: { name: string; address?: string; manager_id?: string | null; status?: number }) {
  return request<{ id: string }>({ url: '/communities', method: 'post', data })
}

export function updateCommunity(id: string, data: { name?: string; address?: string; manager_id?: string | null; status?: number }) {
  return request<null>({ url: `/communities/${id}`, method: 'put', data })
}

export function deleteCommunity(id: string) {
  return request<null>({ url: `/communities/${id}`, method: 'delete' })
}

export function listBuildings(params: { community_id: string; name?: string; type?: string; page?: number; page_size?: number }) {
  return request<PageResult<BuildingItem>>({ url: '/buildings', method: 'get', params })
}

// 小区/楼栋树：点位管理等左树一次加载，替代逐小区请求楼栋
export interface CommunityTreeNode {
  id: string
  name: string
  buildings: { id: string; name: string; type: string }[]
}

export function listCommunityTree() {
  return request<CommunityTreeNode[]>({ url: '/communities/tree', method: 'get' })
}

export function createBuilding(data: { community_id: string; name: string; type: string }) {
  return request<{ id: string }>({ url: '/buildings', method: 'post', data })
}

export function updateBuilding(id: string, data: { name?: string; type?: string }) {
  return request<null>({ url: `/buildings/${id}`, method: 'put', data })
}

export function deleteBuilding(id: string) {
  return request<null>({ url: `/buildings/${id}`, method: 'delete' })
}
