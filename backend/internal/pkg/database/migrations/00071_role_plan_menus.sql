-- +goose Up
-- owner: system
UPDATE sys_menu SET parent_id = NULL, title = '计划管理', sort = 15 WHERE path = '/inspection/plans' AND type = 'menu';

DELETE FROM sys_role_menu WHERE menu_id IN (
    SELECT id FROM sys_menu WHERE path IN ('/system/posts', '/platform/post-templates')
       OR perms IN ('community:duty:edit', 'system:post:list', 'system:post:create', 'system:post:update', 'system:post:delete', 'system:post:duty',
                    'platform:post:list', 'platform:post:create', 'platform:post:update', 'platform:post:delete', 'platform:post:duty')
);
DELETE FROM sys_menu WHERE path IN ('/system/posts', '/platform/post-templates')
    OR perms IN ('community:duty:edit', 'system:post:list', 'system:post:create', 'system:post:update', 'system:post:delete', 'system:post:duty',
                 'platform:post:list', 'platform:post:create', 'platform:post:update', 'platform:post:delete', 'platform:post:duty');
DELETE FROM sys_dict_data WHERE type_code = 'data_scope';

-- +goose Down
SELECT 1;
