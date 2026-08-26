-- 免核验点位：放开「凭证与围栏至少启用一项」硬约束（开放式巡更/演示场景）。
-- 同步删除 point_service 的同名 service 层校验；checkin.checkMode 原本就支持
-- credential=none 免凭证 + require_fence=false 不校验围栏，无需改动。
-- +goose Up
-- +goose StatementBegin

ALTER TABLE inspection_point DROP CONSTRAINT IF EXISTS chk_point_check_valid;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE inspection_point ADD CONSTRAINT chk_point_check_valid CHECK (credential::text <> 'none'::text OR require_fence);

-- +goose StatementEnd
