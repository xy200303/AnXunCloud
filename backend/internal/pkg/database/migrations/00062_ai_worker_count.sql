-- 00062：AI 逐项识别并发数统一为 ai.worker_count=8。
-- 新代码读 ai.worker_count（缺省回退旧键 ai.worker_concurrency 再缺省 8）；
-- 存量库种子行是旧键值 4，这里把旧键值升到 8 并写入新键，统一并发口径。幂等可重复执行。

-- +goose Up

INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.worker_count', 'AI 逐项识别并发数', '8', 'ai',
       '逐项识别 worker 并发路数（旧键 ai.worker_concurrency 仍兼容回退）', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'ai.worker_count' AND deleted_at IS NULL);

UPDATE sys_config SET value = '8', updated_at = now()
WHERE key = 'ai.worker_concurrency' AND deleted_at IS NULL AND value <> '8';

-- +goose Down

UPDATE sys_config SET value = '4', updated_at = now()
WHERE key = 'ai.worker_concurrency' AND deleted_at IS NULL;
DELETE FROM sys_config WHERE key = 'ai.worker_count';
