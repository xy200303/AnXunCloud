-- +goose Up
-- owner: inspection
UPDATE inspection_plan
SET cycle_config = jsonb_set(
    COALESCE(cycle_config, '{}'::jsonb),
    '{rounds}',
    COALESCE(cycle_config->'rounds', jsonb_build_array(jsonb_build_object('name', '', 'window', time_window))),
    true
)
WHERE time_window IS NOT NULL AND time_window <> '';
ALTER TABLE inspection_plan DROP COLUMN time_window;

-- +goose Down
SELECT 1;
