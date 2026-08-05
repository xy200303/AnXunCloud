package database

// initSchemaSQL 与《数据库设计文档》第三章 DDL 一致（含全部 17 张表与索引、分区表默认分区）。
const initSchemaSQL = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 主键策略（迁移说明）：全系统主键/外键为 uuid 类型，应用层生成 UUIDv7（google/uuid NewV7），
-- 不依赖数据库默认值，为多租户与分布式/客户端生成（离线补传幂等）预留。
-- PG 15 无 uuidv7() 函数，统一走应用层。
CREATE SEQUENCE IF NOT EXISTS qrcode_no_seq START 1;   -- 点位二维码编号 P-+6位序列 的序号源

-- ========== 系统管理域 ==========

CREATE TABLE sys_user (
    id              uuid PRIMARY KEY,
    username        varchar(64)  NOT NULL,
    password        varchar(128) NOT NULL,
    name            varchar(64)  NOT NULL,
    phone           varchar(20)  NOT NULL DEFAULT '',
    openid          varchar(64)  NULL,
    avatar          varchar(512) NOT NULL DEFAULT '',
    role_ids        jsonb        NOT NULL DEFAULT '[]'::jsonb,
    community_ids   jsonb        NOT NULL DEFAULT '[]'::jsonb,
    user_type       varchar(16)  NOT NULL DEFAULT 'admin',
    status          varchar(16)  NOT NULL DEFAULT 'enabled',
    last_login_at   timestamptz  NULL,
    remark          varchar(255) NOT NULL DEFAULT '',
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),
    deleted_at      timestamptz  NULL,
    CONSTRAINT chk_sys_user_type   CHECK (user_type IN ('admin','inspector','repair')),
    CONSTRAINT chk_sys_user_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE  sys_user               IS '系统用户（后台管理员/巡检员/维修工）';
COMMENT ON COLUMN sys_user.username      IS '登录账号，全局唯一（软删部分唯一索引）';
COMMENT ON COLUMN sys_user.password      IS 'bcrypt 密码哈希';
COMMENT ON COLUMN sys_user.openid        IS '微信小程序 openid，小程序登录凭据';
COMMENT ON COLUMN sys_user.role_ids      IS 'JSONB 角色 ID 数组，应用层关联 sys_role';
COMMENT ON COLUMN sys_user.community_ids IS 'JSONB 小区 ID 数组，数据权限（按小区隔离）';
COMMENT ON COLUMN sys_user.user_type     IS '用户类型：admin/inspector/repair';
COMMENT ON COLUMN sys_user.status        IS 'enabled 启用 / disabled 停用';

