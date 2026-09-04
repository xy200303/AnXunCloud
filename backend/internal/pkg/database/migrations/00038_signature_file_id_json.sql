-- +goose Up
-- 报告签字 JSON 统一使用 signature_file_id；值此前已是 upload_file.id，仅字段名遗留旧口径。
UPDATE inspection_report
SET inspector_signed = COALESCE((
    SELECT jsonb_agg(
        CASE WHEN entry ? 'signature_file_key' THEN
            (entry - 'signature_file_key') || jsonb_build_object('signature_file_id', entry->'signature_file_key')
        ELSE entry END
    )
    FROM jsonb_array_elements(inspector_signed) AS entry
), '[]'::jsonb)
WHERE jsonb_typeof(inspector_signed) = 'array'
  AND EXISTS (
      SELECT 1
      FROM jsonb_array_elements(inspector_signed) AS entry
      WHERE entry ? 'signature_file_key'
  );

-- +goose Down
SELECT 1;
