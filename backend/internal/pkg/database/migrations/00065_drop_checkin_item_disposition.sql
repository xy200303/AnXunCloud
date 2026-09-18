-- 00065：砍掉「异常处置措施」功能——巡检员只拍现场最终状态，处置是甲方线下的事。
-- 删除 checkin_record_item 的 disposition / resolution_file_ids / resolution_note 列
-- （开发阶段，不保留历史数据兼容）。审核路由语义改为：任一异常项（pass=false 或 exception_type 非空）强制转人工。
-- 幂等可重复执行。

-- +goose Up

ALTER TABLE checkin_record_item
    DROP COLUMN IF EXISTS disposition,
    DROP COLUMN IF EXISTS resolution_file_ids,
    DROP COLUMN IF EXISTS resolution_note;

-- +goose Down

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS disposition varchar(24) NOT NULL DEFAULT '';
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS resolution_file_ids jsonb NOT NULL DEFAULT '[]';
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS resolution_note varchar(512) NOT NULL DEFAULT '';
