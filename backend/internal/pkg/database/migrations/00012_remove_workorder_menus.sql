-- 移除工单模块菜单与权限点（老库升级用；模块代码封存，菜单/权限绑定下线，幂等）
-- 范围：path='/workorders' 整棵子树 + 所有 perms LIKE 'workorder:%' 的按钮权限点；
-- 字典数据（order_priority/work_order_status/order_source）与 work_order 表结构保留不删。
-- +goose Up
-- +goose StatementBegin

-- 目标菜单集合：/workorders 子树（含软删残留）+ workorder:* 权限点
CREATE TEMPORARY TABLE _wo_menu_ids ON COMMIT DROP AS
WITH RECURSIVE subtree AS (
    SELECT id FROM public.sys_menu WHERE path = '/workorders'
    UNION ALL
    SELECT m.id FROM public.sys_menu m JOIN subtree s ON m.parent_id = s.id
)
SELECT id FROM subtree
UNION
SELECT id FROM public.sys_menu WHERE perms LIKE 'workorder:%';

-- 先清角色菜单绑定，再删菜单本体（幂等：目标集为空时影响 0 行）
DELETE FROM public.sys_role_menu WHERE menu_id IN (SELECT id FROM _wo_menu_ids);
DELETE FROM public.sys_menu WHERE id IN (SELECT id FROM _wo_menu_ids);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 菜单数据不回滚（工单模块已下线，重新启用时由 seed 或人工补录）。
-- +goose StatementEnd
