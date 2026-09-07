-- 00043：移除固定巡检员确认权限，报告签字统一由动态审核链授权。

-- +goose Up
DELETE FROM sys_role_menu
WHERE menu_id IN (
    SELECT id FROM sys_menu WHERE perms = 'report:sign:inspector'
);
DELETE FROM sys_menu WHERE perms = 'report:sign:inspector';

-- +goose Down
-- 废弃权限不再恢复；回滚仅回滚迁移版本记录。
