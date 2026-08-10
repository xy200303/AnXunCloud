-- +goose Up
-- 基线迁移：完整初始库结构。
-- 由 dev 库 pg_dump --schema-only 生成后清洗（替代旧自研迁移 v1-v20，项目未上线，历史迁移压缩为单基线）。
-- 说明：checkin_record / sys_operation_log 为按月分区表，月份分区由应用启动时 EnsurePartitions 滚动创建，
--       基线仅包含 DEFAULT 分区兜底；gen_random_uuid 依赖 pgcrypto 扩展（PG13+ 内建，保留扩展声明兼容托管 PG）。
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';
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
    CONSTRAINT chk_building_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[]))),
    CONSTRAINT chk_building_type CHECK (((type)::text = ANY ((ARRAY['building'::character varying, 'area'::character varying])::text[])))
);
COMMENT ON TABLE public.building IS '楼栋/区域（点位挂载单元）';
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
COMMENT ON TABLE public.casbin_rule IS 'Casbin 策略表（p：角色→权限点；g：用户→角色，域 default）';
CREATE SEQUENCE public.casbin_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.casbin_rule_id_seq OWNED BY public.casbin_rule.id;
CREATE TABLE public.check_template (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    point_type character varying(32) DEFAULT ''::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);
COMMENT ON TABLE public.check_template IS '检查项模板：items=[{name,required,photo_required}]，photo_required=none/optional/required（缺省 none），point_type 空为通用';
CREATE TABLE public.check_template_item (
    id uuid NOT NULL,
    template_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    requirement text,
    required boolean DEFAULT false NOT NULL,
    photo_required character varying(16) DEFAULT 'none'::character varying NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_tpl_item_photo_req CHECK (((photo_required)::text = ANY ((ARRAY['none'::character varying, 'optional'::character varying, 'required'::character varying])::text[])))
);
COMMENT ON TABLE public.check_template_item IS '模板检查项（v18 起替代 check_template.items JSONB）；requirement=检查标准要求文本（可空）';
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
    photos jsonb DEFAULT '[]'::jsonb NOT NULL,
    result character varying(16) DEFAULT 'normal'::character varying NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_offline_sync boolean DEFAULT false NOT NULL,
    is_suspect boolean DEFAULT false NOT NULL,
    suspect_reason character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    audit_status character varying(16) DEFAULT 'auto_pass'::character varying NOT NULL,
    audit_by uuid,
    audit_at timestamp with time zone,
    audit_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    ai_verdict character varying(16) DEFAULT ''::character varying NOT NULL,
    ai_reason character varying(512) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT chk_checkin_result CHECK (((result)::text = ANY ((ARRAY['normal'::character varying, 'abnormal'::character varying])::text[]))),
    CONSTRAINT chk_checkin_type CHECK (((checkin_type)::text = ANY ((ARRAY['qrcode'::character varying, 'fence'::character varying, 'offline'::character varying, 'nfc'::character varying])::text[])))
)
PARTITION BY RANGE (created_at);
COMMENT ON TABLE public.checkin_record IS '打卡记录（按月分区，只增不改；不允许删除，防篡改审计要求）';
COMMENT ON COLUMN public.checkin_record.checkin_time IS '服务端时间，准点率/及时率统计以此为准';
COMMENT ON COLUMN public.checkin_record.client_time IS '客户端上报的原始时间，离线补传场景用于复核展示';
COMMENT ON COLUMN public.checkin_record.distance_to_point IS '距点位距离（米），超 fence_radius 即标疑似作弊';
COMMENT ON COLUMN public.checkin_record.photos IS '带水印照片数组 [{item,url,watermarked_url,exif_time}]';
COMMENT ON COLUMN public.checkin_record.audit_status IS 'auto_pass 默认通过 / pending 待审核 / pass 人工通过 / rejected 已打回（不级联任务进度）';
COMMENT ON COLUMN public.checkin_record.ai_verdict IS '大模型结论：pass / review / error（空=未启用）';
CREATE TABLE public.checkin_record_default PARTITION OF public.checkin_record DEFAULT;
CREATE TABLE public.checkin_record_item (
    id uuid NOT NULL,
    record_id uuid NOT NULL,
    template_item_id uuid,
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
    CONSTRAINT chk_rec_item_photo_req CHECK (((photo_required)::text = ANY ((ARRAY['none'::character varying, 'optional'::character varying, 'required'::character varying])::text[])))
);
COMMENT ON TABLE public.checkin_record_item IS '打卡逐项结果快照（v18 起替代 checkin_record.check_items JSONB）；ai_verdict/ai_reason 为逐项大模型结论（预留）';
COMMENT ON COLUMN public.checkin_record_item.record_id IS '打卡记录 id（无 FK：checkin_record 为分区表，主键含分区键）';
COMMENT ON COLUMN public.checkin_record_item.photos IS '该项照片 file_key 数组';
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
    CONSTRAINT chk_community_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[])))
);
COMMENT ON TABLE public.community IS '小区/项目';
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
    CONSTRAINT chk_plan_cycle CHECK (((cycle_type)::text = ANY ((ARRAY['daily'::character varying, 'weekly'::character varying, 'monthly'::character varying])::text[]))),
    CONSTRAINT chk_plan_dates CHECK (((end_date IS NULL) OR (end_date >= start_date))),
    CONSTRAINT chk_plan_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[])))
);
COMMENT ON TABLE public.inspection_plan IS '巡检计划（周期生成任务的模板）';
COMMENT ON COLUMN public.inspection_plan.point_ids IS '有序点位 ID 数组，任务监控按此顺序展示路线';
COMMENT ON COLUMN public.inspection_plan.cycle_config IS '周期细则 JSONB，结构随 cycle_type 变化，见 §5.1';
COMMENT ON COLUMN public.inspection_plan.time_window IS '要求执行时段，格式 HH:MM-HH:MM';
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
    checkin_mode character varying(16) DEFAULT 'either'::character varying NOT NULL,
    required_photo_items jsonb DEFAULT '[]'::jsonb NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    template_id uuid,
    nfc_id character varying(64) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT chk_point_lat CHECK (((latitude >= ('-90'::integer)::numeric) AND (latitude <= (90)::numeric))),
    CONSTRAINT chk_point_lng CHECK (((longitude >= ('-180'::integer)::numeric) AND (longitude <= (180)::numeric))),
    CONSTRAINT chk_point_mode CHECK (((checkin_mode)::text = ANY ((ARRAY['qrcode'::character varying, 'fence'::character varying, 'either'::character varying, 'both'::character varying, 'nfc'::character varying])::text[]))),
    CONSTRAINT chk_point_radius CHECK (((fence_radius >= 10) AND (fence_radius <= 2000))),
    CONSTRAINT chk_point_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[])))
);
COMMENT ON TABLE public.inspection_point IS '巡检点位（二维码 + GPS 围栏双支持）';
COMMENT ON COLUMN public.inspection_point.qrcode_no IS '二维码编号，全局唯一；码值编码规则见 §5.4';
COMMENT ON COLUMN public.inspection_point.longitude IS 'GCJ-02 经度（与腾讯/微信小程序坐标一致，见 §5.2）';
COMMENT ON COLUMN public.inspection_point.checkin_mode IS '打卡方式：qrcode/fence/either(默认)/both';
COMMENT ON COLUMN public.inspection_point.required_photo_items IS '必拍项 JSONB 字符串数组，少拍不能提交';
COMMENT ON COLUMN public.inspection_point.nfc_id IS 'NFC 卡号（打卡方式 nfc 时校验）';
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
    seal_file_key character varying(255) DEFAULT ''::character varying NOT NULL
);
COMMENT ON TABLE public.inspection_report IS '月度巡检报告：pending_inspector→pending_supervisor→pending_manager→approved，驳回回 pending_inspector';
COMMENT ON COLUMN public.inspection_report.supervisor_signature_key IS '主管签字时的手写签名图快照 file_key';
COMMENT ON COLUMN public.inspection_report.manager_signature_key IS '经理终审时的手写签名图快照 file_key';
COMMENT ON COLUMN public.inspection_report.seal_file_key IS '终审通过时的公章资产 file_key 快照（空=终审时无公章或存量报告）';
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
    CONSTRAINT chk_task_points CHECK (((done_points >= 0) AND (total_points >= 0) AND (done_points <= total_points))),
    CONSTRAINT chk_task_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'doing'::character varying, 'done'::character varying, 'overdue'::character varying])::text[])))
);
COMMENT ON TABLE public.inspection_task IS '巡检任务（计划按日/人实例化）';
COMMENT ON COLUMN public.inspection_task.task_date IS '任务归属日期；与 plan_id+inspector_id 联合唯一，保证计划生成幂等';
COMMENT ON COLUMN public.inspection_task.done_points IS '进度计数，打卡成功事务内 +1，与 checkin_record 最终一致';
CREATE SEQUENCE public.qrcode_no_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
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
    CONSTRAINT chk_sign_asset_owner CHECK (((((asset_type)::text = 'user_signature'::text) AND (owner_id IS NOT NULL)) OR (((asset_type)::text = 'company_seal'::text) AND (owner_id IS NULL)))),
    CONSTRAINT chk_sign_asset_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'replaced'::character varying, 'revoked'::character varying])::text[]))),
    CONSTRAINT chk_sign_asset_type CHECK (((asset_type)::text = ANY ((ARRAY['user_signature'::character varying, 'company_seal'::character varying])::text[])))
);
COMMENT ON TABLE public.sign_asset IS '签章资产：user_signature(owner_id=用户)/company_seal(owner_id NULL)，同 type+owner 仅一条 active；version 同 type+owner 内递增';
COMMENT ON COLUMN public.sign_asset.sha256 IS '文件内容 SHA-256 指纹（创建时计算；文件缺失/不可读容错为空串）';
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
COMMENT ON TABLE public.sys_config IS '系统参数配置（围栏半径、水印开关、作弊阈值等）';
COMMENT ON COLUMN public.sys_config.config_group IS '参数分组（按 key 前缀归组：inspection/mp/msg/security/auth，其余 system）';
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
    CONSTRAINT chk_sys_dict_data_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_dict_data IS '字典数据（可配置枚举项）';
