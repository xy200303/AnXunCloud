-- 基线迁移（squash v2）：00001–00024 全部合并为本文件（2026-08-28，计划均分/一项一图/工单删除等演进完成后）。
-- 内容 = pg_dump schema-only（剔除 goose_db_version 与按月分区子表；checkin_record/sys_operation_log
-- 的当月/下月分区由运行期 EnsurePartitions 滚动创建，*_default 默认分区保留在基线内）。
-- 数据（内置角色/菜单/字典/默认租户/岗位模板/槽位默认/超管账号）由 seed.go 负责，本文件只管结构。
-- 老库兼容：旧库 goose_db_version 已记录 v1 及以上版本，goose 会跳过本基线，行为不变；
-- 落后版本的库（< v24）请先升级到旧链最后版本再切到本基线。

-- +goose Up

--
-- PostgreSQL database dump
--


-- Dumped from database version 15.18
-- Dumped by pg_dump version 15.18

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
-- （已剔除 pg_dump 的 search_path 置空语句：goose 会话锁下同会话记录版本号依赖默认 search_path）
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

-- *not* creating schema, since initdb creates it


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS '';


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: app_release; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_release (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    platform character varying(16) NOT NULL,
    version character varying(64) DEFAULT ''::character varying NOT NULL,
    file_key character varying(255) NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    note character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: approval_flow; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_flow (
    id uuid NOT NULL,
    tenant_id uuid,
    project_id uuid,
    flow_code character varying(64) NOT NULL,
    steps jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE approval_flow; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.approval_flow IS '审批链配置（项目级 → 租户级 → 平台默认回落；steps 环节引用职责槽位 code）';


--
-- Name: building; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.building (
    id uuid NOT NULL,
    community_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    type character varying(16) DEFAULT 'building'::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    tenant_id uuid,
    CONSTRAINT chk_building_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text]))),
    CONSTRAINT chk_building_type CHECK (((type)::text = ANY (ARRAY[('building'::character varying)::text, ('area'::character varying)::text])))
);


--
-- Name: TABLE building; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.building IS '楼栋/区域（点位挂载单元）';


--
-- Name: casbin_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.casbin_rule (
    id bigint NOT NULL,
    ptype character varying(100),
    v0 character varying(100),
    v1 character varying(100),
    v2 character varying(100),
    v3 character varying(100),
    v4 character varying(100),
    v5 character varying(100)
);


--
-- Name: TABLE casbin_rule; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.casbin_rule IS 'Casbin 策略表（p：角色→权限点；g：用户→角色，域 default）';


--
-- Name: casbin_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.casbin_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: casbin_rule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.casbin_rule_id_seq OWNED BY public.casbin_rule.id;


--
-- Name: check_template; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.check_template (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    point_type character varying(32) DEFAULT ''::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    tenant_id uuid
);


--
-- Name: TABLE check_template; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.check_template IS '检查项模板：items=[{name,required,photo_required}]，photo_required=none/optional/required（缺省 none），point_type 空为通用';


--
-- Name: check_template_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.check_template_item (
    id uuid NOT NULL,
    template_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    requirement text,
    required boolean DEFAULT false NOT NULL,
    photo_required character varying(16) DEFAULT 'none'::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    ai_hint text,
    judge_type character varying(24) DEFAULT 'general'::character varying NOT NULL,
    judge_config jsonb,
    CONSTRAINT chk_tpl_item_photo_req CHECK (((photo_required)::text = ANY (ARRAY[('none'::character varying)::text, ('optional'::character varying)::text, ('required'::character varying)::text])))
);


--
-- Name: TABLE check_template_item; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.check_template_item IS '模板检查项（v18 起替代 check_template.items JSONB）；requirement=检查标准要求文本（可空）';


--
-- Name: COLUMN check_template_item.ai_hint; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.check_template_item.ai_hint IS 'AI 识别要点（逐项送大模型的提示文本）；NULL/空 = 该项不带识别要点';


--
-- Name: COLUMN check_template_item.judge_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.check_template_item.judge_type IS '判定类型：general/presence/damage/metric/state/label/passage/leak/indicator/tidiness/baseline（二期预留，一期按 general）';


--
-- Name: COLUMN check_template_item.judge_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.check_template_item.judge_config IS '判定参数（metric: {"metric":"温度","unit":"℃","min":0,"max":40}；state/indicator: {"expected":"..."}），NULL 按通用判定';


--
-- Name: checkin_item_draft; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkin_item_draft (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    task_id uuid NOT NULL,
    point_id uuid NOT NULL,
    inspector_id uuid NOT NULL,
    community_id uuid NOT NULL,
    item_name character varying(128) NOT NULL,
    file_keys jsonb DEFAULT '[]'::jsonb NOT NULL,
    ai_status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    ai_verdict character varying(16),
    ai_reason character varying(512),
    ai_reading character varying(64),
    quality_pass boolean,
    quality_issue character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    job_id character varying(64) DEFAULT ''::character varying NOT NULL,
    manual_pass boolean,
    manual_note character varying(512) DEFAULT ''::character varying NOT NULL
);


--
-- Name: TABLE checkin_item_draft; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.checkin_item_draft IS '逐项 AI 识别过程草稿（点位正式提交后删除；仅过程记录，不计入进度）';


--
-- Name: COLUMN checkin_item_draft.ai_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_item_draft.ai_status IS 'pending=识别中 / done=识别完成 / failed=识别失败';


--
-- Name: COLUMN checkin_item_draft.manual_pass; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_item_draft.manual_pass IS '手动项选择结果（NULL=非手动项/未选择）';


--
-- Name: COLUMN checkin_item_draft.manual_note; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_item_draft.manual_note IS '手动项异常描述';


--
-- Name: checkin_record; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkin_record (
    id uuid NOT NULL,
    task_id uuid NOT NULL,
    point_id uuid NOT NULL,
    inspector_id uuid NOT NULL,
    community_id uuid NOT NULL,
    checkin_time timestamp with time zone DEFAULT now() NOT NULL,
    client_time timestamp with time zone,
    longitude numeric(10,7),
    latitude numeric(10,7),
    distance_to_point numeric(10,2),
    checkin_type character varying(16) NOT NULL,
    result character varying(16) DEFAULT 'normal'::character varying NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_suspect boolean DEFAULT false NOT NULL,
    suspect_reason character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    audit_status character varying(16) DEFAULT 'auto_pass'::character varying NOT NULL,
    audit_by uuid,
    audit_at timestamp with time zone,
    audit_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    ai_verdict character varying(16) DEFAULT ''::character varying NOT NULL,
    ai_reason character varying(512) DEFAULT ''::character varying NOT NULL,
    tenant_id uuid,
    audit_step smallint DEFAULT 0 NOT NULL,
    force_submit boolean DEFAULT false NOT NULL,
    ai_quality_pass boolean,
    ai_quality_issue character varying(255) DEFAULT ''::character varying NOT NULL,
    locked_at timestamp with time zone,
    superseded_by character varying(36),
    CONSTRAINT chk_checkin_result CHECK (((result)::text = ANY (ARRAY[('normal'::character varying)::text, ('abnormal'::character varying)::text]))),
    CONSTRAINT chk_checkin_type CHECK (((checkin_type)::text = ANY (ARRAY[('qrcode'::character varying)::text, ('fence'::character varying)::text, ('offline'::character varying)::text, ('nfc'::character varying)::text])))
)
PARTITION BY RANGE (created_at);


