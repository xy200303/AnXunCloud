-- 00052：类型化台账——equipment_type_schema 设备类型字段方案（《巡检与台账优化方案（9.14实测反馈）》§六）。
-- 设备表保持「固定主字段 + extra jsonb 口袋」不变，按设备类型（equipment_type 字典 value）定义四样配置：
-- list_columns 列表列 / export_columns 导出列集 / form_fields 表单字段 / import_headers 导入模板列头。
-- tenant_id NULL = 平台默认方案；租户级方案整体覆盖平台默认（读取时租户优先、缺省回落平台）。

-- +goose Up
CREATE TABLE IF NOT EXISTS public.equipment_type_schema (
    id uuid NOT NULL PRIMARY KEY,
    tenant_id uuid,
    type character varying(64) NOT NULL,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    status character varying(16) NOT NULL DEFAULT 'enabled',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);
-- 租户级与平台级（tenant_id IS NULL）各自唯一（软删行不占位）
CREATE UNIQUE INDEX IF NOT EXISTS uk_equipment_type_schema_tenant ON public.equipment_type_schema (tenant_id, type) WHERE deleted_at IS NULL AND tenant_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_equipment_type_schema_platform ON public.equipment_type_schema (type) WHERE deleted_at IS NULL AND tenant_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_equipment_type_schema_type ON public.equipment_type_schema (type) WHERE deleted_at IS NULL;

COMMENT ON TABLE public.equipment_type_schema IS '设备类型字段方案：按设备类型定义列表列/导出列/表单字段/导入列头；tenant_id NULL=平台默认方案，租户级整体覆盖';
COMMENT ON COLUMN public.equipment_type_schema.type IS '设备类型（equipment_type 字典 value，如 extinguisher/hydrant/elevator/custom_N）';
COMMENT ON COLUMN public.equipment_type_schema.config IS 'jsonb：{"list_columns":[{"key","label","width"}],"export_columns":[{"key","label","value"}],"form_fields":[{"key","label","type"}],"import_headers":["列头"]}；export_columns.value 非空=固定文本列';

-- 平台默认方案：灭火器（精细，导出即甲方换粉台账版式；「所属设备系统」为固定文本列）
INSERT INTO public.equipment_type_schema (id, tenant_id, type, config, status, created_at, updated_at)
SELECT gen_random_uuid(), NULL, 'extinguisher', '{
  "list_columns": [
    {"key": "code", "label": "设备编号", "width": 140},
    {"key": "name", "label": "设备名称", "width": 160},
    {"key": "point_name", "label": "点位信息", "width": 160},
    {"key": "refill_date", "label": "换粉日期", "width": 120},
    {"key": "next_due_date", "label": "下次到期日", "width": 120},
    {"key": "status_label", "label": "状态", "width": 90}
  ],
  "export_columns": [
    {"key": "community_name", "label": "项目名称"},
    {"key": "system", "label": "所属设备系统", "value": "消防设施（器材类）"},
    {"key": "type_label", "label": "设备分类"},
    {"key": "point_name", "label": "点位信息"},
    {"key": "code", "label": "设备编号"},
    {"key": "name", "label": "设备名称"},
    {"key": "refill_date", "label": "换粉日期"},
    {"key": "next_due_date", "label": "下次到期日"}
  ],
  "form_fields": [
    {"key": "refill_date", "label": "换粉日期", "type": "date"},
    {"key": "spec", "label": "规格（kg）", "type": "text"}
  ],
  "import_headers": ["项目名称", "设备编号", "设备名称", "点位信息", "换粉日期", "规格（kg）"]
}'::jsonb, 'enabled', now(), now()
WHERE NOT EXISTS (
  SELECT 1 FROM public.equipment_type_schema WHERE tenant_id IS NULL AND type = 'extinguisher' AND deleted_at IS NULL
);

-- 平台默认方案：消火栓箱（精细）
INSERT INTO public.equipment_type_schema (id, tenant_id, type, config, status, created_at, updated_at)
SELECT gen_random_uuid(), NULL, 'hydrant', '{
  "list_columns": [
    {"key": "code", "label": "设备编号", "width": 140},
    {"key": "name", "label": "设备名称", "width": 160},
    {"key": "point_name", "label": "点位信息", "width": 160},
    {"key": "box_config", "label": "箱体配置", "width": 200},
    {"key": "hydro_test_date", "label": "水压试验日期", "width": 130},
    {"key": "status_label", "label": "状态", "width": 90}
  ],
  "export_columns": [
    {"key": "community_name", "label": "项目名称"},
    {"key": "system", "label": "所属设备系统", "value": "消防设施（器材类）"},
    {"key": "type_label", "label": "设备分类"},
    {"key": "code", "label": "设备编号"},
    {"key": "name", "label": "设备名称"},
    {"key": "point_name", "label": "点位信息"},
    {"key": "box_config", "label": "箱体配置"},
    {"key": "hydro_test_date", "label": "水压试验日期"},
    {"key": "next_due_date", "label": "下次到期日"}
  ],
  "form_fields": [
    {"key": "box_config", "label": "箱体配置", "type": "text"},
    {"key": "hydro_test_date", "label": "水压试验日期", "type": "date"}
  ],
  "import_headers": ["项目名称", "设备编号", "设备名称", "点位信息", "箱体配置", "水压试验日期"]
}'::jsonb, 'enabled', now(), now()
WHERE NOT EXISTS (
  SELECT 1 FROM public.equipment_type_schema WHERE tenant_id IS NULL AND type = 'hydrant' AND deleted_at IS NULL
);

-- +goose Down
DROP TABLE IF EXISTS public.equipment_type_schema;
