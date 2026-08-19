-- 审批链管理独立菜单（扩展方案 §3；审批链从岗位管理页签独立为一级菜单，幂等：已存在则跳过）
-- +goose Up
-- +goose StatementBegin

-- 系统管理 → 审批链管理（租户级，含"保存审核链"按钮）
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '审批链管理', '/system/review-flow', 'Connection', 'menu', 'system:reviewflow:list', 8, true, 'enabled', true, false, now(), now()
FROM sys_menu p WHERE p.path = '/system' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.path = '/system/review-flow' AND m.deleted_at IS NULL);

INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '保存审核链', '', '', 'button', 'system:reviewflow:update', 1, false, 'enabled', true, false, now(), now()
FROM sys_menu p WHERE p.path = '/system/review-flow' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'system:reviewflow:update' AND m.deleted_at IS NULL);

-- 平台管理 → 审批链模板（平台默认，含"保存审核链模板"按钮）
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '审批链模板', '/platform/review-flow-template', 'Share', 'menu', 'platform:reviewflow:list', 7, true, 'enabled', true, true, now(), now()
FROM sys_menu p WHERE p.path = '/platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.path = '/platform/review-flow-template' AND m.deleted_at IS NULL);

INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '保存审核链模板', '', '', 'button', 'platform:reviewflow:update', 1, false, 'enabled', true, true, now(), now()
FROM sys_menu p WHERE p.path = '/platform/review-flow-template' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'platform:reviewflow:update' AND m.deleted_at IS NULL);

-- 角色绑定：超管全量；租户管理员授租户级审批链菜单（不含平台级）
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'super_admin' AND r.tenant_id IS NULL AND r.deleted_at IS NULL
  AND m.perms IN ('system:reviewflow:list', 'system:reviewflow:update', 'platform:reviewflow:list', 'platform:reviewflow:update') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_role_menu rm WHERE rm.role_id = r.id AND rm.menu_id = m.id);

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'tenant_admin' AND r.tenant_id IS NULL AND r.deleted_at IS NULL
  AND m.perms IN ('system:reviewflow:list', 'system:reviewflow:update') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_role_menu rm WHERE rm.role_id = r.id AND rm.menu_id = m.id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE perms IN ('system:reviewflow:list','system:reviewflow:update','platform:reviewflow:list','platform:reviewflow:update'));
DELETE FROM sys_menu WHERE perms IN ('system:reviewflow:list','system:reviewflow:update','platform:reviewflow:list','platform:reviewflow:update');
-- +goose StatementEnd
