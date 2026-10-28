-- +goose Up
-- owner: inspection
-- 在停写、备份和映射审计后执行。过程草稿不是正式事实；无法沿用旧名称键的在途草稿显式失效。
-- +goose StatementBegin
DO $$
DECLARE
    ambiguous_count bigint;
    optional_count bigint;
    draft_count bigint;
BEGIN
    SELECT count(*) INTO optional_count FROM check_template_item WHERE NOT required;
    SELECT count(*) INTO draft_count FROM checkin_item_draft;
    RAISE WARNING 'L1 migration: % legacy optional items become note_required=false; % in-flight drafts require restart', optional_count, draft_count;

    SELECT count(*) INTO ambiguous_count
    FROM checkin_record_item i
    JOIN checkin_record r ON r.id = i.record_id
    WHERE i.judge_type NOT IN ('equipment_validity', 'equipment_date_spot')
      AND (SELECT count(*) FROM point_template pt
           JOIN check_template_item ti ON ti.template_id = pt.template_id AND ti.name = i.name
           WHERE pt.point_id = r.point_id) <> 1;
    IF ambiguous_count > 0 THEN
        RAISE EXCEPTION 'L1 migration stopped: % record items have ambiguous or missing template identity', ambiguous_count;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE check_template_item ADD COLUMN tenant_id uuid;
ALTER TABLE check_template_item ADD COLUMN code varchar(64);
ALTER TABLE check_template_item ADD COLUMN standard text NOT NULL DEFAULT '';
ALTER TABLE check_template_item ADD COLUMN standard_no varchar(16) NOT NULL DEFAULT '';
ALTER TABLE check_template_item ADD COLUMN value_type varchar(16);
ALTER TABLE check_template_item ADD COLUMN unit varchar(16) NOT NULL DEFAULT '';
ALTER TABLE check_template_item ADD COLUMN value_min numeric;
ALTER TABLE check_template_item ADD COLUMN value_max numeric;
ALTER TABLE check_template_item ADD COLUMN enum_options jsonb NOT NULL DEFAULT '[]';
ALTER TABLE check_template_item ADD COLUMN note_required boolean NOT NULL DEFAULT false;
ALTER TABLE check_template_item ADD COLUMN ai_prompt_kind varchar(24) NOT NULL DEFAULT 'general';
ALTER TABLE check_template_item ADD COLUMN ai_enabled boolean NOT NULL DEFAULT true;
ALTER TABLE check_template_item ADD COLUMN builtin_kind varchar(24) NOT NULL DEFAULT '';
ALTER TABLE check_template_item ADD COLUMN retired_at timestamptz;

UPDATE check_template_item i SET tenant_id = t.tenant_id FROM check_template t WHERE t.id = i.template_id;
UPDATE check_template_item SET code = 'item_' || substr(md5(id::text), 1, 24), standard = COALESCE(requirement, ''),
    value_type = CASE WHEN judge_type = 'metric' THEN 'number' WHEN judge_type IN ('state', 'indicator') THEN 'enum' ELSE 'ok' END,
    unit = COALESCE(judge_config->>'unit', ''),
    value_min = CASE WHEN judge_type = 'metric' AND jsonb_typeof(judge_config->'min') = 'number' THEN (judge_config->>'min')::numeric END,
    value_max = CASE WHEN judge_type = 'metric' AND jsonb_typeof(judge_config->'max') = 'number' THEN (judge_config->>'max')::numeric END,
    enum_options = CASE WHEN judge_type IN ('state', 'indicator') AND COALESCE(judge_config->>'expected', '') <> ''
        THEN jsonb_build_array(jsonb_build_object('value', 'expected', 'label', judge_config->>'expected', 'result', 'normal'), jsonb_build_object('value', 'abnormal', 'label', '异常', 'result', 'abnormal'))
        ELSE '[]'::jsonb END,
    ai_prompt_kind = CASE WHEN judge_type IN ('general', 'presence', 'damage', 'metric', 'state', 'label', 'passage', 'leak', 'indicator', 'tidiness', 'baseline') THEN judge_type ELSE 'general' END,
    ai_enabled = judge_type <> 'manual',
    builtin_kind = CASE WHEN judge_type IN ('equipment_validity', 'equipment_date_spot') THEN judge_type ELSE '' END;

ALTER TABLE check_template_item ALTER COLUMN code SET NOT NULL;
ALTER TABLE check_template_item ALTER COLUMN value_type SET NOT NULL;
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_value_type CHECK (value_type IN ('ok', 'number', 'date', 'text', 'enum'));
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_code CHECK (code ~ '^[a-z][a-z0-9_]{2,63}$');
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_range CHECK (value_min IS NULL OR value_max IS NULL OR value_min <= value_max);
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_number CHECK (value_type = 'number' OR (unit = '' AND value_min IS NULL AND value_max IS NULL));
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_enum CHECK ((value_type = 'enum' AND jsonb_array_length(enum_options) >= 2) OR (value_type <> 'enum' AND enum_options = '[]'::jsonb));
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_prompt CHECK (ai_prompt_kind IN ('general', 'presence', 'damage', 'metric', 'state', 'label', 'passage', 'leak', 'indicator', 'tidiness', 'baseline'));
ALTER TABLE check_template_item ADD CONSTRAINT chk_check_item_builtin CHECK (builtin_kind IN ('', 'equipment_validity', 'equipment_date_spot'));
CREATE UNIQUE INDEX uk_check_template_item_code ON check_template_item (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), template_id, code);
CREATE INDEX idx_check_template_item_active ON check_template_item (template_id, sort) WHERE retired_at IS NULL;

