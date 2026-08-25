-- AI 巡检增强（《红色物业AI智能巡检增强方案》，全部幂等）：
-- 1) 检查项判定类型：check_template_item 加 judge_type/judge_config；checkin_record_item 同名快照 + ai_reading；
-- 2) checkin_record 加 force_submit/ai_quality_pass/ai_quality_issue（按月分区表，直接 ALTER 父表，同 00008 做法）；
-- 3) sys_config ai 分组新增 protocol/platform/sync_enabled/sync_timeout_seconds/max_photo_attempts（不覆盖已有值，仿 00011）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE check_template_item ADD COLUMN IF NOT EXISTS judge_type varchar(24) NOT NULL DEFAULT 'general';
COMMENT ON COLUMN check_template_item.judge_type IS '判定类型：general/presence/damage/metric/state/label/passage/leak/indicator/tidiness/baseline（二期预留，一期按 general）';
ALTER TABLE check_template_item ADD COLUMN IF NOT EXISTS judge_config jsonb;
COMMENT ON COLUMN check_template_item.judge_config IS '判定参数（metric: {"metric":"温度","unit":"℃","min":0,"max":40}；state/indicator: {"expected":"..."}），NULL 按通用判定';

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS judge_type varchar(24) NOT NULL DEFAULT 'general';
COMMENT ON COLUMN checkin_record_item.judge_type IS '判定类型快照（打卡当时从模板项复制，历史记录不依赖模板表）';
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS judge_config jsonb;
COMMENT ON COLUMN checkin_record_item.judge_config IS '判定参数快照（打卡当时从模板项复制）';
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS ai_reading varchar(64);
COMMENT ON COLUMN checkin_record_item.ai_reading IS 'AI 读取的表计读数文本（metric 类检查项；NULL=无读数）';

ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS force_submit boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN checkin_record.force_submit IS '重拍次数用尽后强制提交（跳过同步 AI 判定，转人工复核）';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS ai_quality_pass boolean;
COMMENT ON COLUMN checkin_record.ai_quality_pass IS 'AI 照片质量判定（第一层）；NULL=未做质量判定';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS ai_quality_issue varchar(255) NOT NULL DEFAULT '';
COMMENT ON COLUMN checkin_record.ai_quality_issue IS 'AI 照片质量问题描述（质量不达标时的重拍提示）';

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.protocol', 'AI 接口协议', 'openai_chat', 'ai',
       'openai_chat（OpenAI 兼容 Chat Completions）/openai_responses/gemini/claude', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.protocol' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.platform', 'AI 平台预设', '', 'ai',
       '平台预设标识（仅展示用，如 openai/qwen/doubao/gemini/claude）；实际生效以 protocol/base_url/model 为准', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.platform' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.sync_enabled', '打卡同步 AI 判定', 'true', 'ai',
       '开启后打卡提交时同步做质量+内容两层判定：质量不达标拒绝打卡，失败/超时放行转人工复核', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.sync_enabled' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.sync_timeout_seconds', '同步判定超时秒数', '15', 'ai',
       '打卡同步 AI 判定的超时时间；超时按失败放行（ai_verdict=error 转人工复核）', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.sync_timeout_seconds' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.max_photo_attempts', '照片质量重拍放行次数', '3', 'ai',
       '照片质量不达标允许重拍的次数，达到上限后 App 端可强制提交（App 端读取）', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.max_photo_attempts' AND deleted_at IS NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE checkin_record DROP COLUMN IF EXISTS ai_quality_issue;
ALTER TABLE checkin_record DROP COLUMN IF EXISTS ai_quality_pass;
ALTER TABLE checkin_record DROP COLUMN IF EXISTS force_submit;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS ai_reading;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS judge_config;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS judge_type;
ALTER TABLE check_template_item DROP COLUMN IF EXISTS judge_config;
ALTER TABLE check_template_item DROP COLUMN IF EXISTS judge_type;
-- 配置项不回滚，避免误删已调整的 AI 配置。
-- +goose StatementEnd
