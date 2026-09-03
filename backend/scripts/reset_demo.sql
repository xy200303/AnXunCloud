-- 安巡云演示数据重置脚本（仅清空，不再用 SQL 重建演示数据）
--
-- 用途：清空全部业务数据（保留 admin、默认租户、系统配置/菜单/角色/字典）。
-- 注意：本脚本只负责清空；清空后需重新运行 seed-demo 生成演示数据，并重启后端使权限策略生效：
--   dev:  docker exec <backend容器> sh -c 'cd /app && go run ./cmd/seed-demo'
--   prod: docker exec anxuncloud-prod-app /app/seed-demo
--
-- 用法：
--   docker exec -i <postgres容器> psql -U postgres -d anxuncloud -f /path/to/reset_demo.sql
--   或：cat reset_demo.sql | docker exec -i <postgres容器> psql -U postgres -d anxuncloud

BEGIN;

-- 清空业务数据（保留 admin / 默认租户 / 系统配置菜单角色字典）
-- 注意：post_dict / duty_binding / approval_flow 不能整表 TRUNCATE——
-- tenant_id 为空的行是平台模板/默认（开通租户复制源与配置回落末级），只清租户级行。
TRUNCATE checkin_item_draft, checkin_record_item, checkin_record, inspection_task, inspection_plan,
  inspection_point, check_template_item, check_template, building, community,
  project_staff, inspection_report, sys_notice,
  sys_message, user_push_device, upload_file,
  sign_asset, sys_login_log, sys_operation_log, tenant_config CASCADE;

DELETE FROM post_dict WHERE tenant_id IS NOT NULL;
DELETE FROM duty_binding WHERE tenant_id IS NOT NULL OR project_id IS NOT NULL;
DELETE FROM approval_flow WHERE tenant_id IS NOT NULL OR project_id IS NOT NULL;

DELETE FROM sys_user WHERE username <> 'admin';
DELETE FROM tenant WHERE code <> 'default';

-- casbin 用户→角色绑定：只留 admin 的（角色权限 p 规则全部保留）
DELETE FROM casbin_rule WHERE ptype = 'g'
  AND v0 <> 'user:' || (SELECT id FROM sys_user WHERE username = 'admin');

COMMIT;
