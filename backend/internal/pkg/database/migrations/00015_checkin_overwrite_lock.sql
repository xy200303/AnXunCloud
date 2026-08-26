-- 打卡覆盖修改 + 报告归档锁定 + 逐项 AI 识别队列（全部幂等，仿 00011/00013）：
-- 1) checkin_record 加 locked_at（报告归档锁定）/superseded_by（被覆盖的旧记录指向新记录）；
-- 2) sys_config ai 分组新增 result_editable/worker_concurrency（不覆盖已有值）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS locked_at timestamptz;
COMMENT ON COLUMN checkin_record.locked_at IS '报告归档锁定时间（非空=已随周期报告归档，该点位打卡不可覆盖修改）';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS superseded_by varchar(36);
COMMENT ON COLUMN checkin_record.superseded_by IS '覆盖修改：被哪条新打卡记录覆盖（非空=已失效旧记录，列表/统计/报告均过滤）';

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.result_editable', '打卡结果允许修改', 'true', 'ai',
       '开启后同任务同点位未归档锁定的打卡允许重新提交覆盖（旧记录标记 superseded_by）；归档锁定后一律不可修改', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.result_editable' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'ai.worker_concurrency', '逐项 AI 识别队列并发数', '4', 'ai',
       '逐项 AI 识别队列（ai:item:queue）的消费 worker goroutine 数量，服务启动时读取', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'ai.worker_concurrency' AND deleted_at IS NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE checkin_record DROP COLUMN IF EXISTS superseded_by;
ALTER TABLE checkin_record DROP COLUMN IF EXISTS locked_at;
-- 配置项不回滚，避免误删已调整的 AI 配置。
-- +goose StatementEnd
