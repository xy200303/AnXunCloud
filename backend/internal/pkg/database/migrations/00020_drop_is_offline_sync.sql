-- 去除冗余：is_offline_sync 与 checkin_type='offline' 表达同一事实（离线补传时两者必同真）。
-- 保留 checkin_type='offline' 作为唯一事实来源（后台「离线补传」标签/统计均走它），删除布尔列。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE checkin_record DROP COLUMN IF EXISTS is_offline_sync;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS is_offline_sync BOOLEAN NOT NULL DEFAULT false;
UPDATE checkin_record SET is_offline_sync = true WHERE checkin_type = 'offline';

-- +goose StatementEnd
