-- +goose Up
-- owner: acceptance
ALTER TABLE acceptance_batch ADD COLUMN review_step integer NOT NULL DEFAULT 0;
CREATE TABLE acceptance_review_vote (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    step_index integer NOT NULL, user_id uuid NOT NULL, action varchar(16) NOT NULL,
    reason varchar(512) NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_vote UNIQUE (batch_id, step_index, user_id),
    CONSTRAINT chk_acceptance_vote_action CHECK (action IN ('approve', 'reject'))
);
CREATE TABLE acceptance_sign_subject (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    code varchar(64) NOT NULL, name varchar(128) NOT NULL, role_code varchar(64) NOT NULL DEFAULT '',
    external boolean NOT NULL DEFAULT false, required boolean NOT NULL DEFAULT true, sort integer NOT NULL DEFAULT 0,
    CONSTRAINT uk_acceptance_subject UNIQUE (batch_id, code)
);
CREATE TABLE acceptance_signature (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    revision integer NOT NULL, subject_code varchar(64) NOT NULL, representative varchar(128) NOT NULL,
    user_id uuid NOT NULL, file_id uuid NOT NULL REFERENCES upload_file(id), report_sha256 varchar(64) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'valid', invalid_reason varchar(512) NOT NULL DEFAULT '', signed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_signature UNIQUE (batch_id, revision, subject_code, report_sha256),
    CONSTRAINT chk_acceptance_signature_status CHECK (status IN ('valid', 'invalid'))
);
CREATE TABLE acceptance_report_file (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    revision integer NOT NULL, volume_code varchar(64) NOT NULL, volume_name varchar(128) NOT NULL,
    stage varchar(24) NOT NULL, format varchar(16) NOT NULL, file_id uuid REFERENCES upload_file(id),
    sha256 varchar(64) NOT NULL DEFAULT '', status varchar(16) NOT NULL, error_message varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_report_file UNIQUE (batch_id, revision, volume_code, stage, format),
    CONSTRAINT chk_acceptance_report_stage CHECK (stage IN ('frozen', 'archived')),
    CONSTRAINT chk_acceptance_report_format CHECK (format IN ('docx', 'pdf')),
    CONSTRAINT chk_acceptance_report_status CHECK (status IN ('running', 'ready', 'failed'))
);
CREATE TABLE acceptance_handover_log (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    action varchar(32) NOT NULL, actor_id uuid NOT NULL, from_status varchar(24) NOT NULL DEFAULT '',
    to_status varchar(24) NOT NULL, reason varchar(512) NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_acceptance_handover_log ON acceptance_handover_log (batch_id, created_at, id);

-- +goose Down
SELECT 1;
