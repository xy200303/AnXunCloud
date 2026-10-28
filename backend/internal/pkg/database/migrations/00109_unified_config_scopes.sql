-- +goose Up
-- owner: system
ALTER TABLE sys_config ADD COLUMN scope varchar(16) NOT NULL DEFAULT 'platform';
ALTER TABLE sys_config ADD COLUMN scope_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
DROP INDEX IF EXISTS uk_sys_config_key;
INSERT INTO sys_config (id, key, name, value, config_group, remark, scope, scope_id, created_at, updated_at)
SELECT gen_random_uuid(), key, key, value, 'tenant', '由 tenant_config 合并', 'tenant', tenant_id, created_at, updated_at FROM tenant_config;
DROP TABLE tenant_config;
ALTER TABLE sys_config RENAME TO config;
CREATE UNIQUE INDEX uk_sys_config_scope_key ON config(scope, scope_id, key) WHERE deleted_at IS NULL;
ALTER TABLE config ADD CONSTRAINT chk_sys_config_scope CHECK (scope IN ('platform', 'tenant', 'project'));
ALTER TABLE config ADD COLUMN allow_tenant_override boolean NOT NULL DEFAULT false;
ALTER TABLE config ADD COLUMN is_secret boolean NOT NULL DEFAULT false;
UPDATE config SET allow_tenant_override = true
WHERE scope = 'platform' AND key IN ('site.company_name', 'site.slogan', 'site.theme_color', 'site.footer_note', 'site.contact_phone', 'site.contact_email');
UPDATE config SET is_secret = true
WHERE scope = 'platform' AND (key ILIKE '%api_key%' OR key ILIKE '%secret%' OR key ILIKE '%password%' OR key ILIKE '%access_key%');
ALTER TABLE config ADD CONSTRAINT chk_config_tenant_override
CHECK (scope = 'platform' OR (allow_tenant_override = false AND is_secret = false));

ALTER TABLE approval_flow ADD COLUMN scope varchar(16) NOT NULL DEFAULT 'platform';
ALTER TABLE approval_flow ADD COLUMN scope_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
UPDATE approval_flow SET scope = 'tenant', scope_id = tenant_id WHERE tenant_id IS NOT NULL AND project_id IS NULL;
UPDATE approval_flow SET scope = 'project', scope_id = project_id WHERE project_id IS NOT NULL;
ALTER TABLE approval_flow DROP COLUMN tenant_id;
ALTER TABLE approval_flow DROP COLUMN project_id;
DROP INDEX IF EXISTS uk_approval_flow_scope;
CREATE UNIQUE INDEX uk_approval_flow_scope ON approval_flow(scope, scope_id, flow_code);
ALTER TABLE approval_flow ADD CONSTRAINT chk_approval_flow_scope CHECK (scope IN ('platform', 'tenant', 'project'));

-- +goose Down
SELECT 1;
