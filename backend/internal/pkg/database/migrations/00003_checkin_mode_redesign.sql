-- 打卡方式重新设计：checkin_mode 单枚举（qrcode/fence/either/both/nfc）拆为两个独立维度——
-- credential（点位凭证：qrcode/nfc/none）+ require_fence（GPS 围栏硬校验）。
-- 旧值映射：qrcode→扫码；nfc→NFC；fence→纯围栏；either→扫码（OR 语义放弃）；both→扫码+围栏；nfc+围栏为新支持组合。

-- +goose Up
ALTER TABLE inspection_point ADD COLUMN credential VARCHAR(16) NOT NULL DEFAULT 'qrcode';
ALTER TABLE inspection_point ADD COLUMN require_fence BOOLEAN NOT NULL DEFAULT false;

UPDATE inspection_point SET credential = 'qrcode', require_fence = false WHERE checkin_mode IN ('qrcode', 'either');
UPDATE inspection_point SET credential = 'nfc',    require_fence = false WHERE checkin_mode = 'nfc';
UPDATE inspection_point SET credential = 'none',   require_fence = true  WHERE checkin_mode = 'fence';
UPDATE inspection_point SET credential = 'qrcode', require_fence = true  WHERE checkin_mode = 'both';

ALTER TABLE inspection_point DROP COLUMN checkin_mode;
ALTER TABLE inspection_point ADD CONSTRAINT chk_point_credential CHECK (credential IN ('qrcode','nfc','none'));
-- 凭证与围栏至少启用一项
ALTER TABLE inspection_point ADD CONSTRAINT chk_point_check_valid CHECK (credential <> 'none' OR require_fence);

-- checkin_mode 字典随字段一并废弃（展示标签由前端按 credential+require_fence 组合生成）
DELETE FROM sys_dict_data WHERE type_code = 'checkin_mode';
DELETE FROM sys_dict_type WHERE code = 'checkin_mode';

-- +goose Down
ALTER TABLE inspection_point ADD COLUMN checkin_mode VARCHAR(16) NOT NULL DEFAULT 'either';
UPDATE inspection_point SET checkin_mode = CASE
    WHEN credential = 'qrcode' AND require_fence THEN 'both'
    WHEN credential = 'qrcode' THEN 'qrcode'
    WHEN credential = 'nfc' THEN 'nfc'
    ELSE 'fence' END;
ALTER TABLE inspection_point DROP COLUMN credential;
ALTER TABLE inspection_point DROP COLUMN require_fence;
