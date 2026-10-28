-- +goose Up
-- owner: acceptance
ALTER TABLE acceptance_target ADD COLUMN table_codes jsonb NOT NULL DEFAULT '[]'::jsonb;
UPDATE acceptance_target t
SET table_codes = COALESCE((SELECT jsonb_agg(DISTINCT r.table_code) FROM acceptance_record r WHERE r.target_id = t.id), '[]'::jsonb);
UPDATE acceptance_target t
SET table_codes = (
    SELECT jsonb_agg(s.code) FROM acceptance_list_schema s
    WHERE s.category = t.category_code AND s.status = 'enabled'
)
WHERE table_codes = '[]'::jsonb
  AND (SELECT COUNT(*) FROM acceptance_target other WHERE other.batch_id = t.batch_id AND other.category_code = t.category_code) = 1;
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM acceptance_target t JOIN acceptance_batch b ON b.id = t.batch_id
        WHERE t.table_codes = '[]'::jsonb AND b.status <> 'draft'
    ) THEN
        RAISE EXCEPTION '存在未明确专项的非草稿承接对象，请先人工指定 table_codes 再发布';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE acceptance_problem ADD COLUMN risk_level varchar(16) NOT NULL DEFAULT 'normal';
ALTER TABLE acceptance_problem ADD COLUMN safety_measure text NOT NULL DEFAULT '';
ALTER TABLE acceptance_problem ADD COLUMN measure_approved boolean NOT NULL DEFAULT false;
ALTER TABLE acceptance_problem ADD COLUMN accepted boolean NOT NULL DEFAULT false;
ALTER TABLE acceptance_problem ADD COLUMN accepted_by varchar(128) NOT NULL DEFAULT '';
ALTER TABLE acceptance_problem ADD COLUMN accepted_at timestamptz;
ALTER TABLE acceptance_problem ADD COLUMN acceptance_file_id uuid;
ALTER TABLE acceptance_problem ADD CONSTRAINT chk_acceptance_problem_risk CHECK (risk_level IN ('normal', 'major'));

-- +goose Down
SELECT 1;
