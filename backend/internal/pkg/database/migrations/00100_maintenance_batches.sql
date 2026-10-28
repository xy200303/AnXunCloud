-- +goose Up
-- owner: equipment
ALTER TABLE equipment_maintenance ADD COLUMN community_id uuid;
ALTER TABLE equipment_maintenance ADD COLUMN batch_registration_id uuid;
ALTER TABLE equipment_maintenance ADD COLUMN batch_item_id uuid;
UPDATE equipment_maintenance SET community_id = e.community_id FROM equipment e WHERE e.id = equipment_maintenance.equipment_id AND equipment_maintenance.community_id IS NULL;
ALTER TABLE equipment_maintenance ALTER COLUMN community_id SET NOT NULL;
CREATE UNIQUE INDEX uk_maintenance_batch_effective ON equipment_maintenance (tenant_id, batch_registration_id, batch_item_id, maintenance_type) WHERE batch_registration_id IS NOT NULL AND batch_item_id IS NOT NULL;
CREATE INDEX idx_maintenance_community ON equipment_maintenance(tenant_id, community_id, confirm_status, maintenance_date);

CREATE TABLE maintenance_selection (
    token uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    action varchar(32) NOT NULL,
    operation_id varchar(64) NOT NULL,
    payload_hash varchar(64) NOT NULL,
    equipment_ids jsonb NOT NULL,
    excluded_ids jsonb NOT NULL DEFAULT '[]',
    filter jsonb NOT NULL DEFAULT '{}',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_maintenance_selection_operation UNIQUE (tenant_id, actor_id, action, operation_id)
);

CREATE TABLE maintenance_batch (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    maintenance_type varchar(16) NOT NULL,
    maintenance_date date NOT NULL,
    status varchar(16) NOT NULL,
    revision integer NOT NULL DEFAULT 1,
    review_flow jsonb NOT NULL,
    confirm_step integer NOT NULL DEFAULT 0,
    operation_id varchar(64) NOT NULL,
    payload_hash varchar(64) NOT NULL,
    operator_name varchar(64) NOT NULL DEFAULT '',
    vendor varchar(128),
    note text NOT NULL DEFAULT '',
    file_ids jsonb NOT NULL DEFAULT '[]',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    confirmed_at timestamptz,
    reject_reason varchar(512),
    CONSTRAINT chk_maintenance_batch_status CHECK (status IN ('pending', 'rejected', 'confirmed')),
    CONSTRAINT uk_maintenance_batch_operation UNIQUE (tenant_id, created_by, operation_id)
);
CREATE TABLE maintenance_batch_item (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    batch_id uuid NOT NULL REFERENCES maintenance_batch(id),
    equipment_id uuid NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'pending',
    note text NOT NULL DEFAULT '',
    file_ids jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_maintenance_batch_item UNIQUE (batch_id, equipment_id),
    CONSTRAINT chk_maintenance_batch_item_status CHECK (status IN ('pending', 'confirmed', 'rejected'))
);
CREATE INDEX idx_maintenance_batch_item_equipment ON maintenance_batch_item(tenant_id, equipment_id, status);

-- +goose Down
SELECT 1;
