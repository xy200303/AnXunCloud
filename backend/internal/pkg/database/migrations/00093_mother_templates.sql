-- +goose Up
-- owner: report
UPDATE doc_template
SET skeleton_file_id = CASE volume_code
    WHEN 'fire_equipment' THEN '93000000-0000-0000-0000-000000000001'
    WHEN 'fire_system' THEN '93000000-0000-0000-0000-000000000002'
    WHEN 'supply' THEN '93000000-0000-0000-0000-000000000003'
    WHEN 'power' THEN '93000000-0000-0000-0000-000000000004'
    WHEN 'special' THEN '93000000-0000-0000-0000-000000000005'
    ELSE skeleton_file_id
END,
layout = jsonb_build_object(
    'mother', jsonb_build_object('profile', volume_code),
    'sign_slots', COALESCE(layout->'sign_slots', '[]'::jsonb)
)
WHERE doc_kind = 'monthly_report';

ALTER TABLE template_output DROP COLUMN header_groups;
ALTER TABLE template_output DROP COLUMN row_axis;
ALTER TABLE template_output DROP COLUMN header_fields;
ALTER TABLE template_output DROP COLUMN standard_prefix;
ALTER TABLE template_output DROP COLUMN issue_columns;
ALTER TABLE template_output DROP COLUMN photo_inline;
UPDATE template_output SET columns = '[]'::jsonb;

-- +goose Down
SELECT 1;
