-- 文案统一：分诊 → 受理（仅用户可见显示名，权限点/枚举值/字段名不变，幂等）
-- +goose Up
-- +goose StatementBegin

-- 字典：工单状态 reported 的显示名
UPDATE sys_dict_data SET label = '待受理', updated_at = now()
WHERE type_code = 'work_order_status' AND value = 'reported' AND deleted_at IS NULL AND label = '待分诊';

-- 菜单按钮：工单分诊 → 工单受理（权限点 workorder:triage 不变）
UPDATE sys_menu SET title = '工单受理', updated_at = now()
WHERE perms = 'workorder:triage' AND deleted_at IS NULL AND title = '工单分诊';

-- 内置岗位描述：客服主管
UPDATE post_dict SET remark = '管理前台接待和楼管员，报单受理', updated_at = now()
WHERE code = 'service_supervisor' AND remark = '管理前台接待和楼管员，报单分诊';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE sys_dict_data SET label = '待分诊', updated_at = now()
WHERE type_code = 'work_order_status' AND value = 'reported' AND deleted_at IS NULL AND label = '待受理';
UPDATE sys_menu SET title = '工单分诊', updated_at = now()
WHERE perms = 'workorder:triage' AND deleted_at IS NULL AND title = '工单受理';
UPDATE post_dict SET remark = '管理前台接待和楼管员，报单分诊', updated_at = now()
WHERE code = 'service_supervisor' AND remark = '管理前台接待和楼管员，报单受理';
-- +goose StatementEnd
