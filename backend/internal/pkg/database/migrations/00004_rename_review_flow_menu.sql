-- 术语通俗化：审批链 → 审批流程（仅菜单/按钮显示名，路径与权限点不变，幂等）
-- +goose Up
-- +goose StatementBegin

UPDATE sys_menu SET title = '审批流程', updated_at = now()
WHERE path = '/system/review-flow' AND deleted_at IS NULL AND title <> '审批流程';

UPDATE sys_menu SET title = '保存审批流程', updated_at = now()
WHERE perms = 'system:reviewflow:update' AND deleted_at IS NULL AND title <> '保存审批流程';

UPDATE sys_menu SET title = '审批流程模板', updated_at = now()
WHERE path = '/platform/review-flow-template' AND deleted_at IS NULL AND title <> '审批流程模板';

UPDATE sys_menu SET title = '保存审批流程模板', updated_at = now()
WHERE perms = 'platform:reviewflow:update' AND deleted_at IS NULL AND title <> '保存审批流程模板';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE sys_menu SET title = '审批链管理', updated_at = now() WHERE path = '/system/review-flow' AND deleted_at IS NULL;
UPDATE sys_menu SET title = '保存审核链', updated_at = now() WHERE perms = 'system:reviewflow:update' AND deleted_at IS NULL;
UPDATE sys_menu SET title = '审批链模板', updated_at = now() WHERE path = '/platform/review-flow-template' AND deleted_at IS NULL;
UPDATE sys_menu SET title = '保存审核链模板', updated_at = now() WHERE perms = 'platform:reviewflow:update' AND deleted_at IS NULL;
-- +goose StatementEnd
