// Mock 种子数据：后端并行开发期间，保证系统管理全流程可点通
// 字段命名与接口文档一致（蛇形）

// 生成确定性假 UUID（mock 用），与后端 UUIDv7 字符串形态一致
export const mid = (n: number) => `00000000-0000-7000-8000-${String(n).padStart(12, '0')}`
// 超管用户 id（mock 内引用）
export const ADMIN_ID = mid(1)

export interface MockMenu {
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
  children?: MockMenu[]
}

// 菜单树（目录/菜单/按钮三级，权限点命名与接口文档一致）
export const menus: MockMenu[] = [
  {
    id: mid(1), parent_id: mid(0), title: '工作台', path: '/dashboard', icon: 'Odometer',
    type: 'menu', perms: '', sort: 1, visible: 1, status: 1, children: []
  },
  {
    id: mid(10), parent_id: mid(0), title: '系统管理', path: '/system', icon: 'Setting',
    type: 'dir', perms: '', sort: 90, visible: 1, status: 1,
    children: [
      {
        id: mid(11), parent_id: mid(10), title: '用户管理', path: '/system/user', icon: 'User',
        type: 'menu', perms: 'system:user:list', sort: 1, visible: 1, status: 1,
        children: [
          { id: mid(1101), parent_id: mid(11), title: '新增用户', path: '', icon: '', type: 'button', perms: 'system:user:create', sort: 1, visible: 1, status: 1 },
          { id: mid(1102), parent_id: mid(11), title: '编辑用户', path: '', icon: '', type: 'button', perms: 'system:user:update', sort: 2, visible: 1, status: 1 },
          { id: mid(1103), parent_id: mid(11), title: '删除用户', path: '', icon: '', type: 'button', perms: 'system:user:delete', sort: 3, visible: 1, status: 1 },
          { id: mid(1104), parent_id: mid(11), title: '重置密码', path: '', icon: '', type: 'button', perms: 'system:user:reset-password', sort: 4, visible: 1, status: 1 },
          { id: mid(1105), parent_id: mid(11), title: '导入用户', path: '', icon: '', type: 'button', perms: 'system:user:import', sort: 5, visible: 1, status: 1 },
          { id: mid(1106), parent_id: mid(11), title: '导出用户', path: '', icon: '', type: 'button', perms: 'system:user:export', sort: 6, visible: 1, status: 1 }
        ]
      },
      {
        id: mid(12), parent_id: mid(10), title: '角色管理', path: '/system/role', icon: 'Avatar',
        type: 'menu', perms: 'system:role:list', sort: 2, visible: 1, status: 1,
        children: [
          { id: mid(1201), parent_id: mid(12), title: '新增角色', path: '', icon: '', type: 'button', perms: 'system:role:create', sort: 1, visible: 1, status: 1 },
          { id: mid(1202), parent_id: mid(12), title: '编辑角色', path: '', icon: '', type: 'button', perms: 'system:role:update', sort: 2, visible: 1, status: 1 },
          { id: mid(1203), parent_id: mid(12), title: '删除角色', path: '', icon: '', type: 'button', perms: 'system:role:delete', sort: 3, visible: 1, status: 1 }
        ]
      },
      {
        id: mid(13), parent_id: mid(10), title: '菜单管理', path: '/system/menu', icon: 'Menu',
        type: 'menu', perms: 'system:menu:list', sort: 3, visible: 1, status: 1,
        children: [
          { id: mid(1301), parent_id: mid(13), title: '新增菜单', path: '', icon: '', type: 'button', perms: 'system:menu:create', sort: 1, visible: 1, status: 1 },
          { id: mid(1302), parent_id: mid(13), title: '编辑菜单', path: '', icon: '', type: 'button', perms: 'system:menu:update', sort: 2, visible: 1, status: 1 },
          { id: mid(1303), parent_id: mid(13), title: '删除菜单', path: '', icon: '', type: 'button', perms: 'system:menu:delete', sort: 3, visible: 1, status: 1 }
        ]
      },
      {
        id: mid(14), parent_id: mid(10), title: '字典管理', path: '/system/dict', icon: 'Collection',
        type: 'menu', perms: 'system:dict:list', sort: 4, visible: 1, status: 1,
        children: [
          { id: mid(1401), parent_id: mid(14), title: '新增字典', path: '', icon: '', type: 'button', perms: 'system:dict:create', sort: 1, visible: 1, status: 1 },
          { id: mid(1402), parent_id: mid(14), title: '编辑字典', path: '', icon: '', type: 'button', perms: 'system:dict:update', sort: 2, visible: 1, status: 1 },
          { id: mid(1403), parent_id: mid(14), title: '删除字典', path: '', icon: '', type: 'button', perms: 'system:dict:delete', sort: 3, visible: 1, status: 1 }
        ]
      },
      {
        id: mid(15), parent_id: mid(10), title: '参数配置', path: '/system/config', icon: 'Tools',
        type: 'menu', perms: 'system:config:list', sort: 5, visible: 1, status: 1,
        children: [
          { id: mid(1501), parent_id: mid(15), title: '新增参数', path: '', icon: '', type: 'button', perms: 'system:config:create', sort: 1, visible: 1, status: 1 },
          { id: mid(1502), parent_id: mid(15), title: '编辑参数', path: '', icon: '', type: 'button', perms: 'system:config:update', sort: 2, visible: 1, status: 1 },
          { id: mid(1503), parent_id: mid(15), title: '删除参数', path: '', icon: '', type: 'button', perms: 'system:config:delete', sort: 3, visible: 1, status: 1 }
        ]
      },
      {
        id: mid(16), parent_id: mid(10), title: '日志管理', path: '/system/logs', icon: 'Document',
        type: 'menu', perms: 'system:log:list', sort: 6, visible: 1, status: 1, children: []
      }
    ]
  }
]

