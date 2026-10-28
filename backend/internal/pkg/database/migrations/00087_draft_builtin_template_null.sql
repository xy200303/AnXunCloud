-- +goose Up
-- owner: inspection
-- 系统内置合成项没有模板来源；普通模板项由服务层强制 template_id。
ALTER TABLE checkin_item_draft ALTER COLUMN template_id DROP NOT NULL;

-- +goose Down
SELECT 1;