CREATE TABLE public.sys_dict_type (
    id uuid NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);
COMMENT ON TABLE public.sys_dict_type IS '字典类型';
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
    CONSTRAINT chk_sys_login_log_channel CHECK (((channel)::text = ANY ((ARRAY['admin'::character varying, 'mp'::character varying])::text[]))),
    CONSTRAINT chk_sys_login_log_status CHECK (((status)::text = ANY ((ARRAY['success'::character varying, 'fail'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_login_log IS '登录日志（只增不改，无 updated_at/deleted_at）';
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
    CONSTRAINT chk_sys_menu_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[]))),
    CONSTRAINT chk_sys_menu_type CHECK (((type)::text = ANY ((ARRAY['dir'::character varying, 'menu'::character varying, 'button'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_menu IS '菜单与按钮权限点（树形）';
COMMENT ON COLUMN public.sys_menu.type IS 'dir 目录 / menu 菜单 / button 按钮权限点';
COMMENT ON COLUMN public.sys_menu.perms IS '权限标识，后端 RBAC 中间件按此鉴权';
COMMENT ON COLUMN public.sys_menu.is_builtin IS '内置菜单不可删除（错误码 41007）';
CREATE TABLE public.sys_message (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    type character varying(16) NOT NULL,
    title character varying(128) NOT NULL,
    content character varying(512) DEFAULT ''::character varying NOT NULL,
    biz_id uuid,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_sys_message_type CHECK (((type)::text = ANY ((ARRAY['task'::character varying, 'workorder'::character varying, 'export'::character varying, 'system'::character varying, 'checkin_audit'::character varying, 'report'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_message IS '站内消息（任务提醒/工单指派/整改驳回/导出完成）';
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
    CONSTRAINT chk_sys_notice_status CHECK ((status = ANY (ARRAY[0, 1, 2])))
);
COMMENT ON TABLE public.sys_notice IS '通知公告（面向巡检员）';
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
    CONSTRAINT chk_sys_op_log_status CHECK (((status)::text = ANY ((ARRAY['success'::character varying, 'fail'::character varying])::text[])))
)
PARTITION BY RANGE (created_at);
COMMENT ON TABLE public.sys_operation_log IS '操作日志（按月分区，只增不改）';
CREATE TABLE public.sys_operation_log_default PARTITION OF public.sys_operation_log DEFAULT;
CREATE TABLE public.sys_role (
    id uuid NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    data_scope character varying(16) DEFAULT 'custom'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    is_builtin boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_sys_role_scope CHECK (((data_scope)::text = ANY ((ARRAY['all'::character varying, 'custom'::character varying])::text[]))),
    CONSTRAINT chk_sys_role_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_role IS '角色（RBAC）';
COMMENT ON COLUMN public.sys_role.code IS '角色编码，程序内引用（如超管 super_admin），唯一';
COMMENT ON COLUMN public.sys_role.data_scope IS '数据范围：all 全部数据 / custom 按用户 community_ids 过滤';
COMMENT ON COLUMN public.sys_role.is_builtin IS '内置角色不可删除（错误码 41007）';
CREATE TABLE public.sys_role_menu (
    role_id uuid NOT NULL,
    menu_id uuid NOT NULL
);
COMMENT ON TABLE public.sys_role_menu IS '角色-菜单多对多关联';
CREATE TABLE public.sys_user (
    id uuid NOT NULL,
    username character varying(64) NOT NULL,
    password character varying(128) NOT NULL,
    name character varying(64) NOT NULL,
    phone character varying(20) DEFAULT ''::character varying NOT NULL,
    openid character varying(64),
    avatar character varying(512) DEFAULT ''::character varying NOT NULL,
    role_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    community_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    user_type character varying(16) DEFAULT 'admin'::character varying NOT NULL,
    status character varying(16) DEFAULT 'enabled'::character varying NOT NULL,
    last_login_at timestamp with time zone,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    must_change_password boolean DEFAULT false NOT NULL,
    is_builtin boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_sys_user_status CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'disabled'::character varying])::text[]))),
    CONSTRAINT chk_sys_user_type CHECK (((user_type)::text = ANY ((ARRAY['admin'::character varying, 'inspector'::character varying, 'repair'::character varying])::text[])))
);
COMMENT ON TABLE public.sys_user IS '系统用户（后台管理员/巡检员/维修工）';
COMMENT ON COLUMN public.sys_user.username IS '登录账号，全局唯一（软删部分唯一索引）';
COMMENT ON COLUMN public.sys_user.password IS 'bcrypt 密码哈希';
COMMENT ON COLUMN public.sys_user.openid IS '微信小程序 openid，小程序登录凭据';
COMMENT ON COLUMN public.sys_user.role_ids IS 'JSONB 角色 ID 数组，应用层关联 sys_role';
COMMENT ON COLUMN public.sys_user.community_ids IS 'JSONB 小区 ID 数组，数据权限（按小区隔离）';
COMMENT ON COLUMN public.sys_user.user_type IS '用户类型：admin/inspector/repair';
COMMENT ON COLUMN public.sys_user.status IS 'enabled 启用 / disabled 停用';
COMMENT ON COLUMN public.sys_user.must_change_password IS '首次登录强制修改密码标记（批量导入用户置 true）';
COMMENT ON COLUMN public.sys_user.is_builtin IS '内置账号（admin）：禁止删除/停用/移除 super_admin 角色（错误码 41014）';
CREATE TABLE public.upload_file (
    id uuid NOT NULL,
    file_key character varying(255) NOT NULL,
    scene character varying(16) NOT NULL,
    user_id uuid NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    mime_type character varying(64) DEFAULT ''::character varying NOT NULL,
    url character varying(512) DEFAULT ''::character varying NOT NULL,
    watermarked_url character varying(512) DEFAULT ''::character varying NOT NULL,
    exif_time timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_upload_file_scene CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'workorder'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying])::text[])))
);
COMMENT ON TABLE public.upload_file IS '上传文件记录（打卡/工单/头像/导出）';
CREATE TABLE public.work_order (
    id uuid NOT NULL,
    order_no character varying(32) NOT NULL,
    checkin_id uuid,
    community_id uuid NOT NULL,
    point_id uuid,
    title character varying(128) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    photos jsonb DEFAULT '[]'::jsonb NOT NULL,
    reporter_id uuid NOT NULL,
    assignee_id uuid,
    priority character varying(16) DEFAULT 'normal'::character varying NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    fix_photos jsonb DEFAULT '[]'::jsonb NOT NULL,
    fix_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    finished_at timestamp with time zone,
    reviewed_by uuid,
    review_remark character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    items jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT chk_order_priority CHECK (((priority)::text = ANY ((ARRAY['low'::character varying, 'normal'::character varying, 'high'::character varying, 'urgent'::character varying])::text[]))),
    CONSTRAINT chk_order_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'assigned'::character varying, 'processing'::character varying, 'review'::character varying, 'closed'::character varying, 'rejected'::character varying])::text[])))
);
COMMENT ON TABLE public.work_order IS '异常工单（上报→派单→处理→复核→关闭闭环）';
COMMENT ON COLUMN public.work_order.order_no IS '业务单号，全局唯一，规则 WX+日期+序号，见 §5.4';
COMMENT ON COLUMN public.work_order.status IS 'pending 待派单/assigned 已派单/processing 处理中/review 待复核/closed 已关闭/rejected 已驳回';
COMMENT ON COLUMN public.work_order.items IS '不合格项快照：[{name,remark,before_photos[],after_photos[]}]（photos 存 file_key；before=整改前/打卡时，after=整改后/回传）';
CREATE TABLE public.work_order_log (
    id uuid NOT NULL,
    order_id uuid NOT NULL,
    action character varying(32) NOT NULL,
    operator_id uuid NOT NULL,
    detail character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_order_log_action CHECK (((action)::text = ANY ((ARRAY['create'::character varying, 'assign'::character varying, 'accept'::character varying, 'finish'::character varying, 'review_pass'::character varying, 'review_reject'::character varying, 'close'::character varying])::text[])))
);
COMMENT ON TABLE public.work_order_log IS '工单流转留痕（只增不改）';
ALTER TABLE ONLY public.casbin_rule ALTER COLUMN id SET DEFAULT nextval('public.casbin_rule_id_seq'::regclass);
ALTER TABLE ONLY public.building
    ADD CONSTRAINT building_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.casbin_rule
    ADD CONSTRAINT casbin_rule_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.check_template_item
    ADD CONSTRAINT check_template_item_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.check_template
    ADD CONSTRAINT check_template_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.checkin_record
    ADD CONSTRAINT checkin_record_pkey PRIMARY KEY (id, created_at);
