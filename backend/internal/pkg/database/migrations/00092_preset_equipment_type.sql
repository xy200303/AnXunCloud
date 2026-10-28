-- +goose Up
-- owner: community
UPDATE community_preset_pack
SET payload = jsonb_set(
    payload,
    '{equipment_schemas}',
    (
        SELECT jsonb_agg(jsonb_set(item, '{type}', '"extinguisher"'))
        FROM jsonb_array_elements(payload->'equipment_schemas') AS item
    ),
    false
),
updated_at = now()
WHERE code = 'property_fire_baseline'
  AND jsonb_typeof(payload->'equipment_schemas') = 'array';

-- +goose Down
SELECT 1;
