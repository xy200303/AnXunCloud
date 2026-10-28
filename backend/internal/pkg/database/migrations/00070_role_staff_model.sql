-- +goose Up
-- owner: system
-- 角色 + 任职两层授权。迁移前必须停写并完成 DB 与文件恢复演练。
ALTER TABLE sys_role ADD COLUMN scope_level varchar(16);
ALTER TABLE sys_role ADD COLUMN is_supervisor boolean NOT NULL DEFAULT false;
ALTER TABLE sys_role ADD COLUMN claim_work_types jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE sys_user ADD COLUMN role_id uuid;
ALTER TABLE sys_user ADD COLUMN person_key varchar(128);
ALTER TABLE project_staff ADD COLUMN effective_from timestamptz;
ALTER TABLE project_staff ADD COLUMN effective_to timestamptz;
ALTER TABLE project_staff ADD COLUMN delegate_user_id uuid;
ALTER TABLE project_staff ADD COLUMN delegate_until timestamptz;

CREATE TABLE role_migration_audit (
    user_id uuid PRIMARY KEY,
    old_role_ids jsonb NOT NULL,
    source_posts jsonb NOT NULL,
    effective_role_ids jsonb NOT NULL,
    target_role_id uuid
);

WITH RECURSIVE post_tree AS (
    SELECT p.tenant_id, p.code, p.name, p.is_supervisor, p.status, p.role_id
    FROM post_dict p
), builtin(code, name, scope, supervisor) AS (
    VALUES
    ('super_admin', '超级管理员', 'platform', true),
    ('tenant_admin', '租户管理员', 'tenant', true),
    ('project_manager', '项目经理', 'project', true),
    ('quality_supervisor', '品质主管', 'project', true),
    ('engineer_supervisor', '工程主管', 'project', true),
    ('inspector', '巡检员', 'project', false),
    ('maintainer', '维保工', 'project', false),
    ('building_manager', '楼管员', 'project', false),
    ('fire_duty', '消控值班', 'project', false)
), inserted AS (
    INSERT INTO sys_role (id, tenant_id, code, name, scope_level, is_supervisor, claim_work_types, status, is_builtin, created_at, updated_at)
    SELECT md5('builtin-role:' || code)::uuid, NULL, code, name, scope, supervisor, '[]', 'enabled', true, NOW(), NOW() FROM builtin
    ON CONFLICT DO NOTHING
    RETURNING id
)
SELECT 1;

INSERT INTO sys_role (id, tenant_id, code, name, scope_level, is_supervisor, claim_work_types, status, is_builtin, created_at, updated_at)
SELECT md5('custom-role:' || COALESCE(tenant_id::text, '') || ':' || code)::uuid,
       tenant_id, code, name, 'project', is_supervisor, '[]', status, false, NOW(), NOW()
FROM post_dict p
WHERE code NOT IN ('super_admin', 'tenant_admin', 'project_manager', 'safety_supervisor', 'inspector')
  AND NOT EXISTS (SELECT 1 FROM sys_role r WHERE r.code = p.code AND r.tenant_id IS NOT DISTINCT FROM p.tenant_id);

UPDATE sys_role SET scope_level = CASE
    WHEN code = 'super_admin' THEN 'platform'
    WHEN code = 'tenant_admin' THEN 'tenant'
    ELSE 'project' END,
    is_supervisor = CASE WHEN code IN ('super_admin', 'tenant_admin', 'project_admin', 'project_manager', 'quality_supervisor', 'engineer_supervisor') THEN true ELSE is_supervisor END
WHERE scope_level IS NULL;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT target.id, old_menu.menu_id
FROM sys_role old
JOIN sys_role target ON target.code IN ('project_manager', 'quality_supervisor', 'engineer_supervisor') AND target.tenant_id IS NULL
JOIN sys_role_menu old_menu ON old_menu.role_id = old.id
WHERE old.code = 'project_admin'
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT target.id, old_menu.menu_id
FROM sys_role old
JOIN sys_role target ON target.code IN ('inspector', 'maintainer', 'building_manager', 'fire_duty') AND target.tenant_id IS NULL
JOIN sys_role_menu old_menu ON old_menu.role_id = old.id
WHERE old.code = 'field_staff'
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT target.id, old_menu.menu_id
FROM post_dict p
JOIN sys_role target ON target.code = p.code AND target.tenant_id IS NOT DISTINCT FROM p.tenant_id
JOIN sys_role_menu old_menu ON old_menu.role_id = p.role_id
ON CONFLICT DO NOTHING;

