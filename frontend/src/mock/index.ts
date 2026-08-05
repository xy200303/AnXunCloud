// 开发期 Mock：在 axios 适配器层拦截，命中则本地返回，未命中透传真实后端
// 开关：.env.development 中 VITE_USE_MOCK=true（默认开），置 false 则全部走真实接口
import type { AxiosRequestConfig, AxiosResponse } from 'axios'
import {
  menus, flatMenus, allPerms, roles, roleMenus, communities, users, ADMIN_ID,
  dictTypes, dictData, configs, operationLogs, loginLogs
} from './data'

interface MockCtx {
  query: Record<string, any>
  body: any
  params: Record<string, string>
  config: AxiosRequestConfig
}

type Handler = (ctx: MockCtx) => { code?: number; message?: string; data?: any; blob?: Blob; filename?: string }

interface Route {
  method: string
  pattern: RegExp
  handler: Handler
}

const routes: Route[] = []

function on(method: string, pattern: RegExp, handler: Handler) {
  routes.push({ method: method.toUpperCase(), pattern, handler })
}

// 统一分页处理
function paginate(list: any[], query: Record<string, any>) {
  const page = Number(query.page) || 1
  const pageSize = Math.min(Number(query.page_size) || 20, 100)
  const start = (page - 1) * pageSize
  return { list: list.slice(start, start + pageSize), total: list.length, page, page_size: pageSize }
}

