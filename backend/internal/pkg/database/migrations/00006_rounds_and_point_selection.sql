-- 巡更轮次 + 计划点位圈选（《专项巡检与专项检查报告设计方案》§3.2/§3.3，全部幂等）：
-- inspection_task 加轮次快照列（round_name/time_window）与点位名单快照列（point_ids）；
-- inspection_plan 加圈选模式列（selection_mode/point_types）；
-- 任务查重唯一索引扩展轮次维度（轮次化后同计划同天同人生成多个任务）。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE inspection_task ADD COLUMN IF NOT EXISTS round_name varchar(32);
COMMENT ON COLUMN inspection_task.round_name IS '巡更轮次名快照（cycle_config.rounds[].name）；非轮次任务为 NULL';

ALTER TABLE inspection_task ADD COLUMN IF NOT EXISTS time_window varchar(32);
COMMENT ON COLUMN inspection_task.time_window IS '本任务执行时段快照 HH:MM-HH:MM（轮次任务取 rounds[].window，允许跨零点）；非轮次任务为 NULL，展示/统计回落计划 time_window';

ALTER TABLE inspection_task ADD COLUMN IF NOT EXISTS point_ids jsonb;
COMMENT ON COLUMN inspection_task.point_ids IS '任务点位名单快照（生成时展开：explicit 照抄计划名单，by_point_types 实时圈选）；空则消费侧回落计划 point_ids（兼容存量任务）';

ALTER TABLE inspection_plan ADD COLUMN IF NOT EXISTS selection_mode varchar(16) NOT NULL DEFAULT 'explicit';
COMMENT ON COLUMN inspection_plan.selection_mode IS '点位圈选模式：explicit 显式名单（point_ids）/ by_point_types 按点位类型动态圈选（point_types）';

ALTER TABLE inspection_plan ADD COLUMN IF NOT EXISTS point_types jsonb;
COMMENT ON COLUMN inspection_plan.point_types IS '圈选点位类型（point_type 字典值数组，selection_mode=by_point_types 时必填）';

-- 查重唯一索引扩展轮次维度（非轮次任务 round_name 归一为 ''；表达式索引重建无数据冲突——旧索引更严格）
DROP INDEX IF EXISTS uk_task_plan_date_inspector;
CREATE UNIQUE INDEX uk_task_plan_date_inspector ON public.inspection_task USING btree (plan_id, task_date, inspector_id, (COALESCE(round_name, ''::character varying))) WHERE (deleted_at IS NULL);

-- patrol_type 硬编码 CHECK 约束拆除：§3.1 起类型字典驱动（如 fire），DB 层枚举约束与字典扩容冲突，校验归应用层
ALTER TABLE inspection_plan DROP CONSTRAINT IF EXISTS chk_plan_patrol_type;
ALTER TABLE inspection_task DROP CONSTRAINT IF EXISTS chk_task_patrol_type;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE inspection_task ADD CONSTRAINT chk_task_patrol_type CHECK (patrol_type::text = ANY (ARRAY['safety'::varchar::text, 'equipment'::varchar::text, 'environment'::varchar::text, 'building'::varchar::text]));
ALTER TABLE inspection_plan ADD CONSTRAINT chk_plan_patrol_type CHECK (patrol_type::text = ANY (ARRAY['safety'::varchar::text, 'equipment'::varchar::text, 'environment'::varchar::text, 'building'::varchar::text]));
DROP INDEX IF EXISTS uk_task_plan_date_inspector;
CREATE UNIQUE INDEX uk_task_plan_date_inspector ON public.inspection_task USING btree (plan_id, task_date, inspector_id) WHERE (deleted_at IS NULL);
ALTER TABLE inspection_plan DROP COLUMN IF EXISTS point_types;
ALTER TABLE inspection_plan DROP COLUMN IF EXISTS selection_mode;
ALTER TABLE inspection_task DROP COLUMN IF EXISTS point_ids;
ALTER TABLE inspection_task DROP COLUMN IF EXISTS time_window;
ALTER TABLE inspection_task DROP COLUMN IF EXISTS round_name;
-- +goose StatementEnd
