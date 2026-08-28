-- 安巡云演示数据重置脚本（仅清空，不再用 SQL 重建演示数据）
--
-- 用途：清空全部业务数据（保留 admin、默认租户、系统配置/菜单/角色/字典）。
-- 注意：本脚本只负责清空；清空后需重新运行 seed-demo 生成演示数据：
--   docker exec -w /app <backend容器> go run ./cmd/seed-demo   # dev
--   docker compose --env-file .env.prod -f docker-compose.prod.yml run --rm app /app/seed-demo   # prod
--
-- 用法：
--   docker exec -i <postgres容器> psql -U postgres -d anxuncloud -f /path/to/reset_demo.sql
--   或：cat reset_demo.sql | docker exec -i <postgres容器> psql -U postgres -d anxuncloud

BEGIN;

-- 清空业务数据（保留 admin / 默认租户 / 系统配置菜单角色字典）
TRUNCATE checkin_item_draft, checkin_record_item, checkin_record, inspection_task, inspection_plan,
  inspection_point, check_template_item, check_template, building, community,
  project_staff, duty_binding, approval_flow, inspection_report, sys_notice,
  sys_message, user_push_device, upload_file,
  sign_asset, sys_login_log, sys_operation_log, tenant_config, post_dict CASCADE;

DELETE FROM sys_user WHERE username <> 'admin';
DELETE FROM tenant WHERE code <> 'default';

COMMIT;