ALTER TABLE checkin_record_item ADD COLUMN tenant_id uuid;
ALTER TABLE checkin_record_item ADD COLUMN template_id uuid;
ALTER TABLE checkin_record_item ADD COLUMN item_code varchar(64);
ALTER TABLE checkin_record_item ADD COLUMN equipment_id uuid;
ALTER TABLE checkin_record_item ADD COLUMN value_num numeric;
ALTER TABLE checkin_record_item ADD COLUMN value_date date;
ALTER TABLE checkin_record_item ADD COLUMN value_text text;
ALTER TABLE checkin_record_item ADD COLUMN standard text NOT NULL DEFAULT '';
ALTER TABLE checkin_record_item ADD COLUMN builtin_kind varchar(24) NOT NULL DEFAULT '';

UPDATE checkin_record_item i SET tenant_id = r.tenant_id FROM checkin_record r WHERE r.id = i.record_id;
UPDATE checkin_record_item i SET template_id = matched.template_id, item_code = matched.code, standard = COALESCE(i.requirement, '')
FROM (
    SELECT i.id, ti.template_id, ti.code
    FROM checkin_record_item i
    JOIN checkin_record r ON r.id = i.record_id
    JOIN point_template pt ON pt.point_id = r.point_id
    JOIN check_template_item ti ON ti.template_id = pt.template_id AND ti.name = i.name
    WHERE i.judge_type NOT IN ('equipment_validity', 'equipment_date_spot')
) matched WHERE matched.id = i.id;
UPDATE checkin_record_item SET item_code = 'builtin_' || judge_type,
    builtin_kind = judge_type,
    equipment_id = CASE WHEN judge_config->>'equipment_id' ~ '^[0-9a-fA-F-]{36}$' THEN (judge_config->>'equipment_id')::uuid END,
    standard = COALESCE(requirement, '')
WHERE judge_type IN ('equipment_validity', 'equipment_date_spot');

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM checkin_record_item WHERE item_code IS NULL OR tenant_id IS NULL) THEN
        RAISE EXCEPTION 'L1 migration stopped: record item identity or tenant backfill incomplete';
    END IF;
    IF EXISTS (SELECT 1 FROM checkin_record_item WHERE builtin_kind <> '' AND template_id IS NOT NULL) THEN
        RAISE EXCEPTION 'L1 migration stopped: builtin item unexpectedly owns a template identity';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE checkin_record_item ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE checkin_record_item ALTER COLUMN item_code SET NOT NULL;
CREATE UNIQUE INDEX uk_record_item_place ON checkin_record_item (tenant_id, record_id, template_id, item_code) WHERE equipment_id IS NULL;
CREATE UNIQUE INDEX uk_record_item_equipment ON checkin_record_item (tenant_id, record_id, template_id, item_code, equipment_id) WHERE equipment_id IS NOT NULL;
CREATE INDEX idx_record_item_definition ON checkin_record_item (template_id, item_code, equipment_id);

ALTER TABLE checkin_item_draft ADD COLUMN template_id uuid;
ALTER TABLE checkin_item_draft ADD COLUMN item_code varchar(64);
ALTER TABLE checkin_item_draft ADD COLUMN equipment_id uuid;
ALTER TABLE checkin_item_draft ADD COLUMN value_num numeric;
ALTER TABLE checkin_item_draft ADD COLUMN value_date date;
ALTER TABLE checkin_item_draft ADD COLUMN value_text text;
ALTER TABLE checkin_item_draft ADD COLUMN request_id varchar(64) NOT NULL DEFAULT '';
ALTER TABLE checkin_item_draft ADD COLUMN retake_seq integer NOT NULL DEFAULT 0;
ALTER TABLE checkin_item_draft ADD COLUMN photo_set_hash varchar(64) NOT NULL DEFAULT '';
DELETE FROM checkin_item_draft;
DROP INDEX uk_item_draft_item;
ALTER TABLE checkin_item_draft DROP COLUMN item_name;
ALTER TABLE checkin_item_draft ALTER COLUMN template_id SET NOT NULL;
ALTER TABLE checkin_item_draft ALTER COLUMN item_code SET NOT NULL;
ALTER TABLE checkin_item_draft ALTER COLUMN request_id SET DEFAULT '';
CREATE UNIQUE INDEX uk_item_draft_place ON checkin_item_draft (task_id, point_id, template_id, item_code) WHERE equipment_id IS NULL;
CREATE UNIQUE INDEX uk_item_draft_equipment ON checkin_item_draft (task_id, point_id, template_id, item_code, equipment_id) WHERE equipment_id IS NOT NULL;

ALTER TABLE check_template_item DROP COLUMN requirement;
ALTER TABLE check_template_item DROP COLUMN judge_type;
ALTER TABLE check_template_item DROP COLUMN judge_config;
ALTER TABLE check_template_item DROP COLUMN required;
ALTER TABLE checkin_record_item DROP COLUMN requirement;
ALTER TABLE checkin_record_item DROP COLUMN ai_hint;
ALTER TABLE checkin_record_item DROP COLUMN judge_type;
ALTER TABLE checkin_record_item DROP COLUMN judge_config;
ALTER TABLE checkin_record_item DROP COLUMN tags;
ALTER TABLE checkin_record_item DROP COLUMN photo_required;

-- +goose Down
SELECT 1;
