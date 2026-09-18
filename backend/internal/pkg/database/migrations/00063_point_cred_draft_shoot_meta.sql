-- 00063：点位凭证核验草稿（断点恢复，§14.2）+ 照片时空校验（防作弊标记可疑不拒收，§14.3）。
-- 幂等可重复执行。

-- +goose Up

-- 点位凭证核验草稿：扫码/NFC/围栏核验通过即落库（退出向导重进直接恢复，不再要求重新签到）；
-- 点位正式提交成功（checkin_record 落库）后与逐项草稿同事务删除。
CREATE TABLE IF NOT EXISTS checkin_point_cred_draft (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    community_id uuid NOT NULL,
    task_id uuid NOT NULL,
    point_id uuid NOT NULL,
    inspector_id uuid NOT NULL,
    checkin_type character varying(16) NOT NULL,
    cred_no character varying(128) DEFAULT ''::character varying NOT NULL,
    fence_distance double precision,
    verified_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT checkin_point_cred_draft_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_point_cred_draft ON checkin_point_cred_draft USING btree (task_id, point_id, inspector_id);

COMMENT ON TABLE checkin_point_cred_draft IS '点位凭证核验草稿（核验通过即落库，断点恢复用；点位正式提交成功后删除）';
COMMENT ON COLUMN checkin_point_cred_draft.checkin_type IS '凭证方式：qrcode/nfc/fence';
COMMENT ON COLUMN checkin_point_cred_draft.cred_no IS '扫码码值/NFC 卡号（fence 为空）';
COMMENT ON COLUMN checkin_point_cred_draft.fence_distance IS '围栏核验时距点位距离（米；非围栏凭证为 NULL）';

-- 逐项过程草稿：照片拍摄时空信息（拍照时携带的定位/时刻；可空=未上报，防作弊时空一致性判定用）
ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS shoot_lng double precision;
ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS shoot_lat double precision;
ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS shoot_at timestamp with time zone;

COMMENT ON COLUMN checkin_item_draft.shoot_at IS '照片拍摄时刻（客户端上报，解析失败存 NULL）';

-- 打卡逐项结论：照片拍摄时空信息（提交时从逐项草稿带出）+ 可疑标记（标记不拒收，审核页提示）
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS shoot_lng double precision;
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS shoot_lat double precision;
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS shoot_at timestamp with time zone;
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS suspicious boolean DEFAULT false NOT NULL;
ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS suspicious_reason character varying(255) DEFAULT ''::character varying NOT NULL;

COMMENT ON COLUMN checkin_record_item.suspicious IS '时空一致性可疑（围栏外拍摄/拍照早于凭证核验；标记不拒收）';
COMMENT ON COLUMN checkin_record_item.suspicious_reason IS '可疑原因（多原因以「；」连接）';

-- +goose Down

ALTER TABLE checkin_record_item
    DROP COLUMN IF EXISTS shoot_lng,
    DROP COLUMN IF EXISTS shoot_lat,
    DROP COLUMN IF EXISTS shoot_at,
    DROP COLUMN IF EXISTS suspicious,
    DROP COLUMN IF EXISTS suspicious_reason;
ALTER TABLE checkin_item_draft
    DROP COLUMN IF EXISTS shoot_lng,
    DROP COLUMN IF EXISTS shoot_lat,
    DROP COLUMN IF EXISTS shoot_at;
DROP TABLE IF EXISTS checkin_point_cred_draft;
