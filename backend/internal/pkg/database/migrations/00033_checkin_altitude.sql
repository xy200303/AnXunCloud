-- 00033：打卡记录增加定位辅助信息——海拔 altitude（米）与定位精度 accuracy（米）。
-- 仅作参考展示（消费级 GPS 海拔误差 ±20~50m，不可用于校验/判定），可空。
-- checkin_record 为按月分区表，父表加列自动级联到全部分区。

-- +goose Up
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS altitude numeric(10,2);
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS accuracy numeric(10,2);

-- +goose Down
ALTER TABLE checkin_record DROP COLUMN IF EXISTS altitude;
ALTER TABLE checkin_record DROP COLUMN IF EXISTS accuracy;
