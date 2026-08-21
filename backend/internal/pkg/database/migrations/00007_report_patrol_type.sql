-- 报告按巡查类型通用生成（《专项巡检与专项检查报告设计方案》§3.4，全部幂等）：
-- inspection_report 加 patrol_type（空=综合月报）与 plan_id（溯源）；
-- 唯一索引扩展为 (community_id, period, patrol_type)，patrol_type 归一 ''，存量 NULL 行天然兼容。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS patrol_type varchar(32);
COMMENT ON COLUMN inspection_report.patrol_type IS '巡查类型（字典 patrol_type 的 value）；NULL/空 = 综合月报（全类型口径），非空 = 该类型专项检查报告';

ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS plan_id uuid;
COMMENT ON COLUMN inspection_report.plan_id IS '溯源生成它的巡检计划（仅记录，综合月报为 NULL）';

-- 唯一性调为 (community_id, period, patrol_type)：同项目同月综合月报一份、每个专项类型各一份，互不冲突；
-- patrol_type 归一 ''（存量行 NULL 不冲突），表达式索引重建无数据冲突（旧索引更严格）
DROP INDEX IF EXISTS uk_inspection_report;
CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period, (COALESCE(patrol_type, ''::character varying))) WHERE (deleted_at IS NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uk_inspection_report;
CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period) WHERE (deleted_at IS NULL);
ALTER TABLE inspection_report DROP COLUMN IF EXISTS plan_id;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS patrol_type;
-- +goose StatementEnd
