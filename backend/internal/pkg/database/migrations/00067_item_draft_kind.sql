-- 00067：checkin_item_draft 加 draft_kind 显式分类（ai/manual/escape），废除 ai_verdict/manual_pass/exception_type 组合推导。
-- 回填口径：exception_type<>'' → escape；manual_pass 非空 → manual；其余 ai。幂等可重复执行。

-- +goose Up

ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS draft_kind varchar(16) NOT NULL DEFAULT 'ai';

UPDATE checkin_item_draft SET draft_kind = 'escape' WHERE exception_type <> '' AND draft_kind <> 'escape';
UPDATE checkin_item_draft SET draft_kind = 'manual' WHERE exception_type = '' AND manual_pass IS NOT NULL AND draft_kind <> 'manual';

ALTER TABLE checkin_item_draft DROP CONSTRAINT IF EXISTS chk_item_draft_kind;
ALTER TABLE checkin_item_draft
    ADD CONSTRAINT chk_item_draft_kind CHECK (draft_kind IN ('ai', 'manual', 'escape'));

COMMENT ON COLUMN checkin_item_draft.draft_kind IS '草稿分类：ai=逐项 AI 识别 / manual=手动确认项选择 / escape=拍照项逃生佐证（消费端一律按此分发，ai_status 仅 AI 链路语义）';

-- +goose Down

ALTER TABLE checkin_item_draft DROP CONSTRAINT IF EXISTS chk_item_draft_kind;
ALTER TABLE checkin_item_draft DROP COLUMN IF EXISTS draft_kind;