CREATE UNIQUE INDEX uk_sys_user_username ON sys_user (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_sys_user_openid   ON sys_user (openid)   WHERE openid IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_sys_user_type_status    ON sys_user (user_type, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_sys_user_phone          ON sys_user (phone) WHERE deleted_at IS NULL;

CREATE TABLE sys_role (
    id          uuid PRIMARY KEY,
    code        varchar(64)  NOT NULL,
    name        varchar(64)  NOT NULL,
    data_scope  varchar(16)  NOT NULL DEFAULT 'custom',
    remark      varchar(255) NOT NULL DEFAULT '',
    status      varchar(16)  NOT NULL DEFAULT 'enabled',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL,
    CONSTRAINT chk_sys_role_scope  CHECK (data_scope IN ('all','custom')),
    CONSTRAINT chk_sys_role_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE  sys_role            IS '角色（RBAC）';
COMMENT ON COLUMN sys_role.code       IS '角色编码，程序内引用（如超管 super_admin），唯一';
COMMENT ON COLUMN sys_role.data_scope IS '数据范围：all 全部数据 / custom 按用户 community_ids 过滤';

CREATE UNIQUE INDEX uk_sys_role_code ON sys_role (code) WHERE deleted_at IS NULL;

CREATE TABLE sys_menu (
    id          uuid PRIMARY KEY,
    parent_id   uuid        NULL,                           -- 父菜单 ID，NULL 为根（不自建外键）
    title       varchar(64)  NOT NULL,
    path        varchar(255) NOT NULL DEFAULT '',
    icon        varchar(64)  NOT NULL DEFAULT '',
    type        varchar(8)   NOT NULL,
    perms       varchar(128) NOT NULL DEFAULT '',
    sort        int          NOT NULL DEFAULT 0,
    visible     boolean      NOT NULL DEFAULT true,
    status      varchar(16)  NOT NULL DEFAULT 'enabled',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL,
    CONSTRAINT chk_sys_menu_type   CHECK (type IN ('dir','menu','button')),
    CONSTRAINT chk_sys_menu_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE  sys_menu         IS '菜单与按钮权限点（树形）';
COMMENT ON COLUMN sys_menu.perms   IS '权限标识，后端 RBAC 中间件按此鉴权';
COMMENT ON COLUMN sys_menu.type    IS 'dir 目录 / menu 菜单 / button 按钮权限点';

CREATE INDEX idx_sys_menu_parent_sort ON sys_menu (parent_id, sort) WHERE deleted_at IS NULL;
CREATE INDEX idx_sys_menu_perms       ON sys_menu (perms)           WHERE deleted_at IS NULL AND perms <> '';

CREATE TABLE sys_role_menu (
    role_id uuid NOT NULL REFERENCES sys_role (id) ON DELETE CASCADE,
    menu_id uuid NOT NULL REFERENCES sys_menu (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, menu_id)
);
COMMENT ON TABLE sys_role_menu IS '角色-菜单多对多关联';

CREATE INDEX idx_sys_role_menu_menu ON sys_role_menu (menu_id);

CREATE TABLE sys_dict_type (
    id          uuid PRIMARY KEY,
    code        varchar(64)  NOT NULL,
    name        varchar(64)  NOT NULL,
    remark      varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL
);
COMMENT ON TABLE sys_dict_type IS '字典类型';

CREATE UNIQUE INDEX uk_sys_dict_type_code ON sys_dict_type (code);

CREATE TABLE sys_dict_data (
    id          uuid PRIMARY KEY,
    type_code   varchar(64)  NOT NULL REFERENCES sys_dict_type (code),
    label       varchar(64)  NOT NULL,
    value       varchar(64)  NOT NULL,
    sort        int          NOT NULL DEFAULT 0,
    status      varchar(16)  NOT NULL DEFAULT 'enabled',
    remark      varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL,
    CONSTRAINT chk_sys_dict_data_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE sys_dict_data IS '字典数据（可配置枚举项）';

CREATE UNIQUE INDEX uk_sys_dict_data_type_value ON sys_dict_data (type_code, value) WHERE deleted_at IS NULL;
CREATE INDEX idx_sys_dict_data_type ON sys_dict_data (type_code, status, sort) WHERE deleted_at IS NULL;

CREATE TABLE sys_config (
    id          uuid PRIMARY KEY,
    key         varchar(64)  NOT NULL,
    name        varchar(64)  NOT NULL,
    value       text         NOT NULL DEFAULT '',
    remark      varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL
);
COMMENT ON TABLE sys_config IS '系统参数配置（围栏半径、水印开关、作弊阈值等）';

CREATE UNIQUE INDEX uk_sys_config_key ON sys_config (key) WHERE deleted_at IS NULL;

CREATE TABLE sys_login_log (
    id          uuid PRIMARY KEY,
    user_id     uuid       NULL,
    username    varchar(64)  NOT NULL DEFAULT '',
    channel     varchar(16)  NOT NULL DEFAULT 'admin',
    ip          varchar(64)  NOT NULL DEFAULT '',
    ua          varchar(512) NOT NULL DEFAULT '',
    status      varchar(16)  NOT NULL,
    msg         varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_sys_login_log_channel CHECK (channel IN ('admin','mp')),
    CONSTRAINT chk_sys_login_log_status  CHECK (status IN ('success','fail'))
);
COMMENT ON TABLE sys_login_log IS '登录日志（只增不改，无 updated_at/deleted_at）';

CREATE INDEX idx_sys_login_log_user_time ON sys_login_log (user_id, created_at DESC);
CREATE INDEX idx_sys_login_log_time      ON sys_login_log (created_at DESC);

CREATE TABLE sys_operation_log (
    id          uuid,
    user_id     uuid       NULL,
    username    varchar(64)  NOT NULL DEFAULT '',
    module      varchar(64)  NOT NULL DEFAULT '',
    action      varchar(64)  NOT NULL DEFAULT '',
    method      varchar(8)   NOT NULL DEFAULT '',
    path        varchar(255) NOT NULL DEFAULT '',
    params      text         NOT NULL DEFAULT '',
    ip          varchar(64)  NOT NULL DEFAULT '',
    status      varchar(16)  NOT NULL DEFAULT 'success',
    cost_ms     int          NOT NULL DEFAULT 0,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at),
    CONSTRAINT chk_sys_op_log_status CHECK (status IN ('success','fail'))
) PARTITION BY RANGE (created_at);
COMMENT ON TABLE sys_operation_log IS '操作日志（按月分区，只增不改）';

CREATE INDEX idx_sys_op_log_user_time   ON sys_operation_log (user_id, created_at DESC);
CREATE INDEX idx_sys_op_log_module_time ON sys_operation_log (module, created_at DESC);

CREATE TABLE sys_operation_log_default PARTITION OF sys_operation_log DEFAULT;

-- ========== 巡检业务域 ==========

CREATE TABLE community (
    id          uuid PRIMARY KEY,
    name        varchar(128) NOT NULL,
    address     varchar(255) NOT NULL DEFAULT '',
    manager_id  uuid       NULL,
    status      varchar(16)  NOT NULL DEFAULT 'enabled',
    remark      varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL,
    CONSTRAINT chk_community_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE community IS '小区/项目';

CREATE INDEX idx_community_status  ON community (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_community_manager ON community (manager_id) WHERE deleted_at IS NULL;

CREATE TABLE building (
    id           uuid PRIMARY KEY,
    community_id uuid       NOT NULL REFERENCES community (id),
    name         varchar(128) NOT NULL,
    type         varchar(16)  NOT NULL DEFAULT 'building',
    sort         int          NOT NULL DEFAULT 0,
    status       varchar(16)  NOT NULL DEFAULT 'enabled',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    deleted_at   timestamptz  NULL,
    CONSTRAINT chk_building_type   CHECK (type IN ('building','area')),
    CONSTRAINT chk_building_status CHECK (status IN ('enabled','disabled'))
);
COMMENT ON TABLE building IS '楼栋/区域（点位挂载单元）';

CREATE UNIQUE INDEX uk_building_comm_name ON building (community_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_building_community       ON building (community_id, sort)  WHERE deleted_at IS NULL;

CREATE TABLE inspection_point (
    id                   uuid PRIMARY KEY,
    community_id         uuid       NOT NULL REFERENCES community (id),
    building_id          uuid       NULL REFERENCES building (id),
    name                 varchar(128) NOT NULL,
    type                 varchar(32)  NOT NULL DEFAULT 'common',
    qrcode_no            varchar(64)  NOT NULL,
    longitude            numeric(10,7) NOT NULL,
    latitude             numeric(10,7) NOT NULL,
    fence_radius         int          NOT NULL DEFAULT 100,
    checkin_mode         varchar(16)  NOT NULL DEFAULT 'either',
    required_photo_items jsonb        NOT NULL DEFAULT '[]'::jsonb,
    sort                 int          NOT NULL DEFAULT 0,
    status               varchar(16)  NOT NULL DEFAULT 'enabled',
    remark               varchar(255) NOT NULL DEFAULT '',
    created_at           timestamptz  NOT NULL DEFAULT now(),
    updated_at           timestamptz  NOT NULL DEFAULT now(),
    deleted_at           timestamptz  NULL,
    CONSTRAINT chk_point_mode   CHECK (checkin_mode IN ('qrcode','fence','either','both')),
    CONSTRAINT chk_point_status CHECK (status IN ('enabled','disabled')),
    CONSTRAINT chk_point_radius CHECK (fence_radius BETWEEN 10 AND 2000),
    CONSTRAINT chk_point_lng    CHECK (longitude BETWEEN -180 AND 180),
    CONSTRAINT chk_point_lat    CHECK (latitude  BETWEEN -90  AND 90)
);
COMMENT ON TABLE  inspection_point                IS '巡检点位（二维码 + GPS 围栏双支持）';
COMMENT ON COLUMN inspection_point.qrcode_no      IS '二维码编号，全局唯一；码值编码规则见 §5.4';
COMMENT ON COLUMN inspection_point.longitude      IS 'GCJ-02 经度（与腾讯/微信小程序坐标一致，见 §5.2）';
COMMENT ON COLUMN inspection_point.checkin_mode   IS '打卡方式：qrcode/fence/either(默认)/both';
COMMENT ON COLUMN inspection_point.required_photo_items IS '必拍项 JSONB 字符串数组，少拍不能提交';

CREATE UNIQUE INDEX uk_point_qrcode ON inspection_point (qrcode_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_point_community   ON inspection_point (community_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_point_building    ON inspection_point (building_id)          WHERE deleted_at IS NULL;

CREATE TABLE inspection_plan (
    id            uuid PRIMARY KEY,
    community_id  uuid       NOT NULL REFERENCES community (id),
    name          varchar(128) NOT NULL,
    point_ids     jsonb        NOT NULL DEFAULT '[]'::jsonb,
    cycle_type    varchar(16)  NOT NULL,
    cycle_config  jsonb        NOT NULL DEFAULT '{}'::jsonb,
    inspector_ids jsonb        NOT NULL DEFAULT '[]'::jsonb,
    start_date    date         NOT NULL,
    end_date      date         NULL,
    time_window   varchar(32)  NOT NULL DEFAULT '08:00-18:00',
    status        varchar(16)  NOT NULL DEFAULT 'enabled',
    remark        varchar(255) NOT NULL DEFAULT '',
    created_at    timestamptz  NOT NULL DEFAULT now(),
    updated_at    timestamptz  NOT NULL DEFAULT now(),
    deleted_at    timestamptz  NULL,
    CONSTRAINT chk_plan_cycle  CHECK (cycle_type IN ('daily','weekly','monthly')),
    CONSTRAINT chk_plan_status CHECK (status IN ('enabled','disabled')),
    CONSTRAINT chk_plan_dates  CHECK (end_date IS NULL OR end_date >= start_date)
);
COMMENT ON TABLE  inspection_plan              IS '巡检计划（周期生成任务的模板）';
COMMENT ON COLUMN inspection_plan.point_ids    IS '有序点位 ID 数组，任务监控按此顺序展示路线';
COMMENT ON COLUMN inspection_plan.cycle_config IS '周期细则 JSONB，结构随 cycle_type 变化，见 §5.1';
COMMENT ON COLUMN inspection_plan.time_window  IS '要求执行时段，格式 HH:MM-HH:MM';

CREATE INDEX idx_plan_community_status ON inspection_plan (community_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_plan_point_ids_gin    ON inspection_plan USING gin (point_ids jsonb_path_ops);

CREATE TABLE inspection_task (
    id            uuid PRIMARY KEY,
    plan_id       uuid       NOT NULL REFERENCES inspection_plan (id),
    community_id  uuid       NOT NULL REFERENCES community (id),
    inspector_id  uuid       NOT NULL,
    task_date     date         NOT NULL,
    status        varchar(16)  NOT NULL DEFAULT 'pending',
    total_points  int          NOT NULL DEFAULT 0,
    done_points   int          NOT NULL DEFAULT 0,
    started_at    timestamptz  NULL,
    finished_at   timestamptz  NULL,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    updated_at    timestamptz  NOT NULL DEFAULT now(),
    deleted_at    timestamptz  NULL,
    CONSTRAINT chk_task_status CHECK (status IN ('pending','doing','done','overdue')),
    CONSTRAINT chk_task_points CHECK (done_points >= 0 AND total_points >= 0 AND done_points <= total_points)
);
COMMENT ON TABLE  inspection_task            IS '巡检任务（计划按日/人实例化）';
COMMENT ON COLUMN inspection_task.task_date  IS '任务归属日期；与 plan_id+inspector_id 联合唯一，保证计划生成幂等';
COMMENT ON COLUMN inspection_task.done_points IS '进度计数，打卡成功事务内 +1，与 checkin_record 最终一致';

CREATE UNIQUE INDEX uk_task_plan_date_inspector ON inspection_task (plan_id, task_date, inspector_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_task_date_status   ON inspection_task (task_date, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_task_inspector_date ON inspection_task (inspector_id, task_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_task_community_date ON inspection_task (community_id, task_date) WHERE deleted_at IS NULL;

-- ========== 打卡与工单域 ==========

CREATE TABLE checkin_record (
    id                uuid,
    task_id           uuid       NOT NULL REFERENCES inspection_task (id),
    point_id          uuid       NOT NULL REFERENCES inspection_point (id),
    inspector_id      uuid       NOT NULL,
    community_id      uuid       NOT NULL,
    checkin_time      timestamptz  NOT NULL DEFAULT now(),
    client_time       timestamptz  NULL,
    longitude         numeric(10,7) NULL,
    latitude          numeric(10,7) NULL,
    distance_to_point numeric(10,2) NULL,
    checkin_type      varchar(16)  NOT NULL,
    photos            jsonb        NOT NULL DEFAULT '[]'::jsonb,
    result            varchar(16)  NOT NULL DEFAULT 'normal',
    remark            varchar(512) NOT NULL DEFAULT '',
    is_offline_sync   boolean      NOT NULL DEFAULT false,
    is_suspect        boolean      NOT NULL DEFAULT false,
    suspect_reason    varchar(255) NOT NULL DEFAULT '',
    created_at        timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at),
    CONSTRAINT chk_checkin_type   CHECK (checkin_type IN ('qrcode','fence','offline')),
    CONSTRAINT chk_checkin_result CHECK (result IN ('normal','abnormal'))
) PARTITION BY RANGE (created_at);
COMMENT ON TABLE  checkin_record                  IS '打卡记录（按月分区，只增不改；不允许删除，防篡改审计要求）';
COMMENT ON COLUMN checkin_record.checkin_time     IS '服务端时间，准点率/及时率统计以此为准';
COMMENT ON COLUMN checkin_record.client_time      IS '客户端上报的原始时间，离线补传场景用于复核展示';
COMMENT ON COLUMN checkin_record.distance_to_point IS '距点位距离（米），超 fence_radius 即标疑似作弊';
COMMENT ON COLUMN checkin_record.photos           IS '带水印照片数组 [{item,url,watermarked_url,exif_time}]';

CREATE UNIQUE INDEX uk_checkin_task_point ON checkin_record (task_id, point_id, created_at);
CREATE INDEX idx_checkin_task          ON checkin_record (task_id);
CREATE INDEX idx_checkin_inspector_time ON checkin_record (inspector_id, checkin_time DESC);
CREATE INDEX idx_checkin_point_time    ON checkin_record (point_id, checkin_time DESC);
CREATE INDEX idx_checkin_suspect       ON checkin_record (is_suspect, checkin_time DESC) WHERE is_suspect;

CREATE TABLE checkin_record_default PARTITION OF checkin_record DEFAULT;

CREATE TABLE work_order (
    id           uuid PRIMARY KEY,
    order_no     varchar(32)  NOT NULL,
    checkin_id   uuid       NULL,
    community_id uuid       NOT NULL REFERENCES community (id),
    point_id     uuid       NULL REFERENCES inspection_point (id),
    title        varchar(128) NOT NULL,
    description  text         NOT NULL DEFAULT '',
    photos       jsonb        NOT NULL DEFAULT '[]'::jsonb,
    reporter_id  uuid       NOT NULL,
    assignee_id  uuid       NULL,
    priority     varchar(16)  NOT NULL DEFAULT 'normal',
    status       varchar(16)  NOT NULL DEFAULT 'pending',
    fix_photos   jsonb        NOT NULL DEFAULT '[]'::jsonb,
    fix_remark   varchar(512) NOT NULL DEFAULT '',
    finished_at  timestamptz  NULL,
    reviewed_by  uuid       NULL,
    review_remark varchar(512) NOT NULL DEFAULT '',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    deleted_at   timestamptz  NULL,
    CONSTRAINT chk_order_priority CHECK (priority IN ('low','normal','high','urgent')),
    CONSTRAINT chk_order_status   CHECK (status IN ('pending','assigned','processing','review','closed','rejected'))
);
COMMENT ON TABLE  work_order          IS '异常工单（上报→派单→处理→复核→关闭闭环）';
COMMENT ON COLUMN work_order.order_no IS '业务单号，全局唯一，规则 WX+日期+序号，见 §5.4';
COMMENT ON COLUMN work_order.status   IS 'pending 待派单/assigned 已派单/processing 处理中/review 待复核/closed 已关闭/rejected 已驳回';

CREATE UNIQUE INDEX uk_order_no        ON work_order (order_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_order_status          ON work_order (status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_order_community_status ON work_order (community_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_order_assignee        ON work_order (assignee_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_order_reporter        ON work_order (reporter_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_order_checkin         ON work_order (checkin_id) WHERE deleted_at IS NULL;

CREATE TABLE work_order_log (
    id          uuid PRIMARY KEY,
    order_id    uuid       NOT NULL REFERENCES work_order (id) ON DELETE CASCADE,
    action      varchar(32)  NOT NULL,
    operator_id uuid       NOT NULL,
    detail      varchar(512) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_order_log_action CHECK (action IN ('create','assign','accept','finish','review_pass','review_reject','close'))
);
COMMENT ON TABLE work_order_log IS '工单流转留痕（只增不改）';

CREATE INDEX idx_order_log_order ON work_order_log (order_id, created_at);
`
