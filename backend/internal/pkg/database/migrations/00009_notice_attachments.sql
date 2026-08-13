-- sys_notice 增加 attachments 列（JSONB 可空，元素 {name, url}），公告支持图片/文件附件；
-- upload_file.scene 增加 'notice'（公告附件走管理端 /system/upload 上传）。

-- +goose Up
ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS attachments jsonb;

ALTER TABLE upload_file DROP CONSTRAINT chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'workorder'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying, 'notice'::character varying])::text[])));

-- +goose Down
ALTER TABLE upload_file DROP CONSTRAINT chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'workorder'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying])::text[])));

ALTER TABLE sys_notice DROP COLUMN IF EXISTS attachments;
