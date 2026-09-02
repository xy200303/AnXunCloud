-- 00034：修复 00032 的 goose 分段错误——「5) app_release」一段误写在 Down 指令之后，
-- 已跑过 00032 的存量库 app_release 仍缺 file_id 列（上传发布物 INSERT 报 42703）。
-- 幂等：全新库（00032 已修正、file_key 已删）与存量库均安全。

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'app_release' AND column_name = 'file_key'
  ) THEN
    ALTER TABLE app_release ADD COLUMN IF NOT EXISTS file_id uuid;
    UPDATE app_release ar SET file_id = f.id FROM upload_file f
      WHERE ar.file_id IS NULL AND f.storage_key = ar.file_key;
    ALTER TABLE app_release DROP COLUMN file_key;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE app_release ADD COLUMN IF NOT EXISTS file_key varchar(255);
