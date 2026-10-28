-- +goose Up
-- owner: report
UPDATE community_preset_pack
SET payload = jsonb_set(
    payload,
    '{doc_templates}',
    (
        SELECT jsonb_agg(
            jsonb_set(
                jsonb_set(
                    item,
                    '{layout}',
                    jsonb_build_object(
                        'mother', jsonb_build_object('profile', 'fire_equipment'),
                        'sign_slots', COALESCE(item#>>'{layout,sign_slots}', '[]')
                    ),
                    true
                ),
                '{skeleton_file_id}',
                '"93000000-0000-0000-0000-000000000001"',
                true
            )
        )
        FROM jsonb_array_elements(payload->'doc_templates') AS item
    ),
    false
),
updated_at = now()
WHERE code = 'property_fire_baseline'
  AND jsonb_typeof(payload->'doc_templates') = 'array';

-- +goose Down
SELECT 1;
