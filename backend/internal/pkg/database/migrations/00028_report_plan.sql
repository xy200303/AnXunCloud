-- 00028：报告生成计划（report_plan）+ 报告期间范围与计划溯源。
-- 原调度器「每月 1 日为每个小区生成上月综合月报」的硬编码行为，迁移为每小区一条默认月报计划。

-- +goose Up

CREATE TABLE IF NOT EXISTS report_plan (
    id            varchar(36) PRIMARY KEY,
    tenant_id     uuid,
    community_id  uuid        NOT NULL,
    name          varchar(64) NOT NULL DEFAULT '',
    patrol_type   varchar(32) NOT NULL DEFAULT '',
    cycle_type    varchar(16) NOT NULL DEFAULT 'monthly',
    cycle_config  jsonb       NOT NULL DEFAULT '{}',
    gen_time      varchar(8)  NOT NULL DEFAULT '06:00',
    status        varchar(16) NOT NULL DEFAULT 'enabled',
    last_period   varchar(32) NOT NULL DEFAULT '',
    last_error    varchar(255) NOT NULL DEFAULT '',
    remark        varchar(255) NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_report_plan_community ON report_plan (community_id);

ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS report_plan_id uuid;
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS period_start timestamptz;
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS period_end timestamptz;
ALTER TABLE inspection_report ALTER COLUMN period TYPE varchar(32);

-- 计划生成幂等：同计划同期间同类型只出一份报告
CREATE UNIQUE INDEX IF NOT EXISTS uk_report_plan_period
    ON inspection_report (report_plan_id, period_start, COALESCE(patrol_type, ''))
    WHERE report_plan_id IS NOT NULL;

-- 存量启用小区补默认综合月报计划（每月 1 日 06:00 生成上月报告，与原硬编码行为等价）
INSERT INTO report_plan (id, tenant_id, community_id, name, cycle_type, cycle_config, gen_time, status, created_at, updated_at)
SELECT gen_random_uuid()::varchar, c.tenant_id, c.id, '综合月报（每月1日）', 'monthly', '{"day":1}', '06:00', 'enabled', now(), now()
FROM community c
WHERE c.status = 'enabled' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM report_plan p WHERE p.community_id = c.id AND p.cycle_type = 'monthly' AND p.patrol_type = '');

-- +goose Down
DROP TABLE IF EXISTS report_plan;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS report_plan_id;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS period_start;
ALTER TABLE inspection_report DROP COLUMN IF EXISTS period_end;