--
-- Name: TABLE checkin_record; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.checkin_record IS '打卡记录（按月分区，只增不改；不允许删除，防篡改审计要求）';


--
-- Name: COLUMN checkin_record.checkin_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.checkin_time IS '服务端时间，准点率/及时率统计以此为准';


--
-- Name: COLUMN checkin_record.client_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.client_time IS '客户端上报的原始时间，离线补传场景用于复核展示';


--
-- Name: COLUMN checkin_record.distance_to_point; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.distance_to_point IS '距点位距离（米），超 fence_radius 即标疑似作弊';


--
-- Name: COLUMN checkin_record.audit_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.audit_status IS 'auto_pass 默认通过 / pending 待审核 / pass 人工通过 / rejected 已打回（不级联任务进度）';


--
-- Name: COLUMN checkin_record.ai_verdict; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.ai_verdict IS '大模型结论：pass / review / error（空=未启用）';


--
-- Name: COLUMN checkin_record.audit_step; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.audit_step IS '审批链当前进度：已通过环节数（0=待第 1 环节）';


--
-- Name: COLUMN checkin_record.force_submit; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.force_submit IS '重拍次数用尽后强制提交（跳过同步 AI 判定，转人工复核）';


--
-- Name: COLUMN checkin_record.ai_quality_pass; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.ai_quality_pass IS 'AI 照片质量判定（第一层）；NULL=未做质量判定';


--
-- Name: COLUMN checkin_record.ai_quality_issue; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.ai_quality_issue IS 'AI 照片质量问题描述（质量不达标时的重拍提示）';


--
-- Name: COLUMN checkin_record.locked_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.locked_at IS '报告归档锁定时间（非空=已随周期报告归档，该点位打卡不可覆盖修改）';


--
-- Name: COLUMN checkin_record.superseded_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record.superseded_by IS '覆盖修改：被哪条新打卡记录覆盖（非空=已失效旧记录，列表/统计/报告均过滤）';


--
-- Name: checkin_record_default; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkin_record_default (
    id uuid NOT NULL,
    task_id uuid NOT NULL,
    point_id uuid NOT NULL,
    inspector_id uuid NOT NULL,
    community_id uuid NOT NULL,
    checkin_time timestamp with time zone DEFAULT now() NOT NULL,
    client_time timestamp with time zone,
    longitude numeric(10,7),
    latitude numeric(10,7),
    distance_to_point numeric(10,2),
    checkin_type character varying(16) NOT NULL,
    result character varying(16) DEFAULT 'normal'::character varying NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_suspect boolean DEFAULT false NOT NULL,
    suspect_reason character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    audit_status character varying(16) DEFAULT 'auto_pass'::character varying NOT NULL,
    audit_by uuid,
    audit_at timestamp with time zone,
    audit_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    ai_verdict character varying(16) DEFAULT ''::character varying NOT NULL,
    ai_reason character varying(512) DEFAULT ''::character varying NOT NULL,
    tenant_id uuid,
    audit_step smallint DEFAULT 0 NOT NULL,
    force_submit boolean DEFAULT false NOT NULL,
    ai_quality_pass boolean,
    ai_quality_issue character varying(255) DEFAULT ''::character varying NOT NULL,
    locked_at timestamp with time zone,
    superseded_by character varying(36),
    CONSTRAINT chk_checkin_result CHECK (((result)::text = ANY (ARRAY[('normal'::character varying)::text, ('abnormal'::character varying)::text]))),
    CONSTRAINT chk_checkin_type CHECK (((checkin_type)::text = ANY (ARRAY[('qrcode'::character varying)::text, ('fence'::character varying)::text, ('offline'::character varying)::text, ('nfc'::character varying)::text])))
);


--
-- Name: checkin_record_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkin_record_item (
    id uuid NOT NULL,
    record_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    requirement text,
    photo_required character varying(16) DEFAULT 'none'::character varying NOT NULL,
    pass boolean DEFAULT false NOT NULL,
    note character varying(512) DEFAULT ''::character varying NOT NULL,
    photos jsonb DEFAULT '[]'::jsonb NOT NULL,
    ai_verdict character varying(16),
    ai_reason character varying(512),
    sort integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    ai_hint text,
    judge_type character varying(24) DEFAULT 'general'::character varying NOT NULL,
    judge_config jsonb,
    ai_reading character varying(64),
    CONSTRAINT chk_rec_item_photo_req CHECK (((photo_required)::text = ANY (ARRAY[('none'::character varying)::text, ('optional'::character varying)::text, ('required'::character varying)::text])))
);


--
-- Name: TABLE checkin_record_item; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.checkin_record_item IS '打卡逐项结果快照（v18 起替代 checkin_record.check_items JSONB）；ai_verdict/ai_reason 为逐项大模型结论（预留）';


--
-- Name: COLUMN checkin_record_item.record_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.record_id IS '打卡记录 id（无 FK：checkin_record 为分区表，主键含分区键）';


--
-- Name: COLUMN checkin_record_item.photos; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.photos IS '该项照片 file_key 数组';


--
-- Name: COLUMN checkin_record_item.ai_hint; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.ai_hint IS 'AI 识别要点快照（打卡当时从模板项复制，历史记录不依赖模板表）';


--
-- Name: COLUMN checkin_record_item.judge_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.judge_type IS '判定类型快照（打卡当时从模板项复制，历史记录不依赖模板表）';


--
-- Name: COLUMN checkin_record_item.judge_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.judge_config IS '判定参数快照（打卡当时从模板项复制）';


--
-- Name: COLUMN checkin_record_item.ai_reading; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.checkin_record_item.ai_reading IS 'AI 读取的表计读数文本（metric 类检查项；NULL=无读数）';


