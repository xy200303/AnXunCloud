-- +goose Up
-- owner: acceptance
CREATE TABLE acceptance_reopen_request (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    requester_id uuid NOT NULL,
    approver_id uuid,
    reason varchar(512) NOT NULL,
    status varchar(16) NOT NULL,
    decision_reason varchar(512) NOT NULL DEFAULT '',
    decided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_acceptance_reopen_status CHECK (status IN ('pending', 'approved', 'rejected'))
);
CREATE UNIQUE INDEX uk_acceptance_reopen_pending ON acceptance_reopen_request(batch_id) WHERE status = 'pending';
CREATE INDEX idx_acceptance_reopen_scope ON acceptance_reopen_request(tenant_id, batch_id, status);

INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '80000000-0000-0000-0000-000000000009', '80000000-0000-0000-0000-000000000002', '归档重开', '', '', 'button', 'acceptance:reopen', 5, true, 'enabled', true, false
WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE perms = 'acceptance:reopen');

-- +goose Down
SELECT 1;
