-- 补齐腾讯地图配置项（老库升级用，幂等且不覆盖已配置密钥）
-- +goose Up
-- +goose StatementBegin

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'map.tencent_key', '腾讯地图前端 Key', '', 'map',
       '用于管理后台点位地图选点，需在腾讯位置服务配置 JSAPI 域名白名单', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'map.tencent_key' AND deleted_at IS NULL
);

INSERT INTO public.sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'map.tencent_ws_key', '腾讯地图 WebService Key', '', 'map',
       '用于后端代理地点搜索；留空时回退 map.tencent_key', now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_config WHERE key = 'map.tencent_ws_key' AND deleted_at IS NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 数据补丁不回滚，避免误删已经填写的生产地图密钥。
-- +goose StatementEnd
