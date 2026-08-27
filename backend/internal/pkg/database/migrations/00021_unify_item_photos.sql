-- 照片模型统一：全部照片归属检查项（checkin_record_item.photos），废除记录级 photos。
-- 背景：点位强制绑定检查项模板后，必拍项由模板项 photo_required=required 推导，
-- 记录级 photos 与 inspection_point.required_photo_items 均为冗余历史形态。
-- 处理：历史打卡数据（演示数据）直接清空；无模板点位回填自动创建的「通用巡检模板」；
-- 删除 checkin_record.photos 与 inspection_point.required_photo_items 两列。
-- +goose Up
-- +goose StatementBegin

-- 1) 清空演示打卡数据（正式记录/逐项快照/过程草稿；任务进度归零）
DELETE FROM checkin_item_draft;
DELETE FROM checkin_record_item;
DELETE FROM checkin_record;
UPDATE inspection_task SET done_points = 0, status = 'pending', started_at = NULL, finished_at = NULL;

-- 2) 无模板点位回填「通用巡检模板」（按租户分组，含 NULL 租户）
INSERT INTO check_template (id, tenant_id, name, point_type, sort, status, remark, created_at, updated_at)
SELECT gen_random_uuid(), t.tenant_id, '通用巡检模板', '', 0, 'enabled', '系统迁移自动创建：无模板点位的默认模板', now(), now()
FROM (SELECT DISTINCT tenant_id FROM inspection_point WHERE template_id IS NULL) t;

INSERT INTO check_template_item (id, template_id, name, requirement, photo_required, sort, judge_type, judge_config)
SELECT gen_random_uuid(), ct.id, '现场环境全貌', '拍摄点位现场整体环境，画面清晰、覆盖主要区域', 'required', 0, 'general', '{}'::jsonb
FROM check_template ct
WHERE ct.name = '通用巡检模板'
  AND NOT EXISTS (SELECT 1 FROM check_template_item i WHERE i.template_id = ct.id);

UPDATE inspection_point p SET template_id = ct.id
FROM check_template ct
WHERE p.template_id IS NULL AND ct.name = '通用巡检模板'
  AND (ct.tenant_id IS NOT DISTINCT FROM p.tenant_id);

-- 3) 删列
ALTER TABLE checkin_record DROP COLUMN IF EXISTS photos;
ALTER TABLE inspection_point DROP COLUMN IF EXISTS required_photo_items;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE checkin_record ADD COLUMN IF NOT EXISTS photos JSONB NOT NULL DEFAULT '[]';
ALTER TABLE inspection_point ADD COLUMN IF NOT EXISTS required_photo_items JSONB NOT NULL DEFAULT '[]';

-- +goose StatementEnd
