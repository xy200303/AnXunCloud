-- +goose Up
-- owner: workorder
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '72000000-0000-0000-0000-000000000008', '72000000-0000-0000-0000-000000000002', '合并工单', '', '', 'button', 'workorder:merge', 6, true, 'enabled', true, false
WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE id = '72000000-0000-0000-0000-000000000008');
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, '72000000-0000-0000-0000-000000000008' FROM sys_role r
WHERE r.code IN ('project_manager', 'quality_supervisor', 'engineer_supervisor', 'tenant_admin')
ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
