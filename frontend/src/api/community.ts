// 小区/楼栋接口（接口文档 §2.10）
import { request, type PageResult } from '@/utils/request'
import type { CommunityItem, BuildingItem, RoleOption, StaffItem, StaffForm } from './biz-types'
import type { ReviewFlowView, ReviewFlowStep, FlowCode } from './post'

export function listCommunities(params?: { page?: number; page_size?: number; name?: string; status?: number | '' }) {
  return request<PageResult<CommunityItem>>({ url: '/communities', method: 'get', params })
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

export function listRoleOptions(communityId: string) {
  return request<RoleOption[]>({ url: '/role-options', method: 'get', params: { community_id: communityId } })
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

export function getProjectFlow(communityId: string, code: FlowCode) {
  return request<ReviewFlowView>({ url: `/communities/${communityId}/review-flows/${code}`, method: 'get' })
}

export function saveProjectFlow(communityId: string, code: FlowCode, steps: ReviewFlowStep[]) {
  return request<null>({ url: `/communities/${communityId}/review-flows/${code}`, method: 'put', data: { steps } })
}

// 审批链名单预览（GET /communities/:id/flow-preview）：每环节实际审核人+来源+空态原因，配置显性化用
export interface FlowPreviewStep {
  index: number
  name: string
  kind?: string
  mode?: string
  role_code?: string
  voters?: { id: string; name: string; can_see?: boolean }[]
  voter_source?: string
  empty_reason?: '' | 'no_member'
  voter_note?: string
  hidden_count?: number
}
export interface FlowPreviewView {
  source: string
  steps: FlowPreviewStep[]
}
export function getFlowPreview(communityId: string, flowCode: FlowCode) {
  return request<FlowPreviewView>({ url: `/communities/${communityId}/flow-preview`, method: 'get', params: { flow_code: flowCode } })
}

export interface PresetPackSummary {
  code: string
  name: string
  description: string
  status: string
}

export interface PresetPack extends PresetPackSummary {
  payload: {
    point_types: { type_code: string; label: string; value: string; sort: number; status: number; attrs?: Record<string, string> }[]
    templates: import('./template').TemplateImportRow[]
    equipment_schemas: { type: string; config: import('./equipment').EquipmentTypeSchema['config'] }[]
    flows: { flow_code: FlowCode; steps: ReviewFlowStep[] }[]
    doc_templates: import('./docTemplate').DocTemplate[]
  }
}

export function listPresetPacks() {
  return request<PresetPackSummary[]>({ url: '/community/preset-packs', method: 'get' })
}

export function getPresetPack(code: string) {
  return request<PresetPack>({ url: `/community/preset-packs/${code}`, method: 'get' })
}

export function listPlatformPresetPacks() {
  return request<PresetPackSummary[]>({ url: '/platform/preset-packs', method: 'get' })
}

export function getPlatformPresetPack(code: string) {
  return request<PresetPack>({ url: `/platform/preset-packs/${code}`, method: 'get' })
}

export function savePlatformPresetPack(code: string, data: { name: string; description: string; payload: PresetPack['payload']; status: string }) {
  return request<null>({ url: `/platform/preset-packs/${code}`, method: 'put', data })
}
