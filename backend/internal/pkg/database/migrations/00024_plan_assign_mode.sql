-- 计划点位分配方式：all=每个执行日巡全部点位（现状）；split=点位按周期内执行日数连续均分（大点位月检场景，
-- 一个计划圈全部点位，系统自动把点位均摊到每个执行日生成任务快照）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE inspection_plan ADD COLUMN IF NOT EXISTS assign_mode varchar(16) NOT NULL DEFAULT 'all';
ALTER TABLE inspection_plan DROP CONSTRAINT IF EXISTS chk_plan_assign_mode;
ALTER TABLE inspection_plan ADD CONSTRAINT chk_plan_assign_mode CHECK (assign_mode IN ('all', 'split'));
COMMENT ON COLUMN inspection_plan.assign_mode IS '点位分配方式：all 每个执行日全量 / split 按执行日均分（仅 weekly/monthly 合法）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE inspection_plan DROP COLUMN IF EXISTS assign_mode;

-- +goose StatementEnd
