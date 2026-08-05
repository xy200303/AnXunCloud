package database

// phase2SQL 第二阶段补充表：通知公告、站内消息、上传文件记录。
// 说明：数据库设计文档 v1.0 的 17 张表未覆盖这三类数据，此处按接口文档 §2.8/§3.6/§1.7 补充。
const phase2SQL = `
-- 通知公告（接口文档 §2.8；status 为 int：0 草稿 / 1 已发布 / 2 已下线）
CREATE TABLE sys_notice (
    id          uuid PRIMARY KEY,
    title       varchar(64)  NOT NULL,
    content     text         NOT NULL DEFAULT '',
    status      int          NOT NULL DEFAULT 0,
    publish_at  timestamptz  NULL,
    created_by  uuid       NULL,
    created_by_name varchar(64) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL,
    CONSTRAINT chk_sys_notice_status CHECK (status IN (0,1,2))
);
COMMENT ON TABLE sys_notice IS '通知公告（面向巡检员）';
CREATE INDEX idx_sys_notice_status ON sys_notice (status, publish_at DESC) WHERE deleted_at IS NULL;

-- 站内消息（接口文档 §3.6；微信订阅消息推送前的落地记录）
CREATE TABLE sys_message (
    id          uuid PRIMARY KEY,
    user_id     uuid       NOT NULL,
    type        varchar(16)  NOT NULL,
    title       varchar(128) NOT NULL,
    content     varchar(512) NOT NULL DEFAULT '',
    biz_id      uuid       NULL,
    is_read     boolean      NOT NULL DEFAULT false,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_sys_message_type CHECK (type IN ('task','workorder','export','system'))
);
COMMENT ON TABLE sys_message IS '站内消息（任务提醒/工单指派/整改驳回/导出完成）';
CREATE INDEX idx_sys_message_user ON sys_message (user_id, is_read, created_at DESC);

-- 上传文件记录（OSS 回调或 dev 本地上传后写入；业务接口按 file_key 校验 43106）
CREATE TABLE upload_file (
    id              uuid PRIMARY KEY,
    file_key        varchar(255) NOT NULL,
    scene           varchar(16)  NOT NULL,
    user_id         uuid       NOT NULL,
    size            bigint       NOT NULL DEFAULT 0,
    mime_type       varchar(64)  NOT NULL DEFAULT '',
    url             varchar(512) NOT NULL DEFAULT '',
    watermarked_url varchar(512) NOT NULL DEFAULT '',
    exif_time       timestamptz  NULL,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_upload_file_scene CHECK (scene IN ('checkin','workorder','avatar','export'))
);
COMMENT ON TABLE upload_file IS '上传文件记录（打卡/工单/头像/导出）';
CREATE UNIQUE INDEX uk_upload_file_key ON upload_file (file_key);
CREATE INDEX idx_upload_file_user ON upload_file (user_id, scene);

-- 第二阶段新增系统参数
-- 数据迁移场景内联生成 UUID（非列默认值；应用层 UUIDv7 之外的存量数据迁移例外）
INSERT INTO sys_config (id, key, name, value, remark)
SELECT gen_random_uuid(), 'inspection.time_deviation_seconds', '客户端时间偏差阈值(秒)', '300', 'client_time 与服务端时间允许偏差，超出标记疑似作弊'
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'inspection.time_deviation_seconds');
`
