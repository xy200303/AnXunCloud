/**
 * 认证与个人资料域：登录/注册/登出、个人信息、密码修改。
 * 类型与后端 JSON 蛇形字段对齐；ID 字段全系统 v2 起为 UUIDv7 string。
 */

import { httpGet, httpPost, httpPut } from '@/services/request'
import { toId } from './common'
import { checkPerm } from '@/utils/perm'

/** 后端 /profile roles 元素。 */
export type RoleEntry = { id: string; code: string; name: string }

export type UserInfo = {
  id: string
  username: string
  name: string
  phone: string
  avatar: string
  /** 手写签名图 URL（月报签字用，空 = 未配置，签字时弹签名板现场手写） */
  signature_url: string
  /** 权限点集合（report:sign:* 等；超管以 roles 含 super_admin 兜底） */
  perms: string[]
  /** 角色 code 数组；业务身份以 staffs.post_names 岗位为准。 */
  roles: string[]
  /** 我的在职项目（project_staff 推导；name 可空，前端自行解析） */
  projects: Array<{ id: string; name?: string }>
  /** 所属公司（租户名） */
  tenant_name?: string
  /** 在职编制明细：小区 + 岗位名列表 */
  staffs?: Array<{ community_id: string; community_name: string; post_names: string[] }>
}

export type LoginResult = {
  access_token: string
  refresh_token: string
  expires_in: number
  /** APP 账号密码登录是否回包 user 以后端为准，可能为空（为空则登录后调 fetchProfile） */
  user: UserInfo | null
}

type RawUser = {
  id?: string | number
  username?: string
  name?: string
  phone?: string
  avatar?: string
  signature_url?: string
  perms?: string[]
  roles?: RoleEntry[]
  projects?: Array<{ id?: string | number; name?: string }>
  tenant_name?: string
  staffs?: Array<{ community_id?: string | number; community_name?: string; post_names?: string[] }>
}

type RawLoginData = {
  access_token?: string
  refresh_token?: string
  expires_in?: number
  user?: RawUser | null
}

export function toUserInfo(raw: RawUser): UserInfo {
  return {
    id: toId(raw.id),
    username: raw.username ?? '',
    name: raw.name ?? '',
    phone: raw.phone ?? '',
    avatar: raw.avatar ?? '',
    signature_url: raw.signature_url ?? '',
    perms: (raw.perms ?? []).map((p) => String(p)),
    roles: (raw.roles ?? []).map((r) => r.code).filter((code) => code != ''),
    projects: (raw.projects ?? []).map((p) => ({ id: toId(p.id), name: p.name })),
    tenant_name: raw.tenant_name ?? '',
    staffs: (raw.staffs ?? []).map((s) => ({
      community_id: toId(s.community_id),
      community_name: s.community_name ?? '',
      post_names: (s.post_names ?? []).map((n) => String(n))
    }))
  }
}

/**
 * 权限点判断（超管以 super_admin 角色兜底，与后端通配策略同口径）。
 * 纯逻辑已收敛到 utils/perm.checkPerm，本函数仅为兼容旧签名（单个 code）的转发。
 */
export function hasPerm(u: UserInfo | null, perm: string): boolean {
  return checkPerm(u, perm)
}

/** 登录 POST /login {username, password, tenant_code?}（用户名跨租户重名时传所选公司 code 消歧） */
export function apiLogin(username: string, password: string, tenantCode?: string): Promise<LoginResult> {
  return new Promise<LoginResult>((resolve, reject) => {
    httpPost<RawLoginData>('/login', { username: username, password: password, tenant_code: tenantCode || undefined }, false)
      .then((d) => {
        if (d == null) {
          reject(new Error('登录响应异常'))
          return
        }
        resolve({
          access_token: d.access_token ?? '',
          refresh_token: d.refresh_token ?? '',
          expires_in: d.expires_in ?? 0,
          user: d.user != null ? toUserInfo(d.user) : null
        })
      })
      .catch(reject)
  })
}


/** 注册开关 GET /auth/register-config → data.enabled */
export function apiRegisterConfig(): Promise<boolean> {
  return new Promise<boolean>((resolve, reject) => {
    httpGet<{ enabled?: boolean }>('/auth/register-config', false)
      .then((d) => {
        resolve(d?.enabled ?? false)
      })
      .catch(reject)
  })
}

/** 注册可选公司列表 GET /auth/register-tenants → [{code, name}]（免登录，注册开关关闭时 40303） */
export function apiRegisterTenants(): Promise<Array<{ code: string; name: string }>> {
  return new Promise((resolve, reject) => {
    httpGet<Array<{ code?: string; name?: string }>>('/auth/register-tenants', false)
      .then((d) => {
        resolve((d ?? []).map((t) => ({ code: t.code ?? '', name: t.name ?? '' })))
      })
      .catch(reject)
  })
}

/** 注册 POST /auth/register {username,password,name,phone,tenant_code?}（tenant_code 选填：多租户时传所选公司 code，不传 = 默认租户） */
export function apiRegister(username: string, password: string, name: string, phone: string, tenantCode?: string): Promise<null> {
  return httpPost<null>('/auth/register', {
    username: username,
    password: password,
    name: name,
    phone: phone,
    tenant_code: tenantCode || undefined
  }, false)
}

/** 登出 POST /auth/logout */
export function apiLogout(): Promise<null> {
  return httpPost<null>('/auth/logout', null, true)
}

/** 个人信息 GET /profile */
export function apiProfile(): Promise<UserInfo> {
  return new Promise<UserInfo>((resolve, reject) => {
    httpGet<RawUser>('/profile')
      .then((d) => {
        if (d == null) {
          reject(new Error('个人信息响应异常'))
        } else {
          resolve(toUserInfo(d))
        }
      })
      .catch(reject)
  })
}

/**
 * 修改本人资料 PUT /profile（对齐 userCtl.UpdateProfile：name/phone 必传）；
 * signatureFileID 不传 = 不改动签名；传入则创建/替换当前用户 active 签名资产。
 * avatarFileID 不传 = 不改动头像；传入则更新头像。
 */
export function apiUpdateProfile(name: string, phone: string, signatureFileID?: string, avatarFileID?: string): Promise<UserInfo> {
  const body: Record<string, any> = { name: name, phone: phone }
  if (signatureFileID != null) body.signature_file_id = signatureFileID
  if (avatarFileID != null) body.avatar = avatarFileID
  return new Promise<UserInfo>((resolve, reject) => {
    httpPut<RawUser>('/profile', body, true)
      .then((d) => {
        if (d == null) {
          reject(new Error('资料更新响应异常'))
        } else {
          resolve(toUserInfo(d))
        }
      })
      .catch(reject)
  })
}

/** 修改本人密码 PUT /password {old_password,new_password}（8–32 位含字母数字，新旧不可相同） */
export function apiChangePassword(oldPwd: string, newPwd: string): Promise<null> {
  return httpPut<null>('/password', { old_password: oldPwd, new_password: newPwd }, true)
}
