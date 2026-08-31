-- +goose Up

ALTER TABLE upload_file RENAME COLUMN file_key TO storage_key;
ALTER INDEX IF EXISTS uk_upload_file_key RENAME TO uk_upload_file_storage_key;

-- 迁移巡检照片与逐项识别草稿：数组元素统一为 upload_file.id，彻底移除旧 file_key 引用。
UPDATE checkin_record_item AS i
SET photos = COALESCE((
    SELECT jsonb_agg(to_jsonb(f.id::text) ORDER BY e.ord)
    FROM jsonb_array_elements_text(i.photos) WITH ORDINALITY AS e(ref, ord)
    JOIN upload_file AS f ON f.id::text = e.ref
), '[]'::jsonb)
WHERE jsonb_typeof(i.photos) = 'array';

UPDATE checkin_item_draft AS d
SET file_keys = COALESCE((
    SELECT jsonb_agg(to_jsonb(f.id::text) ORDER BY e.ord)
    FROM jsonb_array_elements_text(d.file_keys) WITH ORDINALITY AS e(ref, ord)
    JOIN upload_file AS f ON f.id::text = e.ref
), '[]'::jsonb)
WHERE jsonb_typeof(d.file_keys) = 'array';

ALTER TABLE checkin_item_draft RENAME COLUMN file_keys TO file_ids;

-- +goose Down

ALTER TABLE upload_file RENAME COLUMN storage_key TO file_key;
ALTER INDEX IF EXISTS uk_upload_file_storage_key RENAME TO uk_upload_file_key;
