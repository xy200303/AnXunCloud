-- 地图服务配置种子：腾讯地图 Key（用于 PC 后台点位地图选点）。
-- 全新部署 Seed 与本迁移幂等共存；已有部署由本迁移补插。

-- +goose Up
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'map.tencent_key', '腾讯地图Key', '', 'map',
    '申请地址 https://lbs.qq.com/ ；key 需在该平台绑定调用域名白名单；用于 PC 后台点位地图选点',
    now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'map.tencent_key' AND deleted_at IS NULL);

-- +goose Down
DELETE FROM sys_config WHERE key = 'map.tencent_key' AND value = '' AND config_group = 'map';
