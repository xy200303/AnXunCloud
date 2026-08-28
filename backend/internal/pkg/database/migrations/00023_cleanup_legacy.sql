-- 遗留清理：死列删除 + 工单时代枚举/字典收尾。
-- 1) checkin_record_item.template_item_id 只写不读（血缘字段，全库无查询引用），删列删索引；
-- 2) upload_file.size 只写不读（从未被查询使用），删列；
-- 3) 收紧 CHECK 枚举：sys_message.type / upload_file.scene 去掉 'workorder'（表与代码已随 00022 删除）；
-- 4) 清理老库工单字典残留（order_priority/work_order_status/order_source，00012 起代码已不插入）。
-- +goose Up
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_rec_item_tplitem;
ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS template_item_id;

ALTER TABLE upload_file DROP COLUMN IF EXISTS size;

ALTER TABLE sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type
    CHECK (type IN ('task', 'export', 'system', 'checkin_audit', 'report', 'announcement'));

ALTER TABLE upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (scene IN ('checkin', 'avatar', 'export', 'signature', 'seal', 'notice', 'app'));

DELETE FROM sys_dict_data WHERE type_code IN ('order_priority', 'work_order_status', 'order_source');
DELETE FROM sys_dict_type WHERE code IN ('order_priority', 'work_order_status', 'order_source');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS template_item_id uuid;
CREATE INDEX IF NOT EXISTS idx_rec_item_tplitem ON checkin_record_item USING btree (template_item_id);

ALTER TABLE upload_file ADD COLUMN IF NOT EXISTS size bigint DEFAULT 0 NOT NULL;

ALTER TABLE sys_message DROP CONSTRAINT IF EXISTS chk_sys_message_type;
ALTER TABLE sys_message ADD CONSTRAINT chk_sys_message_type
    CHECK (type IN ('task', 'workorder', 'export', 'system', 'checkin_audit', 'report', 'announcement'));

ALTER TABLE upload_file DROP CONSTRAINT IF EXISTS chk_upload_file_scene;
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_scene
    CHECK (scene IN ('checkin', 'workorder', 'avatar', 'export', 'signature', 'seal', 'notice', 'app'));

-- 字典数据不回滚（如需恢复从 seed 逻辑重建）

-- +goose StatementEnd