// 拍平菜单树（mock 内部使用）
export function flatMenus(list: MockMenu[] = menus, result: MockMenu[] = []): MockMenu[] {
  for (const m of list) {
    result.push(m)
    if (m.children?.length) flatMenus(m.children, result)
  }
  return result
}

// 收集全部权限点
export function allPerms(): string[] {
  return flatMenus().map((m) => m.perms).filter(Boolean)
}

export const roles = [
  { id: mid(1), code: 'super_admin', name: '超级管理员', data_scope: 'all', remark: '内置角色，拥有全部权限', status: 1, user_count: 1, created_at: '2026-06-01 10:00:00', builtin: true },
  { id: mid(2), code: 'manager', name: '物业主管', data_scope: 'custom', remark: '按小区查看数据', status: 1, user_count: 5, created_at: '2026-06-01 10:00:00', builtin: false },
  { id: mid(3), code: 'inspector', name: '巡检员', data_scope: 'custom', remark: '小程序巡检端', status: 1, user_count: 18, created_at: '2026-06-01 10:00:00', builtin: false },
  { id: mid(4), code: 'repairer', name: '维修人员', data_scope: 'custom', remark: '处理异常工单', status: 1, user_count: 6, created_at: '2026-06-01 10:00:00', builtin: false }
]

// 角色已分配的菜单 id（超管为全量）
export const roleMenus: Record<string, string[]> = {
  1: flatMenus().map((m) => m.id),
  [mid(2)]: [mid(1), mid(10), mid(11), mid(12), mid(14), mid(15), mid(16), mid(17), mid(18)],
  [mid(3)]: [mid(1)],
  [mid(4)]: [mid(1)]}

export const communities = [
  { id: mid(1), name: '翡翠湾小区', address: '杭州市滨江区江南大道 100 号', manager_id: mid(2), manager_name: '王主管', building_count: 12, point_count: 86, status: 1, created_at: '2026-06-01 10:00:00' },
  { id: mid(2), name: '滨江花园', address: '杭州市滨江区江陵路 88 号', manager_id: mid(2), manager_name: '王主管', building_count: 9, point_count: 64, status: 1, created_at: '2026-06-01 10:00:00' },
  { id: mid(3), name: '翠湖天地', address: '杭州市西湖区文三路 200 号', manager_id: mid(5), manager_name: '李主管', building_count: 7, point_count: 45, status: 1, created_at: '2026-06-01 10:00:00' }
]

