-- +goose Up

ALTER TABLE checkin_record_item ADD COLUMN IF NOT EXISTS exception_type varchar(24) DEFAULT '' NOT NULL;
ALTER TABLE checkin_item_draft ADD COLUMN IF NOT EXISTS exception_type varchar(24) DEFAULT '' NOT NULL;

-- +goose Down

ALTER TABLE checkin_record_item DROP COLUMN IF EXISTS exception_type;
ALTER TABLE checkin_item_draft DROP COLUMN IF EXISTS exception_type;
