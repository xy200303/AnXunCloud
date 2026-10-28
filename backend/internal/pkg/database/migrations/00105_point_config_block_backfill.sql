-- +goose Up
-- owner: inspection
INSERT INTO inspection_point_config_block (id, tenant_id, community_id, point_id, reason, started_at)
SELECT gen_random_uuid(), p.tenant_id, p.community_id, p.id,
       CASE
           WHEN p.require_fence AND p.longitude = 0 AND p.latitude = 0 THEN '点位已启用围栏但未配置坐标'
           WHEN p.require_fence AND (p.fence_radius < 10 OR p.fence_radius > 2000) THEN '点位围栏半径须在 10-2000 米之间'
           WHEN p.credential = 'qrcode' AND COALESCE(p.qrcode_no, '') = '' THEN '点位使用二维码凭证但未配置二维码'
           WHEN p.credential = 'nfc' AND COALESCE(p.nfc_id, '') = '' THEN '点位使用 NFC 凭证但未配置 NFC ID'
           WHEN p.credential = 'any' AND COALESCE(p.qrcode_no, '') = '' AND COALESCE(p.nfc_id, '') = '' THEN '点位允许二维码或 NFC，但未配置任何凭证'
           ELSE '点位凭证类型配置无效'
       END,
       now()
FROM inspection_point p
WHERE p.tenant_id IS NOT NULL AND p.deleted_at IS NULL
  AND (
      (p.require_fence AND ((p.longitude = 0 AND p.latitude = 0) OR p.fence_radius < 10 OR p.fence_radius > 2000))
      OR (p.credential = 'qrcode' AND COALESCE(p.qrcode_no, '') = '')
      OR (p.credential = 'nfc' AND COALESCE(p.nfc_id, '') = '')
      OR (p.credential = 'any' AND COALESCE(p.qrcode_no, '') = '' AND COALESCE(p.nfc_id, '') = '')
      OR p.credential NOT IN ('qrcode', 'nfc', 'any', 'none')
  )
ON CONFLICT (point_id) WHERE ended_at IS NULL DO UPDATE SET reason = EXCLUDED.reason;

UPDATE inspection_point_config_block b
SET ended_at = now()
WHERE b.ended_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM inspection_point p
      WHERE p.id = b.point_id AND p.deleted_at IS NULL
        AND (
            (p.require_fence AND ((p.longitude = 0 AND p.latitude = 0) OR p.fence_radius < 10 OR p.fence_radius > 2000))
            OR (p.credential = 'qrcode' AND COALESCE(p.qrcode_no, '') = '')
            OR (p.credential = 'nfc' AND COALESCE(p.nfc_id, '') = '')
            OR (p.credential = 'any' AND COALESCE(p.qrcode_no, '') = '' AND COALESCE(p.nfc_id, '') = '')
            OR p.credential NOT IN ('qrcode', 'nfc', 'any', 'none')
        )
  );

-- +goose Down
SELECT 1;
