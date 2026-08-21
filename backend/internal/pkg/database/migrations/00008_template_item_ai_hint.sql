-- 模板项 AI 识别要点（《专项巡检与专项检查报告设计方案》§3.3，全部幂等）：
-- check_template_item 加 ai_hint（空=该项不送 AI 逐项识别）；
-- checkin_record_item 加 ai_hint（打卡快照时从模板项复制，与 name/requirement 同机制）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE check_template_item ADD COLUMN IF NOT EXISTS ai_hint text;
COMMENT ON COLUMN check_template_item.ai_hint IS 'AI 识别要点（逐项送大模型的提示文本）；NULL/空 = 该项不带识别要点';

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS ai_hint text;
COMMENT ON COLUMN checkin_record_item.ai_hint IS 'AI 识别要点快照（打卡当时从模板项复制，历史记录不依赖模板表）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS ai_hint;
ALTER TABLE check_template_item DROP COLUMN IF EXISTS ai_hint;
-- +goose StatementEnd
