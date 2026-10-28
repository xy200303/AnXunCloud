-- +goose Up
-- owner: file
ALTER TABLE upload_file ADD COLUMN parent_file_id uuid REFERENCES upload_file(id);
ALTER TABLE upload_file ADD COLUMN evidence_role varchar(24) NOT NULL DEFAULT 'original';
ALTER TABLE upload_file ADD COLUMN sha256 varchar(64) NOT NULL DEFAULT '';
ALTER TABLE upload_file ADD COLUMN derivative_meta jsonb NOT NULL DEFAULT '{}';
ALTER TABLE upload_file ADD CONSTRAINT chk_upload_file_evidence_role CHECK (evidence_role IN ('original', 'cleaned', 'watermarked', 'thumbnail'));
CREATE INDEX idx_upload_file_parent ON upload_file(parent_file_id) WHERE parent_file_id IS NOT NULL;

CREATE TABLE file_object_ref (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    file_id uuid NOT NULL REFERENCES upload_file(id) ON DELETE CASCADE,
    object_type varchar(48) NOT NULL,
    object_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_file_object_ref UNIQUE (file_id, object_type, object_id)
);
CREATE INDEX idx_file_object_ref_file ON file_object_ref(file_id, tenant_id, community_id);
CREATE INDEX idx_file_object_ref_object ON file_object_ref(tenant_id, object_type, object_id);

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:report:' || rf.id::text)::uuid, rf.tenant_id, r.community_id, rf.file_id, 'inspection_report', r.id
FROM report_file rf JOIN inspection_report r ON r.id = rf.report_id
WHERE rf.file_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:acceptance:' || rf.id::text)::uuid, rf.tenant_id, b.community_id, rf.file_id, 'acceptance_batch', b.id
FROM acceptance_report_file rf JOIN acceptance_batch b ON b.id = rf.batch_id
WHERE rf.file_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:checkin:' || i.id::text || ':' || ref.file_id)::uuid,
       i.tenant_id, r.community_id, ref.file_id::uuid, 'checkin_record', r.id
FROM checkin_record_item i
JOIN checkin_record r ON r.id = i.record_id
CROSS JOIN LATERAL jsonb_array_elements_text(i.photos) AS ref(file_id)
WHERE ref.file_id ~ '^[0-9a-fA-F-]{36}$'
ON CONFLICT DO NOTHING;

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:workorder:' || i.id::text || ':' || ref.file_id)::uuid,
       i.tenant_id, i.community_id, ref.file_id::uuid, 'work_order_issue', i.id
FROM work_order_issue i
CROSS JOIN LATERAL jsonb_array_elements_text(i.photos) AS ref(file_id)
WHERE ref.file_id ~ '^[0-9a-fA-F-]{36}$'
ON CONFLICT DO NOTHING;

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:workorder-fix:' || a.id::text || ':' || ref.file_id)::uuid,
       o.tenant_id, o.community_id, ref.file_id::uuid, 'work_order_fix_attempt', a.id
FROM work_order_fix_attempt a JOIN work_order o ON o.id = a.order_id
CROSS JOIN LATERAL jsonb_array_elements_text(a.photos) AS ref(file_id)
WHERE ref.file_id ~ '^[0-9a-fA-F-]{36}$'
ON CONFLICT DO NOTHING;

INSERT INTO file_object_ref (id, tenant_id, community_id, file_id, object_type, object_id)
SELECT md5('file-ref:acceptance-record:' || r.id::text || ':' || ref.file_id)::uuid,
       r.tenant_id, b.community_id, ref.file_id::uuid, 'acceptance_record', r.id
FROM acceptance_record r JOIN acceptance_batch b ON b.id = r.batch_id
CROSS JOIN LATERAL jsonb_array_elements_text(r.photos) AS ref(file_id)
WHERE ref.file_id ~ '^[0-9a-fA-F-]{36}$'
ON CONFLICT DO NOTHING;

-- +goose Down
SELECT 1;
