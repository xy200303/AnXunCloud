-- 00050：审批链流程快照——记录提交时把当时命中的流程（approval_flow.steps 解析结果）固化到记录上，
-- 审核/闸门推进一律读快照：在途记录按提交时的规则审完，改流程只影响新单（防止在途单环节错位/卡死）。
-- NULL = 存量记录/空流程（读取时回落现配）。

-- +goose Up
ALTER TABLE public.checkin_record ADD COLUMN IF NOT EXISTS flow_snapshot jsonb NULL;
COMMENT ON COLUMN public.checkin_record.flow_snapshot IS '打卡审核链快照（提交时命中的 approval_flow.steps；NULL=存量/空流程，回落现配）';

ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS flow_snapshot jsonb NULL;
COMMENT ON COLUMN public.equipment_maintenance.flow_snapshot IS '维保审核链快照（登记时命中的 approval_flow.steps；NULL=存量/空流程，回落现配）';

-- +goose Down
ALTER TABLE public.checkin_record DROP COLUMN IF EXISTS flow_snapshot;
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS flow_snapshot;
