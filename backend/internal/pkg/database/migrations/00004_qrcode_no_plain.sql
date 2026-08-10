-- 二维码编号规范化：P-000015 → P000015（去短横线，码内容即裸编号，提升扫码识别率）。

-- +goose Up
UPDATE inspection_point SET qrcode_no = replace(qrcode_no, 'P-', 'P') WHERE qrcode_no LIKE 'P-%';

-- +goose Down
UPDATE inspection_point SET qrcode_no = 'P-' || substring(qrcode_no from 2) WHERE qrcode_no ~ '^P[0-9]+$';