--
-- Name: community; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.community (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    address character varying(255) DEFAULT ''::character varying NOT NULL,
    manager_id uuid,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    tenant_id uuid NOT NULL,
    CONSTRAINT chk_community_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE community; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.community IS '小区/项目';


--
-- Name: COLUMN community.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.community.tenant_id IS '所属租户（业务表经 community 链路天然隔离）';


--
-- Name: duty_binding; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.duty_binding (
    id uuid NOT NULL,
    tenant_id uuid,
    project_id uuid,
    slot character varying(64) NOT NULL,
    post_codes jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE duty_binding; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.duty_binding IS '职责槽位绑定：project_id 空=租户/平台级默认，非空=项目级覆盖；post_codes 空=该环节跳过/降级';


--
-- Name: COLUMN duty_binding.slot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.duty_binding.slot IS '槽位代码（系统固定枚举：report_sign_supervisor/report_sign_manager/order_triage/order_dispatch/order_accept/patrol_execute/patrol_report_line）';


--
-- Name: inspection_plan; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inspection_plan (
    id uuid NOT NULL,
    community_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    point_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    cycle_type character varying(16) NOT NULL,
    cycle_config jsonb DEFAULT '{}'::jsonb NOT NULL,
    inspector_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    start_date date NOT NULL,
    end_date date,
    time_window character varying(32) DEFAULT '08:00-18:00'::character varying NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    patrol_type character varying(16) DEFAULT 'safety'::character varying NOT NULL,
    tenant_id uuid,
    selection_mode character varying(16) DEFAULT 'explicit'::character varying NOT NULL,
    point_types jsonb,
    assign_mode character varying(16) DEFAULT 'all'::character varying NOT NULL,
    CONSTRAINT chk_plan_assign_mode CHECK (((assign_mode)::text = ANY ((ARRAY['all'::character varying, 'split'::character varying])::text[]))),
    CONSTRAINT chk_plan_cycle CHECK (((cycle_type)::text = ANY (ARRAY[('daily'::character varying)::text, ('weekly'::character varying)::text, ('monthly'::character varying)::text]))),
    CONSTRAINT chk_plan_dates CHECK (((end_date IS NULL) OR (end_date >= start_date))),
    CONSTRAINT chk_plan_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE inspection_plan; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.inspection_plan IS '巡检计划（周期生成任务的模板）';


--
-- Name: COLUMN inspection_plan.point_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.point_ids IS '有序点位 ID 数组，任务监控按此顺序展示路线';


--
-- Name: COLUMN inspection_plan.cycle_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.cycle_config IS '周期细则 JSONB，结构随 cycle_type 变化，见 §5.1';


--
-- Name: COLUMN inspection_plan.time_window; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.time_window IS '要求执行时段，格式 HH:MM-HH:MM';


--
-- Name: COLUMN inspection_plan.patrol_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.patrol_type IS '巡查类型：safety 安全巡查（默认）/ equipment 设备设施专项 / environment 环境 / building 楼栋';


--
-- Name: COLUMN inspection_plan.selection_mode; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.selection_mode IS '点位圈选模式：explicit 显式名单（point_ids）/ by_point_types 按点位类型动态圈选（point_types）';


--
-- Name: COLUMN inspection_plan.point_types; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.point_types IS '圈选点位类型（point_type 字典值数组，selection_mode=by_point_types 时必填）';


--
-- Name: COLUMN inspection_plan.assign_mode; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_plan.assign_mode IS '点位分配方式：all 每个执行日全量 / split 按执行日均分（仅 weekly/monthly 合法）';


--
-- Name: inspection_point; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inspection_point (
    id uuid NOT NULL,
    community_id uuid NOT NULL,
    building_id uuid,
    name character varying(128) NOT NULL,
    type character varying(32) DEFAULT 'common'::character varying NOT NULL,
    qrcode_no character varying(64) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    fence_radius integer DEFAULT 100 NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    template_id uuid,
    nfc_id character varying(64) DEFAULT ''::character varying NOT NULL,
    credential character varying(16) DEFAULT 'qrcode'::character varying NOT NULL,
    require_fence boolean DEFAULT false NOT NULL,
    tenant_id uuid,
    unit_no integer,
    floor integer,
    CONSTRAINT chk_point_credential CHECK (((credential)::text = ANY (ARRAY[('qrcode'::character varying)::text, ('nfc'::character varying)::text, ('none'::character varying)::text, ('any'::character varying)::text]))),
    CONSTRAINT chk_point_lat CHECK (((latitude >= ('-90'::integer)::numeric) AND (latitude <= (90)::numeric))),
    CONSTRAINT chk_point_lng CHECK (((longitude >= ('-180'::integer)::numeric) AND (longitude <= (180)::numeric))),
    CONSTRAINT chk_point_radius CHECK (((fence_radius >= 10) AND (fence_radius <= 2000))),
    CONSTRAINT chk_point_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE inspection_point; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.inspection_point IS '巡检点位（二维码 + GPS 围栏双支持）';


--
-- Name: COLUMN inspection_point.qrcode_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_point.qrcode_no IS '二维码编号，全局唯一；码值编码规则见 §5.4';


--
-- Name: COLUMN inspection_point.longitude; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_point.longitude IS 'GCJ-02 经度（与腾讯/微信小程序坐标一致，见 §5.2）';


--
-- Name: COLUMN inspection_point.nfc_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_point.nfc_id IS 'NFC 卡号（打卡方式 nfc 时校验）';


--
-- Name: COLUMN inspection_point.unit_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_point.unit_no IS '单元号（NULL=不分单元/非楼栋点位）';


--
-- Name: COLUMN inspection_point.floor; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_point.floor IS '楼层（负数=地下层，-1 即 B1；NULL=非楼栋点位）';


--
-- Name: inspection_report; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inspection_report (
    id uuid NOT NULL,
    community_id uuid NOT NULL,
    period character(7) NOT NULL,
    title character varying(128) NOT NULL,
    status character varying(24) DEFAULT 'pending_inspector'::character varying NOT NULL,
    stats jsonb DEFAULT '{}'::jsonb NOT NULL,
    inspector_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    inspector_signed jsonb DEFAULT '[]'::jsonb NOT NULL,
    supervisor_by uuid,
    supervisor_at timestamp with time zone,
    supervisor_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    manager_by uuid,
    manager_at timestamp with time zone,
    manager_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    reject_reason character varying(512) DEFAULT ''::character varying NOT NULL,
    file_key character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    supervisor_signature_key character varying(255) DEFAULT ''::character varying NOT NULL,
    manager_signature_key character varying(255) DEFAULT ''::character varying NOT NULL,
    seal_file_key character varying(255) DEFAULT ''::character varying NOT NULL,
    supervisor_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    manager_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    tenant_id uuid,
    patrol_type character varying(32),
    plan_id uuid
);


--
-- Name: TABLE inspection_report; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.inspection_report IS '月度巡检报告：pending_inspector→pending_supervisor→pending_manager→approved，驳回回 pending_inspector';


--
-- Name: COLUMN inspection_report.supervisor_signature_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.supervisor_signature_key IS '主管签字时的手写签名图快照 file_key';


--
-- Name: COLUMN inspection_report.manager_signature_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.manager_signature_key IS '经理终审时的手写签名图快照 file_key';


--
-- Name: COLUMN inspection_report.seal_file_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.seal_file_key IS '终审通过时的公章资产 file_key 快照（空=终审时无公章或存量报告）';


--
-- Name: COLUMN inspection_report.supervisor_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.supervisor_ids IS 'JSONB 指定安全主管签字人 ID 数组（空=该级跳过）';


--
-- Name: COLUMN inspection_report.manager_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.manager_ids IS 'JSONB 指定物业经理签字人 ID 数组（空=该级跳过）';


--
-- Name: COLUMN inspection_report.patrol_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.patrol_type IS '巡查类型（字典 patrol_type 的 value）；NULL/空 = 综合月报（全类型口径），非空 = 该类型专项检查报告';


--
-- Name: COLUMN inspection_report.plan_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_report.plan_id IS '溯源生成它的巡检计划（仅记录，综合月报为 NULL）';


--
-- Name: inspection_task; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inspection_task (
    id uuid NOT NULL,
    plan_id uuid NOT NULL,
    community_id uuid NOT NULL,
    inspector_id uuid NOT NULL,
    task_date date NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    total_points integer DEFAULT 0 NOT NULL,
    done_points integer DEFAULT 0 NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    patrol_type character varying(16) DEFAULT 'safety'::character varying NOT NULL,
    tenant_id uuid,
    round_name character varying(32),
    time_window character varying(32),
    point_ids jsonb,
    CONSTRAINT chk_task_points CHECK (((done_points >= 0) AND (total_points >= 0) AND (done_points <= total_points))),
    CONSTRAINT chk_task_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('doing'::character varying)::text, ('done'::character varying)::text, ('overdue'::character varying)::text])))
);


