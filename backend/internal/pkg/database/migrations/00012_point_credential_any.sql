-- 点位凭证新增「任一」（any：扫码或 NFC 均可作为打卡凭证）。
-- 存在同一点位同时贴二维码和 NFC 标签的场景，巡检员用任一方式核验即可。

-- +goose Up
ALTER TABLE inspection_point DROP CONSTRAINT chk_point_credential;
ALTER TABLE inspection_point ADD CONSTRAINT chk_point_credential CHECK (credential IN ('qrcode','nfc','none','any'));

-- +goose Down
UPDATE inspection_point SET credential = 'qrcode' WHERE credential = 'any';
ALTER TABLE inspection_point DROP CONSTRAINT chk_point_credential;
ALTER TABLE inspection_point ADD CONSTRAINT chk_point_credential CHECK (credential IN ('qrcode','nfc','none'));