ALTER TABLE ONLY public.checkin_record_item
    ADD CONSTRAINT checkin_record_item_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.community
    ADD CONSTRAINT community_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.inspection_plan
    ADD CONSTRAINT inspection_plan_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.inspection_report
    ADD CONSTRAINT inspection_report_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sign_asset
    ADD CONSTRAINT sign_asset_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_config
    ADD CONSTRAINT sys_config_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_dict_data
    ADD CONSTRAINT sys_dict_data_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_dict_type
    ADD CONSTRAINT sys_dict_type_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_login_log
    ADD CONSTRAINT sys_login_log_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_menu
    ADD CONSTRAINT sys_menu_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_message
    ADD CONSTRAINT sys_message_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_notice
    ADD CONSTRAINT sys_notice_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_operation_log
    ADD CONSTRAINT sys_operation_log_pkey PRIMARY KEY (id, created_at);
ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_pkey PRIMARY KEY (role_id, menu_id);
ALTER TABLE ONLY public.sys_role
    ADD CONSTRAINT sys_role_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sys_user
    ADD CONSTRAINT sys_user_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.upload_file
    ADD CONSTRAINT upload_file_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.work_order_log
    ADD CONSTRAINT work_order_log_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.work_order
    ADD CONSTRAINT work_order_pkey PRIMARY KEY (id);
