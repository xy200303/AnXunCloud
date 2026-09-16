-- 00055：维保类型扩展 refill（换粉）/ hydro_test（水压试验）+ 日期标签抽查默认比例 10%→2%（甲方口径）。
-- 换粉确认后与 repair 同规则回写重算 next_due_date（换粉到期预警由 expire_job 每日扫描自动生效）；
-- 水压试验只留记录不改到期（回写跳过逻辑在 ApplyLedgerWriteback，迁移只管数据）。
-- sys_dict_data 为平台级（无 tenant_id 维度），一次插入全局生效。全部幂等可重复执行。

-- +goose Up

-- 1) 抽查默认比例：代码读取缺省已改 2，这里同步已落库的 sys_config 值（enabled 种子即 true，无需改）
UPDATE sys_config SET value = '2', updated_at = now()
WHERE key = 'equipment.spotcheck_ratio' AND deleted_at IS NULL AND value <> '2';

-- 2) 维保类型字典补「换粉」「水压试验」（排在现有 5 项之后；PC 端维保类型下拉字典驱动即生效）
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment_maint_type', t.label, t.value, t.sort, 'enabled', now(), now()
FROM (VALUES
  ('换粉',     'refill',     6),
  ('水压试验', 'hydro_test', 7)
) AS t(label, value, sort)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_dict_data d WHERE d.type_code = 'equipment_maint_type' AND d.value = t.value AND d.deleted_at IS NULL
);

-- 3) 列注释同步新类型枚举
COMMENT ON COLUMN public.equipment_maintenance.maintenance_type IS 'repair 维修充粉 / maintain 保养 / inspect 检测 / replace 更换 / ledger_fix 台账补录 / refill 换粉 / hydro_test 水压试验';

-- +goose Down
-- 还原：比例回 10、删新增字典项、列注释还原。
UPDATE sys_config SET value = '10', updated_at = now()
WHERE key = 'equipment.spotcheck_ratio' AND deleted_at IS NULL;

DELETE FROM sys_dict_data WHERE type_code = 'equipment_maint_type' AND value IN ('refill', 'hydro_test');

COMMENT ON COLUMN public.equipment_maintenance.maintenance_type IS 'repair 维修充粉 / maintain 保养 / inspect 检测 / replace 更换 / ledger_fix 台账补录';
