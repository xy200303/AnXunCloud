// 接口类型定义（与接口文档字段一致，蛇形命名）
// 后端已迁移 UUIDv7 主键：凡实体 ID 字段一律 string，根节点 parent_id 为空串 ''

export interface LoginParams {
  username: string
  password: string
  // 公司代码（P3 多租户：用户名命中多个租户时必填，用于消歧）
  tenant_code?: string
}

export interface LoginResult {
  token_type: string
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface RoleBrief {
  id: string
  code?: string
  name: string
}

export type DataScope = 'all' | 'project' | 'self'

export interface UserInfo {
  id: string
  username: string
  name: string
  phone: string
  avatar: string
  roles: RoleBrief[]
  data_scope: DataScope
  perms: string[]
  // 个人中心扩展字段（后端并行开发中，可能缺省）
  signature_url?: string // 手写签名图 URL（月报签字栏用，可空）
  created_at?: string
  last_login_at?: string
  last_login_ip?: string
  openid?: string
  is_builtin?: boolean
}

// 动态路由菜单节点（dir/menu）
export interface RouteMenu {
  id: string
  parent_id: string
  title: string
  path: string
  icon: string
  type: 'dir' | 'menu'
  sort: number
  children: RouteMenu[]
}

// 完整菜单节点（含按钮）
export interface MenuNode {
  id: string
  parent_id: string
  title: string
  path: string
  icon: string
  type: 'dir' | 'menu' | 'button'
  perms: string
  sort: number
  visible: number
  status: number
  children?: MenuNode[]
}

export interface UserItem {
  id: string
  username: string
  name: string
  phone: string
  avatar: string
  openid: string
  roles?: RoleBrief[]
  role_ids?: string[]
  status: number
  is_builtin?: boolean
  last_login_at: string
  created_at: string
}

export interface UserForm {
  username: string
  password?: string
  name: string
  phone: string
  role_ids: string[]
  status: number
}

export interface RoleItem {
  id: string
  code: string
  name: string
  data_scope: DataScope
  remark: string
  status: number
  user_count: number
  builtin?: boolean
  created_at: string
}

export interface RoleDetail extends RoleItem {
  menu_ids: string[]
}

export interface DictType {
  id: string
  code: string
  name: string
  builtin?: boolean
  data_count?: number
  created_at: string
}

export interface DictData {
  id: string
  type_code: string
  label: string
  value: string
  sort: number
  status: number
}

export interface ConfigItem {
  id: string
  key: string
  name: string
  value: string
  remark: string
  config_group: string
  builtin?: boolean
  updated_at: string
}

export interface OperationLog {
  id: string
  user_id: string | null
  username: string
  module: string
  action: string
  // 中文动作名（后端新增；未上线时缺省，前端回退显示 action 原值）
  action_name?: string
  method: string
  path: string
  params: string
  ip: string
  status: number
  cost_ms: number
  created_at: string
}

export interface LoginLog {
  id: string
  user_id: string
  username: string
  ip: string
  ua: string
  // 后端实际字段为 channel（admin 后台 / mp 小程序）
  channel?: string
  source?: string
  status: number
  msg: string
  created_at: string
}

export interface Community {
  id: string
  name: string
  status: number
}

export interface ImportResult {
  total: number
  success_count: number
  fail_count: number
  fail_details: { row: number; phone: string; reason: string }[]
}

export interface DashboardData {
  today_completion: { total: number; done: number; rate: number }
  doing_tasks: number
  pending_workorders: number
  overdue_tasks: number
  trend_7d: { date: string; total: number; done: number; rate: number }[]
  community_rank: { community_id: string; community_name: string; total: number; done: number; rate: number }[]
  latest_workorders: {
    id: string
    order_no: string
    title: string
    community_name: string
    priority: string
    status: string
    created_at: string
  }[]
  task_timeline: { time: string; inspector_name: string; task_id: string; task_name: string; action: string }[]
}
