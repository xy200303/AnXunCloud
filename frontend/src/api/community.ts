// 小区/楼栋接口（接口文档 §2.10）
import { request, type PageResult } from '@/utils/request'
import type { CommunityItem, BuildingItem, PostDictItem, StaffItem, StaffForm, DutyBindingItem } from './biz-types'
import type { ReviewFlowView } from './post'

export function listCommunities(params?: { page?: number; page_size?: number; name?: string; status?: number | '' }) {
  return request<PageResult<CommunityItem>>({ url: '/communities', method: 'get', params })
}

export function getCommunity(id: string) {
  return request<CommunityItem & { buildings: BuildingItem[] }>({ url: `/communities/${id}`, method: 'get' })
}

export function createCommunity(data: { name: string; address?: string; manager_id?: string | null; status?: number; wo_triage_enabled?: boolean; wo_grab_enabled?: boolean }) {
  return request<{ id: string }>({ url: '/communities', method: 'post', data })
}

export function updateCommunity(id: string, data: { name?: string; address?: string; manager_id?: string | null; status?: number; wo_triage_enabled?: boolean; wo_grab_enabled?: boolean }) {
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

// ===== 岗位字典 / 项目编制 / 职责槽位绑定（名单制授权，设计方案 §3.2） =====

// 岗位字典（必传 community_id：按小区所属租户返回岗位，含停用与业务线；前端按 status===1 过滤启用项）
export function listPostDict(communityId: string) {
  return request<PostDictItem[]>({ url: '/post-dict', method: 'get', params: { community_id: communityId } })
}

export function listStaff(communityId: string) {
  return request<StaffItem[]>({ url: `/communities/${communityId}/staff`, method: 'get' })
}

export function createStaff(communityId: string, data: StaffForm) {
  return request<{ id: string }>({ url: `/communities/${communityId}/staff`, method: 'post', data })
}

export function updateStaff(communityId: string, staffId: string, data: StaffForm) {
  return request<null>({ url: `/communities/${communityId}/staff/${staffId}`, method: 'put', data })
}

export function deleteStaff(communityId: string, staffId: string) {
  return request<null>({ url: `/communities/${communityId}/staff/${staffId}`, method: 'delete' })
}

export function listDutyBindings(communityId: string) {
  return request<DutyBindingItem[]>({ url: `/communities/${communityId}/duty-bindings`, method: 'get' })
}

// 项目级槽位绑定整体保存（upsert 语义；post_codes 空数组 = 本项目该环节跳过）
export function saveDutyBindings(communityId: string, bindings: { slot: string; post_codes: string[] }[]) {
  return request<null>({ url: `/communities/${communityId}/duty-bindings`, method: 'put', data: { bindings } })
}

// 项目级打卡审核链（扩展方案 §3；source 为 project/tenant/platform/default）
export function getReviewFlow(communityId: string) {
  return request<ReviewFlowView>({ url: `/communities/${communityId}/review-flow`, method: 'get' })
}

export function saveReviewFlow(communityId: string, steps: { slot: string; name: string }[]) {
  return request<null>({ url: `/communities/${communityId}/review-flow`, method: 'put', data: { steps } })
}
