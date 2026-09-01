-- 00030：明细策略 detail_mode（full=全量点位 / abnormal=仅异常点位），
-- 报告生成计划与报告本体同口径（手动生成可覆盖；重建沿用报告上的口径）。

-- +goose Up
ALTER TABLE report_plan ADD COLUMN IF NOT EXISTS detail_mode varchar(16) NOT NULL DEFAULT 'full';
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS detail_mode varchar(16) NOT NULL DEFAULT 'full';

-- +goose Down
ALTER TABLE report_plan DROP COLUMN IF EXISTS detail_mode;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS detail_mode;
