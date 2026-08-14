-- upload_file 补充统一文件元数据：原始文件名 name、内容摘要 md5、存储驱动 storage（local/oss/cos）。
-- 为 /api/files 统一上传下载与后续 COS 驱动做准备；md5 用于完整性校验与可选的同内容去重（不作主键）。
-- 服务端生成的文件（月报 PDF/二维码包/统计导出）也登记到本表，user_id 用零 UUID 表示系统生成。

-- +goose Up
ALTER TABLE upload_file ADD COLUMN name character varying(255) DEFAULT ''::character varying NOT NULL;
ALTER TABLE upload_file ADD COLUMN md5 character varying(64) DEFAULT ''::character varying NOT NULL;
ALTER TABLE upload_file ADD COLUMN storage character varying(16) DEFAULT ''::character varying NOT NULL;
COMMENT ON COLUMN public.upload_file.name IS '原始文件名（上传时的文件名/生成文件名）';
COMMENT ON COLUMN public.upload_file.md5 IS '内容 MD5（完整性校验/去重查询用）';
COMMENT ON COLUMN public.upload_file.storage IS '存储驱动：local/oss/cos';
-- 存量数据回填驱动：本地静态 URL 判 local，其余判 oss
UPDATE upload_file SET storage = CASE WHEN url LIKE '%/uploads/%' THEN 'local' ELSE 'oss' END WHERE storage = '';
CREATE INDEX idx_upload_file_md5 ON public.upload_file USING btree (md5) WHERE (md5 <> ''::character varying);

-- +goose Down
DROP INDEX IF EXISTS idx_upload_file_md5;
ALTER TABLE upload_file DROP COLUMN storage;
ALTER TABLE upload_file DROP COLUMN md5;
ALTER TABLE upload_file DROP COLUMN name;
