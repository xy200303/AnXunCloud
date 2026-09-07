-- 00042：移除报告固定三级审核字段，统一使用动态 review_steps。

-- +goose Up
ALTER TABLE inspection_report DROP COLUMN IF EXISTS supervisor_ids;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS manager_ids;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS supervisor_by;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS supervisor_at;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS supervisor_remark;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS manager_by;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS manager_at;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS manager_remark;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS supervisor_signature_id;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS manager_signature_id;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS inspector_signed;

-- +goose Down
-- 固定三级审核字段不再恢复，回滚仅回滚迁移版本记录。
