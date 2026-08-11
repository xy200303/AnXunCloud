-- 巡检员角色默认授予「月度报告」菜单（report:list）：巡检员可在 PC 后台查看所辖小区报告并做电子确认。
-- 仅下放查看+确认；生成/下载/主管与经理签字权限点仍不授予，页面按钮按权限点自动隐藏。
-- 全新部署时由 Seed 写入等价授权，本迁移 NOT EXISTS 自然跳过。

-- +goose Up
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, mi.id
FROM sys_role r
JOIN sys_menu mi ON mi.perms = 'report:list'
WHERE r.code = 'inspector'
  AND NOT EXISTS (SELECT 1 FROM sys_role_menu x WHERE x.role_id = r.id AND x.menu_id = mi.id);

-- +goose Down
DELETE FROM sys_role_menu
WHERE role_id IN (SELECT id FROM sys_role WHERE code = 'inspector')
  AND menu_id IN (SELECT id FROM sys_menu WHERE perms = 'report:list');
