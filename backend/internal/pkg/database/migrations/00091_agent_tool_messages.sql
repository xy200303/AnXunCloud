-- +goose Up
-- owner: ai
ALTER TABLE ai_message DROP CONSTRAINT chk_ai_message_role;
ALTER TABLE ai_message ADD CONSTRAINT chk_ai_message_role CHECK (role IN ('user', 'assistant', 'tool'));

-- +goose Down
SELECT 1;
