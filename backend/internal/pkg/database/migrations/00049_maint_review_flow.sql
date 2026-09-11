-- 00049：维保登记审核链接入通用审批引擎（approval_flow flow_code=maint_review）。
-- equipment_maintenance 增加 confirm_step（当前环节下标，与打卡链 audit_step 同语义）。

-- +goose Up
ALTER TABLE public.equipment_maintenance ADD COLUMN IF NOT EXISTS confirm_step smallint NOT NULL DEFAULT 0;
COMMENT ON COLUMN public.equipment_maintenance.confirm_step IS '维保审核链当前环节下标（approval_flow flow_code=maint_review；0 起）';

-- +goose Down
ALTER TABLE public.equipment_maintenance DROP COLUMN IF EXISTS confirm_step;
