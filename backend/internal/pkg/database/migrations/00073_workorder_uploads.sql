-- +goose Up
-- owner: file
ALTER TABLE upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (scene IN ('checkin', 'avatar', 'export', 'signature', 'seal', 'notice', 'app', 'equipment', 'workorder'));

-- +goose Down
SELECT 1;
