-- 审批链配置（《汇报线与审批链扩展设计方案》§3，P2）：打卡审核按链执行
-- +goose Up
-- +goose StatementBegin

-- 审批链配置表（作用域镜像 duty_binding：project_id/tenant_id 均空 = 平台默认；
-- tenant_id 非空 project_id 空 = 租户级默认；project_id 非空 = 项目级覆盖）
CREATE TABLE approval_flow (
    id uuid PRIMARY KEY,
    tenant_id uuid,
    project_id uuid,
    flow_code varchar(64) NOT NULL,          -- 流程 code（系统定义：checkin_review 打卡审核链）
    steps jsonb NOT NULL DEFAULT '[]',       -- 有序环节 [{slot, name}]，环节名单解析复用职责槽位
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE approval_flow IS '审批链配置（项目级 → 租户级 → 平台默认回落；steps 环节引用职责槽位 code）';
CREATE UNIQUE INDEX uk_approval_flow_scope ON public.approval_flow USING btree (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), flow_code);

-- 打卡记录：当前审批进度（0 = 待第 1 环节；通过全部环节后 audit_status=pass；驳回回 0 重新走链）
ALTER TABLE checkin_record ADD COLUMN audit_step smallint NOT NULL DEFAULT 0;
COMMENT ON COLUMN checkin_record.audit_step IS '审批链当前进度：已通过环节数（0=待第 1 环节）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE checkin_record DROP COLUMN audit_step;
DROP TABLE approval_flow;
-- +goose StatementEnd
