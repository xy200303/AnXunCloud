-- +goose Up
-- owner: acceptance
CREATE TABLE acceptance_list_schema (
    id uuid PRIMARY KEY, scope varchar(16) NOT NULL, scope_id uuid NOT NULL, code varchar(64) NOT NULL,
    category varchar(32) NOT NULL, name varchar(128) NOT NULL, table_kind varchar(24) NOT NULL,
    template_id uuid, status varchar(16) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_schema UNIQUE (scope, scope_id, code),
    CONSTRAINT chk_acceptance_schema_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_acceptance_schema_kind CHECK (table_kind IN ('matrix', 'documents', 'keys', 'assets')),
    CONSTRAINT chk_acceptance_schema_status CHECK (status IN ('draft', 'enabled', 'disabled'))
);
CREATE TABLE acceptance_list_row (
    id uuid PRIMARY KEY, scope varchar(16) NOT NULL, scope_id uuid NOT NULL, table_code varchar(64) NOT NULL,
    code varchar(64) NOT NULL, name varchar(128) NOT NULL, specification varchar(255) NOT NULL DEFAULT '',
    unit varchar(16) NOT NULL DEFAULT '', required boolean NOT NULL DEFAULT false, sort integer NOT NULL DEFAULT 0,
    status varchar(16) NOT NULL DEFAULT 'enabled', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_row UNIQUE (scope, scope_id, table_code, code),
    CONSTRAINT chk_acceptance_row_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_acceptance_row_status CHECK (status IN ('enabled', 'disabled'))
);
CREATE TABLE acceptance_batch (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, community_id uuid NOT NULL, name varchar(128) NOT NULL,
    ledger_type varchar(32) NOT NULL, period_start timestamptz NOT NULL, period_end timestamptz NOT NULL,
    manager_id uuid NOT NULL, status varchar(24) NOT NULL, revision integer NOT NULL DEFAULT 1,
    review_flow jsonb NOT NULL DEFAULT '[]', template_snapshot jsonb NOT NULL DEFAULT '{}', locked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_acceptance_batch_type CHECK (ledger_type IN ('institution_change', 'unit_acceptance')),
    CONSTRAINT chk_acceptance_batch_status CHECK (status IN ('draft', 'inspecting', 'reviewing', 'handover', 'done')),
    CONSTRAINT chk_acceptance_batch_period CHECK (period_end > period_start)
);
CREATE INDEX idx_acceptance_batch_scope ON acceptance_batch (tenant_id, community_id, status, updated_at DESC);
CREATE TABLE acceptance_target (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    target_type varchar(24) NOT NULL, ref_id varchar(64) NOT NULL, category_code varchar(64) NOT NULL,
    name varchar(128) NOT NULL, location varchar(255) NOT NULL DEFAULT '', coverage_confirmed boolean NOT NULL DEFAULT false,
    covered_by uuid, covered_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_target UNIQUE (batch_id, target_type, ref_id, category_code),
    CONSTRAINT chk_acceptance_target_type CHECK (target_type IN ('documents', 'building', 'unit', 'area', 'equipment'))
);
CREATE TABLE acceptance_member (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    user_id uuid NOT NULL, work_scope varchar(16) NOT NULL, category_codes jsonb NOT NULL DEFAULT '[]', building_ids jsonb NOT NULL DEFAULT '[]',
    external_name varchar(128) NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_member UNIQUE (batch_id, user_id, work_scope),
    CONSTRAINT chk_acceptance_member_scope CHECK (work_scope IN ('docs', 'site'))
);
CREATE TABLE acceptance_record (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    target_id uuid NOT NULL REFERENCES acceptance_target(id), table_code varchar(64) NOT NULL, item_code varchar(64) NOT NULL,
    category_code varchar(64) NOT NULL, result varchar(24) NOT NULL, value_text text NOT NULL DEFAULT '', note text NOT NULL DEFAULT '',
    photos jsonb NOT NULL DEFAULT '[]', revision integer NOT NULL DEFAULT 1, created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_acceptance_record UNIQUE (tenant_id, batch_id, target_id, table_code, item_code),
    CONSTRAINT chk_acceptance_record_result CHECK (result IN ('missing', 'problem'))
);
CREATE TABLE acceptance_problem (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, batch_id uuid NOT NULL REFERENCES acceptance_batch(id),
    record_id uuid NOT NULL UNIQUE REFERENCES acceptance_record(id), fact text NOT NULL, location varchar(255) NOT NULL DEFAULT '',
    photos jsonb NOT NULL DEFAULT '[]', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '80000000-0000-0000-0000-000000000001', NULL, '承接查验', '/acceptance', 'Checked', 'dir', '', 37, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/acceptance');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '80000000-0000-0000-0000-000000000002', '80000000-0000-0000-0000-000000000001', '批次管理', '/acceptance/batches', 'Document', 'menu', 'acceptance:list', 1, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/acceptance/batches');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '80000000-0000-0000-0000-000000000003', (SELECT id FROM sys_menu WHERE path='/templates' LIMIT 1), '承接查验内容模板', '/templates/acceptance', 'Collection', 'menu', 'acceptance:schema:list', 2, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/templates/acceptance');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform) VALUES
('80000000-0000-0000-0000-000000000004', '80000000-0000-0000-0000-000000000002', '管理批次', '', '', 'button', 'acceptance:manage', 1, true, 'enabled', true, false),
('80000000-0000-0000-0000-000000000005', '80000000-0000-0000-0000-000000000002', '提交查验', '', '', 'button', 'acceptance:submit', 2, true, 'enabled', true, false),
('80000000-0000-0000-0000-000000000006', '80000000-0000-0000-0000-000000000002', '审核查验', '', '', 'button', 'acceptance:review', 3, true, 'enabled', true, false),
('80000000-0000-0000-0000-000000000007', '80000000-0000-0000-0000-000000000002', '签署交接', '', '', 'button', 'acceptance:sign', 4, true, 'enabled', true, false),
('80000000-0000-0000-0000-000000000008', '80000000-0000-0000-0000-000000000003', '维护内容模板', '', '', 'button', 'acceptance:schema:update', 1, true, 'enabled', true, false)
ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m
WHERE m.id IN ('80000000-0000-0000-0000-000000000001', '80000000-0000-0000-0000-000000000002', '80000000-0000-0000-0000-000000000003')
  AND r.code IN ('super_admin', 'tenant_admin', 'project_manager', 'quality_supervisor', 'engineer_supervisor') ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m
WHERE m.id IN ('80000000-0000-0000-0000-000000000004', '80000000-0000-0000-0000-000000000005', '80000000-0000-0000-0000-000000000006', '80000000-0000-0000-0000-000000000007', '80000000-0000-0000-0000-000000000008')
  AND r.code IN ('super_admin', 'tenant_admin', 'project_manager', 'quality_supervisor', 'engineer_supervisor') ON CONFLICT DO NOTHING;
ALTER TABLE upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (scene IN ('checkin', 'avatar', 'export', 'signature', 'seal', 'notice', 'app', 'equipment', 'workorder', 'acceptance'));

-- +goose Down
SELECT 1;
