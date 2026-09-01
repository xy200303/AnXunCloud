-- 00031：百万级性能——checkin_record 补 (community_id, checkin_time) 索引。
-- 背景：分区键为 created_at，但业务查询全部按 checkin_time 过滤，分区裁剪不生效（已知取舍，
-- 暂不改分区键/查询谓词，先以索引降低各分区扫描成本；声明式分区在父表建索引自动落到各分区）。
-- 同时补 result 过滤用部分索引（任务监控异常/疑似 Tab 子查询）。

-- +goose Up
CREATE INDEX IF NOT EXISTS idx_checkin_comm_time ON checkin_record (community_id, checkin_time DESC);
CREATE INDEX IF NOT EXISTS idx_checkin_result_active ON checkin_record (result) WHERE superseded_by IS NULL AND result = 'abnormal';
CREATE INDEX IF NOT EXISTS idx_checkin_suspect_active ON checkin_record (is_suspect) WHERE superseded_by IS NULL AND is_suspect;

-- +goose Down
DROP INDEX IF EXISTS idx_checkin_comm_time;
DROP INDEX IF EXISTS idx_checkin_result_active;
DROP INDEX IF EXISTS idx_checkin_suspect_active;
