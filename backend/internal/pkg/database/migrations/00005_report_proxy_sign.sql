-- 月报新增「代签」按钮权限点 report:sign:proxy（仅授予超管；其他角色需要时由角色管理配置）。
-- 全新部署时菜单树由 Seed 写入（含该按钮），本迁移的父菜单查找为空、自然跳过。

-- +goose Up
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, created_at, updated_at)
SELECT gen_random_uuid(), m.id, '代签', '', '', 'button', 'report:sign:proxy', 6, false, 'enabled', true, now(), now()
FROM sys_menu m
WHERE m.perms = 'report:list'
  AND NOT EXISTS (SELECT 1 FROM sys_menu x WHERE x.perms = 'report:sign:proxy');

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, mi.id
FROM sys_role r
JOIN sys_menu mi ON mi.perms = 'report:sign:proxy'
WHERE r.code = 'super_admin'
  AND NOT EXISTS (SELECT 1 FROM sys_role_menu x WHERE x.role_id = r.id AND x.menu_id = mi.id);

-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE perms = 'report:sign:proxy');
DELETE FROM sys_menu WHERE perms = 'report:sign:proxy';
