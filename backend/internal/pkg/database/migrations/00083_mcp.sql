-- +goose Up
-- owner: ai
CREATE TABLE ai_mcp_token (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, user_id uuid NOT NULL, name varchar(128) NOT NULL,
    prefix varchar(24) NOT NULL, secret_hash varchar(64) NOT NULL UNIQUE, scopes jsonb NOT NULL DEFAULT '[]',
    expires_at timestamptz, revoked_at timestamptz, last_used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_mcp_token_tenant ON ai_mcp_token (tenant_id, created_at DESC);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '83000000-0000-0000-0000-000000000001', (SELECT id FROM sys_menu WHERE path='/assistant' LIMIT 1), '助手设置', '/assistant/settings', 'Setting', 'menu', 'assistant:settings', 2, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/assistant/settings');
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, '83000000-0000-0000-0000-000000000001'::uuid FROM sys_role r WHERE r.code IN ('super_admin', 'tenant_admin') ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
