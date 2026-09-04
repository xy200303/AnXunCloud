-- +goose Up
ALTER TABLE upload_file ADD COLUMN IF NOT EXISTS size bigint NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_upload_file_dedup ON upload_file (scene, md5, size) WHERE md5 <> '' AND size > 0;

-- +goose Down
DROP INDEX IF EXISTS idx_upload_file_dedup;
ALTER TABLE upload_file DROP COLUMN IF EXISTS size;