function now() {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// 内存态数据（CRUD 直接影响 mock 数据，刷新页面后重置）
const db = {
  users: users.map((u) => ({ ...u })),
  roles: roles.map((r) => ({ ...r })),
  roleMenus: JSON.parse(JSON.stringify(roleMenus)) as Record<string, string[]>,
  menus: JSON.parse(JSON.stringify(menus)),
  dictTypes: dictTypes.map((d) => ({ ...d })),
  dictData: dictData.map((d) => ({ ...d })),
  configs: configs.map((c) => ({ ...c })),
  operationLogs: operationLogs.map((l) => ({ ...l })),
  loginLogs: loginLogs.map((l) => ({ ...l }))
}

// 自增序号 → 假 UUID（与后端 UUIDv7 字符串形态一致）
let seq = 1000
function mockId(): string {
  return `00000000-0000-7000-9fff-${String(++seq).padStart(12, '0')}`
}

// 记录一条操作日志，让日志页"活"起来
function recordOperLog(action: string, path: string, method: string, status = 1) {
  db.operationLogs.unshift({
    id: mockId(), user_id: ADMIN_ID, username: 'admin', module: 'system', action,
    method, path, params: '', ip: '127.0.0.1', status, cost_ms: 12, created_at: now()
  })
}

function roleNamesOf(user: { role_ids: string[] }) {
  return db.roles.filter((r) => user.role_ids.includes(r.id)).map((r) => ({ id: r.id, name: r.name }))
}

function communityNamesOf(user: { community_ids: string[] }) {
  return communities.filter((c) => user.community_ids.includes(c.id)).map((c) => c.name)
}

function userView(u: (typeof db.users)[number]) {
  return { ...u, roles: roleNamesOf(u), community_names: communityNamesOf(u) }
}

// ============ 认证 ============
on('POST', /^\/auth\/login$/, ({ body }) => {
  const { username, password } = body || {}
  const user = db.users.find((u) => u.username === username)
  if (!user || !password) {
    return { code: 40105, message: '用户名或密码错误', data: null }
  }
  if (user.status === 0) {
    return { code: 40104, message: '账号已停用，请联系管理员', data: null }
  }
  db.loginLogs.unshift({
    id: mockId(), user_id: user.id, username: user.username, ip: '127.0.0.1',
    ua: 'Mozilla/5.0 (Mock)', source: '后台', status: 1, msg: '登录成功', created_at: now()
  })
  return {
    code: 0, message: 'success',
    data: {
      token_type: 'Bearer',
      access_token: `mock-access-token-${user.id}-${Date.now()}`,
      refresh_token: `mock-refresh-token-${user.id}`,
      expires_in: 7200
    }
  }
})

on('POST', /^\/auth\/logout$/, () => ({ code: 0, message: 'success', data: null }))

on('GET', /^\/auth\/info$/, () => ({
  code: 0, message: 'success',
  data: {
    id: 1, username: 'admin', name: '系统管理员', phone: '13800000000', avatar: '',
    roles: [{ id: 1, code: 'super_admin', name: '超级管理员' }],
    community_ids: [], data_scope: 'all', perms: allPerms()
  }
}))

// 动态路由：仅下发 dir/menu 类型
function toRouteTree(list: typeof menus): any[] {
  return list
    .filter((m) => m.type !== 'button' && m.status === 1 && m.visible === 1)
    .sort((a, b) => a.sort - b.sort)
    .map((m) => ({
      id: m.id, parent_id: m.parent_id, title: m.title, path: m.path,
      icon: m.icon, type: m.type, sort: m.sort,
      children: m.children?.length ? toRouteTree(m.children) : []
    }))
}

on('GET', /^\/auth\/routes$/, () => ({ code: 0, message: 'success', data: toRouteTree(db.menus) }))

// ============ 工作台（占位数据，统计接口后续接真实实现） ============
on('GET', /^\/dashboard$/, () => ({
  code: 0, message: 'success',
  data: {
    today_completion: { total: 36, done: 30, rate: 83.3 },
    doing_tasks: 4,
    pending_workorders: 3,
    overdue_tasks: 1,
    trend_7d: [],
    community_rank: [],
    latest_workorders: [],
    task_timeline: []
  }
}))

// ============ 用户管理 ============
on('GET', /^\/system\/users$/, ({ query }) => {
  let list = db.users.map(userView)
  if (query.username) {
    const kw = String(query.username)
    list = list.filter((u) => u.username.includes(kw) || u.name.includes(kw))
  }
  if (query.phone) list = list.filter((u) => u.phone.includes(String(query.phone)))
  if (query.role_id) list = list.filter((u) => u.role_ids.includes(String(query.role_id)))
  if (query.community_id) list = list.filter((u) => u.community_ids.includes(String(query.community_id)))
  if (query.status !== undefined && query.status !== '') list = list.filter((u) => u.status === Number(query.status))
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('POST', /^\/system\/users$/, ({ body }) => {
  if (db.users.some((u) => u.username === body.username)) {
    return { code: 41001, message: '用户名已存在', data: null }
  }
  if (db.users.some((u) => u.phone === body.phone)) {
    return { code: 41008, message: '手机号已存在', data: null }
  }
  const id = mockId()
  db.users.push({
    id, username: body.username, name: body.name, phone: body.phone, avatar: '',
    openid: '', role_ids: body.role_ids || [], community_ids: body.community_ids || [],
    status: body.status ?? 1, last_login_at: '', created_at: now()
  })
  recordOperLog('create', '/api/admin/system/users', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('GET', /^\/system\/users\/([^/]+)$/, ({ params }) => {
  const id = params[0]
  const user = db.users.find((x) => x.id === id)
  if (!user) return { code: 40400, message: '用户不存在', data: null }
  return { code: 0, message: 'success', data: { ...userView(user), updated_at: user.created_at } }
})

on('PUT', /^\/system\/users\/([^/]+)$/, ({ params, body }) => {
  const user = db.users.find((x) => x.id === params[0])
  if (!user) return { code: 40400, message: '用户不存在', data: null }
  Object.assign(user, {
    name: body.name ?? user.name,
    phone: body.phone ?? user.phone,
    role_ids: body.role_ids ?? user.role_ids,
    community_ids: body.community_ids ?? user.community_ids,
    status: body.status ?? user.status
  })
  recordOperLog('update', `/api/admin/system/users/${user.id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('PUT', /^\/system\/users\/([^/]+)\/password\/reset$/, ({ params }) => {
  const user = db.users.find((x) => x.id === params[0])
  if (!user) return { code: 40400, message: '用户不存在', data: null }
  recordOperLog('update', `/api/admin/system/users/${user.id}/password/reset`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('PUT', /^\/system\/users\/([^/]+)\/status$/, ({ params, body }) => {
  const id = params[0]
  if (id === ADMIN_ID) return { code: 41006, message: '不能停用当前登录账号', data: null }
  const user = db.users.find((x) => x.id === id)
  if (!user) return { code: 40400, message: '用户不存在', data: null }
  user.status = Number(body.status)
  recordOperLog('update', `/api/admin/system/users/${id}/status`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/users\/([^/]+)$/, ({ params }) => {
  const id = params[0]
  if (id === ADMIN_ID) return { code: 41006, message: '不能删除当前登录账号', data: null }
  const idx = db.users.findIndex((x) => x.id === id)
  if (idx < 0) return { code: 40400, message: '用户不存在', data: null }
  db.users.splice(idx, 1)
  recordOperLog('delete', `/api/admin/system/users/${id}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

// 导入模板下载（mock 返回一个伪 xlsx 文件流）
on('GET', /^\/system\/users\/import-template$/, () => ({
  code: 0,
  blob: new Blob(
    ['\ufeff姓名,手机号,角色,所属小区,初始密码,状态,备注\n张三,13900001111,巡检员,翡翠湾小区,,启用,示例行（导入时跳过）\n'],
    { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }
  ),
  filename: 'user_import_template.xlsx'
}))

// 批量导入：mock 逐行校验过程，返回成功/失败明细
on('POST', /^\/system\/users\/import$/, ({ body }) => {
  const file: File | undefined = body instanceof FormData ? (body.get('file') as File) : undefined
  if (!file) return { code: 40001, message: '请上传文件', data: null }
  if (!file.name.endsWith('.xlsx')) {
    return { code: 41011, message: '导入文件格式错误，仅支持 .xlsx', data: null }
  }
  recordOperLog('import', '/api/admin/system/users/import', 'POST')
  return {
    code: 0,
    message: '导入完成：成功 47 条，失败 3 条',
    data: {
      total: 50,
      success_count: 47,
      fail_count: 3,
      fail_details: [
        { row: 5, phone: '13800001111', reason: '手机号已存在' },
        { row: 12, phone: '137', reason: '手机号格式错误' },
        { row: 30, phone: '13922223333', reason: '角色「保安」不存在' }
      ]
    }
  }
})

// 用户导出（mock 返回伪 xlsx 文件流）
on('GET', /^\/system\/users\/export$/, () => {
  const header = '用户名,姓名,手机号,角色,所属小区,状态,最后登录时间,创建时间\n'
  const rows = db.users
    .map((u) => [u.username, u.name, u.phone, roleNamesOf(u).map((r) => r.name).join('|'), communityNamesOf(u).join('|'), u.status === 1 ? '启用' : '停用', u.last_login_at, u.created_at].join(','))
    .join('\n')
  recordOperLog('export', '/api/admin/system/users/export', 'GET')
  return {
    code: 0,
    blob: new Blob(['\ufeff' + header + rows], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    filename: `users_${now().slice(0, 10).replace(/-/g, '')}.xlsx`
  }
})

// ============ 角色管理 ============
on('GET', /^\/system\/roles$/, ({ query }) => {
  let list = db.roles.map((r) => ({ ...r }))
  if (query.name) list = list.filter((r) => r.name.includes(String(query.name)))
  if (query.status !== undefined && query.status !== '') list = list.filter((r) => r.status === Number(query.status))
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('POST', /^\/system\/roles$/, ({ body }) => {
  if (db.roles.some((r) => r.code === body.code)) {
    return { code: 41002, message: '角色编码已存在', data: null }
  }
  const id = mockId()
  db.roles.push({
    id, code: body.code, name: body.name, data_scope: body.data_scope || 'all',
    remark: body.remark || '', status: body.status ?? 1, user_count: 0, created_at: now(), builtin: false
  })
  db.roleMenus[id] = body.menu_ids || []
  recordOperLog('create', '/api/admin/system/roles', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('GET', /^\/system\/roles\/([^/]+)$/, ({ params }) => {
  const role = db.roles.find((r) => r.id === params[0])
  if (!role) return { code: 40400, message: '角色不存在', data: null }
  return { code: 0, message: 'success', data: { ...role, menu_ids: db.roleMenus[role.id] || [] } }
})

on('PUT', /^\/system\/roles\/([^/]+)\/menus$/, ({ params, body }) => {
  const role = db.roles.find((r) => r.id === params[0])
  if (!role) return { code: 40400, message: '角色不存在', data: null }
  if (role.code === 'super_admin') return { code: 41007, message: '内置角色不可修改', data: null }
  db.roleMenus[role.id] = body.menu_ids || []
  role.data_scope = body.data_scope || role.data_scope
  recordOperLog('update', `/api/admin/system/roles/${role.id}/menus`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('PUT', /^\/system\/roles\/([^/]+)$/, ({ params, body }) => {
  const role = db.roles.find((r) => r.id === params[0])
  if (!role) return { code: 40400, message: '角色不存在', data: null }
  if (role.code === 'super_admin') return { code: 41007, message: '内置角色不可修改', data: null }
  Object.assign(role, {
    name: body.name ?? role.name,
    data_scope: body.data_scope ?? role.data_scope,
    remark: body.remark ?? role.remark,
    status: body.status ?? role.status
  })
  if (body.menu_ids) db.roleMenus[role.id] = body.menu_ids
  recordOperLog('update', `/api/admin/system/roles/${role.id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/roles\/([^/]+)$/, ({ params }) => {
  const id = params[0]
  // 内置超管角色按编码判断，不可删除
  if (db.roles.find((r) => r.id === id)?.code === 'super_admin') return { code: 41007, message: '内置角色不可删除', data: null }
  const role = db.roles.find((r) => r.id === id)
  if (!role) return { code: 40400, message: '角色不存在', data: null }
  if (role.user_count > 0) return { code: 41003, message: '角色下存在用户，不可删除', data: null }
  db.roles.splice(db.roles.indexOf(role), 1)
  delete db.roleMenus[id]
  recordOperLog('delete', `/api/admin/system/roles/${id}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

// ============ 菜单管理 ============
on('GET', /^\/system\/menus$/, ({ query }) => {
  // title 过滤：保留命中节点及其祖先
  if (!query.title) return { code: 0, message: 'success', data: db.menus }
  const kw = String(query.title)
  const flat = flatMenus(db.menus)
  const hitIds = new Set<string>()
  for (const m of flat) {
    if (m.title.includes(kw)) {
      let cur: (typeof flat)[number] | undefined = m
      while (cur) {
        hitIds.add(cur.id)
        cur = flat.find((x) => x.id === cur!.parent_id)
      }
    }
  }
  const prune = (list: typeof menus): any[] =>
    list.filter((m) => hitIds.has(m.id)).map((m) => ({ ...m, children: prune(m.children || []) }))
  return { code: 0, message: 'success', data: prune(db.menus) }
})

function findMenu(list: typeof menus, id: string): (typeof menus)[number] | null {
  for (const m of list) {
    if (m.id === id) return m
    const hit = m.children?.length ? findMenu(m.children, id) : null
    if (hit) return hit
  }
  return null
}

function removeMenu(list: typeof menus, id: string): boolean {
  const idx = list.findIndex((m) => m.id === id)
  if (idx >= 0) {
    list.splice(idx, 1)
    return true
  }
  return list.some((m) => m.children?.length && removeMenu(m.children, id))
}

on('POST', /^\/system\/menus$/, ({ body }) => {
  const id = mockId()
  const node = {
    id, parent_id: body.parent_id || '', title: body.title, path: body.path || '',
    icon: body.icon || '', type: body.type, perms: body.perms || '',
    sort: body.sort ?? 0, visible: body.visible ?? 1, status: body.status ?? 1, children: []
  }
  if (!node.parent_id) {
    db.menus.push(node)
  } else {
    const parent = findMenu(db.menus, node.parent_id)
    if (!parent) return { code: 40001, message: '上级菜单不存在', data: null }
    parent.children = parent.children || []
    parent.children.push(node)
  }
  recordOperLog('create', '/api/admin/system/menus', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('GET', /^\/system\/menus\/([^/]+)$/, ({ params }) => {
  const node = findMenu(db.menus, params[0])
  if (!node) return { code: 40400, message: '菜单不存在', data: null }
  const { children, ...rest } = node
  return { code: 0, message: 'success', data: rest }
})

on('PUT', /^\/system\/menus\/([^/]+)$/, ({ params, body }) => {
  const id = params[0]
  const node = findMenu(db.menus, id)
  if (!node) return { code: 40400, message: '菜单不存在', data: null }
  if (body.parent_id !== undefined && body.parent_id !== node.parent_id) {
    if (body.parent_id === id) return { code: 40001, message: '不能挂到自身节点下', data: null }
    removeMenu(db.menus, id)
    node.parent_id = body.parent_id
    if (!body.parent_id) {
      db.menus.push(node)
    } else {
      const parent = findMenu(db.menus, body.parent_id)
      if (!parent) return { code: 40001, message: '上级菜单不存在', data: null }
      parent.children = parent.children || []
      parent.children.push(node)
    }
  }
  Object.assign(node, {
    title: body.title ?? node.title, path: body.path ?? node.path,
    icon: body.icon ?? node.icon, type: body.type ?? node.type,
    perms: body.perms ?? node.perms, sort: body.sort ?? node.sort,
    visible: body.visible ?? node.visible, status: body.status ?? node.status
  })
  recordOperLog('update', `/api/admin/system/menus/${id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/menus\/([^/]+)$/, ({ params }) => {
  const id = params[0]
  const node = findMenu(db.menus, id)
  if (!node) return { code: 40400, message: '菜单不存在', data: null }
  if (node.children?.length) return { code: 40001, message: '存在子菜单，不可删除', data: null }
  removeMenu(db.menus, id)
  recordOperLog('delete', `/api/admin/system/menus/${id}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

// ============ 字典管理 ============
function withDataCount(t: (typeof db.dictTypes)[number]) {
  return { ...t, data_count: db.dictData.filter((d) => d.type_code === t.code).length }
}

on('GET', /^\/system\/dict-types$/, ({ query }) => {
  let list = db.dictTypes.map(withDataCount)
  if (query.code) list = list.filter((t) => t.code.includes(String(query.code)))
  if (query.name) list = list.filter((t) => t.name.includes(String(query.name)))
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('POST', /^\/system\/dict-types$/, ({ body }) => {
  if (db.dictTypes.some((t) => t.code === body.code)) {
    return { code: 41004, message: '字典编码已存在', data: null }
  }
  const id = mockId()
  db.dictTypes.push({ id, code: body.code, name: body.name, builtin: false, created_at: now() })
  recordOperLog('create', '/api/admin/system/dict-types', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('PUT', /^\/system\/dict-types\/([^/]+)$/, ({ params, body }) => {
  const t = db.dictTypes.find((x) => x.id === params[0])
  if (!t) return { code: 40400, message: '字典类型不存在', data: null }
  t.name = body.name ?? t.name
  recordOperLog('update', `/api/admin/system/dict-types/${t.id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/dict-types\/([^/]+)$/, ({ params }) => {
  const id = params[0]
  const idx = db.dictTypes.findIndex((x) => x.id === id)
  if (idx < 0) return { code: 40400, message: '字典类型不存在', data: null }
  const code = db.dictTypes[idx].code
  db.dictTypes.splice(idx, 1)
  db.dictData = db.dictData.filter((d) => d.type_code !== code)
  recordOperLog('delete', `/api/admin/system/dict-types/${id}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

on('GET', /^\/system\/dict-data$/, ({ query }) => {
  let list = db.dictData.filter((d) => d.type_code === query.type_code)
  if (query.label) list = list.filter((d) => d.label.includes(String(query.label)))
  if (query.status !== undefined && query.status !== '') list = list.filter((d) => d.status === Number(query.status))
  list = [...list].sort((a, b) => a.sort - b.sort)
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('POST', /^\/system\/dict-data$/, ({ body }) => {
  if (db.dictData.some((d) => d.type_code === body.type_code && d.value === body.value)) {
    return { code: 40001, message: '同类型下存储值已存在', data: null }
  }
  const id = mockId()
  db.dictData.push({
    id, type_code: body.type_code, label: body.label, value: body.value,
    sort: body.sort ?? 0, status: body.status ?? 1
  })
  recordOperLog('create', '/api/admin/system/dict-data', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('PUT', /^\/system\/dict-data\/([^/]+)$/, ({ params, body }) => {
  const d = db.dictData.find((x) => x.id === params[0])
  if (!d) return { code: 40400, message: '字典数据不存在', data: null }
  Object.assign(d, {
    label: body.label ?? d.label, value: body.value ?? d.value,
    sort: body.sort ?? d.sort, status: body.status ?? d.status
  })
  recordOperLog('update', `/api/admin/system/dict-data/${d.id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/dict-data\/([^/]+)$/, ({ params }) => {
  const idx = db.dictData.findIndex((x) => x.id === params[0])
  if (idx < 0) return { code: 40400, message: '字典数据不存在', data: null }
  db.dictData.splice(idx, 1)
  recordOperLog('delete', `/api/admin/system/dict-data/${params[0]}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

// ============ 参数配置 ============
on('GET', /^\/system\/configs$/, ({ query }) => {
  let list = db.configs.map((c) => ({ ...c }))
  if (query.key) list = list.filter((c) => c.key.includes(String(query.key)))
  if (query.name) list = list.filter((c) => c.name.includes(String(query.name)))
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('POST', /^\/system\/configs$/, ({ body }) => {
  if (db.configs.some((c) => c.key === body.key)) {
    return { code: 41005, message: '参数 key 已存在', data: null }
  }
  const id = mockId()
  db.configs.push({ id, key: body.key, name: body.name, value: body.value, remark: body.remark || '', builtin: false, updated_at: now() })
  recordOperLog('create', '/api/admin/system/configs', 'POST')
  return { code: 0, message: 'success', data: { id } }
})

on('PUT', /^\/system\/configs\/([^/]+)$/, ({ params, body }) => {
  const c = db.configs.find((x) => x.id === params[0])
  if (!c) return { code: 40400, message: '参数不存在', data: null }
  Object.assign(c, {
    name: body.name ?? c.name, value: body.value ?? c.value,
    remark: body.remark ?? c.remark, updated_at: now()
  })
  recordOperLog('update', `/api/admin/system/configs/${c.id}`, 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('DELETE', /^\/system\/configs\/([^/]+)$/, ({ params }) => {
  const idx = db.configs.findIndex((x) => x.id === params[0])
  if (idx < 0) return { code: 40400, message: '参数不存在', data: null }
  if (db.configs[idx].builtin) return { code: 41007, message: '内置参数不可删除', data: null }
  db.configs.splice(idx, 1)
  recordOperLog('delete', `/api/admin/system/configs/${params[0]}`, 'DELETE')
  return { code: 0, message: 'success', data: null }
})

// ============ 日志（只读） ============
on('GET', /^\/system\/logs\/operations$/, ({ query }) => {
  let list = db.operationLogs.map((l) => ({ ...l }))
  if (query.username) list = list.filter((l) => l.username.includes(String(query.username)))
  if (query.module) list = list.filter((l) => l.module === query.module)
  if (query.action) list = list.filter((l) => l.action === query.action)
  if (query.status !== undefined && query.status !== '') list = list.filter((l) => l.status === Number(query.status))
  if (query.start_time) list = list.filter((l) => l.created_at >= query.start_time)
  if (query.end_time) list = list.filter((l) => l.created_at <= query.end_time)
  return { code: 0, message: 'success', data: paginate(list, query) }
})

on('GET', /^\/system\/logs\/logins$/, ({ query }) => {
  let list = db.loginLogs.map((l) => ({ ...l }))
  if (query.username) list = list.filter((l) => l.username.includes(String(query.username)))
  if (query.ip) list = list.filter((l) => l.ip.includes(String(query.ip)))
  if (query.status !== undefined && query.status !== '') list = list.filter((l) => l.status === Number(query.status))
  if (query.start_time) list = list.filter((l) => l.created_at >= query.start_time)
  if (query.end_time) list = list.filter((l) => l.created_at <= query.end_time)
  return { code: 0, message: 'success', data: paginate(list, query) }
})

// ============ 统计导出（日志导出共用入口，接口文档 §2.9 指向 stats/export） ============
on('GET', /^\/stats\/export$/, ({ query }) => {
  recordOperLog('export', `/api/admin/stats/export?type=${query.type || ''}`, 'GET')
  return {
    code: 0,
    blob: new Blob(['﻿mock 导出文件（后端接入后返回真实 Excel）\n'], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
    }),
    filename: `${query.type || 'export'}_${now().slice(0, 10).replace(/-/g, '')}.xlsx`
  }
})

// ============ 个人中心（接口文档补充：system 模块子动作） ============
on('PUT', /^\/system\/users\/profile$/, () => {
  recordOperLog('update', '/api/admin/system/users/profile', 'PUT')
  return { code: 0, message: 'success', data: null }
})

on('PUT', /^\/system\/users\/password$/, ({ body }) => {
  if (!body?.old_password || !body?.new_password) {
    return { code: 40001, message: '请填写完整密码信息', data: null }
  }
  recordOperLog('update', '/api/admin/system/users/password', 'PUT')
  return { code: 0, message: 'success', data: null }
})

// ============ 小区（下拉候选，仅需登录） ============
on('GET', /^\/communities$/, ({ query }) => {
  let list = communities.map((c) => ({ ...c }))
  if (query.name) list = list.filter((c) => c.name.includes(String(query.name)))
  if (query.status !== undefined && query.status !== '') list = list.filter((c) => c.status === Number(query.status))
  return { code: 0, message: 'success', data: paginate(list, query) }
})

// 未命中返回 null，由适配器决定是否透传真实后端
function match(method: string, url: string) {
  const path = url.split('?')[0]
  for (const r of routes) {
    if (r.method !== method) continue
    const m = path.match(r.pattern)
    if (m) {
      const params: Record<string, string> = {}
      m.slice(1).forEach((v, i) => (params[String(i)] = v))
      return { handler: r.handler, params }
    }
  }
  return null
}

// Mock 适配器：返回 null 表示未命中
export async function mockAdapter(config: AxiosRequestConfig): Promise<AxiosResponse | null> {
  const hit = match((config.method || 'get').toUpperCase(), config.url || '')
  if (!hit) return null

  // 模拟网络延迟
  await new Promise((r) => setTimeout(r, 150 + Math.random() * 250))

  let body = config.data
  if (typeof body === 'string' && body) {
    try { body = JSON.parse(body) } catch { /* 保持原样 */ }
  }

  const result = hit.handler({
    query: (config.params as Record<string, any>) || {},
    body,
    params: hit.params,
    config
  })

  const headers: Record<string, string> = {}
  if (result.blob) {
    headers['content-type'] = result.blob.type
    if (result.filename) {
      headers['content-disposition'] = `attachment; filename="${result.filename}"`
    }
  }

  const response: AxiosResponse = {
    data: result.blob ?? { code: result.code ?? 0, message: result.message ?? 'success', data: result.data ?? null },
    status: result.code && result.code !== 0 ? 400 : 200,
    statusText: 'OK',
    headers,
    config: config as any
  }
  return response
}
