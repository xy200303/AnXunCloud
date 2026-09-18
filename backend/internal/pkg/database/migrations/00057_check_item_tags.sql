-- 00057：检查项 tag 数组（甲方口径：一项一张照片，细分观察点收敛为项内 tag）。
-- 模板项 tags=观察点标签数组（如 水带在位/箱门完好）；打卡项快照 tags + 提交回传 abnormal_tags（异常 tag 列表，
-- 非空即该项判异常）；逐项 AI 草稿 abnormal_tags 供断点恢复与过程查看。全部幂等可重复执行。

-- +goose Up

ALTER TABLE check_template_item ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN public.check_template_item.tags IS '观察点 tag 数组（一项一张照片，tag 不带图；空=无 tag 的传统项）';

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN public.checkin_record_item.tags IS 'tag 快照（打卡当时从模板项复制，与 name/requirement 同机制）';

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS abnormal_tags jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN public.checkin_record_item.abnormal_tags IS '异常 tag 列表（⊆ tags；非空=该项异常，服务端强制 pass=false）';

ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS abnormal_tags jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN public.checkin_item_draft.abnormal_tags IS '逐项 AI 识别的异常 tag（断点恢复/过程查看）';

-- +goose Down

ALTER TABLE check_template_item DROP COLUMN IF EXISTS tags;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS tags;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS abnormal_tags;
ALTER TABLE checkin_item_draft DROP COLUMN IF EXISTS abnormal_tags;