export const users = [
  { id: mid(1), username: 'admin', name: '系统管理员', phone: '13800000000', avatar: '', openid: '', role_ids: [mid(1)], community_ids: [], status: 1, last_login_at: '2026-08-05 08:01:22', created_at: '2026-06-01 10:00:00' },
  { id: mid(2), username: 'wangzg', name: '王主管', phone: '13900000001', avatar: '', openid: '', role_ids: [mid(2)], community_ids: [mid(1), mid(2)], status: 1, last_login_at: '2026-08-05 07:55:10', created_at: '2026-06-01 10:00:00' },
  { id: mid(3), username: '13900001111', name: '张三', phone: '13900001111', avatar: '', openid: 'oXyz9abc', role_ids: [mid(3)], community_ids: [mid(1), mid(2)], status: 1, last_login_at: '2026-08-05 08:01:22', created_at: '2026-06-01 10:00:00' },
  { id: mid(4), username: '13900002222', name: '李四', phone: '13900002222', avatar: '', openid: 'oXyz9abd', role_ids: [mid(3)], community_ids: [mid(2)], status: 1, last_login_at: '2026-08-04 18:20:41', created_at: '2026-06-02 10:00:00' },
  { id: mid(5), username: 'lizg', name: '李主管', phone: '13900000003', avatar: '', openid: '', role_ids: [mid(2)], community_ids: [mid(3)], status: 1, last_login_at: '2026-08-03 09:12:05', created_at: '2026-06-02 10:00:00' },
  { id: mid(6), username: '13900003333', name: '王五', phone: '13900003333', avatar: '', openid: 'oXyz9abe', role_ids: [mid(3)], community_ids: [mid(1)], status: 1, last_login_at: '2026-08-05 06:40:00', created_at: '2026-06-03 10:00:00' },
  { id: mid(7), username: '13900004444', name: '赵六', phone: '13900004444', avatar: '', openid: 'oXyz9abf', role_ids: [mid(4)], community_ids: [mid(1), mid(2), mid(3)], status: 1, last_login_at: '2026-08-04 15:33:27', created_at: '2026-06-03 10:00:00' },
  { id: mid(8), username: '13900005555', name: '孙七', phone: '13900005555', avatar: '', openid: '', role_ids: [mid(3)], community_ids: [mid(3)], status: 0, last_login_at: '2026-07-20 11:02:13', created_at: '2026-06-05 10:00:00' },
  { id: mid(9), username: '13900006666', name: '周八', phone: '13900006666', avatar: '', openid: 'oXyz9ac0', role_ids: [mid(4)], community_ids: [mid(2)], status: 1, last_login_at: '2026-08-05 08:30:00', created_at: '2026-06-06 10:00:00' },
  { id: mid(10), username: '13900007777', name: '吴九', phone: '13900007777', avatar: '', openid: 'oXyz9ac1', role_ids: [mid(3)], community_ids: [mid(1), mid(3)], status: 1, last_login_at: '2026-08-05 07:12:48', created_at: '2026-06-08 10:00:00' }
]

export const dictTypes = [
  { id: mid(1), code: 'point_type', name: '点位类型', builtin: true, created_at: '2026-06-01 10:00:00' },
  { id: mid(2), code: 'abnormal_type', name: '异常类型', builtin: true, created_at: '2026-06-01 10:00:00' },
  { id: mid(3), code: 'work_order_status', name: '工单状态', builtin: true, created_at: '2026-06-01 10:00:00' },
  { id: mid(4), code: 'work_order_priority', name: '工单优先级', builtin: true, created_at: '2026-06-01 10:00:00' }
]

export const dictData = [
  { id: mid(11), type_code: 'point_type', label: '大堂', value: 'lobby', sort: 1, status: 1 },
  { id: mid(12), type_code: 'point_type', label: '配电房', value: 'power_room', sort: 2, status: 1 },
  { id: mid(13), type_code: 'point_type', label: '消防栓', value: 'hydrant', sort: 3, status: 1 },
  { id: mid(14), type_code: 'point_type', label: '电梯机房', value: 'elevator_room', sort: 4, status: 1 },
  { id: mid(15), type_code: 'point_type', label: '地下车库', value: 'garage', sort: 5, status: 1 },
  { id: mid(21), type_code: 'abnormal_type', label: '设备故障', value: 'device_fault', sort: 1, status: 1 },
  { id: mid(22), type_code: 'abnormal_type', label: '安全隐患', value: 'safety_risk', sort: 2, status: 1 },
  { id: mid(23), type_code: 'abnormal_type', label: '环境卫生', value: 'sanitation', sort: 3, status: 1 },
  { id: mid(24), type_code: 'abnormal_type', label: '其他', value: 'other', sort: 4, status: 1 },
  { id: mid(31), type_code: 'work_order_status', label: '待派单', value: 'pending', sort: 1, status: 1 },
  { id: mid(32), type_code: 'work_order_status', label: '处理中', value: 'processing', sort: 2, status: 1 },
  { id: mid(33), type_code: 'work_order_status', label: '待复核', value: 'reviewing', sort: 3, status: 1 },
  { id: mid(34), type_code: 'work_order_status', label: '已关闭', value: 'closed', sort: 4, status: 1 },
  { id: mid(41), type_code: 'work_order_priority', label: '紧急', value: 'urgent', sort: 1, status: 1 },
  { id: mid(42), type_code: 'work_order_priority', label: '一般', value: 'normal', sort: 2, status: 1 },
  { id: mid(43), type_code: 'work_order_priority', label: '低', value: 'low', sort: 3, status: 1 }
]

