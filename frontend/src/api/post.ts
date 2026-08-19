// 岗位管理 / 岗位模板库接口（《管理后台信息架构与菜单归位方案》第三章）
// 两组接口完全同构：岗位管理作用于当前租户上下文，岗位模板库作用于平台模板行（tenant_id 为空，仅超管）
import { request } from '@/utils/request'

// 业务线五档（与后端 model.PostLineNames 一致）
export type PostLine = 'safety' | 'engineering' | 'environment' | 'service' | 'general'

export const POST_LINES: { value: PostLine; label: string }[] = [
  { value: 'safety', label: '安全' },
  { value: 'engineering', label: '工程' },
  { value: 'environment', label: '环境' },
  { value: 'service', label: '客服' },
  { value: 'general', label: '综合' }
]

export function postLineName(line: string) {
  return POST_LINES.find((l) => l.value === line)?.label || line || '--'
}

// 岗位列表项（GET /system/posts、/system/post-templates）
export interface PostItem {
  id: string
  code: string
  name: string
  line: string
  line_name: string
  is_supervisor: boolean
  role_id: string
  role_name: string
  sort: number
  status: number // 1 启用 / 0 停用
  remark: string
  created_at: string
}

// 岗位新增/修改保存体（code 创建必填、更新不可改；role_id 空 = 不绑角色）
export interface PostSaveReq {
  code?: string
  name: string
  line: PostLine | ''
  role_id?: string
  is_supervisor: boolean
  sort: number
  status?: number
  remark?: string
}

// 职责槽位默认绑定视图项（租户级：source 为 tenant/platform；平台模板库：恒 platform）
export interface PostDutyBindingView {
  slot: string
  name: string
  post_codes: string[]
  post_names: string[]
  source: 'tenant' | 'platform'
}

// ===== 岗位管理（系统管理 /system/posts，租户上下文由拦截器统一携带） =====

export function listPosts() {
  return request<PostItem[]>({ url: '/system/posts', method: 'get' })
}

export function createPost(data: PostSaveReq) {
  return request<{ id: string }>({ url: '/system/posts', method: 'post', data })
}

export function updatePost(id: string, data: PostSaveReq) {
  return request<null>({ url: `/system/posts/${id}`, method: 'put', data })
}

export function deletePost(id: string) {
  return request<null>({ url: `/system/posts/${id}`, method: 'delete' })
}

export function listPostDutyBindings() {
  return request<PostDutyBindingView[]>({ url: '/system/posts/duty-bindings', method: 'get' })
}

// 租户级槽位默认绑定整体保存（post_codes 空数组 = 该环节跳过）
export function savePostDutyBindings(bindings: { slot: string; post_codes: string[] }[]) {
  return request<null>({ url: '/system/posts/duty-bindings', method: 'put', data: { bindings } })
}

// ===== 打卡审核链（扩展方案 §3；steps 有序环节，环节引用职责槽位） =====

export interface ReviewFlowStep {
  slot: string
  name: string
}
export interface ReviewFlowView {
  flow_code: string
  steps: ReviewFlowStep[]
  source: 'project' | 'tenant' | 'platform' | 'default'
}

export function getReviewFlow() {
  return request<ReviewFlowView>({ url: '/system/posts/review-flow', method: 'get' })
}

export function saveReviewFlow(steps: ReviewFlowStep[]) {
  return request<null>({ url: '/system/posts/review-flow', method: 'put', data: { steps } })
}

// ===== 岗位模板库（平台管理 /system/post-templates，仅超管；开通租户时的初始拷贝源） =====

export function listPostTemplates() {
  return request<PostItem[]>({ url: '/system/post-templates', method: 'get' })
}

export function createPostTemplate(data: PostSaveReq) {
  return request<{ id: string }>({ url: '/system/post-templates', method: 'post', data })
}

export function updatePostTemplate(id: string, data: PostSaveReq) {
  return request<null>({ url: `/system/post-templates/${id}`, method: 'put', data })
}

export function deletePostTemplate(id: string) {
  return request<null>({ url: `/system/post-templates/${id}`, method: 'delete' })
}

export function listPostTemplateDutyBindings() {
  return request<PostDutyBindingView[]>({ url: '/system/post-templates/duty-bindings', method: 'get' })
}

// 平台级槽位默认绑定整体保存
export function savePostTemplateDutyBindings(bindings: { slot: string; post_codes: string[] }[]) {
  return request<null>({ url: '/system/post-templates/duty-bindings', method: 'put', data: { bindings } })
}

// 平台默认打卡审核链
export function getPostTemplateReviewFlow() {
  return request<ReviewFlowView>({ url: '/system/post-templates/review-flow', method: 'get' })
}

export function savePostTemplateReviewFlow(steps: ReviewFlowStep[]) {
  return request<null>({ url: '/system/post-templates/review-flow', method: 'put', data: { steps } })
}
