-- 点位结构化位置：单元号 + 楼层（楼栋已有 building_id）。
-- 用途：楼内巡检路线按「单元 → 楼层自上而下」排序；非楼栋点位（车库/公园/区域）两列为 NULL。
-- 批量建点同步支持单元维度并写入这两列。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE inspection_point ADD COLUMN IF NOT EXISTS unit_no INT NULL;
ALTER TABLE inspection_point ADD COLUMN IF NOT EXISTS floor INT NULL;
COMMENT ON COLUMN inspection_point.unit_no IS '单元号（NULL=不分单元/非楼栋点位）';
COMMENT ON COLUMN inspection_point.floor IS '楼层（负数=地下层，-1 即 B1；NULL=非楼栋点位）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE inspection_point DROP COLUMN IF EXISTS unit_no;
ALTER TABLE inspection_point DROP COLUMN IF EXISTS floor;

-- +goose StatementEnd
