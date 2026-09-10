-- 00044：设备台账与维保周期管理（《设备台账与维保周期管理设计方案》v1.4）。
-- equipment 设备台账主表（一具设备一行，记当前状态，到期判定只看 next_due_date）；
-- equipment_maintenance 维保流水（一次维保一行，只增不改，经理确认后回写台账）。

-- +goose Up
CREATE TABLE IF NOT EXISTS public.equipment (
    id uuid NOT NULL PRIMARY KEY,
    tenant_id uuid,
    community_id uuid NOT NULL,
    building_id uuid,
    point_id uuid,
    type character varying(32) NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(128) NOT NULL,
    manufacture_date date,
    last_maintenance_date date,
    next_due_date date,
    scrap_date date,
    warn_days integer,
    last_notified_at timestamp with time zone,
    status character varying(16) NOT NULL DEFAULT 'in_service',
    extra jsonb NOT NULL DEFAULT '{}'::jsonb,
    remark character varying(255) NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);
-- 设备编号租户内唯一（软删行不占位，删除后同编号可重建）
CREATE UNIQUE INDEX IF NOT EXISTS uk_equipment_tenant_code ON public.equipment (tenant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_equipment_community ON public.equipment (community_id);
CREATE INDEX IF NOT EXISTS idx_equipment_point ON public.equipment (point_id) WHERE point_id IS NOT NULL;
-- 每日到期扫描走此索引（只在用设备）
CREATE INDEX IF NOT EXISTS idx_equipment_due_scan ON public.equipment (next_due_date) WHERE deleted_at IS NULL AND status = 'in_service' AND next_due_date IS NOT NULL;

COMMENT ON TABLE public.equipment IS '设备台账：一具设备一行记当前状态；类型走 equipment_type 字典（attrs 携带维保规则）；点位关联多对一（一点多具）';
COMMENT ON COLUMN public.equipment.manufacture_date IS '出厂日期（瓶体钢印），可空；为空时防换设备比对不可用，首轮巡检补录';
COMMENT ON COLUMN public.equipment.last_maintenance_date IS '最近维保日期；仅由 confirm_status=confirmed 的维保流水回写';
COMMENT ON COLUMN public.equipment.next_due_date IS '下次到期日：规则计算（首次月数/周期月数），允许人工覆盖；NULL=无规则或无日期数据（不判到期不提醒）';
COMMENT ON COLUMN public.equipment.scrap_date IS '报废日期（灭火器=出厂+120月，自动计算展示，一期不做催办）';
COMMENT ON COLUMN public.equipment.last_notified_at IS '上次提醒时间（临期每周期只提醒一次的防打扰依据）';
COMMENT ON COLUMN public.equipment.status IS 'in_service 在用 / maintaining 维保中 / stopped 停用 / scrapped 报废';

CREATE TABLE IF NOT EXISTS public.equipment_maintenance (
    id uuid NOT NULL PRIMARY KEY,
    tenant_id uuid,
    equipment_id uuid NOT NULL,
    maintenance_type character varying(16) NOT NULL DEFAULT 'repair',
    maintenance_date date NOT NULL,
    vendor character varying(128),
    operator_name character varying(64) NOT NULL DEFAULT '',
    note character varying(255) NOT NULL DEFAULT '',
    file_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    confirm_status character varying(16) NOT NULL DEFAULT 'pending',
    confirmed_by uuid,
    confirmed_at timestamp with time zone,
    reject_reason character varying(255),
    ai_verdict character varying(16),
    ai_reason character varying(512),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_equipment_maintenance_equipment ON public.equipment_maintenance (equipment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_equipment_maintenance_pending ON public.equipment_maintenance (tenant_id, confirm_status) WHERE confirm_status = 'pending';

COMMENT ON TABLE public.equipment_maintenance IS '维保流水：一次维保一行只增不改；台账唯一写入口（confirmed 才回写）；台账补录也走此表（maintenance_type=ledger_fix）';
COMMENT ON COLUMN public.equipment_maintenance.confirm_status IS 'pending 待确认 / confirmed 已确认 / rejected 已驳回；登记默认 pending，经理确认后生效';
COMMENT ON COLUMN public.equipment_maintenance.maintenance_type IS 'repair 维修充粉 / maintain 保养 / inspect 检测 / replace 更换 / ledger_fix 台账补录';
COMMENT ON COLUMN public.equipment_maintenance.ai_verdict IS 'AI 预检结论：pass/review（只标记不拦截），review 在确认列表置顶';
COMMENT ON COLUMN public.equipment_maintenance.payload IS '随单附加数据：ledger_fix 单存补录日期 {manufacture_date,last_maintenance_date}（YYYY-MM-DD），确认时回写台账';

-- +goose Down
DROP TABLE IF EXISTS public.equipment_maintenance;
DROP TABLE IF EXISTS public.equipment;
