-- 00032：文件 ID 统一收尾——签章与报告签字/公章/PDF 快照字段全部从存储路径（file_key）
-- 迁移为 upload_file.id，消除双标识（此后 uploadfile.ByRef/ByRefs 兼容层删除）。
-- 转换不上的历史值（文件未登记）按「无文件」置空——此前这些引用下载同样 404。

-- +goose Up

-- 1) sign_asset：file_key → file_id
ALTER TABLE sign_asset ADD COLUMN file_id uuid;
UPDATE sign_asset sa SET file_id = f.id FROM upload_file f WHERE f.storage_key = sa.file_key;
ALTER TABLE sign_asset DROP COLUMN file_key;

-- 2) inspection_report：公章快照
ALTER TABLE inspection_report ADD COLUMN seal_file_id uuid;
UPDATE inspection_report r SET seal_file_id = f.id FROM upload_file f WHERE f.storage_key = r.seal_file_key;
ALTER TABLE inspection_report DROP COLUMN seal_file_key;

-- 3) inspection_report：主管/经理签字快照
ALTER TABLE inspection_report ADD COLUMN supervisor_signature_id uuid;
UPDATE inspection_report r SET supervisor_signature_id = f.id FROM upload_file f WHERE f.storage_key = r.supervisor_signature_key;
ALTER TABLE inspection_report DROP COLUMN supervisor_signature_key;

ALTER TABLE inspection_report ADD COLUMN manager_signature_id uuid;
UPDATE inspection_report r SET manager_signature_id = f.id FROM upload_file f WHERE f.storage_key = r.manager_signature_key;
ALTER TABLE inspection_report DROP COLUMN manager_signature_key;

-- 4) inspection_report：归档 PDF（archivePDF 生成时已登记 upload_file）
ALTER TABLE inspection_report ADD COLUMN file_id uuid;
UPDATE inspection_report r SET file_id = f.id FROM upload_file f WHERE f.storage_key = r.file_key;
ALTER TABLE inspection_report DROP COLUMN file_key;

-- 5) app_release：安装包引用统一为 upload_file.id（上传时同事务已登记）
ALTER TABLE app_release ADD COLUMN file_id uuid;
UPDATE app_release ar SET file_id = f.id FROM upload_file f WHERE f.storage_key = ar.file_key;
ALTER TABLE app_release DROP COLUMN file_key;

-- +goose Down
-- 不可逆（file_key 已按 id 重写）：仅还原列结构，不回填数据。
ALTER TABLE sign_asset ADD COLUMN file_key varchar(255);
ALTER TABLE inspection_report ADD COLUMN seal_file_key varchar(255);
ALTER TABLE inspection_report ADD COLUMN supervisor_signature_key varchar(255);
ALTER TABLE inspection_report ADD COLUMN manager_signature_key varchar(255);
ALTER TABLE inspection_report ADD COLUMN file_key varchar(255);
ALTER TABLE app_release ADD COLUMN file_key varchar(255);
