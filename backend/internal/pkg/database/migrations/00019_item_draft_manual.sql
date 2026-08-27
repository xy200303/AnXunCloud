-- 逐项草稿支持手动确认项（有无异味等感官项）：巡检员选择即落云端草稿。
-- manual_pass/manual_note 与 AI 字段互不干扰（ai_verdict 保持 NULL，最终提交时不会被误当 AI 结论）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS manual_pass BOOLEAN NULL;
ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS manual_note VARCHAR(512) NOT NULL DEFAULT '';
COMMENT ON COLUMN checkin_item_draft.manual_pass IS '手动项选择结果（NULL=非手动项/未选择）';
COMMENT ON COLUMN checkin_item_draft.manual_note IS '手动项异常描述';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE checkin_item_draft DROP COLUMN IF EXISTS manual_pass;
ALTER TABLE checkin_item_draft DROP COLUMN IF EXISTS manual_note;

-- +goose StatementEnd
