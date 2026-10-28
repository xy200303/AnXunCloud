-- +goose Up
-- owner: report
CREATE TABLE report_render_task (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    report_id uuid NOT NULL REFERENCES inspection_report(id),
    stage varchar(24) NOT NULL,
    status varchar(16) NOT NULL,
    error_message varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_report_render_task UNIQUE (report_id, stage),
    CONSTRAINT chk_report_render_stage CHECK (stage IN ('frozen', 'archived')),
    CONSTRAINT chk_report_render_status CHECK (status IN ('pending', 'running', 'done', 'failed'))
);
CREATE INDEX idx_report_render_status ON report_render_task(tenant_id, status, updated_at);

-- +goose Down
SELECT 1;