--
-- Name: TABLE inspection_task; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.inspection_task IS '巡检任务（计划按日/人实例化）';


--
-- Name: COLUMN inspection_task.task_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.task_date IS '任务归属日期；与 plan_id+inspector_id 联合唯一，保证计划生成幂等';


--
-- Name: COLUMN inspection_task.done_points; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.done_points IS '进度计数，打卡成功事务内 +1，与 checkin_record 最终一致';


--
-- Name: COLUMN inspection_task.patrol_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.patrol_type IS '巡查类型（生成任务时从计划快照，同 inspection_plan.patrol_type）';


--
-- Name: COLUMN inspection_task.round_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.round_name IS '巡更轮次名快照（cycle_config.rounds[].name）；非轮次任务为 NULL';


--
-- Name: COLUMN inspection_task.time_window; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.time_window IS '本任务执行时段快照 HH:MM-HH:MM（轮次任务取 rounds[].window，允许跨零点）；非轮次任务为 NULL，展示/统计回落计划 time_window';


--
-- Name: COLUMN inspection_task.point_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.inspection_task.point_ids IS '任务点位名单快照（生成时展开：explicit 照抄计划名单，by_point_types 实时圈选）；空则消费侧回落计划 point_ids（兼容存量任务）';


--
-- Name: post_dict; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_dict (
    id uuid NOT NULL,
    tenant_id uuid,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    is_supervisor boolean DEFAULT false NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    line character varying(32) DEFAULT 'general'::character varying NOT NULL,
    role_id uuid,
    sort integer DEFAULT 0 NOT NULL,
    CONSTRAINT chk_post_dict_line CHECK (((line)::text = ANY (ARRAY[('safety'::character varying)::text, ('engineering'::character varying)::text, ('environment'::character varying)::text, ('service'::character varying)::text, ('general'::character varying)::text]))),
    CONSTRAINT chk_post_dict_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE post_dict; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.post_dict IS '岗位字典（tenant_id 空=平台内置模板岗位，开通租户时复制；project_staff.posts 引用 code）';


--
-- Name: COLUMN post_dict.is_supervisor; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.post_dict.is_supervisor IS '是否主管级（数据范围推导用：主管级 → project 档）';


--
-- Name: COLUMN post_dict.line; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.post_dict.line IS '业务线：safety 安全 / engineering 工程 / environment 环境 / service 客服 / general 综合（扁平+分组展示，不做树）';


--
-- Name: COLUMN post_dict.role_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.post_dict.role_id IS '岗位绑定角色（→sys_role.id，可空）：有效角色=手动分配角色∪在职岗位绑定角色（实时并集，不落库）';


--
-- Name: COLUMN post_dict.sort; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.post_dict.sort IS '业务线内排序（升序）';


--
-- Name: project_staff; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_staff (
    id uuid NOT NULL,
    tenant_id uuid,
    project_id uuid NOT NULL,
    user_id uuid NOT NULL,
    posts jsonb DEFAULT '[]'::jsonb NOT NULL,
    building_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_project_staff_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE project_staff; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.project_staff IS '项目岗位编制：一人多岗（posts 引用 post_dict.code），一人多项目；project_manager 每项目至多一人（保存时校验）';


--
-- Name: COLUMN project_staff.project_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.project_staff.project_id IS '项目（→ community.id）';


--
-- Name: COLUMN project_staff.building_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.project_staff.building_ids IS '责任楼栋（仅楼管员用，空=全部楼栋）';


--
-- Name: qrcode_no_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.qrcode_no_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sign_asset; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sign_asset (
    id uuid NOT NULL,
    asset_type character varying(20) NOT NULL,
    owner_id uuid,
    file_key character varying(255) NOT NULL,
    sha256 character varying(64) DEFAULT ''::character varying NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    revoked_reason character varying(255) DEFAULT ''::character varying NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sign_asset_owner CHECK (((((asset_type)::text = 'user_signature'::text) AND (owner_id IS NOT NULL)) OR (((asset_type)::text = 'company_seal'::text) AND (owner_id IS NULL)))),
    CONSTRAINT chk_sign_asset_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('replaced'::character varying)::text, ('revoked'::character varying)::text]))),
    CONSTRAINT chk_sign_asset_type CHECK (((asset_type)::text = ANY (ARRAY[('user_signature'::character varying)::text, ('company_seal'::character varying)::text])))
);


--
-- Name: TABLE sign_asset; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sign_asset IS '签章资产：user_signature(owner_id=用户)/company_seal(owner_id NULL)，同租户+type+owner 仅一条 active；version 同租户+type+owner 内递增';


--
-- Name: COLUMN sign_asset.sha256; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sign_asset.sha256 IS '文件内容 SHA-256 指纹（创建时计算；文件缺失/不可读容错为空串）';


--
-- Name: sys_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_config (
    id uuid NOT NULL,
    key character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    config_group character varying(50) DEFAULT 'system'::character varying NOT NULL
);


--
-- Name: TABLE sys_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_config IS '系统参数配置（围栏半径、水印开关、作弊阈值等）';


