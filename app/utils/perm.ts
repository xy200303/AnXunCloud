/**
 * 权限点判断纯逻辑（单一来源）：perms 数组命中任一 + super_admin 角色兜底。
 * services/api 的 hasPerm 与 stores/auth 的 hasPerm getter 均转发到这里，避免双实现漂移。
 * 仅类型依赖 UserInfo（import type，无运行时循环）。
 */

import type { UserInfo } from '@/services/modules/auth'

/** 传入单个 code 或数组，任一命中即 true；userInfo 为空返回 false。旧缓存用户缺 perms 字段时容错。 */
export function checkPerm(user: UserInfo | null, codes: string | string[]): boolean {
  if (user == null) return false
  if ((user.roles ?? []).indexOf('super_admin') >= 0) return true
  const list = typeof codes == 'string' ? [codes] : codes
  const perms = user.perms ?? []
  for (let i = 0; i < list.length; i++) {
    if (perms.indexOf(list[i]) >= 0) return true
  }
  return false
}
