-- +goose Up
-- 任务执行时段统一以任务快照为唯一事实来源。
UPDATE inspection_task AS task
SET time_window = plan.time_window
FROM inspection_plan AS plan
WHERE task.plan_id = plan.id
  AND COALESCE(task.time_window, '') = '';

-- +goose Down
SELECT 1;
