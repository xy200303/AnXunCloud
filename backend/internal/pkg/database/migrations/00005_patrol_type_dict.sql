-- 巡查类型字典化（《专项巡检与专项检查报告设计方案》§3.1，全部幂等）：
-- sys_dict_data 加通用扩展列 attrs；patrol_type 新增 fire（消防设施专项）并补大类标记；
-- point_type 新增消防箱/灭火器；fire 汇报线维度槽位平台默认绑工程主管。
-- +goose Up
-- +goose StatementBegin

-- 字典数据通用扩展属性（如 patrol_type 的 category 大类标记：daily_patrol 日常巡逻 / special 专项检查）
ALTER TABLE sys_dict_data ADD COLUMN IF NOT EXISTS attrs jsonb;
COMMENT ON COLUMN sys_dict_data.attrs IS '通用扩展属性（jsonb）；patrol_type 用 attrs.category 标记大类 daily_patrol/special';

-- patrol_type 新增「消防设施专项」（sort 接在现有 4 项后）
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, attrs, created_at, updated_at)
SELECT gen_random_uuid(), 'patrol_type', '消防设施专项', 'fire', 5, 'enabled', '{"category":"special"}'::jsonb, now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_data WHERE type_code = 'patrol_type' AND value = 'fire' AND deleted_at IS NULL);

-- patrol_type 存量项补大类标记（safety→日常巡逻，equipment/environment/building→专项检查；已标记不覆盖）
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || '{"category":"daily_patrol"}'::jsonb, updated_at = now()
WHERE type_code = 'patrol_type' AND value = 'safety' AND deleted_at IS NULL
  AND attrs->>'category' IS DISTINCT FROM 'daily_patrol';

UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || '{"category":"special"}'::jsonb, updated_at = now()
WHERE type_code = 'patrol_type' AND value IN ('equipment', 'environment', 'building') AND deleted_at IS NULL
  AND attrs->>'category' IS DISTINCT FROM 'special';

-- point_type 新增消防器材点位类型
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, created_at, updated_at)
SELECT gen_random_uuid(), 'point_type', '消防箱', 'fire_cabinet', 7, 'enabled', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_data WHERE type_code = 'point_type' AND value = 'fire_cabinet' AND deleted_at IS NULL);

INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, created_at, updated_at)
SELECT gen_random_uuid(), 'point_type', '灭火器', 'fire_extinguisher', 8, 'enabled', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_data WHERE type_code = 'point_type' AND value = 'fire_extinguisher' AND deleted_at IS NULL);

-- fire 汇报线维度槽位平台默认绑定（project_id/tenant_id 均空 = 平台级）：工程主管
INSERT INTO duty_binding (id, tenant_id, project_id, slot, post_codes, created_at, updated_at)
SELECT gen_random_uuid(), NULL, NULL, 'patrol_report_line.fire', '["engineering_supervisor"]'::jsonb, now(), now()
WHERE NOT EXISTS (SELECT 1 FROM duty_binding WHERE project_id IS NULL AND tenant_id IS NULL AND slot = 'patrol_report_line.fire');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM duty_binding WHERE project_id IS NULL AND tenant_id IS NULL AND slot = 'patrol_report_line.fire';
DELETE FROM sys_dict_data WHERE type_code = 'point_type' AND value IN ('fire_cabinet', 'fire_extinguisher');
DELETE FROM sys_dict_data WHERE type_code = 'patrol_type' AND value = 'fire';
UPDATE sys_dict_data SET attrs = attrs - 'category', updated_at = now() WHERE type_code = 'patrol_type' AND attrs IS NOT NULL;
ALTER TABLE sys_dict_data DROP COLUMN IF EXISTS attrs;
-- +goose StatementEnd
