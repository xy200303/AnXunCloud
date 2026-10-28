-- +goose Up
-- owner: report
INSERT INTO sys_dict_type (id, code, name, remark)
SELECT md5('dict-type:point_type')::uuid, 'point_type', '点位类型', '点位与设备系统分类'
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_type WHERE code = 'point_type');
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '供电设备', 'system_sort', 2) WHERE type_code='point_type' AND value='power_room';
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '供水设备', 'system_sort', 1) WHERE type_code='point_type' AND value='pump_room';
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '消防设施（系统类）', 'system_sort', 4) WHERE type_code='point_type' AND value='fire_control';
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '特种设备', 'system_sort', 5) WHERE type_code='point_type' AND value IN ('elevator', 'garage');
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '消防设施（器材类）', 'system_sort', 3) WHERE type_code='point_type' AND value IN ('fire_cabinet', 'fire_extinguisher');
UPDATE sys_dict_data SET attrs = COALESCE(attrs, '{}'::jsonb) || jsonb_build_object('system', '', 'system_sort', 99) WHERE type_code='point_type' AND value='common';
INSERT INTO sys_dict_data (id, type_code, label, value, sort, status, attrs) VALUES
(md5('point_type:supply_room')::uuid, 'point_type', '供水设备房', 'supply_room', 20, 'enabled', '{"level":"place","system":"供水设备","system_sort":1}'),
(md5('point_type:water_pump')::uuid, 'point_type', '生活水泵（水箱）', 'water_pump', 21, 'enabled', '{"level":"object","system":"供水设备","system_sort":1}'),
(md5('point_type:fire_pump')::uuid, 'point_type', '消防水泵（水池）', 'fire_pump', 22, 'enabled', '{"level":"object","system":"供水设备","system_sort":1}'),
(md5('point_type:water_meter')::uuid, 'point_type', '公区水表', 'water_meter', 23, 'enabled', '{"level":"object","system":"供水设备","system_sort":1}'),
(md5('point_type:sub_meter')::uuid, 'point_type', '分户水表', 'sub_meter', 24, 'enabled', '{"level":"object","system":"供水设备","system_sort":1}'),
(md5('point_type:generator')::uuid, 'point_type', '柴油发电机（房）', 'generator', 25, 'enabled', '{"level":"object","system":"供电设备","system_sort":2}'),
(md5('point_type:distribution_box')::uuid, 'point_type', '配电箱（柜）', 'distribution_box', 26, 'enabled', '{"level":"object","system":"供电设备","system_sort":2}'),
(md5('point_type:fire_door')::uuid, 'point_type', '防火门·应急灯等', 'fire_door', 27, 'enabled', '{"level":"object","system":"消防设施（器材类）","system_sort":3}'),
(md5('point_type:control_room')::uuid, 'point_type', '消防控制室', 'control_room', 28, 'enabled', '{"level":"place","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:hydrant_test')::uuid, 'point_type', '消火栓联动', 'hydrant_test', 29, 'enabled', '{"level":"object","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:sprinkler_test')::uuid, 'point_type', '喷淋联动', 'sprinkler_test', 30, 'enabled', '{"level":"object","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:broadcast_test')::uuid, 'point_type', '广播', 'broadcast_test', 31, 'enabled', '{"level":"object","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:phone_test')::uuid, 'point_type', '消防电话', 'phone_test', 32, 'enabled', '{"level":"object","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:fan_test')::uuid, 'point_type', '风机', 'fan_test', 33, 'enabled', '{"level":"object","system":"消防设施（系统类）","system_sort":4}'),
(md5('point_type:elevator_talk')::uuid, 'point_type', '电梯五方对讲', 'elevator_talk', 34, 'enabled', '{"level":"object","system":"特种设备","system_sort":5}'),
(md5('point_type:car_park')::uuid, 'point_type', '机械式立体车库', 'car_park', 35, 'enabled', '{"level":"object","system":"特种设备","system_sort":5}')
ON CONFLICT DO NOTHING;

