-- +goose Up
-- owner: inspection
ALTER TABLE check_template ADD COLUMN code varchar(64);
ALTER TABLE check_template ADD COLUMN purpose varchar(16);
ALTER TABLE check_template ADD COLUMN published_at timestamptz;
UPDATE check_template SET code = md5(COALESCE(tenant_id::text, 'platform') || ':' || id::text), purpose = 'patrol', point_type = 'common' WHERE code IS NULL;
UPDATE check_template SET point_type = 'common' WHERE point_type IS NULL OR btrim(point_type) = '';
UPDATE check_template SET status = 'draft' WHERE status = 'disabled';
UPDATE check_template SET published_at = updated_at WHERE status = 'enabled' AND published_at IS NULL;
ALTER TABLE check_template ALTER COLUMN code SET NOT NULL;
ALTER TABLE check_template ALTER COLUMN purpose SET NOT NULL;
ALTER TABLE check_template ALTER COLUMN point_type SET NOT NULL;
ALTER TABLE check_template ADD CONSTRAINT chk_check_template_purpose CHECK (purpose IN ('patrol', 'spot', 'verify'));
ALTER TABLE check_template ADD CONSTRAINT chk_check_template_status CHECK (status IN ('draft', 'enabled'));
ALTER TABLE check_template ADD CONSTRAINT chk_check_template_point_type CHECK (btrim(point_type) <> '');
CREATE UNIQUE INDEX uk_check_template_code ON check_template (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code) WHERE deleted_at IS NULL;

-- +goose Down
SELECT 1;
