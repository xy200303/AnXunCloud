-- +goose Up
-- owner: workorder
CREATE TABLE work_order (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    order_no varchar(32) NOT NULL UNIQUE,
    work_type varchar(32) NOT NULL,
    title varchar(255) NOT NULL,
    status varchar(24) NOT NULL,
    priority varchar(16) NOT NULL DEFAULT 'normal',
    assignee_id uuid,
    deadline timestamptz,
    review_flow jsonb NOT NULL DEFAULT '[]',
    review_step integer NOT NULL DEFAULT 0,
    revision integer NOT NULL DEFAULT 1,
    close_reason varchar(512) NOT NULL DEFAULT '',
    closed_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT chk_work_order_status CHECK (status IN ('pending', 'in_progress', 'pending_review', 'done', 'closed')),
    CONSTRAINT chk_work_order_type CHECK (work_type IN ('inspection_fix', 'maintenance'))
);
CREATE INDEX idx_work_order_pool ON work_order (tenant_id, community_id, status, deadline) WHERE deleted_at IS NULL;
CREATE INDEX idx_work_order_assignee ON work_order (assignee_id, status) WHERE deleted_at IS NULL;

CREATE TABLE work_order_issue (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    order_id uuid NOT NULL REFERENCES work_order(id),
    source_type varchar(48) NOT NULL,
    source_record_id uuid NOT NULL,
    source_item_id uuid NOT NULL,
    source_revision integer NOT NULL DEFAULT 1,
    title varchar(255) NOT NULL,
    problem text NOT NULL DEFAULT '',
    location varchar(255) NOT NULL DEFAULT '',
    equipment_id uuid,
    photos jsonb NOT NULL DEFAULT '[]',
    status varchar(24) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_work_order_issue_source UNIQUE (tenant_id, source_type, source_item_id),
    CONSTRAINT chk_work_order_issue_status CHECK (status IN ('pending', 'in_progress', 'pending_review', 'done', 'closed'))
);
CREATE INDEX idx_work_order_issue_order ON work_order_issue (order_id, status);

CREATE TABLE work_order_fix_attempt (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES work_order(id),
    issue_id uuid NOT NULL REFERENCES work_order_issue(id),
    round integer NOT NULL,
    operator_id uuid NOT NULL,
    note text NOT NULL,
    photos jsonb NOT NULL DEFAULT '[]',
    submitted_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_work_order_fix_round UNIQUE (issue_id, round)
);

CREATE TABLE work_order_event (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL REFERENCES work_order(id),
    issue_id uuid,
    event_type varchar(32) NOT NULL,
    operation_id varchar(64) NOT NULL,
    actor_id uuid NOT NULL,
    from_status varchar(24) NOT NULL DEFAULT '',
    to_status varchar(24) NOT NULL,
    revision integer NOT NULL,
    reason varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_work_order_event_operation UNIQUE (order_id, operation_id)
);
CREATE INDEX idx_work_order_event_timeline ON work_order_event (order_id, created_at, id);

ALTER TABLE project_staff ADD COLUMN temporary_order_id uuid REFERENCES work_order(id);
CREATE INDEX idx_project_staff_temporary_order ON project_staff (temporary_order_id) WHERE temporary_order_id IS NOT NULL;

UPDATE sys_role SET claim_work_types = '["inspection_fix"]' WHERE code = 'inspector';
UPDATE sys_role SET claim_work_types = '["maintenance"]' WHERE code = 'maintainer';

INSERT INTO approval_flow (id, tenant_id, project_id, flow_code, steps)
SELECT md5('workorder-flow:' || COALESCE(id::text, 'platform'))::uuid,
       id, NULL, 'workorder_review', '[{"role_code":"project_manager","name":"工单审核","mode":"any"}]'
FROM tenant
ON CONFLICT DO NOTHING;
INSERT INTO approval_flow (id, tenant_id, project_id, flow_code, steps)
SELECT md5('workorder-flow:platform')::uuid, NULL, NULL, 'workorder_review',
       '[{"role_code":"project_manager","name":"工单审核","mode":"any"}]'
ON CONFLICT DO NOTHING;

INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000001', NULL, '工单管理', '/workorder', 'Tickets', 'dir', '', 35, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000002', '72000000-0000-0000-0000-000000000001', '工单池', '/workorder/pool', 'List', 'menu', 'workorder:list', 1, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000003', '72000000-0000-0000-0000-000000000002', '抢单', '', '', 'button', 'workorder:claim', 1, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000004', '72000000-0000-0000-0000-000000000002', '分派', '', '', 'button', 'workorder:assign', 2, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000005', '72000000-0000-0000-0000-000000000002', '整改提交', '', '', 'button', 'workorder:finish', 3, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000006', '72000000-0000-0000-0000-000000000002', '关闭工单', '', '', 'button', 'workorder:close', 4, true, 'enabled', true, false) ON CONFLICT DO NOTHING;
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES ('72000000-0000-0000-0000-000000000007', '72000000-0000-0000-0000-000000000002', '导出工单', '', '', 'button', 'workorder:export', 5, true, 'enabled', true, false) ON CONFLICT DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m
WHERE m.id IN ('72000000-0000-0000-0000-000000000001', '72000000-0000-0000-0000-000000000002')
  AND r.code IN ('project_manager', 'quality_supervisor', 'engineer_supervisor', 'tenant_admin', 'inspector', 'maintainer')
ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m
WHERE m.id IN ('72000000-0000-0000-0000-000000000003', '72000000-0000-0000-0000-000000000005')
  AND r.code IN ('inspector', 'maintainer')
ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m
WHERE m.id IN ('72000000-0000-0000-0000-000000000004', '72000000-0000-0000-0000-000000000006', '72000000-0000-0000-0000-000000000007')
  AND r.code IN ('project_manager', 'quality_supervisor', 'engineer_supervisor', 'tenant_admin')
ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
