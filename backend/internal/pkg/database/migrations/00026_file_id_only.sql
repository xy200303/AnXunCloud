-- +goose Up

ALTER TABLE upload_file RENAME COLUMN file_key TO storage_key;
ALTER INDEX IF EXISTS uk_upload_file_key RENAME TO uk_upload_file_storage_key;

-- 迁移巡检照片与逐项识别草稿：数组元素统一为 upload_file.id，彻底移除旧 file_key 引用。
-- JOIN 条件必须是 storage_key（旧数组元素是 file_key=存储路径，不是 UUID；误用 f.id 会把 photos 清成空数组）。
UPDATE checkin_record_item AS i
SET photos = COALESCE((
    SELECT jsonb_agg(to_jsonb(f.id::text) ORDER BY e.ord)
    FROM jsonb_array_elements_text(i.photos) WITH ORDINALITY AS e(ref, ord)
    JOIN upload_file AS f ON f.storage_key = e.ref
), '[]'::jsonb)
WHERE jsonb_typeof(i.photos) = 'array';

UPDATE checkin_item_draft AS d
SET file_keys = COALESCE((
    SELECT jsonb_agg(to_jsonb(f.id::text) ORDER BY e.ord)
    FROM jsonb_array_elements_text(d.file_keys) WITH ORDINALITY AS e(ref, ord)
    JOIN upload_file AS f ON f.storage_key = e.ref
), '[]'::jsonb)
WHERE jsonb_typeof(d.file_keys) = 'array';

ALTER TABLE checkin_item_draft RENAME COLUMN file_keys TO file_ids;

-- +goose Down

ALTER TABLE upload_file RENAME COLUMN storage_key TO file_key;
ALTER INDEX IF EXISTS uk_upload_file_storage_key RENAME TO uk_upload_file_key;
