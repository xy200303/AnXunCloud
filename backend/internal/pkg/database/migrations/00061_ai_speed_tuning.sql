-- 00061：AI 提速配置——异步识别超时 60s→180s（视觉模型逐项识别排队+推理裕量）；
-- 新增 ai.disable_thinking=true（关闭模型思维链提速；qwen enable_thinking=false /
-- gemini thinkingBudget=0 / responses reasoning minimal，不识别的网关忽略）。幂等可重复执行。

-- +goose Up

UPDATE sys_config SET value = '180', updated_at = now()
WHERE key = 'ai.timeout_seconds' AND deleted_at IS NULL AND value <> '180';

INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.disable_thinking', '关闭思考模式', 'true', 'ai',
       '提速：关模型思维链（qwen enable_thinking/gemini thinkingBudget/responses minimal；false=保持模型默认）', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'ai.disable_thinking' AND deleted_at IS NULL);

-- +goose Down

UPDATE sys_config SET value = '60', updated_at = now()
WHERE key = 'ai.timeout_seconds' AND deleted_at IS NULL;
DELETE FROM sys_config WHERE key = 'ai.disable_thinking';
