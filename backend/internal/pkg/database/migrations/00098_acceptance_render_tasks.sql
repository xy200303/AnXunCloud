-- +goose Up
-- owner: acceptance
CREATE TABLE acceptance_render_task (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    stage varchar(24) NOT NULL,
    status varchar(16) NOT NULL,
    error_message varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_render_task UNIQUE (batch_id, stage),
    CONSTRAINT chk_acceptance_render_stage CHECK (stage IN ('frozen', 'archived')),
    CONSTRAINT chk_acceptance_render_status CHECK (status IN ('pending', 'running', 'done', 'failed'))
);
CREATE INDEX idx_acceptance_render_status ON acceptance_render_task(tenant_id, status, updated_at);

-- +goose Down
SELECT 1;
