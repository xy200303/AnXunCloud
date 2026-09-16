-- 00054：每月抽查计划（与巡检计划同表分级，plan_kind 区分）：
--   inspection_plan 增 plan_kind（patrol 巡检默认 / spotcheck 抽查）与 spotcheck_config
--     （JSONB：ratio_percent 抽查比例 / fixed_count 固定数量（二选一，fixed_count>0 优先）/
--      strategy 抽取策略 random|longest_unseen / no_repeat 连中保护 / due_day 完成期限日，0 或 -1=月末）；
--   inspection_task 增 due_date（抽查任务完成期限，期限日次日才翻转逾期；日常任务为空）。

-- +goose Up

ALTER TABLE inspection_plan ADD COLUMN IF NOT EXISTS plan_kind varchar(16) NOT NULL DEFAULT 'patrol';
ALTER TABLE inspection_plan ADD COLUMN IF NOT EXISTS spotcheck_config jsonb;
ALTER TABLE inspection_task ADD COLUMN IF NOT EXISTS due_date date;

CREATE INDEX IF NOT EXISTS idx_plan_kind ON inspection_plan (plan_kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_task_due_date ON inspection_task (due_date) WHERE deleted_at IS NULL AND due_date IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_task_due_date;
DROP INDEX IF EXISTS idx_plan_kind;
ALTER TABLE inspection_task DROP COLUMN IF EXISTS due_date;
ALTER TABLE inspection_plan DROP COLUMN IF EXISTS spotcheck_config;
ALTER TABLE inspection_plan DROP COLUMN IF EXISTS plan_kind;