WITH staff_posts AS (
    SELECT ps.user_id, c.tenant_id, array_agg(DISTINCT p.code ORDER BY p.code) AS codes,
           array_agg(DISTINCT p.role_id::text ORDER BY p.role_id::text) AS role_ids
    FROM project_staff ps
    JOIN community c ON c.id = ps.project_id
    JOIN post_dict p ON p.tenant_id = c.tenant_id AND p.code IN (SELECT jsonb_array_elements_text(ps.posts))
    WHERE ps.status = 'enabled'
    GROUP BY ps.user_id, c.tenant_id
), direct_roles AS (
    SELECT u.id, array_agg(DISTINCT value ORDER BY value) AS role_ids
    FROM sys_user u LEFT JOIN jsonb_array_elements_text(u.role_ids) value ON true
    GROUP BY u.id
), effective AS (
    SELECT u.id, u.tenant_id,
           COALESCE((SELECT array_agg(DISTINCT value ORDER BY value) FROM (
               SELECT value FROM jsonb_array_elements_text(COALESCE(to_jsonb(dr.role_ids), '[]'::jsonb)) value
               UNION SELECT unnest(sp.role_ids)
           ) roles(value)), '{}') AS role_ids,
           COALESCE(sp.codes, '{}') AS posts
    FROM sys_user u
    LEFT JOIN direct_roles dr ON dr.id = u.id
    LEFT JOIN staff_posts sp ON sp.user_id = u.id AND sp.tenant_id = u.tenant_id
), mapped AS (
    SELECT e.id, e.tenant_id, e.role_ids, e.posts,
      CASE
        WHEN COALESCE(array_length(e.role_ids, 1), 0) = 0 THEN NULL
        WHEN COALESCE(array_length(e.role_ids, 1), 0) > 1 THEN
          (SELECT NULLIF('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000')::uuid)
        WHEN COALESCE(array_length(e.posts, 1), 0) = 1 THEN
          (SELECT r.id FROM sys_role r WHERE r.tenant_id IS NULL AND r.code = CASE e.posts[1]
             WHEN 'safety_supervisor' THEN 'quality_supervisor' ELSE e.posts[1] END)
        WHEN e.role_ids[1] = (SELECT id::text FROM sys_role WHERE code = 'project_admin' AND tenant_id IS NULL) THEN
          (SELECT id FROM sys_role WHERE code = 'project_manager' AND tenant_id IS NULL)
        WHEN e.role_ids[1] = (SELECT id::text FROM sys_role WHERE code = 'field_staff' AND tenant_id IS NULL) THEN NULL
        ELSE e.role_ids[1]::uuid
      END AS target_id
    FROM effective e
)
INSERT INTO role_migration_audit (user_id, old_role_ids, source_posts, effective_role_ids, target_role_id)
SELECT u.id, u.role_ids, to_jsonb(e.posts), to_jsonb(e.role_ids), e.target_id
FROM sys_user u JOIN mapped e ON e.id = u.id;

-- +goose StatementBegin
DO $$
DECLARE bad record;
BEGIN
    FOR bad IN SELECT a.user_id, u.username, a.effective_role_ids FROM role_migration_audit a JOIN sys_user u ON u.id = a.user_id
        WHERE jsonb_array_length(a.effective_role_ids) > 1 OR (jsonb_array_length(a.effective_role_ids) = 1 AND a.target_role_id IS NULL)
    LOOP
        RAISE EXCEPTION '账号 % (%) 无法无损映射为单角色，请先确认工种：%', bad.username, bad.user_id, bad.effective_role_ids;
    END LOOP;
END $$;
-- +goose StatementEnd

UPDATE sys_user u SET role_id = a.target_role_id,
    person_key = COALESCE(NULLIF(u.phone, ''), u.id::text)
FROM role_migration_audit a WHERE a.user_id = u.id;

