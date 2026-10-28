-- +goose Up
-- owner: workorder
CREATE TABLE work_order_maintenance_item (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL REFERENCES work_order(id),
    issue_id uuid NOT NULL UNIQUE REFERENCES work_order_issue(id),
    equipment_id uuid NOT NULL,
    maintenance_action varchar(16) NOT NULL,
    due_period varchar(7) NOT NULL,
    maintenance_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_workorder_maintenance_item UNIQUE (tenant_id, order_id, equipment_id, maintenance_action)
);
CREATE INDEX idx_workorder_maintenance_due ON work_order_maintenance_item(tenant_id, equipment_id, maintenance_action, due_period);

-- +goose Down
SELECT 1;
