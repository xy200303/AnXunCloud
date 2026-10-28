-- +goose Up
-- owner: ai
CREATE TABLE ai_quota (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, task_type varchar(48) NOT NULL,
    max_requests_daily integer NOT NULL, max_tokens_daily bigint NOT NULL, max_concurrent integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_ai_quota_task UNIQUE (tenant_id, task_type),
    CONSTRAINT chk_ai_quota_task CHECK (task_type IN ('image_inspection', 'meter_reading', 'label_date_ocr', 'doc_scan_extract', 'assistant', 'agent_review')),
    CONSTRAINT chk_ai_quota_positive CHECK (max_requests_daily > 0 AND max_tokens_daily > 0 AND max_concurrent > 0)
);
CREATE TABLE ai_usage_reservation (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, task_type varchar(48) NOT NULL,
    status varchar(16) NOT NULL, started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
    prompt_tokens integer NOT NULL DEFAULT 0, completion_tokens integer NOT NULL DEFAULT 0,
    CONSTRAINT chk_ai_usage_status CHECK (status IN ('running', 'done', 'failed'))
);
CREATE INDEX idx_ai_usage_running ON ai_usage_reservation (tenant_id, task_type, status, started_at);

-- +goose Down
SELECT 1;
