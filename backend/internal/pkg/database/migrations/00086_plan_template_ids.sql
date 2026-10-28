-- +goose Up
-- owner: inspection
ALTER TABLE inspection_plan ADD COLUMN template_ids jsonb NOT NULL DEFAULT '[]';
ALTER TABLE inspection_task ADD COLUMN template_ids jsonb NOT NULL DEFAULT '[]';

UPDATE inspection_plan p SET template_ids = COALESCE((
    SELECT jsonb_agg(DISTINCT pt.template_id)
    FROM point_template pt
    WHERE pt.point_id IN (SELECT value::uuid FROM jsonb_array_elements_text(p.point_ids))
), '[]'::jsonb)
WHERE selection_mode <> 'by_point_types';

UPDATE inspection_plan p SET template_ids = COALESCE((
    SELECT jsonb_agg(DISTINCT pt.template_id)
    FROM point_template pt
    JOIN inspection_point ip ON ip.id = pt.point_id AND ip.deleted_at IS NULL
    WHERE ip.community_id = p.community_id
      AND ip.type IN (SELECT value FROM jsonb_array_elements_text(p.point_types))
), '[]'::jsonb)
WHERE selection_mode = 'by_point_types';

UPDATE inspection_task t SET template_ids = COALESCE((SELECT p.template_ids FROM inspection_plan p WHERE p.id = t.plan_id), '[]'::jsonb);

DROP INDEX uk_check_template_code;
CREATE UNIQUE INDEX uk_check_template_code ON check_template (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code);

-- +goose Down
SELECT 1;