--
-- Name: COLUMN sys_config.config_group; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_config.config_group IS '参数分组（按 key 前缀归组：inspection/mp/msg/security/auth，其余 system）';


--
-- Name: sys_dict_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_dict_data (
    id uuid NOT NULL,
    type_code character varying(64) NOT NULL,
    label character varying(64) NOT NULL,
    value character varying(64) NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    attrs jsonb,
    CONSTRAINT chk_sys_dict_data_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE sys_dict_data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_dict_data IS '字典数据（可配置枚举项）';


--
-- Name: COLUMN sys_dict_data.attrs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict_data.attrs IS '通用扩展属性（jsonb）；patrol_type 用 attrs.category 标记大类 daily_patrol/special';


--
-- Name: sys_dict_type; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_dict_type (
    id uuid NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: TABLE sys_dict_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_dict_type IS '字典类型';


--
-- Name: sys_login_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_login_log (
    id uuid NOT NULL,
    user_id uuid,
    username character varying(64) DEFAULT ''::character varying NOT NULL,
    channel character varying(16) DEFAULT 'admin'::character varying NOT NULL,
    ip character varying(64) DEFAULT ''::character varying NOT NULL,
    ua character varying(512) DEFAULT ''::character varying NOT NULL,
    status character varying(16) NOT NULL,
    msg character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sys_login_log_channel CHECK (((channel)::text = ANY (ARRAY[('admin'::character varying)::text, ('mp'::character varying)::text, ('app'::character varying)::text]))),
    CONSTRAINT chk_sys_login_log_status CHECK (((status)::text = ANY (ARRAY[('success'::character varying)::text, ('fail'::character varying)::text])))
);


--
-- Name: TABLE sys_login_log; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_login_log IS '登录日志（只增不改，无 updated_at/deleted_at）';


--
-- Name: COLUMN sys_login_log.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_login_log.tenant_id IS '所属租户（日志管理按租户上下文过滤；无法识别用户时归默认租户）';


--
-- Name: sys_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_menu (
    id uuid NOT NULL,
    parent_id uuid,
    title character varying(64) NOT NULL,
    path character varying(255) DEFAULT ''::character varying NOT NULL,
    icon character varying(64) DEFAULT ''::character varying NOT NULL,
    type character varying(8) NOT NULL,
    perms character varying(128) DEFAULT ''::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    visible boolean DEFAULT true NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    is_builtin boolean DEFAULT false NOT NULL,
    is_platform boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_sys_menu_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text]))),
    CONSTRAINT chk_sys_menu_type CHECK (((type)::text = ANY (ARRAY[('dir'::character varying)::text, ('menu'::character varying)::text, ('button'::character varying)::text])))
);


--
-- Name: TABLE sys_menu; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_menu IS '菜单与按钮权限点（树形）';


--
-- Name: COLUMN sys_menu.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_menu.type IS 'dir 目录 / menu 菜单 / button 按钮权限点';


--
-- Name: COLUMN sys_menu.perms; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_menu.perms IS '权限标识，后端 RBAC 中间件按此鉴权';


--
-- Name: COLUMN sys_menu.is_builtin; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_menu.is_builtin IS '内置菜单不可删除（错误码 41007）';


--
-- Name: COLUMN sys_menu.is_platform; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_menu.is_platform IS '平台级菜单（系统管理目录整棵子树）：仅超管可见可授权，租户角色不可分配';


--
-- Name: sys_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_message (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    type character varying(16) NOT NULL,
    title character varying(128) NOT NULL,
    content character varying(512) DEFAULT ''::character varying NOT NULL,
    biz_id uuid,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sys_message_type CHECK (((type)::text = ANY ((ARRAY['task'::character varying, 'export'::character varying, 'system'::character varying, 'checkin_audit'::character varying, 'report'::character varying, 'announcement'::character varying])::text[])))
);


--
-- Name: TABLE sys_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_message IS '站内消息（任务提醒/工单指派/整改驳回/导出完成）';


--
-- Name: sys_notice; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_notice (
    id uuid NOT NULL,
    title character varying(64) NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    publish_at timestamp with time zone,
    created_by uuid,
    created_by_name character varying(64) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    attachments jsonb,
    tenant_id uuid,
    CONSTRAINT chk_sys_notice_status CHECK ((status = ANY (ARRAY[0, 1, 2])))
);


--
-- Name: TABLE sys_notice; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_notice IS '通知公告（面向巡检员）';


--
-- Name: sys_operation_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_operation_log (
    id uuid NOT NULL,
    user_id uuid,
    username character varying(64) DEFAULT ''::character varying NOT NULL,
    module character varying(64) DEFAULT ''::character varying NOT NULL,
    action character varying(64) DEFAULT ''::character varying NOT NULL,
    method character varying(8) DEFAULT ''::character varying NOT NULL,
    path character varying(255) DEFAULT ''::character varying NOT NULL,
    params text DEFAULT ''::text NOT NULL,
    ip character varying(64) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'success'::character varying NOT NULL,
    cost_ms integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sys_op_log_status CHECK (((status)::text = ANY (ARRAY[('success'::character varying)::text, ('fail'::character varying)::text])))
)
PARTITION BY RANGE (created_at);


--
-- Name: TABLE sys_operation_log; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_operation_log IS '操作日志（按月分区，只增不改）';


--
-- Name: COLUMN sys_operation_log.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_operation_log.tenant_id IS '操作者所属租户（日志管理按租户上下文过滤）';


--
-- Name: sys_operation_log_default; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_operation_log_default (
    id uuid NOT NULL,
    user_id uuid,
    username character varying(64) DEFAULT ''::character varying NOT NULL,
    module character varying(64) DEFAULT ''::character varying NOT NULL,
    action character varying(64) DEFAULT ''::character varying NOT NULL,
    method character varying(8) DEFAULT ''::character varying NOT NULL,
    path character varying(255) DEFAULT ''::character varying NOT NULL,
    params text DEFAULT ''::text NOT NULL,
    ip character varying(64) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'success'::character varying NOT NULL,
    cost_ms integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sys_op_log_status CHECK (((status)::text = ANY (ARRAY[('success'::character varying)::text, ('fail'::character varying)::text])))
);


