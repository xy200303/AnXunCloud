-- +goose Up
-- owner: ai
CREATE TABLE ai_item_job (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    status varchar(16) NOT NULL,
    verdict varchar(16) NOT NULL DEFAULT '',
    reason text NOT NULL DEFAULT '',
    reading varchar(64) NOT NULL DEFAULT '',
    abnormal_tags jsonb NOT NULL DEFAULT '[]',
    quality_pass boolean,
    quality_issue varchar(255) NOT NULL DEFAULT '',
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_ai_item_job_status CHECK (status IN ('pending', 'running', 'done', 'failed'))
);
CREATE INDEX idx_ai_item_job_user ON ai_item_job(user_id, tenant_id, updated_at DESC);
CREATE INDEX idx_ai_item_job_status ON ai_item_job(tenant_id, status, updated_at);

-- +goose Down
SELECT 1;
