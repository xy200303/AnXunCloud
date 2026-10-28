-- +goose Up
-- owner: report
CREATE TABLE report_file (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    report_id uuid NOT NULL REFERENCES inspection_report(id),
    doc_kind varchar(24) NOT NULL DEFAULT 'monthly_report',
    volume_code varchar(64) NOT NULL DEFAULT 'legacy',
    volume_name varchar(128) NOT NULL DEFAULT '历史月报',
    stage varchar(24) NOT NULL,
    format varchar(16) NOT NULL,
    file_id uuid REFERENCES upload_file(id),
    sha256 varchar(64) NOT NULL DEFAULT '',
    generation_batch uuid NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'ready',
    error_message varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_report_file_stage CHECK (stage IN ('preview', 'frozen', 'archived', 'legacy_archive')),
    CONSTRAINT chk_report_file_format CHECK (format IN ('docx', 'pdf', 'xlsx', 'png')),
    CONSTRAINT chk_report_file_status CHECK (status IN ('pending', 'running', 'ready', 'failed')),
    CONSTRAINT chk_report_file_doc_kind CHECK (doc_kind IN ('monthly_report', 'acceptance', 'maintenance_ledger')),
    CONSTRAINT uk_report_file_generation UNIQUE (report_id, volume_code, stage, format, generation_batch)
);
CREATE INDEX idx_report_file_report ON report_file (report_id, stage, status, created_at);
INSERT INTO report_file (id, tenant_id, report_id, stage, format, file_id, generation_batch, status, created_at)
SELECT md5('legacy-report-file:' || r.id::text)::uuid, c.tenant_id, r.id, 'legacy_archive', 'pdf', r.file_id,
       md5('legacy-report-batch:' || r.id::text)::uuid, 'ready', r.updated_at
FROM inspection_report r JOIN community c ON c.id = r.community_id
WHERE r.file_id IS NOT NULL AND r.file_id::text <> '';
UPDATE inspection_report SET stats = jsonb_set(COALESCE(stats, '{}'::jsonb), '{seal_file_id}', to_jsonb(seal_file_id::text), true)
WHERE seal_file_id IS NOT NULL AND seal_file_id::text <> '';
ALTER TABLE inspection_report DROP COLUMN file_id;
ALTER TABLE inspection_report DROP COLUMN seal_file_id;

-- +goose Down
SELECT 1;