--
-- Name: sys_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_role (
    id uuid NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    data_scope character varying(16) DEFAULT 'project'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    is_builtin boolean DEFAULT false NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_sys_role_scope CHECK (((data_scope)::text = ANY (ARRAY[('all'::character varying)::text, ('project'::character varying)::text, ('self'::character varying)::text]))),
    CONSTRAINT chk_sys_role_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE sys_role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_role IS '角色（RBAC）';


--
-- Name: COLUMN sys_role.code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_role.code IS '角色编码，程序内引用（如超管 super_admin），租户内唯一（内置角色全平台唯一）';


--
-- Name: COLUMN sys_role.data_scope; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_role.data_scope IS '数据范围上限：all 全部项目 / project 所在项目（按岗位编制推导）/ self 仅本人相关';


--
-- Name: COLUMN sys_role.is_builtin; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_role.is_builtin IS '内置角色不可删除（错误码 41007）';


--
-- Name: COLUMN sys_role.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_role.tenant_id IS '所属租户；NULL=内置角色（全平台共享，只读不可改）';


--
-- Name: sys_role_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_role_menu (
    role_id uuid NOT NULL,
    menu_id uuid NOT NULL
);


--
-- Name: TABLE sys_role_menu; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_role_menu IS '角色-菜单多对多关联';


--
-- Name: sys_user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_user (
    id uuid NOT NULL,
    username character varying(64) NOT NULL,
    password character varying(128) NOT NULL,
    name character varying(64) NOT NULL,
    phone character varying(20) DEFAULT ''::character varying NOT NULL,
    openid character varying(64),
    avatar character varying(512) DEFAULT ''::character varying NOT NULL,
    role_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    last_login_at timestamp with time zone,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    must_change_password boolean DEFAULT false NOT NULL,
    is_builtin boolean DEFAULT false NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT chk_sys_user_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE sys_user; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_user IS '系统用户（后台管理员/巡检员/维修工）';


--
-- Name: COLUMN sys_user.username; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.username IS '登录账号，租户内唯一（软删部分唯一索引）';


--
-- Name: COLUMN sys_user.password; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.password IS 'bcrypt 密码哈希';


--
-- Name: COLUMN sys_user.openid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.openid IS '微信小程序 openid，小程序登录凭据';


--
-- Name: COLUMN sys_user.role_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.role_ids IS 'JSONB 角色 ID 数组，应用层关联 sys_role';


--
-- Name: COLUMN sys_user.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.status IS 'enabled 启用 / disabled 停用';


--
-- Name: COLUMN sys_user.must_change_password; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.must_change_password IS '首次登录强制修改密码标记（批量导入用户置 true）';


--
-- Name: COLUMN sys_user.is_builtin; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.is_builtin IS '内置账号（admin）：禁止删除/停用/移除 super_admin 角色（错误码 41014）';


--
-- Name: COLUMN sys_user.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_user.tenant_id IS '所属租户（登录账号租户内唯一）';


--
-- Name: tenant; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(128) NOT NULL,
    contact_name character varying(64) DEFAULT ''::character varying NOT NULL,
    contact_phone character varying(20) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_tenant_status CHECK (((status)::text = ANY (ARRAY[('enabled'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: TABLE tenant; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenant IS '租户（物业公司）：数据隔离基本单位；code 用于登录时多租户消歧（公司代码）';


--
-- Name: COLUMN tenant.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant.status IS 'enabled 启用 / disabled 停用（停用则该租户全部账号无法登录）';


--
-- Name: tenant_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_config (
    id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    key character varying(64) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE tenant_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenant_config IS '租户配置覆盖：读取规则「租户值→平台默认」（config.Resolve）；仅白名单 key 可写（品牌类先行），密钥类配置永不下放';


--
-- Name: upload_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upload_file (
    id uuid NOT NULL,
    file_key character varying(255) NOT NULL,
    scene character varying(16) NOT NULL,
    user_id uuid NOT NULL,
    mime_type character varying(128) DEFAULT ''::character varying NOT NULL,
    url character varying(512) DEFAULT ''::character varying NOT NULL,
    watermarked_url character varying(512) DEFAULT ''::character varying NOT NULL,
    exif_time timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    md5 character varying(64) DEFAULT ''::character varying NOT NULL,
    storage character varying(16) DEFAULT ''::character varying NOT NULL,
    tenant_id uuid,
    CONSTRAINT chk_upload_file_scene CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying, 'notice'::character varying, 'app'::character varying])::text[])))
);


--
-- Name: TABLE upload_file; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.upload_file IS '上传文件记录（打卡/工单/头像/导出）';


--
-- Name: COLUMN upload_file.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upload_file.name IS '原始文件名（上传时的文件名/生成文件名）';


--
-- Name: COLUMN upload_file.md5; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upload_file.md5 IS '内容 MD5（完整性校验/去重查询用）';


--
-- Name: COLUMN upload_file.storage; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upload_file.storage IS '存储驱动：local/oss/cos';


--
-- Name: user_push_device; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_push_device (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    cid character varying(128) NOT NULL,
    platform character varying(16) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_push_device; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_push_device IS 'App 推送设备绑定（uniPush 2.0 / 个推 V2 cid → 用户；一个 cid 只属于最后登录的人）';


--
-- Name: COLUMN user_push_device.cid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_push_device.cid IS '个推 SDK 客户端标识（CID），全局唯一，换账号登录即改绑';


--
-- Name: COLUMN user_push_device.platform; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_push_device.platform IS '设备平台：android / ios（离线推送厂商通道配置参考）';


--
-- Name: checkin_record_default; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkin_record ATTACH PARTITION public.checkin_record_default DEFAULT;


--
-- Name: sys_operation_log_default; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_operation_log ATTACH PARTITION public.sys_operation_log_default DEFAULT;


--
-- Name: casbin_rule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.casbin_rule ALTER COLUMN id SET DEFAULT nextval('public.casbin_rule_id_seq'::regclass);


--
-- Name: app_release app_release_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_release
    ADD CONSTRAINT app_release_pkey PRIMARY KEY (id);


--
-- Name: approval_flow approval_flow_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_flow
    ADD CONSTRAINT approval_flow_pkey PRIMARY KEY (id);


--
-- Name: building building_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.building
    ADD CONSTRAINT building_pkey PRIMARY KEY (id);


--
-- Name: casbin_rule casbin_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.casbin_rule
    ADD CONSTRAINT casbin_rule_pkey PRIMARY KEY (id);


--
-- Name: check_template_item check_template_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_template_item
    ADD CONSTRAINT check_template_item_pkey PRIMARY KEY (id);


--
-- Name: check_template check_template_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_template
    ADD CONSTRAINT check_template_pkey PRIMARY KEY (id);


--
-- Name: checkin_item_draft checkin_item_draft_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkin_item_draft
    ADD CONSTRAINT checkin_item_draft_pkey PRIMARY KEY (id);


--
-- Name: checkin_record_item checkin_record_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkin_record_item
    ADD CONSTRAINT checkin_record_item_pkey PRIMARY KEY (id);


--
-- Name: checkin_record checkin_record_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkin_record
    ADD CONSTRAINT checkin_record_pkey PRIMARY KEY (id, created_at);


--
-- Name: community community_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.community
    ADD CONSTRAINT community_pkey PRIMARY KEY (id);


--
-- Name: duty_binding duty_binding_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duty_binding
    ADD CONSTRAINT duty_binding_pkey PRIMARY KEY (id);


--
-- Name: inspection_plan inspection_plan_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_plan
    ADD CONSTRAINT inspection_plan_pkey PRIMARY KEY (id);


--
-- Name: inspection_point inspection_point_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_pkey PRIMARY KEY (id);


--
-- Name: inspection_report inspection_report_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_report
    ADD CONSTRAINT inspection_report_pkey PRIMARY KEY (id);


--
-- Name: inspection_task inspection_task_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_pkey PRIMARY KEY (id);


--
-- Name: post_dict post_dict_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_dict
    ADD CONSTRAINT post_dict_pkey PRIMARY KEY (id);


--
-- Name: project_staff project_staff_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_staff
    ADD CONSTRAINT project_staff_pkey PRIMARY KEY (id);


--
-- Name: sign_asset sign_asset_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sign_asset
    ADD CONSTRAINT sign_asset_pkey PRIMARY KEY (id);


--
-- Name: sys_config sys_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_config
    ADD CONSTRAINT sys_config_pkey PRIMARY KEY (id);


--
-- Name: sys_dict_data sys_dict_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_data
    ADD CONSTRAINT sys_dict_data_pkey PRIMARY KEY (id);


--
-- Name: sys_dict_type sys_dict_type_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_type
    ADD CONSTRAINT sys_dict_type_pkey PRIMARY KEY (id);


--
-- Name: sys_login_log sys_login_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_login_log
    ADD CONSTRAINT sys_login_log_pkey PRIMARY KEY (id);


--
-- Name: sys_menu sys_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_menu
    ADD CONSTRAINT sys_menu_pkey PRIMARY KEY (id);


--
-- Name: sys_message sys_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_message
    ADD CONSTRAINT sys_message_pkey PRIMARY KEY (id);


--
-- Name: sys_notice sys_notice_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_notice
    ADD CONSTRAINT sys_notice_pkey PRIMARY KEY (id);


--
-- Name: sys_operation_log sys_operation_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_operation_log
    ADD CONSTRAINT sys_operation_log_pkey PRIMARY KEY (id, created_at);


--
-- Name: sys_role_menu sys_role_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_pkey PRIMARY KEY (role_id, menu_id);


--
-- Name: sys_role sys_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role
    ADD CONSTRAINT sys_role_pkey PRIMARY KEY (id);


--
-- Name: sys_user sys_user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_user
    ADD CONSTRAINT sys_user_pkey PRIMARY KEY (id);


--
-- Name: tenant_config tenant_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_config
    ADD CONSTRAINT tenant_config_pkey PRIMARY KEY (id);


--
-- Name: tenant tenant_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant
    ADD CONSTRAINT tenant_pkey PRIMARY KEY (id);


--
-- Name: project_staff uk_project_staff; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_staff
    ADD CONSTRAINT uk_project_staff UNIQUE (project_id, user_id);


--
-- Name: user_push_device uk_push_device_cid; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_push_device
    ADD CONSTRAINT uk_push_device_cid UNIQUE (cid);


--
-- Name: tenant uk_tenant_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant
    ADD CONSTRAINT uk_tenant_code UNIQUE (code);


--
-- Name: tenant_config uk_tenant_config; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_config
    ADD CONSTRAINT uk_tenant_config UNIQUE (tenant_id, key);


--
-- Name: upload_file upload_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_file
    ADD CONSTRAINT upload_file_pkey PRIMARY KEY (id);


--
-- Name: user_push_device user_push_device_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_push_device
    ADD CONSTRAINT user_push_device_pkey PRIMARY KEY (id);


--
-- Name: idx_app_release_platform; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_app_release_platform ON public.app_release USING btree (platform, created_at DESC);


--
-- Name: idx_building_community; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_building_community ON public.building USING btree (community_id, sort) WHERE (deleted_at IS NULL);


--
-- Name: idx_casbin_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_casbin_rule ON public.casbin_rule USING btree (ptype, v0, v1, v2, v3, v4, v5);


--
-- Name: idx_checkin_audit; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_checkin_audit ON ONLY public.checkin_record USING btree (audit_status, created_at DESC);


--
-- Name: idx_checkin_inspector_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_checkin_inspector_time ON ONLY public.checkin_record USING btree (inspector_id, checkin_time DESC);


--
-- Name: idx_checkin_point_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_checkin_point_time ON ONLY public.checkin_record USING btree (point_id, checkin_time DESC);


--
-- Name: idx_checkin_suspect; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_checkin_suspect ON ONLY public.checkin_record USING btree (is_suspect, checkin_time DESC) WHERE is_suspect;


--
-- Name: idx_checkin_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_checkin_task ON ONLY public.checkin_record USING btree (task_id);


--
-- Name: idx_community_manager; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_community_manager ON public.community USING btree (manager_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_community_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_community_status ON public.community USING btree (status) WHERE (deleted_at IS NULL);


--
-- Name: idx_community_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_community_tenant ON public.community USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_item_draft_inspector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_item_draft_inspector ON public.checkin_item_draft USING btree (inspector_id, updated_at);


--
-- Name: idx_item_draft_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_item_draft_task ON public.checkin_item_draft USING btree (task_id, point_id);


--
-- Name: idx_plan_community_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_community_status ON public.inspection_plan USING btree (community_id, status) WHERE (deleted_at IS NULL);


--
-- Name: idx_plan_point_ids_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_point_ids_gin ON public.inspection_plan USING gin (point_ids jsonb_path_ops);


--
-- Name: idx_point_building; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_point_building ON public.inspection_point USING btree (building_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_point_community; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_point_community ON public.inspection_point USING btree (community_id, status) WHERE (deleted_at IS NULL);


--
-- Name: idx_post_dict_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_post_dict_role ON public.post_dict USING btree (role_id);


--
-- Name: idx_project_staff_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_staff_user ON public.project_staff USING btree (user_id);


--
-- Name: idx_push_device_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_push_device_user ON public.user_push_device USING btree (user_id);


--
-- Name: idx_rec_item_rec; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rec_item_rec ON public.checkin_record_item USING btree (record_id, sort);


--
-- Name: idx_sign_asset_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sign_asset_owner ON public.sign_asset USING btree (asset_type, owner_id, status);


--
-- Name: idx_sys_dict_data_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_dict_data_type ON public.sys_dict_data USING btree (type_code, status, sort) WHERE (deleted_at IS NULL);


--
-- Name: idx_sys_login_log_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_login_log_tenant ON public.sys_login_log USING btree (tenant_id);


--
-- Name: idx_sys_login_log_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_login_log_time ON public.sys_login_log USING btree (created_at DESC);


--
-- Name: idx_sys_login_log_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_login_log_user_time ON public.sys_login_log USING btree (user_id, created_at DESC);


--
-- Name: idx_sys_menu_parent_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_menu_parent_sort ON public.sys_menu USING btree (parent_id, sort) WHERE (deleted_at IS NULL);


--
-- Name: idx_sys_menu_perms; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_menu_perms ON public.sys_menu USING btree (perms) WHERE ((deleted_at IS NULL) AND ((perms)::text <> ''::text));


--
-- Name: idx_sys_message_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_message_user ON public.sys_message USING btree (user_id, is_read, created_at DESC);


--
-- Name: idx_sys_notice_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_notice_status ON public.sys_notice USING btree (status, publish_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_sys_op_log_module_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_op_log_module_time ON ONLY public.sys_operation_log USING btree (module, created_at DESC);


--
-- Name: idx_sys_op_log_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_op_log_user_time ON ONLY public.sys_operation_log USING btree (user_id, created_at DESC);


--
-- Name: idx_sys_operation_log_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_operation_log_tenant ON ONLY public.sys_operation_log USING btree (tenant_id);


--
-- Name: idx_sys_role_menu_menu; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_role_menu_menu ON public.sys_role_menu USING btree (menu_id);


--
-- Name: idx_sys_role_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_role_tenant ON public.sys_role USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_sys_user_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_user_phone ON public.sys_user USING btree (phone) WHERE (deleted_at IS NULL);


--
-- Name: idx_sys_user_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_user_tenant ON public.sys_user USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_task_community_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_community_date ON public.inspection_task USING btree (community_id, task_date) WHERE (deleted_at IS NULL);


--
-- Name: idx_task_date_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_date_status ON public.inspection_task USING btree (task_date, status) WHERE (deleted_at IS NULL);


--
-- Name: idx_task_inspector_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_inspector_date ON public.inspection_task USING btree (inspector_id, task_date DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_tpl_item_tpl; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tpl_item_tpl ON public.check_template_item USING btree (template_id, sort);


--
-- Name: idx_upload_file_md5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upload_file_md5 ON public.upload_file USING btree (md5) WHERE ((md5)::text <> (''::character varying)::text);


--
-- Name: idx_upload_file_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upload_file_user ON public.upload_file USING btree (user_id, scene);


--
-- Name: sys_operation_log_default_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sys_operation_log_default_tenant_id_idx ON public.sys_operation_log_default USING btree (tenant_id);


--
-- Name: uk_approval_flow_scope; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_approval_flow_scope ON public.approval_flow USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), flow_code);


--
-- Name: uk_building_comm_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_building_comm_name ON public.building USING btree (community_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uk_checkin_task_point; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_checkin_task_point ON ONLY public.checkin_record USING btree (task_id, point_id, created_at);


--
-- Name: uk_duty_binding_scope_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_duty_binding_scope_slot ON public.duty_binding USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), slot);


--
-- Name: uk_inspection_report; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period, COALESCE(patrol_type, ''::character varying)) WHERE (deleted_at IS NULL);


--
-- Name: uk_item_draft_item; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_item_draft_item ON public.checkin_item_draft USING btree (task_id, point_id, item_name);


--
-- Name: uk_point_qrcode; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_point_qrcode ON public.inspection_point USING btree (qrcode_no) WHERE (deleted_at IS NULL);


--
-- Name: uk_post_dict_tenant_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_post_dict_tenant_code ON public.post_dict USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code);


--
-- Name: uk_sign_asset_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sign_asset_active ON public.sign_asset USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), asset_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE ((status)::text = 'active'::text);


--
-- Name: uk_sys_config_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_config_key ON public.sys_config USING btree (key) WHERE (deleted_at IS NULL);


--
-- Name: uk_sys_dict_data_type_value; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_dict_data_type_value ON public.sys_dict_data USING btree (type_code, value) WHERE (deleted_at IS NULL);


--
-- Name: uk_sys_dict_type_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_dict_type_code ON public.sys_dict_type USING btree (code);


--
-- Name: uk_sys_role_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_role_code ON public.sys_role USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code) WHERE (deleted_at IS NULL);


--
-- Name: uk_sys_user_openid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_user_openid ON public.sys_user USING btree (openid) WHERE ((openid IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: uk_sys_user_username; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_sys_user_username ON public.sys_user USING btree (tenant_id, username) WHERE (deleted_at IS NULL);


--
-- Name: uk_task_plan_date_inspector; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_task_plan_date_inspector ON public.inspection_task USING btree (plan_id, task_date, inspector_id, COALESCE(round_name, ''::character varying)) WHERE (deleted_at IS NULL);


--
-- Name: uk_upload_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_upload_file_key ON public.upload_file USING btree (file_key);


--
-- Name: sys_operation_log_default_tenant_id_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_sys_operation_log_tenant ATTACH PARTITION public.sys_operation_log_default_tenant_id_idx;


--
-- Name: building building_community_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.building
    ADD CONSTRAINT building_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);


--
-- Name: check_template_item check_template_item_template_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_template_item
    ADD CONSTRAINT check_template_item_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.check_template(id) ON DELETE CASCADE;


--
-- Name: checkin_record checkin_record_point_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.checkin_record
    ADD CONSTRAINT checkin_record_point_id_fkey FOREIGN KEY (point_id) REFERENCES public.inspection_point(id);


--
-- Name: checkin_record checkin_record_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.checkin_record
    ADD CONSTRAINT checkin_record_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.inspection_task(id);


--
-- Name: inspection_plan inspection_plan_community_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_plan
    ADD CONSTRAINT inspection_plan_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);


--
-- Name: inspection_point inspection_point_building_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_building_id_fkey FOREIGN KEY (building_id) REFERENCES public.building(id);


--
-- Name: inspection_point inspection_point_community_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);


--
-- Name: inspection_task inspection_task_community_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);


--
-- Name: inspection_task inspection_task_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.inspection_plan(id);


--
-- Name: sys_dict_data sys_dict_data_type_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_data
    ADD CONSTRAINT sys_dict_data_type_code_fkey FOREIGN KEY (type_code) REFERENCES public.sys_dict_type(code);


--
-- Name: sys_role_menu sys_role_menu_menu_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_menu_id_fkey FOREIGN KEY (menu_id) REFERENCES public.sys_menu(id) ON DELETE CASCADE;


--
-- Name: sys_role_menu sys_role_menu_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.sys_role(id) ON DELETE CASCADE;


--
-- Name: tenant_config tenant_config_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_config
    ADD CONSTRAINT tenant_config_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenant(id) ON DELETE CASCADE;


--
-- Name: user_push_device user_push_device_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_push_device
    ADD CONSTRAINT user_push_device_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.sys_user(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

-- +goose Down
-- 基线回滚 = 清空整个 public schema（仅用于开发重置，生产请勿执行 goose down）
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
