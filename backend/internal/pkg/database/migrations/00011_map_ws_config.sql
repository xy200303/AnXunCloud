-- 地图服务配置种子：腾讯 WebService 专用 Key（服务端地点搜索代理用）。
-- 前端地图选点用 map.tencent_key（JS API，绑域名白名单）；
-- 服务端搜索走 WebService，请求无 Referer，需专用 key（不设来源限制或绑服务器 IP）。
-- 留空时后端回退使用 map.tencent_key。

-- +goose Up
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'map.tencent_ws_key', '腾讯WebService专用Key', '', 'map',
    '服务端地点搜索（/map/search）专用；申请地址 https://lbs.qq.com/ ，勾选 WebserviceAPI 且不要设域名白名单（可绑服务器 IP）；留空则回退使用腾讯地图Key',
    now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'map.tencent_ws_key' AND deleted_at IS NULL);

-- +goose Down
DELETE FROM sys_config WHERE key = 'map.tencent_ws_key' AND value = '' AND config_group = 'map';
