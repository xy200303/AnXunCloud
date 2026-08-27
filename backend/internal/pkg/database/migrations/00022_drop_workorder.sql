-- 工单模块彻底删除：模块代码/菜单（00012）/前端页面已下线，本次删除数据表与项目级工单开关列。
-- work_order/work_order_log 自 00012 起无任何写入入口（死表）。
-- +goose Up
-- +goose StatementBegin

DROP TABLE IF EXISTS work_order_log;
DROP TABLE IF EXISTS work_order;

ALTER TABLE community DROP COLUMN IF EXISTS wo_triage_enabled;
ALTER TABLE community DROP COLUMN IF EXISTS wo_grab_enabled;

-- 工单槽位绑定一并清理（槽位枚举 order_triage/order_dispatch/order_accept 同步从代码移除）
DELETE FROM duty_binding WHERE slot IN ('order_triage', 'order_dispatch', 'order_accept');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE community ADD COLUMN IF NOT EXISTS wo_triage_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE community ADD COLUMN IF NOT EXISTS wo_grab_enabled BOOLEAN NOT NULL DEFAULT false;
-- 表结构不回滚（模块已删除，如需恢复从历史迁移重建）

-- +goose StatementEnd
