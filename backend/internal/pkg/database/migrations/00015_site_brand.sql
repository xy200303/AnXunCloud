-- 品牌官网：下载渠道（安装包/小程序码）+ 官网页面配置 + 「品牌官网」菜单及权限点。
-- 全新部署由 Seed 写入等价数据，本迁移 NOT EXISTS 自然跳过；存量库由本迁移补齐。

-- +goose Up
-- 下载渠道表：每行一个发布物（Android APK / 鸿蒙 HAP / iOS IPA / 微信小程序码），同平台取最新一条对外展示
CREATE TABLE IF NOT EXISTS app_release (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    platform    varchar(16) NOT NULL,               -- android / harmony / ios / wechat_mp
    version     varchar(64) NOT NULL DEFAULT '',    -- 安装包版本号（小程序码可空）
    file_key    varchar(255) NOT NULL,              -- 统一文件层 file_key（scene=app）
    name        varchar(255) NOT NULL DEFAULT '',   -- 原始文件名
    size        bigint NOT NULL DEFAULT 0,
    note        varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_app_release_platform ON app_release (platform, created_at DESC);

-- upload_file.scene 增加 'app'（官网发布物：安装包/小程序码）
ALTER TABLE upload_file DROP CONSTRAINT chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'workorder'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying, 'notice'::character varying, 'app'::character varying])::text[])));

-- 官网页面配置（group=site）
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), v.key, v.name, v.value, 'site', v.remark, now(), now()
FROM (VALUES
    ('site.slogan',        '官网标语',   '二维码 / NFC / GPS 围栏三重到点校验，拍照留证、工单闭环、月度报告电子签，巡检情况后台一目了然。', '官网首页主标题下的一句话介绍'),
    ('site.contact_phone', '联系电话',   '',                                        '官网页脚展示，留空不显示'),
    ('site.contact_email', '联系邮箱',   '',                                        '官网页脚展示，留空不显示'),
    ('site.theme_color',   '官网主题色', '#2b5aed',                                 '官网按钮/强调色，十六进制色值'),
    ('site.footer_note',   '页脚文案', '安巡云 AnXunCloud · 让每一次巡检都有据可查', '官网页脚底部一行文案'),
    ('site.show_admin_entry', '显示管理后台入口', 'false',                           'true 时官网导航/页脚显示「管理后台」链接，默认隐藏不暴露后台地址')
) AS v(key, name, value, remark)
WHERE NOT EXISTS (SELECT 1 FROM sys_config x WHERE x.key = v.key);

-- 「品牌官网」菜单（挂在系统管理目录下）+ 按钮权限点
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, created_at, updated_at)
SELECT gen_random_uuid(), m.id, '品牌官网', '/system/site', 'Platform', 'menu', 'system:site:list', 9, true, 'enabled', true, now(), now()
FROM sys_menu m
WHERE m.path = '/system'
  AND NOT EXISTS (SELECT 1 FROM sys_menu x WHERE x.perms = 'system:site:list');

INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, created_at, updated_at)
SELECT gen_random_uuid(), p.id, b.title, '', '', 'button', b.perms, b.sort, false, 'enabled', true, now(), now()
FROM sys_menu p
JOIN (VALUES
    ('保存页面配置', 'system:site:update', 1),
    ('上传发布物',   'system:site:upload', 2),
    ('删除发布物',   'system:site:delete', 3)
) AS b(title, perms, sort) ON true
WHERE p.perms = 'system:site:list'
  AND NOT EXISTS (SELECT 1 FROM sys_menu x WHERE x.perms = b.perms);

-- 授权超管
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, mi.id
FROM sys_role r
JOIN sys_menu mi ON mi.perms IN ('system:site:list', 'system:site:update', 'system:site:upload', 'system:site:delete')
WHERE r.code = 'super_admin'
  AND NOT EXISTS (SELECT 1 FROM sys_role_menu x WHERE x.role_id = r.id AND x.menu_id = mi.id);

-- +goose Down
ALTER TABLE upload_file DROP CONSTRAINT chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (((scene)::text = ANY ((ARRAY['checkin'::character varying, 'workorder'::character varying, 'avatar'::character varying, 'export'::character varying, 'signature'::character varying, 'seal'::character varying, 'notice'::character varying])::text[])));

DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE perms IN ('system:site:list', 'system:site:update', 'system:site:upload', 'system:site:delete'));
DELETE FROM sys_menu WHERE perms IN ('system:site:list', 'system:site:update', 'system:site:upload', 'system:site:delete');
DELETE FROM sys_config WHERE config_group = 'site';
DROP TABLE IF EXISTS app_release;
