package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// migration 单条迁移：版本号 + SQL 脚本。
type migration struct {
	Version int
	Name    string
	SQL     string
}

// migrations 全量迁移脚本。v1 与《数据库设计文档》第三章 DDL 一一对应。
var migrations = []migration{
	{Version: 1, Name: "init_schema", SQL: initSchemaSQL},
	{Version: 2, Name: "sys_user_must_change_password", SQL: `
ALTER TABLE sys_user ADD COLUMN IF NOT EXISTS must_change_password boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_user.must_change_password IS '首次登录强制修改密码标记（批量导入用户置 true）';
`},
	{Version: 3, Name: "builtin_flags", SQL: `
ALTER TABLE sys_role ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_role.is_builtin IS '内置角色不可删除（错误码 41007）';
COMMENT ON COLUMN sys_menu.is_builtin IS '内置菜单不可删除（错误码 41007）';
`},
	{Version: 4, Name: "phase2_tables", SQL: phase2SQL},
	{Version: 5, Name: "casbin_rule", SQL: `
-- Casbin 策略表（RBAC with domains，见 internal/pkg/authz；gorm-adapter 也会自动建表，此处显式迁移）
CREATE TABLE IF NOT EXISTS casbin_rule (
    id    bigserial PRIMARY KEY,
    ptype varchar(100) NULL,
    v0    varchar(100) NULL,
    v1    varchar(100) NULL,
    v2    varchar(100) NULL,
    v3    varchar(100) NULL,
    v4    varchar(100) NULL,
    v5    varchar(100) NULL
);
COMMENT ON TABLE casbin_rule IS 'Casbin 策略表（p：角色→权限点；g：用户→角色，域 default）';
`},
	{Version: 6, Name: "casbin_rule_rebuild", SQL: `
-- 策略模型调整为三元组（sub, dom, obj=完整权限点字符串）：casbin_rule 为派生数据，清空后由启动 SyncAll 重建
DELETE FROM casbin_rule;
`},
	{Version: 7, Name: "config_register_enabled", SQL: `
-- 开放注册开关（存量库幂等补充；id 内联生成，见 v4 说明）
INSERT INTO sys_config (id, key, name, value, remark)
SELECT gen_random_uuid(), 'auth.register_enabled', '是否开放注册', 'false', '开启后允许通过 /api/admin/auth/register 自助注册后台账号'
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'auth.register_enabled');
`},
	{Version: 8, Name: "sys_user_is_builtin", SQL: `
-- 内置账号标记：唯一超级管理员 admin 不可删除/停用/移除超管角色
ALTER TABLE sys_user ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_user.is_builtin IS '内置账号（admin）：禁止删除/停用/移除 super_admin 角色（错误码 41014）';
-- 存量库：admin 账号或持有 super_admin 角色的账号标记为内置
UPDATE sys_user SET is_builtin = true
WHERE username = 'admin'
   OR role_ids @> ('["' || (SELECT id FROM sys_role WHERE code = 'super_admin' AND deleted_at IS NULL LIMIT 1) || '"]')::jsonb;
`},
	{Version: 9, Name: "sys_config_group", SQL: `
-- 参数配置改名"系统配置"并支持分组
ALTER TABLE sys_config ADD COLUMN IF NOT EXISTS config_group varchar(50) NOT NULL DEFAULT 'system';
COMMENT ON COLUMN sys_config.config_group IS '参数分组（按 key 前缀归组：inspection/mp/msg/security/auth，其余 system）';
-- 存量数据按 key 前缀归组（幂等，可重复执行）
UPDATE sys_config SET config_group = CASE
	WHEN key LIKE 'inspection.%' THEN 'inspection'
	WHEN key LIKE 'mp.%' THEN 'mp'
	WHEN key LIKE 'msg.%' THEN 'msg'
	WHEN key LIKE 'security.%' THEN 'security'
	WHEN key LIKE 'auth.%' THEN 'auth'
	ELSE 'system' END;
-- 菜单"参数配置"改名"系统配置"
UPDATE sys_menu SET title = '系统配置' WHERE perms = 'system:config:list' AND title = '参数配置';
`},
	{Version: 10, Name: "checklist_audit_report", SQL: `
-- ===== 检查项模板 =====
CREATE TABLE IF NOT EXISTS check_template (
    id          uuid PRIMARY KEY,
    name        varchar(128) NOT NULL,
    point_type  varchar(32)  NOT NULL DEFAULT '',
    items       jsonb        NOT NULL DEFAULT '[]',
    sort        int          NOT NULL DEFAULT 0,
    status      varchar(16)  NOT NULL DEFAULT 'enabled',
    remark      varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz  NULL
);
COMMENT ON TABLE check_template IS '检查项模板：items=[{name,required}]，point_type 空为通用';

-- ===== 点位：检查项模板 + NFC =====
ALTER TABLE inspection_point ADD COLUMN IF NOT EXISTS template_id uuid NULL;
ALTER TABLE inspection_point ADD COLUMN IF NOT EXISTS nfc_id varchar(64) NOT NULL DEFAULT '';
COMMENT ON COLUMN inspection_point.nfc_id IS 'NFC 卡号（打卡方式 nfc 时校验）';

-- ===== 打卡记录：逐项结果 + 审核状态机 + AI 结论（分区父表，加列级联子分区）=====
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS check_items jsonb NOT NULL DEFAULT '[]';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS audit_status varchar(16) NOT NULL DEFAULT 'auto_pass';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS audit_by uuid NULL;
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS audit_at timestamptz NULL;
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS audit_remark varchar(512) NOT NULL DEFAULT '';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS ai_verdict varchar(16) NOT NULL DEFAULT '';
ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS ai_reason varchar(512) NOT NULL DEFAULT '';
COMMENT ON COLUMN checkin_record.audit_status IS 'auto_pass 默认通过 / pending 待审核 / pass 人工通过 / rejected 已打回（不级联任务进度）';
COMMENT ON COLUMN checkin_record.ai_verdict IS '大模型结论：pass / review / error（空=未启用）';
CREATE INDEX IF NOT EXISTS idx_checkin_audit ON checkin_record (audit_status, created_at DESC);

-- ===== 月度报告（三级签字 + PDF 归档）=====
CREATE TABLE IF NOT EXISTS inspection_report (
    id                uuid PRIMARY KEY,
    community_id      uuid         NOT NULL,
    period            char(7)      NOT NULL,
    title             varchar(128) NOT NULL,
    status            varchar(24)  NOT NULL DEFAULT 'pending_inspector',
    stats             jsonb        NOT NULL DEFAULT '{}',
    inspector_ids     jsonb        NOT NULL DEFAULT '[]',
    inspector_signed  jsonb        NOT NULL DEFAULT '[]',
    supervisor_by     uuid         NULL,
    supervisor_at     timestamptz  NULL,
    supervisor_remark varchar(512) NOT NULL DEFAULT '',
    manager_by        uuid         NULL,
    manager_at        timestamptz  NULL,
    manager_remark    varchar(512) NOT NULL DEFAULT '',
    reject_reason     varchar(512) NOT NULL DEFAULT '',
    file_key          varchar(255) NOT NULL DEFAULT '',
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),
    deleted_at        timestamptz  NULL
);
COMMENT ON TABLE inspection_report IS '月度巡检报告：pending_inspector→pending_supervisor→pending_manager→approved，驳回回 pending_inspector';
CREATE UNIQUE INDEX IF NOT EXISTS uk_inspection_report ON inspection_report (community_id, period) WHERE deleted_at IS NULL;

-- ===== 系统配置 ai 分组（幂等）=====
INSERT INTO sys_config (id, key, name, value, config_group, remark)
SELECT gen_random_uuid(), k, n, v, 'ai', r FROM (VALUES
    ('ai.enabled',         '启用大模型审核', 'false',                      '开启后打卡照片由大模型辅助审查'),
    ('ai.base_url',        'API 地址',       'https://api.openai.com/v1',  'OpenAI 兼容接口地址'),
    ('ai.api_key',         'API Key',        '',                           'sk-...（服务端保管，不回传前端）'),
    ('ai.model',           '模型名称',       'gpt-4o-mini',                '须支持图像理解（vision）'),
    ('ai.timeout_seconds', '超时秒数',       '60',                         '超时/失败默认通过并记录 ai_verdict=error'),
    ('ai.prompt',          '审查规则',       '',                           '留空用内置规则：判断照片清晰度、与点位/检查项匹配度、有无明显异常，输出 pass/review')
) AS t(k,n,v,r) WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = t.k);

-- ===== 字典：打卡方式/类型增加 NFC（类型由 seed 保证，此处仅存量库补充）=====
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status)
SELECT gen_random_uuid(), 'checkin_mode', 'NFC', 'nfc', 5, 'enabled'
WHERE EXISTS (SELECT 1 FROM sys_dict_type WHERE code = 'checkin_mode')
  AND NOT EXISTS (SELECT 1 FROM sys_dict_data WHERE type_code = 'checkin_mode' AND value = 'nfc' AND deleted_at IS NULL);
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status)
SELECT gen_random_uuid(), 'checkin_type', 'NFC', 'nfc', 4, 'enabled'
WHERE EXISTS (SELECT 1 FROM sys_dict_type WHERE code = 'checkin_type')
  AND NOT EXISTS (SELECT 1 FROM sys_dict_data WHERE type_code = 'checkin_type' AND value = 'nfc' AND deleted_at IS NULL);

-- ===== 新菜单与权限点（幂等；新库由 seed 写入时此处被 NOT EXISTS 跳过）=====
-- 巡检管理：检查项模板
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, '检查项模板', '/inspection/templates', 'Finished', 'menu', 'inspection:template:list', 5, true, 'enabled', true
FROM sys_menu p WHERE p.path = '/inspection' AND p.type = 'dir' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'inspection:template:list' AND m.deleted_at IS NULL);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, b.t, '', '', 'button', b.p, b.s, false, 'enabled', true
FROM sys_menu p CROSS JOIN (VALUES
    ('新增模板', 'inspection:template:create', 1),
    ('编辑模板', 'inspection:template:update', 2),
    ('删除模板', 'inspection:template:delete', 3)
) AS b(t,p,s)
WHERE p.perms = 'inspection:template:list' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = b.p AND m.deleted_at IS NULL);
-- 巡检管理：记录审核
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, '记录审核', '/inspection/review', 'Stamp', 'menu', 'inspection:checkin:review', 6, true, 'enabled', true
FROM sys_menu p WHERE p.path = '/inspection' AND p.type = 'dir' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'inspection:checkin:review' AND m.deleted_at IS NULL);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, '发起抽查', '', '', 'button', 'inspection:checkin:spotcheck', 1, false, 'enabled', true
FROM sys_menu p WHERE p.perms = 'inspection:checkin:review' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'inspection:checkin:spotcheck' AND m.deleted_at IS NULL);
-- 统计分析：月度报告
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, '月度报告', '/stats/reports', 'Notebook', 'menu', 'report:list', 5, true, 'enabled', true
FROM sys_menu p WHERE p.path = '/stats' AND p.type = 'dir' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'report:list' AND m.deleted_at IS NULL);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, b.t, '', '', 'button', b.p, b.s, false, 'enabled', true
FROM sys_menu p CROSS JOIN (VALUES
    ('生成报告',   'report:generate',        1),
    ('巡检员确认', 'report:sign:inspector',  2),
    ('主管签字',   'report:sign:supervisor', 3),
    ('经理终审',   'report:sign:manager',    4),
    ('下载PDF',    'report:download',        5)
) AS b(t,p,s)
WHERE p.perms = 'report:list' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = b.p AND m.deleted_at IS NULL);

-- ===== 角色菜单分配（超管全量新菜单；主管加模板/审核/报告相关）=====
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'super_admin' AND r.deleted_at IS NULL AND m.deleted_at IS NULL
  AND m.perms IN ('inspection:template:list','inspection:template:create','inspection:template:update','inspection:template:delete',
                  'inspection:checkin:review','inspection:checkin:spotcheck',
                  'report:list','report:generate','report:sign:inspector','report:sign:supervisor','report:sign:manager','report:download')
ON CONFLICT DO NOTHING;
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'manager' AND r.deleted_at IS NULL AND m.deleted_at IS NULL
  AND m.perms IN ('inspection:template:list','inspection:checkin:review','inspection:checkin:spotcheck',
                  'report:list','report:generate','report:sign:supervisor','report:sign:manager','report:download')
ON CONFLICT DO NOTHING;
`},
	{Version: 11, Name: "point_mode_nfc", SQL: `
-- v10 补充：打卡方式 CHECK 约束放行 nfc
ALTER TABLE inspection_point DROP CONSTRAINT IF EXISTS chk_point_mode;
ALTER TABLE inspection_point ADD CONSTRAINT chk_point_mode CHECK (checkin_mode IN ('qrcode','fence','either','both','nfc'));
`},
	{Version: 12, Name: "sys_message_checkin_audit", SQL: `
-- 站内消息类型放行 checkin_audit（打卡审核打回通知）
ALTER TABLE sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type CHECK (type IN ('task','workorder','export','system','checkin_audit'));
`},
	{Version: 13, Name: "checkin_type_nfc", SQL: `
-- 打卡类型 CHECK 约束放行 nfc（分区父表，级联子分区）
ALTER TABLE checkin_record DROP CONSTRAINT IF EXISTS chk_checkin_type;
ALTER TABLE checkin_record ADD CONSTRAINT chk_checkin_type CHECK (checkin_type IN ('qrcode','fence','offline','nfc'));
`},
	{Version: 14, Name: "sys_message_report_type", SQL: `
-- 站内消息类型放行 report（月报签批流转通知）
ALTER TABLE sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type CHECK (type IN ('task','workorder','export','system','checkin_audit','report'));
-- 巡检员角色补月报权限（列表 + 巡检员确认）
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'inspector' AND r.deleted_at IS NULL AND m.deleted_at IS NULL
  AND m.perms IN ('report:list','report:sign:inspector')
ON CONFLICT DO NOTHING;
`},
	{Version: 15, Name: "report_signature_seal", SQL: `
-- 用户手写签名图（月报签字栏用）
ALTER TABLE sys_user ADD COLUMN IF NOT EXISTS signature_file_key varchar(255) NOT NULL DEFAULT '';
COMMENT ON COLUMN sys_user.signature_file_key IS '手写签名图片 file_key（空=未设置；签字时快照进报告留痕）';

-- 报告主管/经理签字签名图快照（巡检员快照存 inspector_signed JSONB 元素内）
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS supervisor_signature_key varchar(255) NOT NULL DEFAULT '';
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS manager_signature_key varchar(255) NOT NULL DEFAULT '';
COMMENT ON COLUMN inspection_report.supervisor_signature_key IS '主管签字时的手写签名图快照 file_key';
COMMENT ON COLUMN inspection_report.manager_signature_key IS '经理终审时的手写签名图快照 file_key';

-- 上传场景放行签名/公章（管理端 /system/upload）
ALTER TABLE upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene CHECK (scene IN ('checkin','workorder','avatar','export','signature','seal'));

-- 月报配置分组（幂等）
INSERT INTO sys_config (id, key, name, value, config_group, remark)
SELECT gen_random_uuid(), k, n, v, 'report', r FROM (VALUES
    ('report.company_name',  '管理单位落款',   '', '月报封面"管理单位"与页尾落款单位名称；空则留白'),
    ('report.seal_file_key', '公章图片',       '', '公章图片 file_key；报告终审通过后封面自动加盖公章，空则不盖')
) AS t(k,n,v,r) WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = t.k);
`},
	{Version: 16, Name: "sign_asset", SQL: `
-- ===== 签章资产表：手写签名/公章独立资产化（法律追溯：换签名/换章不删旧记录，保留版本链）=====
CREATE TABLE IF NOT EXISTS sign_asset (
    id             uuid PRIMARY KEY,
    asset_type     varchar(20)  NOT NULL,
    owner_id       uuid         NULL,
    file_key       varchar(255) NOT NULL,
    sha256         varchar(64)  NOT NULL DEFAULT '',
    version        int          NOT NULL DEFAULT 1,
    status         varchar(20)  NOT NULL DEFAULT 'active',
    remark         varchar(255) NOT NULL DEFAULT '',
    created_by     uuid         NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    revoked_at     timestamptz  NULL,
    revoked_reason varchar(255) NOT NULL DEFAULT '',
    CONSTRAINT chk_sign_asset_type CHECK (asset_type IN ('user_signature','company_seal')),
    CONSTRAINT chk_sign_asset_status CHECK (status IN ('active','replaced','revoked')),
    CONSTRAINT chk_sign_asset_owner CHECK (
        (asset_type = 'user_signature' AND owner_id IS NOT NULL)
        OR (asset_type = 'company_seal' AND owner_id IS NULL)
    )
);
COMMENT ON TABLE sign_asset IS '签章资产：user_signature(owner_id=用户)/company_seal(owner_id NULL)，同 type+owner 仅一条 active；version 同 type+owner 内递增';
COMMENT ON COLUMN sign_asset.sha256 IS '文件内容 SHA-256 指纹（创建时计算；文件缺失/不可读容错为空串）';
CREATE INDEX IF NOT EXISTS idx_sign_asset_owner ON sign_asset (asset_type, owner_id, status);
-- 同 type+owner 的 active 唯一（公章 owner_id NULL 用零 UUID 归一）
CREATE UNIQUE INDEX IF NOT EXISTS uk_sign_asset_active
    ON sign_asset (asset_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE status = 'active';

-- 报告公章快照：终审时固化当前 active 公章 file_key（重渲染优先快照，无快照回退当前 active）
ALTER TABLE inspection_report ADD COLUMN IF NOT EXISTS seal_file_key varchar(255) NOT NULL DEFAULT '';
COMMENT ON COLUMN inspection_report.seal_file_key IS '终审通过时的公章资产 file_key 快照（空=终审时无公章或存量报告）';

-- 存量签名/公章数据迁移为资产 + 删除 report.seal_file_key 配置行：见 migrateSignAssetData（需读文件算 sha256，Go 侧执行，幂等）

-- ===== 菜单：系统管理 → 签章管理（幂等）=====
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, '签章管理', '/system/sign-assets', 'Stamp', 'menu', 'system:signasset:list', 8, true, 'enabled', true
FROM sys_menu p WHERE p.path = '/system' AND p.type = 'dir' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = 'system:signasset:list' AND m.deleted_at IS NULL);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin)
SELECT gen_random_uuid(), p.id, b.t, '', '', 'button', b.p, b.s, false, 'enabled', true
FROM sys_menu p CROSS JOIN (VALUES
    ('新增签章', 'system:signasset:create', 1),
    ('作废签章', 'system:signasset:revoke', 2)
) AS b(t,p,s)
WHERE p.perms = 'system:signasset:list' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.perms = b.p AND m.deleted_at IS NULL);
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r, sys_menu m
WHERE r.code = 'super_admin' AND r.deleted_at IS NULL AND m.deleted_at IS NULL
  AND m.perms IN ('system:signasset:list','system:signasset:create','system:signasset:revoke')
ON CONFLICT DO NOTHING;
`},
	{Version: 17, Name: "item_photos", SQL: `
-- ===== 检查项逐项拍照 + 工单不合格项快照（整改前后对比）=====
-- 工单不合格项快照：items=[{name,remark,before_photos[],after_photos[]}]，photos 存 file_key
ALTER TABLE work_order ADD COLUMN IF NOT EXISTS items jsonb NOT NULL DEFAULT '[]';
COMMENT ON COLUMN work_order.items IS '不合格项快照：[{name,remark,before_photos[],after_photos[]}]（photos 存 file_key；before=整改前/打卡时，after=整改后/回传）';

-- 模板项拍照要求 photo_required（none/optional/required，缺省 none）：
-- 存量数据补齐缺省值；灭火器演示模板"压力表指针在绿区"置 required，其余 optional
UPDATE check_template t SET items = (
    SELECT jsonb_agg(
        elem || jsonb_build_object('photo_required',
            CASE
                WHEN t.name LIKE '灭火器%' AND elem->>'name' = '压力表指针在绿区' THEN 'required'
                WHEN t.name LIKE '灭火器%' THEN 'optional'
                ELSE COALESCE(NULLIF(elem->>'photo_required', ''), 'none')
            END)
    )
    FROM jsonb_array_elements(t.items) elem
);
COMMENT ON TABLE check_template IS '检查项模板：items=[{name,required,photo_required}]，photo_required=none/optional/required（缺省 none），point_type 空为通用';
`},
	{Version: 18, Name: "check_item_tables", SQL: `
-- ===== 检查项拆分为独立表（v18）：模板项 + 打卡逐项结果快照 =====
-- 旧 JSONB 列 check_template.items / checkin_record.check_items 保留不删（代码不再读写，仅存量搬迁读取）。
-- 存量数据搬迁见 migrateCheckItemData（Go 侧，幂等，随每次启动重复执行）。

-- 模板检查项（更新模板=事务内整表替换项行）
CREATE TABLE IF NOT EXISTS check_template_item (
    id             uuid PRIMARY KEY,
    template_id    uuid         NOT NULL REFERENCES check_template (id) ON DELETE CASCADE,
    name           varchar(128) NOT NULL,
    requirement    text         NULL,
    required       boolean      NOT NULL DEFAULT false,
    photo_required varchar(16)  NOT NULL DEFAULT 'none',
    sort           int          NOT NULL DEFAULT 0,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_tpl_item_photo_req CHECK (photo_required IN ('none','optional','required'))
);
COMMENT ON TABLE check_template_item IS '模板检查项（v18 起替代 check_template.items JSONB）；requirement=检查标准要求文本（可空）';
CREATE INDEX IF NOT EXISTS idx_tpl_item_tpl ON check_template_item (template_id, sort);

-- 打卡逐项结果快照：打卡当时从模板项复制 name/requirement/photo_required，
-- template_item_id 仅作可空血缘字段（统计用），历史记录内容绝不依赖 join 模板表。
-- record_id 不加 FK：checkin_record 为按月分区表（主键 id+created_at），FK 须包含分区键，
-- 故仅建普通索引，由打卡事务同时写入保证一致性。
CREATE TABLE IF NOT EXISTS checkin_record_item (
    id               uuid PRIMARY KEY,
    record_id        uuid         NOT NULL,
    template_item_id uuid         NULL,
    name             varchar(128) NOT NULL,
    requirement      text         NULL,
    photo_required   varchar(16)  NOT NULL DEFAULT 'none',
    pass             boolean      NOT NULL DEFAULT false,
    note             varchar(512) NOT NULL DEFAULT '',
    photos           jsonb        NOT NULL DEFAULT '[]',
    ai_verdict       varchar(16)  NULL,
    ai_reason        varchar(512) NULL,
    sort             int          NOT NULL DEFAULT 0,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_rec_item_photo_req CHECK (photo_required IN ('none','optional','required'))
);
COMMENT ON TABLE checkin_record_item IS '打卡逐项结果快照（v18 起替代 checkin_record.check_items JSONB）；ai_verdict/ai_reason 为逐项大模型结论（预留）';
COMMENT ON COLUMN checkin_record_item.record_id IS '打卡记录 id（无 FK：checkin_record 为分区表，主键含分区键）';
COMMENT ON COLUMN checkin_record_item.photos IS '该项照片 file_key 数组';
CREATE INDEX IF NOT EXISTS idx_rec_item_rec ON checkin_record_item (record_id, sort);
CREATE INDEX IF NOT EXISTS idx_rec_item_tplitem ON checkin_record_item (template_item_id);
`},
	{Version: 19, Name: "drop_deprecated_jsonb_columns", SQL: `
-- ===== 清理废弃列（v19）：v16/v18 数据搬迁已完成，删除旧 JSONB/标量列 =====
ALTER TABLE check_template DROP COLUMN IF EXISTS items;
ALTER TABLE checkin_record DROP COLUMN IF EXISTS check_items;
ALTER TABLE sys_user DROP COLUMN IF EXISTS signature_file_key;
`},
}