export const configs = [
  { id: mid(1), key: 'fence.default_radius', name: '围栏默认半径（米）', value: '100', remark: '新建点位默认值，范围 50-500', builtin: true, updated_at: '2026-06-01 10:00:00' },
  { id: mid(2), key: 'watermark.enabled', name: '照片水印开关', value: 'true', remark: '打卡照片叠加水印', builtin: true, updated_at: '2026-06-01 10:00:00' },
  { id: mid(3), key: 'suspect.distance_threshold', name: '疑似作弊距离阈值（米）', value: '100', remark: '超过则标记疑似作弊', builtin: true, updated_at: '2026-06-01 10:00:00' },
  { id: mid(4), key: 'notify.subscribe_msg', name: '微信订阅消息开关', value: 'true', remark: '', builtin: true, updated_at: '2026-06-01 10:00:00' },
  { id: mid(5), key: 'photo.max_count', name: '单次打卡最多照片数', value: '6', remark: '', builtin: false, updated_at: '2026-07-01 09:00:00' }
]

// 生成操作日志 mock 数据
function buildOperationLogs() {
  const modules = ['system', 'community', 'inspection', 'workorder', 'stats']
  const actions = ['create', 'update', 'delete', 'export', 'login']
  const methods = ['POST', 'PUT', 'DELETE', 'GET']
  const list = []
  for (let i = 0; i < 56; i++) {
    const mod = modules[i % modules.length]
    const action = actions[i % actions.length]
    list.push({
      id: mid(9001) + i,
      user_id: mid((i % 5) + 1),
      username: ['admin', 'wangzg', 'lizg', 'admin', 'wangzg'][i % 5],
      module: mod,
      action,
      method: action === 'create' ? 'POST' : action === 'update' ? 'PUT' : action === 'delete' ? 'DELETE' : methods[i % methods.length],
      path: `/api/admin/${mod}/items/${(i % 9) + 1}`,
      params: JSON.stringify({ name: `示例数据${i + 1}`, page: 1 }),
      ip: `10.0.1.${(i % 50) + 10}`,
      status: i % 11 === 0 ? 0 : 1,
      cost_ms: 8 + ((i * 7) % 120),
      created_at: `2026-08-0${(i % 5) + 1} ${String(8 + (i % 12)).padStart(2, '0')}:${String((i * 13) % 60).padStart(2, '0')}:03`
    })
  }
  return list
}

// 生成登录日志 mock 数据
function buildLoginLogs() {
  const list = []
  for (let i = 0; i < 42; i++) {
    const fail = i % 9 === 0
    const fromMp = i % 3 === 0
    list.push({
      id: mid(3201) + i,
      user_id: mid((i % 8) + 1),
      username: ['admin', 'wangzg', '13900001111', '13900002222', 'lizg', '13900003333', '13900004444', '13900007777'][i % 8],
      ip: `112.10.22.${(i % 60) + 8}`,
      ua: fromMp ? 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) MicroMessenger/8.0' : 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0',
      source: fromMp ? '小程序' : '后台',
      status: fail ? 0 : 1,
      msg: fail ? '用户名或密码错误' : fromMp ? '小程序登录成功' : '登录成功',
      created_at: `2026-08-0${(i % 5) + 1} ${String(7 + (i % 13)).padStart(2, '0')}:${String((i * 17) % 60).padStart(2, '0')}:22`
    })
  }
  return list
}

export const operationLogs = buildOperationLogs()
export const loginLogs = buildLoginLogs()
