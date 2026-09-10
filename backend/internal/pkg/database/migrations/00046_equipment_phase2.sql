-- 00046：设备台账二期——标签缺失逃生通道 + 日期标签抽查 + 报废催办（《设备台账与维保周期管理设计方案》v1.7）。
-- equipment.label_missing：标签缺失标记（确认链打上/清除，退出自动到期判定与抽查）；
-- equipment_maintenance.label_missing：登记随单标记（确认时回写设备）；
-- sys_message.type 约束扩展 equipment_scrap（报废提醒）；
-- sys_config 补 equipment.spotcheck_salt（抽查哈希盐值；00045 未含）。

-- +goose Up
ALTER TABLE public.equipment ADD COLUMN IF NOT EXISTS label_missing boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN public.equipment.label_missing IS '标签缺失/无法辨认（钢印磨损）：确认链打上，经理处置后清除；true=退出自动到期判定与抽查';

ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS label_missing boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN public.equipment_maintenance.label_missing IS '标签缺失登记：照片仍必传（拍设备本体作证），日期免填；确认后设备打 label_missing 标记';

ALTER TABLE public.sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE public.sys_message ADD CONSTRAINT chk_sys_message_type
  CHECK (type IN ('task','export','system','checkin_audit','report','announcement','equipment_expire','equipment_maint_reject','equipment_scrap'));

-- upload_file.scene 白名单扩展 equipment（管理端维保登记照片）
ALTER TABLE public.upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE public.upload_file ADD CONSTRAINT chk_upload_file_scene
  CHECK (scene IN ('checkin','avatar','export','signature','seal','notice','app','equipment'));

-- 抽查哈希盐值（每部署随机；已有则不动）
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment.spotcheck_salt', '设备抽查哈希盐值', md5(random()::text || clock_timestamp()::text), 'equipment', '日期标签抽查确定性哈希盐值（防推算）', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config c WHERE c.key = 'equipment.spotcheck_salt' AND c.deleted_at IS NULL);

-- +goose Down
ALTER TABLE public.equipment DROP COLUMN IF EXISTS label_missing;
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS label_missing;
ALTER TABLE public.sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE public.sys_message ADD CONSTRAINT chk_sys_message_type
  CHECK (type IN ('task','export','system','checkin_audit','report','announcement','equipment_expire','equipment_maint_reject'));
ALTER TABLE public.upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE public.upload_file ADD CONSTRAINT chk_upload_file_scene
  CHECK (scene IN ('checkin','avatar','export','signature','seal','notice','app'));
DELETE FROM sys_config WHERE key = 'equipment.spotcheck_salt';
