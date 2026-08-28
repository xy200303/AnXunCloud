-- 00002：问题清单并入巡检记录（列表按结果筛选 + 导出），删除「问题清单」菜单。
-- 老库清理存量菜单行（新库 seed 本就不再创建）；同步清理角色-菜单关联，casbin 策略启动时自动重建。

-- +goose Up
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE path = '/inspection/issues');
DELETE FROM sys_menu WHERE path = '/inspection/issues';

-- +goose Down
-- 菜单行由 seed 重建（seedMenus 幂等），不回滚。
SELECT 1;
