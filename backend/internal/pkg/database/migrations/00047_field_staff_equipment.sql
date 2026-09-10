-- 00047：一线人员（field_staff）摘除设备台账查看权限（equipment:list）。
-- 台账管理是管理岗能力；巡检员的设备触点为打卡合成项/维保待办卡片/登记入口（appAuth /equipment/maintenance，
-- 由 equipment:maintenance 权限覆盖），均不依赖 equipment:list。seed.go 已同步，本迁移清理存量库的角色菜单绑定。

-- +goose Up
DELETE FROM public.sys_role_menu
WHERE role_id IN (SELECT id FROM public.sys_role WHERE code = 'field_staff' AND deleted_at IS NULL)
  AND menu_id IN (SELECT id FROM public.sys_menu WHERE perms = 'equipment:list' AND deleted_at IS NULL);

-- +goose Down
-- 回滚：恢复 field_staff ↔ equipment:list 菜单绑定（仅当菜单存在时）
INSERT INTO public.sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM public.sys_role r, public.sys_menu m
WHERE r.code = 'field_staff' AND r.deleted_at IS NULL
  AND m.perms = 'equipment:list' AND m.deleted_at IS NULL
ON CONFLICT DO NOTHING;
