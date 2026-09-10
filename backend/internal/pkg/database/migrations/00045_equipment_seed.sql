-- 00045：设备台账配套数据——菜单/角色授权/字典/系统配置（存量库迁移，全部幂等可重复执行）。
-- 全新库由 seed.go 直接生成（NOT EXISTS 守卫自动跳过）。
-- 授权口径：超管/租户管理员/项目管理员全量；一线人员（巡检员）授 equipment:list + equipment:maintenance。

-- +goose Up

-- 1) 「设备台账」菜单（巡检管理目录下，sort 5）
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '设备台账', '/inspection/equipment', 'Box', 'menu', 'equipment:list', 5, true, 'enabled', true, false, now(), now()
FROM sys_menu p
WHERE p.path = '/inspection' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.path = '/inspection/equipment' AND m.deleted_at IS NULL);

-- 2) 菜单按钮
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), m.id, t.title, '', '', 'button', t.perms, t.sort, false, 'enabled', true, false, now(), now()
FROM (SELECT id FROM sys_menu WHERE path = '/inspection/equipment' AND deleted_at IS NULL) m
CROSS JOIN (VALUES
  ('新增设备', 'equipment:create', 1),
  ('编辑设备', 'equipment:update', 2),
  ('删除设备', 'equipment:delete', 3),
  ('批量导入', 'equipment:import', 4),
  ('导出',     'equipment:export', 5),
  ('维保登记', 'equipment:maintenance', 6),
  ('维保确认', 'equipment:confirm', 7)
) AS t(title, perms, sort)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_menu b WHERE b.parent_id = m.id AND b.perms = t.perms AND b.deleted_at IS NULL
);

-- 3) 角色授权：超管/租户管理员/项目管理员全量；一线人员（巡检员）仅 list + maintenance
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r, sys_menu m
WHERE r.code IN ('super_admin', 'tenant_admin', 'project_admin')
  AND (m.path = '/inspection/equipment'
       OR m.parent_id = (SELECT id FROM sys_menu WHERE path = '/inspection/equipment' AND deleted_at IS NULL))
  AND m.deleted_at IS NULL
ON CONFLICT (role_id, menu_id) DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r, sys_menu m
WHERE r.code = 'field_staff'
  AND m.perms IN ('equipment:list', 'equipment:maintenance')
  AND m.deleted_at IS NULL
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- 4) 字典：设备类型（attrs 携带维保规则）+ 维保类型
INSERT INTO sys_dict_type (id, code, name, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment_type', '设备类型', '系统预置', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_type WHERE code = 'equipment_type');

INSERT INTO sys_dict_type (id, code, name, remark, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment_maint_type', '维保类型', '系统预置', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_type WHERE code = 'equipment_maint_type');

INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, attrs, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment_type', t.label, t.value, t.sort, 'enabled', t.attrs, now(), now()
FROM (VALUES
  ('干粉灭火器', 'extinguisher', 1, '{"first_months":60,"cycle_months":24,"remind":true,"scrap_months":120}'::jsonb),
  ('消火栓',     'hydrant',      2, '{"first_months":0,"cycle_months":12,"remind":true,"scrap_months":0}'::jsonb),
  ('水泵',       'pump',         3, '{"first_months":0,"cycle_months":6,"remind":true,"scrap_months":0}'::jsonb),
  ('电梯',       'elevator',     4, '{"first_months":0,"cycle_months":1,"remind":false,"scrap_months":0}'::jsonb),
  ('配电柜',     'distribution', 5, '{"first_months":0,"cycle_months":12,"remind":true,"scrap_months":0}'::jsonb),
  ('烟感',       'smoke_detector', 6, '{"first_months":0,"cycle_months":12,"remind":true,"scrap_months":0}'::jsonb)
) AS t(label, value, sort, attrs)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_dict_data d WHERE d.type_code = 'equipment_type' AND d.value = t.value AND d.deleted_at IS NULL
);

INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, created_at, updated_at)
SELECT gen_random_uuid(), 'equipment_maint_type', t.label, t.value, t.sort, 'enabled', now(), now()
FROM (VALUES
  ('维修充粉', 'repair', 1),
  ('保养',     'maintain', 2),
  ('检测',     'inspect', 3),
  ('更换',     'replace', 4),
  ('台账补录', 'ledger_fix', 5)
) AS t(label, value, sort)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_dict_data d WHERE d.type_code = 'equipment_maint_type' AND d.value = t.value AND d.deleted_at IS NULL
);

-- 4.5) 消息类型 CHECK 约束扩充：设备到期提醒 + 维保驳回通知
ALTER TABLE sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type
  CHECK (type IN ('task','export','system','checkin_audit','report','announcement','equipment_expire','equipment_maint_reject'));

-- 5) 系统配置默认值
INSERT INTO sys_config (id, key, name, value, config_group, remark, created_at, updated_at)
SELECT gen_random_uuid(), t.key, t.name, t.value, 'equipment', t.remark, now(), now()
FROM (VALUES
  ('equipment.expire_warn_days', '设备临期提醒提前量(天)', '30', '下次到期日前 N 天进入临期状态并提醒'),
  ('equipment.expire_check_time', '设备到期扫描时间', '08:00', '每日该时刻扫描临期/逾期设备并推送提醒'),
  ('equipment.overdue_remind_interval_days', '设备逾期重复提醒间隔(天)', '7', '逾期后每隔 N 天重复提醒直至登记维保'),
  ('equipment.escalate_days', '设备逾期升级经理天数', '7', '逾期超过 N 天未登记时升级通知物业经理/安全主管'),
  ('equipment.spotcheck_enabled', '设备日期抽查开关', 'true', '二期：灭火器日期标签抽查总开关'),
  ('equipment.spotcheck_ratio', '设备日期抽查比例(%)', '10', '二期：默认抽查比例，可被检查项 judge_config.ratio 覆盖')
) AS t(key, name, value, remark)
WHERE NOT EXISTS (SELECT 1 FROM sys_config c WHERE c.key = t.key AND c.deleted_at IS NULL);

-- +goose Down
-- 还原：删菜单（role_menu 级联清理）、字典与配置。
DELETE FROM sys_menu WHERE path = '/inspection/equipment'
   OR parent_id = (SELECT id FROM sys_menu WHERE path = '/inspection/equipment');
DELETE FROM sys_dict_data WHERE type_code IN ('equipment_type', 'equipment_maint_type');
DELETE FROM sys_dict_type WHERE code IN ('equipment_type', 'equipment_maint_type');
DELETE FROM sys_config WHERE key LIKE 'equipment.%';
