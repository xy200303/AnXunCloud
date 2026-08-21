-- uniPush 2.0（个推 V2）App 推送设备绑定表（全部幂等）：
-- 用户登录 App 后把个推 SDK 返回的 cid 绑定到本人；一个 cid 全局唯一，重复绑定即改绑到最后登录的人。
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS public.user_push_device (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    cid character varying(128) NOT NULL,
    platform character varying(16) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_push_device_pkey PRIMARY KEY (id),
    CONSTRAINT user_push_device_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.sys_user (id) ON DELETE CASCADE,
    CONSTRAINT uk_push_device_cid UNIQUE (cid)
);

COMMENT ON TABLE public.user_push_device IS 'App 推送设备绑定（uniPush 2.0 / 个推 V2 cid → 用户；一个 cid 只属于最后登录的人）';
COMMENT ON COLUMN public.user_push_device.cid IS '个推 SDK 客户端标识（CID），全局唯一，换账号登录即改绑';
COMMENT ON COLUMN public.user_push_device.platform IS '设备平台：android / ios（离线推送厂商通道配置参考）';

CREATE INDEX IF NOT EXISTS idx_push_device_user ON public.user_push_device USING btree (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS public.user_push_device;
-- +goose StatementEnd
