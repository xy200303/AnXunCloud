-- 00048：打卡巡检 × 设备维保融合（《打卡巡检×设备维保融合实施方案》）。
-- checkin_record_item：异常项处置方式 disposition + 处置照片/说明（resolution_*）；
-- equipment_maintenance：流水来源 source（form=登记表单/checkin=打卡拍新标签自动生成）、
-- 关联打卡记录 checkin_record_id、确认方式 confirm_mode（manual=人工/ai=AI 可信自动确认，confirmed_by 置空）。

-- +goose Up
ALTER TABLE public.checkin_record_item ADD COLUMN IF NOT EXISTS disposition varchar(24) NOT NULL DEFAULT '';
COMMENT ON COLUMN public.checkin_record_item.disposition IS '异常项处置方式：''''=未处置/on_site_resolved=现场已处理/maintenance_registered=登记维保/report_pending=上报待处理（强制转人工审核）';

ALTER TABLE public.checkin_record_item ADD COLUMN IF NOT EXISTS resolution_file_ids jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN public.checkin_record_item.resolution_file_ids IS '处置照片 file_id 数组（on_site_resolved 必传 ≥1 张，归属校验同逐项照片）';

ALTER TABLE public.checkin_record_item ADD COLUMN IF NOT EXISTS resolution_note varchar(512) NOT NULL DEFAULT '';
COMMENT ON COLUMN public.checkin_record_item.resolution_note IS '处置说明（可空）';

ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS source varchar(16) NOT NULL DEFAULT 'form';
COMMENT ON COLUMN public.equipment_maintenance.source IS '流水来源：form=登记表单（mp/admin Register）/checkin=打卡拍新标签自动生成';

ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS checkin_record_id uuid NULL;
COMMENT ON COLUMN public.equipment_maintenance.checkin_record_id IS '来源打卡记录 id（source=checkin 时非空；checkin_record 为按月分区表，主键含分区键，不加 FK）';

ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS confirm_mode varchar(8) NOT NULL DEFAULT 'manual';
COMMENT ON COLUMN public.equipment_maintenance.confirm_mode IS '确认方式：manual=经理人工确认/ai=AI 核验可信自动确认（confirmed_by 置空）';

-- +goose Down
ALTER TABLE public.checkin_record_item DROP COLUMN IF EXISTS disposition;
ALTER TABLE public.checkin_record_item DROP COLUMN IF EXISTS resolution_file_ids;
ALTER TABLE public.checkin_record_item DROP COLUMN IF EXISTS resolution_note;
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS source;
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS checkin_record_id;
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS confirm_mode;
