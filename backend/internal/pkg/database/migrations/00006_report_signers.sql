-- 月报指定签字人：supervisor_ids / manager_ids 为生成时圈定的该级签字人名单（空数组 = 该级无人可签，自动跳过且 PDF 签字栏留空）。
-- 候选人规则：启用用户 ∧ 持有对应签字权限点 ∧ 数据范围覆盖该小区（角色 data_scope=all 或用户 community_ids 含该小区）∧ 非超管。
-- +goose Up
ALTER TABLE inspection_report
    ADD COLUMN IF NOT EXISTS supervisor_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS manager_ids    jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN public.inspection_report.supervisor_ids IS 'JSONB 指定安全主管签字人 ID 数组（空=该级跳过）';
COMMENT ON COLUMN public.inspection_report.manager_ids    IS 'JSONB 指定物业经理签字人 ID 数组（空=该级跳过）';

-- 存量未归档报告回填签字人名单（与 signCandidates 同规则；回填为空的级别请重新生成报告以跳过）
UPDATE inspection_report r SET supervisor_ids = COALESCE((
    SELECT jsonb_agg(u.id) FROM sys_user u
    WHERE u.status = 'enabled' AND u.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                      JOIN sys_role sr ON sr.id = e.rid::uuid WHERE sr.code = 'super_admin')
      AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                  JOIN sys_role sr ON sr.id = e.rid::uuid AND sr.status = 'enabled'
                  JOIN sys_role_menu rm ON rm.role_id = sr.id
                  JOIN sys_menu m ON m.id = rm.menu_id AND m.status = 'enabled'
                  WHERE m.perms = 'report:sign:supervisor')
      AND (EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                   JOIN sys_role sr ON sr.id = e.rid::uuid
                   WHERE sr.status = 'enabled' AND sr.data_scope = 'all')
           OR u.community_ids ? r.community_id::text)
), '[]'::jsonb)
WHERE r.status <> 'approved';

UPDATE inspection_report r SET manager_ids = COALESCE((
    SELECT jsonb_agg(u.id) FROM sys_user u
    WHERE u.status = 'enabled' AND u.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                      JOIN sys_role sr ON sr.id = e.rid::uuid WHERE sr.code = 'super_admin')
      AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                  JOIN sys_role sr ON sr.id = e.rid::uuid AND sr.status = 'enabled'
                  JOIN sys_role_menu rm ON rm.role_id = sr.id
                  JOIN sys_menu m ON m.id = rm.menu_id AND m.status = 'enabled'
                  WHERE m.perms = 'report:sign:manager')
      AND (EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.role_ids) e(rid)
                   JOIN sys_role sr ON sr.id = e.rid::uuid
                   WHERE sr.status = 'enabled' AND sr.data_scope = 'all')
           OR u.community_ids ? r.community_id::text)
), '[]'::jsonb)
WHERE r.status <> 'approved';

-- +goose Down
ALTER TABLE inspection_report
    DROP COLUMN IF EXISTS supervisor_ids,
    DROP COLUMN IF EXISTS manager_ids;
