-- +goose Up
-- owner: system
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '76000000-0000-0000-0000-000000000001', NULL, '模板中心', '/templates', 'Files', 'dir', '', 45, true, 'enabled', true, false
WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE id = '76000000-0000-0000-0000-000000000001');
UPDATE sys_menu SET parent_id = '76000000-0000-0000-0000-000000000001', path = '/templates/check' WHERE path = '/inspection/templates';
UPDATE sys_menu SET parent_id = NULL WHERE path = '/community' AND type = 'menu';
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '76000000-0000-0000-0000-000000000002', NULL, '小区管理', '/community', 'OfficeBuilding', 'dir', '', 50, true, 'enabled', true, false
WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE id = '76000000-0000-0000-0000-000000000002');
UPDATE sys_menu SET parent_id = '76000000-0000-0000-0000-000000000002', path = '/community/list' WHERE path = '/community' AND type = 'menu';
UPDATE sys_menu SET parent_id = '76000000-0000-0000-0000-000000000002', path = '/community/points' WHERE path = '/inspection/points';
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT role_id, '76000000-0000-0000-0000-000000000001'::uuid FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE path = '/templates/check') ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT role_id, '76000000-0000-0000-0000-000000000002'::uuid FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE path IN ('/community/list', '/community/points')) ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