CREATE INDEX idx_checkin_audit ON ONLY public.checkin_record USING btree (audit_status, created_at DESC);
CREATE INDEX idx_checkin_inspector_time ON ONLY public.checkin_record USING btree (inspector_id, checkin_time DESC);
CREATE INDEX idx_checkin_suspect ON ONLY public.checkin_record USING btree (is_suspect, checkin_time DESC) WHERE is_suspect;
CREATE INDEX idx_checkin_point_time ON ONLY public.checkin_record USING btree (point_id, checkin_time DESC);
CREATE INDEX idx_checkin_task ON ONLY public.checkin_record USING btree (task_id);
CREATE UNIQUE INDEX uk_checkin_task_point ON ONLY public.checkin_record USING btree (task_id, point_id, created_at);
CREATE INDEX idx_building_community ON public.building USING btree (community_id, sort) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX idx_casbin_rule ON public.casbin_rule USING btree (ptype, v0, v1, v2, v3, v4, v5);
CREATE INDEX idx_community_manager ON public.community USING btree (manager_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_community_status ON public.community USING btree (status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_order_assignee ON public.work_order USING btree (assignee_id, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_order_checkin ON public.work_order USING btree (checkin_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_order_community_status ON public.work_order USING btree (community_id, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_order_log_order ON public.work_order_log USING btree (order_id, created_at);
CREATE INDEX idx_order_reporter ON public.work_order USING btree (reporter_id, created_at DESC) WHERE (deleted_at IS NULL);
CREATE INDEX idx_order_status ON public.work_order USING btree (status, created_at DESC) WHERE (deleted_at IS NULL);
CREATE INDEX idx_plan_community_status ON public.inspection_plan USING btree (community_id, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_plan_point_ids_gin ON public.inspection_plan USING gin (point_ids jsonb_path_ops);
CREATE INDEX idx_point_building ON public.inspection_point USING btree (building_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_point_community ON public.inspection_point USING btree (community_id, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_rec_item_rec ON public.checkin_record_item USING btree (record_id, sort);
CREATE INDEX idx_rec_item_tplitem ON public.checkin_record_item USING btree (template_item_id);
CREATE INDEX idx_sign_asset_owner ON public.sign_asset USING btree (asset_type, owner_id, status);
CREATE INDEX idx_sys_dict_data_type ON public.sys_dict_data USING btree (type_code, status, sort) WHERE (deleted_at IS NULL);
CREATE INDEX idx_sys_login_log_time ON public.sys_login_log USING btree (created_at DESC);
CREATE INDEX idx_sys_login_log_user_time ON public.sys_login_log USING btree (user_id, created_at DESC);
CREATE INDEX idx_sys_menu_parent_sort ON public.sys_menu USING btree (parent_id, sort) WHERE (deleted_at IS NULL);
CREATE INDEX idx_sys_menu_perms ON public.sys_menu USING btree (perms) WHERE ((deleted_at IS NULL) AND ((perms)::text <> ''::text));
CREATE INDEX idx_sys_message_user ON public.sys_message USING btree (user_id, is_read, created_at DESC);
CREATE INDEX idx_sys_notice_status ON public.sys_notice USING btree (status, publish_at DESC) WHERE (deleted_at IS NULL);
CREATE INDEX idx_sys_op_log_module_time ON ONLY public.sys_operation_log USING btree (module, created_at DESC);
CREATE INDEX idx_sys_op_log_user_time ON ONLY public.sys_operation_log USING btree (user_id, created_at DESC);
CREATE INDEX idx_sys_role_menu_menu ON public.sys_role_menu USING btree (menu_id);
CREATE INDEX idx_sys_user_phone ON public.sys_user USING btree (phone) WHERE (deleted_at IS NULL);
CREATE INDEX idx_sys_user_type_status ON public.sys_user USING btree (user_type, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_task_community_date ON public.inspection_task USING btree (community_id, task_date) WHERE (deleted_at IS NULL);
CREATE INDEX idx_task_date_status ON public.inspection_task USING btree (task_date, status) WHERE (deleted_at IS NULL);
CREATE INDEX idx_task_inspector_date ON public.inspection_task USING btree (inspector_id, task_date DESC) WHERE (deleted_at IS NULL);
CREATE INDEX idx_tpl_item_tpl ON public.check_template_item USING btree (template_id, sort);
CREATE INDEX idx_upload_file_user ON public.upload_file USING btree (user_id, scene);
CREATE UNIQUE INDEX uk_building_comm_name ON public.building USING btree (community_id, name) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_inspection_report ON public.inspection_report USING btree (community_id, period) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_order_no ON public.work_order USING btree (order_no) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_point_qrcode ON public.inspection_point USING btree (qrcode_no) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_sign_asset_active ON public.sign_asset USING btree (asset_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE ((status)::text = 'active'::text);
CREATE UNIQUE INDEX uk_sys_config_key ON public.sys_config USING btree (key) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_sys_dict_data_type_value ON public.sys_dict_data USING btree (type_code, value) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_sys_dict_type_code ON public.sys_dict_type USING btree (code);
CREATE UNIQUE INDEX uk_sys_role_code ON public.sys_role USING btree (code) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_sys_user_openid ON public.sys_user USING btree (openid) WHERE ((openid IS NOT NULL) AND (deleted_at IS NULL));
CREATE UNIQUE INDEX uk_sys_user_username ON public.sys_user USING btree (username) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_task_plan_date_inspector ON public.inspection_task USING btree (plan_id, task_date, inspector_id) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX uk_upload_file_key ON public.upload_file USING btree (file_key);
ALTER TABLE ONLY public.building
    ADD CONSTRAINT building_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);
ALTER TABLE ONLY public.check_template_item
    ADD CONSTRAINT check_template_item_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.check_template(id) ON DELETE CASCADE;
ALTER TABLE public.checkin_record
    ADD CONSTRAINT checkin_record_point_id_fkey FOREIGN KEY (point_id) REFERENCES public.inspection_point(id);
ALTER TABLE public.checkin_record
    ADD CONSTRAINT checkin_record_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.inspection_task(id);
ALTER TABLE ONLY public.inspection_plan
    ADD CONSTRAINT inspection_plan_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);
ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_building_id_fkey FOREIGN KEY (building_id) REFERENCES public.building(id);
ALTER TABLE ONLY public.inspection_point
    ADD CONSTRAINT inspection_point_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);
ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);
ALTER TABLE ONLY public.inspection_task
    ADD CONSTRAINT inspection_task_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.inspection_plan(id);
ALTER TABLE ONLY public.sys_dict_data
    ADD CONSTRAINT sys_dict_data_type_code_fkey FOREIGN KEY (type_code) REFERENCES public.sys_dict_type(code);
ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_menu_id_fkey FOREIGN KEY (menu_id) REFERENCES public.sys_menu(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.sys_role_menu
    ADD CONSTRAINT sys_role_menu_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.sys_role(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.work_order
    ADD CONSTRAINT work_order_community_id_fkey FOREIGN KEY (community_id) REFERENCES public.community(id);
ALTER TABLE ONLY public.work_order_log
    ADD CONSTRAINT work_order_log_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.work_order(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.work_order
    ADD CONSTRAINT work_order_point_id_fkey FOREIGN KEY (point_id) REFERENCES public.inspection_point(id);

-- +goose Down
-- 基线回滚 = 清空整个 public schema（仅用于开发重置，生产请勿执行 goose down）
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
