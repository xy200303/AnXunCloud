-- +goose Up
-- owner: ai
CREATE TABLE ai_conversation (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, user_id uuid NOT NULL,
    title varchar(128) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_conversation_user ON ai_conversation (tenant_id, user_id, updated_at DESC);
CREATE TABLE ai_message (
    id uuid PRIMARY KEY, conversation_id uuid NOT NULL REFERENCES ai_conversation(id) ON DELETE CASCADE,
    role varchar(16) NOT NULL, content text NOT NULL, provider_code varchar(64) NOT NULL DEFAULT '', model_id varchar(128) NOT NULL DEFAULT '',
    prompt_tokens integer NOT NULL DEFAULT 0, completion_tokens integer NOT NULL DEFAULT 0,
    status varchar(16) NOT NULL, error_message varchar(512) NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_ai_message_role CHECK (role IN ('user', 'assistant')),
    CONSTRAINT chk_ai_message_status CHECK (status IN ('ready', 'failed'))
);
CREATE INDEX idx_ai_message_conversation ON ai_message (conversation_id, created_at, id);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '79000000-0000-0000-0000-000000000001', NULL, '智能助手', '/assistant', 'ChatDotRound', 'dir', '', 46, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/assistant');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '79000000-0000-0000-0000-000000000002', '79000000-0000-0000-0000-000000000001', '对话', '/assistant/chat', 'ChatLineRound', 'menu', 'assistant:chat', 1, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/assistant/chat');
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m WHERE m.id IN ('79000000-0000-0000-0000-000000000001', '79000000-0000-0000-0000-000000000002') ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