CREATE TABLE doc_template (
    id uuid PRIMARY KEY, tenant_id uuid, code varchar(64) NOT NULL, name varchar(128) NOT NULL,
    doc_kind varchar(24) NOT NULL, engine varchar(16) NOT NULL, format varchar(16) NOT NULL,
    layout jsonb NOT NULL DEFAULT '{"blocks":[]}', style_pack_id uuid, skeleton_file_id uuid,
    volume_code varchar(64) NOT NULL, system_filter varchar(64) NOT NULL DEFAULT '',
    status varchar(16) NOT NULL, published_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_doc_template_code UNIQUE (tenant_id, code),
    CONSTRAINT chk_doc_template_kind CHECK (doc_kind IN ('monthly_report', 'acceptance', 'maintenance_ledger')),
    CONSTRAINT chk_doc_template_engine CHECK (engine IN ('ooxml', 'xlsx_template')),
    CONSTRAINT chk_doc_template_format CHECK (format IN ('pdf', 'docx', 'xlsx')),
    CONSTRAINT chk_doc_template_status CHECK (status IN ('draft', 'enabled'))
);
CREATE UNIQUE INDEX uk_doc_template_platform_code ON doc_template (code) WHERE tenant_id IS NULL;
INSERT INTO doc_template (id, tenant_id, code, name, doc_kind, engine, format, layout, volume_code, system_filter, status, published_at) VALUES
('82000000-0000-0000-0000-000000000001', NULL, 'monthly_report_supply', '供水设备（房）月度巡检维保台账', 'monthly_report', 'ooxml', 'docx', '{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"template_loop"},{"type":"sign"}]}', 'supply', '供水设备', 'enabled', now()),
('82000000-0000-0000-0000-000000000002', NULL, 'monthly_report_power', '供电设备（房）月度巡检维保台账', 'monthly_report', 'ooxml', 'docx', '{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"template_loop"},{"type":"sign"}]}', 'power', '供电设备', 'enabled', now()),
('82000000-0000-0000-0000-000000000003', NULL, 'monthly_report_fire_equipment', '物业消防设施（器材类）月度巡检维保台账', 'monthly_report', 'ooxml', 'docx', '{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"template_loop"},{"type":"sign"}]}', 'fire_equipment', '消防设施（器材类）', 'enabled', now()),
('82000000-0000-0000-0000-000000000004', NULL, 'monthly_report_fire_system', '物业消防设施（系统类）月度巡检测试台帐', 'monthly_report', 'ooxml', 'docx', '{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"template_loop"},{"type":"sign"}]}', 'fire_system', '消防设施（系统类）', 'enabled', now()),
('82000000-0000-0000-0000-000000000005', NULL, 'monthly_report_special', '物业特种设备（房）月度巡检维保台账', 'monthly_report', 'ooxml', 'docx', '{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"template_loop"},{"type":"sign"}]}', 'special', '特种设备', 'enabled', now()),
('82000000-0000-0000-0000-000000000006', NULL, 'maintenance_refill_ledger', '灭火器换粉维修台账', 'maintenance_ledger', 'xlsx_template', 'xlsx', '{"blocks":[{"type":"summary_table"}]}', 'refill', '', 'enabled', now())
ON CONFLICT DO NOTHING;
UPDATE doc_template SET layout = jsonb_set(layout, '{sign_slots}', '[{"code":"site","label":"现场确认","step_index":0,"required":true},{"code":"supervisor","label":"主管审核","step_index":1,"required":true},{"code":"manager","label":"月报审批","step_index":2,"required":true}]', true) WHERE doc_kind = 'monthly_report';
CREATE TABLE template_output (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, doc_template_id uuid NOT NULL, template_id uuid NOT NULL,
    sort integer NOT NULL DEFAULT 0, columns jsonb, header_groups jsonb NOT NULL DEFAULT '[]',
    row_mode varchar(16) NOT NULL DEFAULT 'point', row_axis jsonb NOT NULL DEFAULT '{}', header_fields jsonb NOT NULL DEFAULT '{}',
    standard_prefix varchar(32), issue_columns jsonb NOT NULL DEFAULT '[]', photo_inline jsonb NOT NULL DEFAULT '{}',
    CONSTRAINT uk_template_output UNIQUE (tenant_id, doc_template_id, template_id),
    CONSTRAINT chk_template_output_row_mode CHECK (row_mode IN ('date', 'point', 'grid', 'equipment'))
);
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '82000000-0000-0000-0000-000000000007', (SELECT id FROM sys_menu WHERE path='/templates' LIMIT 1), '输出模板', '/templates/output', 'Document', 'menu', 'report:template:list', 2, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE path='/templates/output');
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform)
SELECT '82000000-0000-0000-0000-000000000008', '82000000-0000-0000-0000-000000000007', '维护输出模板', '', '', 'button', 'report:template:update', 1, true, 'enabled', true, false WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE perms='report:template:update');
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r CROSS JOIN sys_menu m WHERE m.id IN ('82000000-0000-0000-0000-000000000007', '82000000-0000-0000-0000-000000000008') AND r.code IN ('super_admin', 'tenant_admin') ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
