-- +goose Up
-- owner: acceptance
ALTER TABLE acceptance_target ADD COLUMN building_id uuid;
ALTER TABLE acceptance_record ADD COLUMN value_map jsonb NOT NULL DEFAULT '{}'::jsonb;
UPDATE acceptance_target SET building_id = ref_id::uuid WHERE target_type = 'building' AND ref_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
CREATE INDEX idx_acceptance_target_building ON acceptance_target(batch_id, building_id) WHERE building_id IS NOT NULL;

-- +goose Down
SELECT 1;