-- +goose StatementBegin
DO $$
DECLARE bad record;
BEGIN
    FOR bad IN
        SELECT u.id, u.username FROM sys_user u
        JOIN role_migration_audit a ON a.user_id = u.id
        WHERE COALESCE(array_length(ARRAY(
            SELECT DISTINCT sm.perms FROM sys_role_menu rm
            JOIN sys_menu sm ON sm.id = rm.menu_id AND sm.deleted_at IS NULL
            WHERE rm.role_id IN (SELECT jsonb_array_elements_text(a.effective_role_ids)::uuid)
        ), 1), 0) <> COALESCE(array_length(ARRAY(
            SELECT DISTINCT sm.perms FROM sys_role_menu rm
            JOIN sys_menu sm ON sm.id = rm.menu_id AND sm.deleted_at IS NULL
            WHERE rm.role_id = u.role_id
        ), 1), 0)
          OR EXISTS (
            SELECT 1 FROM sys_role_menu rm JOIN sys_menu sm ON sm.id = rm.menu_id AND sm.deleted_at IS NULL
            WHERE rm.role_id IN (SELECT jsonb_array_elements_text(a.effective_role_ids)::uuid)
            EXCEPT
            SELECT 1 FROM sys_role_menu rm JOIN sys_menu sm ON sm.id = rm.menu_id AND sm.deleted_at IS NULL
            WHERE rm.role_id = u.role_id
          )
    LOOP
        RAISE EXCEPTION '账号 % (%) 新旧权限点不一致，迁移中止', bad.username, bad.id;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.flow_role_code(slot text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE slot
        WHEN 'report_inspector' THEN 'inspector'
        WHEN 'patrol_execute' THEN 'inspector'
        WHEN 'patrol_report_line' THEN 'quality_supervisor'
        WHEN 'project_review' THEN 'project_manager'
        ELSE slot END;
$$;
-- +goose StatementEnd

UPDATE approval_flow SET steps = COALESCE((
    SELECT jsonb_agg((step - 'slot') || jsonb_build_object('role_code', pg_temp.flow_role_code(step->>'slot')) ORDER BY ord)
    FROM jsonb_array_elements(steps) WITH ORDINALITY AS j(step, ord)
), '[]'::jsonb);
UPDATE approval_flow SET steps = '[{"role_code":"quality_supervisor","name":"打卡复核","mode":"any"}]' WHERE flow_code = 'checkin_review' AND steps = '[]'::jsonb;
UPDATE approval_flow SET steps = '[{"role_code":"engineer_supervisor","name":"维保审核","mode":"any"}]' WHERE flow_code = 'maint_review' AND steps = '[]'::jsonb;
UPDATE approval_flow SET steps = '[{"role_code":"project_manager","name":"月报审批","mode":"any"}]' WHERE flow_code = 'report_review' AND steps = '[]'::jsonb;
UPDATE checkin_record SET flow_snapshot = COALESCE((
    SELECT jsonb_agg((step - 'slot') || jsonb_build_object('role_code', pg_temp.flow_role_code(step->>'slot')) ORDER BY ord)
    FROM jsonb_array_elements(flow_snapshot) WITH ORDINALITY AS j(step, ord)
), '[]'::jsonb) WHERE flow_snapshot IS NOT NULL;
UPDATE equipment_maintenance SET flow_snapshot = COALESCE((
    SELECT jsonb_agg((step - 'slot') || jsonb_build_object('role_code', pg_temp.flow_role_code(step->>'slot')) ORDER BY ord)
    FROM jsonb_array_elements(flow_snapshot) WITH ORDINALITY AS j(step, ord)
), '[]'::jsonb) WHERE flow_snapshot IS NOT NULL;
UPDATE inspection_report SET review_steps = COALESCE((
    SELECT jsonb_agg((step - 'slot') || jsonb_build_object('role_code', pg_temp.flow_role_code(step->>'slot')) ORDER BY ord)
    FROM jsonb_array_elements(review_steps) WITH ORDINALITY AS j(step, ord)
), '[]'::jsonb);

ALTER TABLE sys_user DROP CONSTRAINT IF EXISTS chk_sys_user_roles;
ALTER TABLE sys_user DROP COLUMN role_ids;
ALTER TABLE sys_role DROP COLUMN data_scope;
ALTER TABLE sys_role ALTER COLUMN scope_level SET NOT NULL;
ALTER TABLE sys_role ADD CONSTRAINT chk_sys_role_scope CHECK (scope_level IN ('platform', 'tenant', 'project'));
ALTER TABLE sys_user ADD CONSTRAINT fk_sys_user_role FOREIGN KEY (role_id) REFERENCES sys_role(id);
CREATE UNIQUE INDEX uk_sys_user_person ON sys_user (tenant_id, person_key) WHERE deleted_at IS NULL;
CREATE INDEX idx_sys_user_role ON sys_user (role_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_project_staff_effective ON project_staff (user_id, status, effective_from, effective_to);
ALTER TABLE project_staff DROP COLUMN posts;
DROP TABLE duty_binding;
DROP TABLE post_dict;
DELETE FROM sys_role WHERE code IN ('project_admin', 'field_staff');

-- +goose Down
-- 破坏性模型合并不支持逆向恢复；从迁移前备份恢复。
SELECT 1;
