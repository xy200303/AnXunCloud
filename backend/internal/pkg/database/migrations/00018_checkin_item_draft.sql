-- 逐项 AI 识别过程草稿：拍完一项、AI 识别完即落库（过程记录，不代表点位已打卡）。
-- 语义：巡检员在点位逐项拍照识别时，服务端把每项的照片 keys + AI 结论实时写入本表；
-- 点位最终提交（checkin_record 落库）成功后才同事务删除草稿——本表只是"进行中"的过程数据，
-- 不影响任务进度/点位状态，后台与 App 断点恢复可据此看到正式提交前的逐项识别结果。
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS checkin_item_draft (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NULL,
    task_id       UUID NOT NULL,
    point_id      UUID NOT NULL,
    inspector_id  UUID NOT NULL,
    community_id  UUID NOT NULL,
    item_name     VARCHAR(128) NOT NULL,
    job_id        VARCHAR(64) NOT NULL DEFAULT '',
    file_keys     JSONB NOT NULL DEFAULT '[]',
    ai_status     VARCHAR(16) NOT NULL DEFAULT 'pending',
    ai_verdict    VARCHAR(16) NULL,
    ai_reason     VARCHAR(512) NULL,
    ai_reading    VARCHAR(64) NULL,
    quality_pass  BOOLEAN NULL,
    quality_issue VARCHAR(255) NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE checkin_item_draft IS '逐项 AI 识别过程草稿（点位正式提交后删除；仅过程记录，不计入进度）';
COMMENT ON COLUMN checkin_item_draft.ai_status IS 'pending=识别中 / done=识别完成 / failed=识别失败';

CREATE UNIQUE INDEX IF NOT EXISTS uk_item_draft_item ON checkin_item_draft (task_id, point_id, item_name);
CREATE INDEX IF NOT EXISTS idx_item_draft_task ON checkin_item_draft (task_id, point_id);
CREATE INDEX IF NOT EXISTS idx_item_draft_inspector ON checkin_item_draft (inspector_id, updated_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS checkin_item_draft;

-- +goose StatementEnd
