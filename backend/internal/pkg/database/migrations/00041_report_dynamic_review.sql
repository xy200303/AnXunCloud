-- 00041：报告审核链动态化。流程由 approval_flow(flow_code=report_review) 配置，
-- inspection_report 保存生成时的步骤与候选人快照。

-- +goose Up
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS review_steps jsonb NOT NULL DEFAULT '[]';
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS review_step integer NOT NULL DEFAULT 0;
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS review_current_ids jsonb NOT NULL DEFAULT '[]';
CREATE INDEX IF NOT EXISTS idx_inspection_report_review_current_ids
    ON inspection_report USING gin (review_current_ids);

-- +goose Down
DROP INDEX IF EXISTS idx_inspection_report_review_current_ids;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS review_current_ids;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS review_step;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS review_steps;
