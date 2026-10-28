-- +goose Up
-- owner: inspection
ALTER TABLE checkin_record ADD COLUMN round_seq integer NOT NULL DEFAULT 1;
ALTER TABLE checkin_record ADD COLUMN revision integer NOT NULL DEFAULT 1;
ALTER TABLE checkin_record ADD COLUMN original_record_id uuid;
CREATE UNIQUE INDEX uk_checkin_record_round ON checkin_record (tenant_id, task_id, point_id, round_seq, created_at);
CREATE INDEX idx_checkin_record_original ON checkin_record(original_record_id) WHERE original_record_id IS NOT NULL;

CREATE TABLE inspection_point_config_block (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    point_id uuid NOT NULL REFERENCES inspection_point(id),
    reason varchar(255) NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz
);
CREATE UNIQUE INDEX uk_point_config_block_active ON inspection_point_config_block(point_id) WHERE ended_at IS NULL;
CREATE INDEX idx_point_config_block_scope ON inspection_point_config_block(tenant_id, community_id, ended_at);

-- +goose Down
SELECT 1;
