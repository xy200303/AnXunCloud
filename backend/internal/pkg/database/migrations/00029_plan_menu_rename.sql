-- 00029：「巡检计划」菜单更名「计划任务」（巡检计划/报告计划双 tab 入口）。

-- +goose Up
UPDATE sys_menu SET title = '计划任务' WHERE path = '/inspection/plans' AND title = '巡检计划';

-- +goose Down
UPDATE sys_menu SET title = '巡检计划' WHERE path = '/inspection/plans' AND title = '计划任务';
