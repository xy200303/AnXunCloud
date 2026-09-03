-- 00035：app_release 增加强制更新标记（App 启动检查更新：强制=不可跳过，弱更新=可以后再说）。

-- +goose Up
ALTER TABLE app_release ADD COLUMN force_update boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE app_release DROP COLUMN IF EXISTS force_update;
