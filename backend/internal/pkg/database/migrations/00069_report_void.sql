-- 00069：报告作废（归档留痕）。status 新增 voided 终态：作废不删行，签字流/统计数据原样保留；
-- void_reason 必填（作废原因），voided_by/voided_at 留痕。
-- 唯一索引 uk_inspection_report 排除 voided 行：作废后同小区同期间同类型可重新生成新报告（作废旧报告仍在）。

-- +goose Up

ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS void_reason varchar(512) NOT NULL DEFAULT '';
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS voided_by uuid;
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS voided_at timestamptz;

COMMENT ON COLUMN inspection_report.void_reason IS '作废原因（status=voided 时必填）';
COMMENT ON COLUMN inspection_report.voided_by IS '作废操作人';
COMMENT ON COLUMN inspection_report.voided_at IS '作废时间';

DROP INDEX IF EXISTS uk_inspection_report;
CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period, COALESCE(patrol_type, ''::character varying)) WHERE (deleted_at IS NULL AND status <> 'voided');

-- +goose Down

DROP INDEX IF EXISTS uk_inspection_report;
CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period, COALESCE(patrol_type, ''::character varying)) WHERE (deleted_at IS NULL);

ALTER TABLE inspection_report DROP COLUMN IF EXISTS voided_at;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS voided_by;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS void_reason;
