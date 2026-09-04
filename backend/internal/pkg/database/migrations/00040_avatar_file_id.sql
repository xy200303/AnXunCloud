-- +goose Up
-- 头像统一保存 upload_file.id；存量数据中的 URL/存储路径按登记记录转换，无法对应的值清空。
UPDATE sys_user AS u
SET avatar = f.id::text
FROM upload_file AS f
WHERE u.avatar <> ''
  AND (u.avatar = f.id::text
       OR u.avatar = f.url
       OR u.avatar = f.storage_key
       OR regexp_replace(u.avatar, '^.*/uploads/', '') = f.storage_key)
  AND f.scene = 'avatar';

UPDATE sys_user AS u
SET avatar = ''
WHERE avatar <> ''
  AND NOT EXISTS (
      SELECT 1 FROM upload_file AS f
      WHERE f.id::text = u.avatar AND f.scene = 'avatar'
  );

-- +goose Down
SELECT 1;
