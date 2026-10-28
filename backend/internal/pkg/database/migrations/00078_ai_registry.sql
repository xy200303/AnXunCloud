-- +goose Up
-- owner: ai
CREATE TABLE ai_provider (
    id uuid PRIMARY KEY, scope varchar(16) NOT NULL, scope_id uuid NOT NULL,
    code varchar(64) NOT NULL, name varchar(128) NOT NULL, protocol varchar(32) NOT NULL,
    base_url varchar(512) NOT NULL, credential_ref varchar(128) NOT NULL,
    status varchar(16) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_ai_provider_code UNIQUE (scope, scope_id, code),
    CONSTRAINT chk_ai_provider_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_ai_provider_protocol CHECK (protocol IN ('openai_chat', 'openai_responses', 'gemini', 'claude')),
    CONSTRAINT chk_ai_provider_status CHECK (status IN ('enabled', 'disabled'))
);
CREATE TABLE ai_model (
    id uuid PRIMARY KEY, scope varchar(16) NOT NULL, scope_id uuid NOT NULL,
    provider_code varchar(64) NOT NULL, model_id varchar(128) NOT NULL, display_name varchar(128) NOT NULL,
    context_window integer NOT NULL, max_output integer NOT NULL, capabilities jsonb NOT NULL DEFAULT '[]',
    status varchar(16) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_ai_model_id UNIQUE (scope, scope_id, provider_code, model_id),
    CONSTRAINT chk_ai_model_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_ai_model_status CHECK (status IN ('enabled', 'disabled')),
    CONSTRAINT chk_ai_model_limits CHECK (context_window > 0 AND max_output > 0)
);
CREATE TABLE ai_task_binding (
    id uuid PRIMARY KEY, scope varchar(16) NOT NULL, scope_id uuid NOT NULL,
    task_type varchar(48) NOT NULL, provider_code varchar(64) NOT NULL, model_id varchar(128) NOT NULL,
    timeout_seconds integer NOT NULL, max_retries integer NOT NULL DEFAULT 0, custom_prompt text NOT NULL DEFAULT '',
    status varchar(16) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_ai_task_binding UNIQUE (scope, scope_id, task_type),
    CONSTRAINT chk_ai_task_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_ai_task_type CHECK (task_type IN ('image_inspection', 'meter_reading', 'label_date_ocr', 'doc_scan_extract', 'assistant', 'agent_review')),
    CONSTRAINT chk_ai_task_status CHECK (status IN ('enabled', 'disabled')),
    CONSTRAINT chk_ai_task_policy CHECK (timeout_seconds BETWEEN 1 AND 300 AND max_retries BETWEEN 0 AND 3)
);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '78000000-0000-0000-0000-000000000001', (SELECT id FROM sys_menu WHERE path='/system' AND type='dir' LIMIT 1), 'AI 配置', '/system/ai-config', 'MagicStick', 'menu', 'ai:config:list', 9, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/system/ai-config');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '78000000-0000-0000-0000-000000000002', '78000000-0000-0000-0000-000000000001', '保存 AI 配置', '', '', 'button', 'ai:config:update', 1, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE perms='ai:config:update');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '78000000-0000-0000-0000-000000000003', (SELECT id FROM sys_menu WHERE path='/platform' AND type='dir' LIMIT 1), 'AI 配置', '/platform/ai-config', 'MagicStick', 'menu', 'ai:config:list', 8, true, 'enabled', true, true WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/platform/ai-config');
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r JOIN sys_menu m ON m.id IN ('78000000-0000-0000-0000-000000000001', '78000000-0000-0000-0000-000000000002')
WHERE r.code IN ('tenant_admin', 'super_admin') ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, '78000000-0000-0000-0000-000000000003'::uuid FROM sys_role r WHERE r.code='super_admin' ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
