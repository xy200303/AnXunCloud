-- upload_file.mime_type 加宽到 128（Office 文档 MIME 超 64 导致登记失败）。

-- +goose Up
ALTER TABLE upload_file ALTER COLUMN mime_type TYPE character varying(128);

-- +goose Down
ALTER TABLE upload_file ALTER COLUMN mime_type TYPE character varying(64);
