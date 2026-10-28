-- +goose Up
-- owner: ai
CREATE TABLE ai_agent_tool (
    id uuid PRIMARY KEY,
    scope varchar(16) NOT NULL,
    scope_id uuid NOT NULL,
    code varchar(64) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'enabled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (scope, scope_id, code),
    CONSTRAINT chk_agent_tool_scope CHECK (scope IN ('platform', 'tenant')),
    CONSTRAINT chk_agent_tool_status CHECK (status IN ('enabled', 'disabled'))
);
INSERT INTO ai_agent_tool (id, scope, scope_id, code, status)
SELECT '90000000-0000-0000-0000-000000000090', 'platform', '00000000-0000-0000-0000-000000000000', 'inspection.summary', 'enabled';

CREATE TABLE ai_agent_call (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    tool_code varchar(64) NOT NULL,
    arguments jsonb NOT NULL,
    result jsonb,
    status varchar(16) NOT NULL,
    error_message varchar(512) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_agent_call_conversation ON ai_agent_call (conversation_id, created_at);

ALTER TABLE ai_message ADD COLUMN tool_calls jsonb;
ALTER TABLE ai_message ADD COLUMN tool_call_id varchar(64) NOT NULL DEFAULT '';
ALTER TABLE ai_message ADD COLUMN tool_name varchar(64) NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
