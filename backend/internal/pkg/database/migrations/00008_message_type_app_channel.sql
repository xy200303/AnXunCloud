-- sys_message.type 增加 'announcement'（公告发布全员广播站内消息）；
-- sys_login_log.channel 增加 'app'（/api/app 登录日志此前一直被 CHECK 拦掉）。

-- +goose Up
ALTER TABLE sys_message DROP CONSTRAINT chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type
    CHECK (((type)::text = ANY ((ARRAY['task'::character varying, 'workorder'::character varying, 'export'::character varying, 'system'::character varying, 'checkin_audit'::character varying, 'report'::character varying, 'announcement'::character varying])::text[])));

ALTER TABLE sys_login_log DROP CONSTRAINT chk_sys_login_log_channel;
ALTER TABLE sys_login_log ADD CONSTRAINT chk_sys_login_log_channel
    CHECK (((channel)::text = ANY ((ARRAY['admin'::character varying, 'mp'::character varying, 'app'::character varying])::text[])));

-- +goose Down
ALTER TABLE sys_message DROP CONSTRAINT chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type
    CHECK (((type)::text = ANY ((ARRAY['task'::character varying, 'workorder'::character varying, 'export'::character varying, 'system'::character varying, 'checkin_audit'::character varying, 'report'::character varying])::text[])));

ALTER TABLE sys_login_log DROP CONSTRAINT chk_sys_login_log_channel;
ALTER TABLE sys_login_log ADD CONSTRAINT chk_sys_login_log_channel
    CHECK (((channel)::text = ANY ((ARRAY['admin'::character varying, 'mp'::character varying])::text[])));