// Migrate 执行未应用过的迁移，并确保当月与下月分区存在。
func Migrate(db *gorm.DB) error {
	// 迁移记录表
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    int PRIMARY KEY,
		name       varchar(128) NOT NULL,
		applied_at timestamptz  NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return err
	}

	for _, m := range migrations {
		var count int64
		if err := db.Raw(`SELECT count(*) FROM schema_migrations WHERE version = ?`, m.Version).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(m.SQL).Error; err != nil {
				return fmt.Errorf("迁移 v%d(%s) 执行失败: %w", m.Version, m.Name, err)
			}
			return tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name).Error
		}); err != nil {
			return err
		}
	}

	// 按月分区表：确保当月与下月分区已创建（月度滚动由运维脚本兜底，这里保证启动即可写）
	if err := EnsurePartitions(db, "checkin_record"); err != nil {
		return err
	}
	return EnsurePartitions(db, "sys_operation_log")
}

// EnsurePartitions 为分区表创建当月与下月分区（不存在才建）。
func EnsurePartitions(db *gorm.DB, parent string) error {
	now := time.Now()
	for i := 0; i < 2; i++ {
		first := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.Local)
		next := first.AddDate(0, 1, 0)
		partName := fmt.Sprintf("%s_%04d_%02d", parent, first.Year(), int(first.Month()))
		sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)`,
			partName, parent,
			quoteLiteral(first.Format("2006-01-02")), quoteLiteral(next.Format("2006-01-02")))
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("创建分区 %s 失败: %w", partName, err)
		}
	}
	return nil
}

func quoteLiteral(s string) string { return "'" + s + "'" }
