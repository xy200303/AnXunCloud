-- 新增「问题清单」菜单（异常打卡记录只读出口，老库升级用，幂等）
-- 挂在巡检管理目录下，perm 复用巡检记录列表权限点 inspection:record:list；
-- 角色绑定：已拥有巡检记录菜单（path='/inspection/records'）的角色自动绑上本菜单。
-- +goose Up
-- +goose StatementBegin

INSERT INTO public.sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '问题清单', '/inspection/issues', 'Warning', 'menu', 'inspection:record:list', 5,
       true, 'enabled', true, false, now(), now()
FROM public.sys_menu p
WHERE p.path = '/inspection' AND p.type = 'dir' AND p.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM public.sys_menu WHERE path = '/inspection/issues' AND deleted_at IS NULL
  );

INSERT INTO public.sys_role_menu (role_id, menu_id)
SELECT DISTINCT rm.role_id, m.id
FROM public.sys_role_menu rm
JOIN public.sys_menu src ON src.id = rm.menu_id AND src.path = '/inspection/records'
CROSS JOIN public.sys_menu m
WHERE m.path = '/inspection/issues' AND m.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM public.sys_role_menu x WHERE x.role_id = rm.role_id AND x.menu_id = m.id
  );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 菜单数据不回滚（已被角色授权引用，回滚易误删；需要时下线参照 00012 的模式单独处理）。
-- +goose StatementEnd
