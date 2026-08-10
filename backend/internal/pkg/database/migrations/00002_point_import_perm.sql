-- 点位管理新增「批量导入」按钮权限点 inspection:point:import。
-- 全新部署时菜单树由 Seed 写入（含该按钮），本迁移的父菜单查找为空、自然跳过；
-- 已有部署（dev/prod 老库）由本迁移补插菜单行并授予已拥有「新增点位」权限的角色。

-- +goose Up
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, created_at, updated_at)
SELECT gen_random_uuid(), m.id, '批量导入', '', '', 'button', 'inspection:point:import', 5, false, 'enabled', true, now(), now()
FROM sys_menu m
WHERE m.perms = 'inspection:point:list'
  AND NOT EXISTS (SELECT 1 FROM sys_menu x WHERE x.perms = 'inspection:point:import');

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT rm.role_id, mi.id
FROM sys_role_menu rm
JOIN sys_menu mc ON mc.id = rm.menu_id AND mc.perms = 'inspection:point:create'
JOIN sys_menu mi ON mi.perms = 'inspection:point:import'
WHERE NOT EXISTS (SELECT 1 FROM sys_role_menu x WHERE x.role_id = rm.role_id AND x.menu_id = mi.id);

-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE perms = 'inspection:point:import');
DELETE FROM sys_menu WHERE perms = 'inspection:point:import';
