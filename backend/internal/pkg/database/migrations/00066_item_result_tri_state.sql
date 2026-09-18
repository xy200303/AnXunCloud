-- 00066：checkin_record_item 引入显式三态 result（normal/abnormal/escaped），废除 pass+exception_type 组合推导。
-- result：normal=正常（检了没问题）/ abnormal=异常（检了有问题）/ escaped=无法检查（逃生：设备不存在/无法拍摄/标签磨损，没检成）。
-- exception_type 列保留（存逃生具体类型，仅 escaped 态有意义）；pass 布尔列删除（开发阶段不留双写）。
-- 回填口径：exception_type<>'' → escaped；否则 pass=false → abnormal；其余 normal。幂等可重复执行。

-- +goose Up

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS result varchar(16) NOT NULL DEFAULT 'normal';

UPDATE checkin_record_item SET result = 'escaped' WHERE exception_type <> '' AND result <> 'escaped';
UPDATE checkin_record_item SET result = 'abnormal' WHERE exception_type = '' AND pass = false AND result <> 'abnormal';

ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS pass;

ALTER TABLE checkin_record_item DROP CONSTRAINT IF EXISTS chk_rec_item_result;
ALTER TABLE checkin_record_item
    ADD CONSTRAINT chk_rec_item_result CHECK (result IN ('normal', 'abnormal', 'escaped'));

COMMENT ON COLUMN checkin_record_item.result IS '逐项结论三态：normal=正常/abnormal=异常/escaped=无法检查（逃生，exception_type 存具体类型）';

-- +goose Down

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS pass boolean NOT NULL DEFAULT false;
UPDATE checkin_record_item SET pass = true WHERE result = 'normal';
ALTER TABLE checkin_record_item DROP CONSTRAINT IF EXISTS chk_rec_item_result;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS result;
