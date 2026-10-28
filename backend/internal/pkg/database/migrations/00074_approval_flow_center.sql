-- +goose Up
-- owner: system
INSERT INTO approval_flow (id, tenant_id, project_id, flow_code, steps)
SELECT md5('acceptance-flow:platform')::uuid, NULL, NULL, 'acceptance_review',
       '[{"role_code":"project_manager","name":"查验报告签署","mode":"any"}]'
ON CONFLICT DO NOTHING;
INSERT INTO approval_flow (id, tenant_id, project_id, flow_code, steps)
SELECT md5('acceptance-flow:' || id::text)::uuid, id, NULL, 'acceptance_review',
       '[{"role_code":"project_manager","name":"查验报告签署","mode":"any"}]'
FROM tenant
ON CONFLICT DO NOTHING;
UPDATE sys_menu SET title = '审批流程' WHERE path = '/platform/review-flow-template';
UPDATE sys_menu SET title = '岗位' WHERE path = '/system/roles';

-- +goose Down
SELECT 1;
