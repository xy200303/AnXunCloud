-- 品牌官网配置扩充：联系方式（公司名/微信号/地址/备案号）；下线「显示管理后台入口」开关（官网不再出现后台链接）。

-- +goose Up
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), v.key, v.name, v.value, 'site', v.remark, now(), now()
FROM (VALUES
    ('site.company_name',    '公司名称',   '', '官网联系区块与结构化数据展示'),
    ('site.contact_wechat',  '微信号',     '', '官网联系区块展示，留空不显示'),
    ('site.address',         '公司地址',   '', '官网联系区块与结构化数据展示，留空不显示'),
    ('site.icp',             'ICP 备案号', '', '官网页脚展示（如 鄂ICP备2024xxxxxx号-1），留空不显示')
) AS v(key, name, value, remark)
WHERE NOT EXISTS (SELECT 1 FROM sys_config x WHERE x.key = v.key);

DELETE FROM sys_config WHERE key = 'site.show_admin_entry';

-- +goose Down
DELETE FROM sys_config WHERE key IN ('site.company_name', 'site.contact_wechat', 'site.address', 'site.icp');
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'site.show_admin_entry', '显示管理后台入口', 'false', 'site', 'true 时官网导航/页脚显示「管理后台」链接，默认隐藏不暴露后台地址', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'site.show_admin_entry');
