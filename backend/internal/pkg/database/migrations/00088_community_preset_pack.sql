-- +goose Up
-- owner: community
CREATE TABLE community_preset_pack (
    id uuid PRIMARY KEY,
    code varchar(64) NOT NULL UNIQUE,
    name varchar(128) NOT NULL,
    description varchar(500) NOT NULL DEFAULT '',
    payload jsonb NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'enabled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE community_preset_pack IS '平台发布的新项目上线预置包；租户导入按层预览冲突，不静默覆盖';

INSERT INTO community_preset_pack (id, code, name, description, payload, status) VALUES (
'90000000-0000-0000-0000-000000000088',
'property_fire_baseline',
'物业消防基础预置包',
'点位类型、检查项模板、设备字段、审批流程与月报输出样式。导入只处理差异，不覆盖已有配置。',
'{
  "point_types": [
    {"type_code":"point_type","label":"消火栓箱","value":"fire_hydrant_box","sort":10,"status":1,"attrs":{"level":"place"}},
    {"type_code":"point_type","label":"消防泵房","value":"fire_pump_room","sort":20,"status":1,"attrs":{"level":"place"}},
    {"type_code":"point_type","label":"消防控制室","value":"control_room","sort":30,"status":1,"attrs":{"level":"place"}}
  ],
  "templates": [
    {"point_type":"fire_hydrant_box","template_name":"消火栓箱日常巡检","purpose":"patrol","item_code":"box_visual","item_name":"箱体与器材外观","value_type":"ok","standard":"箱体、水带、枪头齐全无破损","photo_required":"必拍","note_required":"否","ai_prompt_kind":"damage","ai_enabled":"是","tags":"水带缺失,枪头缺失","guide":"打开箱门拍全景","standard_no":"1","ai_hint":"核对箱体、水带和枪头"},
    {"point_type":"fire_hydrant_box","template_name":"消火栓箱日常巡检","purpose":"patrol","item_code":"water_pressure","item_name":"静水压力","value_type":"number","standard":"压力在允许区间","photo_required":"选拍","unit":"MPa","value_min":"0.1","value_max":"1.6","note_required":"否","ai_prompt_kind":"metric","ai_enabled":"是","guide":"拍压力表读数","standard_no":"2","ai_hint":"读取压力表"},
    {"point_type":"fire_hydrant_box","template_name":"消火栓箱日常巡检","purpose":"patrol","item_code":"channel_clear","item_name":"栓箱通道","value_type":"ok","standard":"通道无遮挡","photo_required":"必拍","note_required":"否","ai_prompt_kind":"passage","ai_enabled":"是","guide":"拍栓箱正面及通道","standard_no":"3","ai_hint":"检查遮挡堆物"},
    {"point_type":"fire_pump_room","template_name":"消防泵房日常巡检","purpose":"patrol","item_code":"pump_visual","item_name":"泵体与管路","value_type":"ok","standard":"无锈蚀、无渗漏","photo_required":"必拍","note_required":"否","ai_prompt_kind":"leak","ai_enabled":"是","guide":"拍泵体和管路","standard_no":"1","ai_hint":"检查锈蚀与渗漏"},
    {"point_type":"fire_pump_room","template_name":"消防泵房日常巡检","purpose":"patrol","item_code":"pump_state","item_name":"泵组运行状态","value_type":"enum","standard":"运行状态正常","photo_required":"选拍","enum_options":"正常=normal;异常=abnormal","note_required":"否","ai_prompt_kind":"state","ai_enabled":"是","guide":"拍控制柜状态","standard_no":"2","ai_hint":"识别运行状态"},
    {"point_type":"fire_pump_room","template_name":"消防泵房日常巡检","purpose":"patrol","item_code":"inspection_date","item_name":"巡检日期","value_type":"date","standard":"填写实际巡检日期","photo_required":"无需拍照","note_required":"否","ai_prompt_kind":"general","ai_enabled":"否","guide":"","standard_no":"3","ai_hint":""}
  ],
  "equipment_schemas": [
    {"type":"fire_extinguisher","config":{"list_columns":[{"key":"code","label":"设备编号","width":150},{"key":"name","label":"设备名称","width":170},{"key":"next_due_date","label":"下次到期日","width":130}],"export_columns":[{"key":"code","label":"设备编号"},{"key":"name","label":"设备名称"},{"key":"manufacture_date","label":"生产日期"},{"key":"next_due_date","label":"下次到期日"}],"form_fields":[{"key":"pressure","label":"压力","type":"text"}],"import_headers":["设备编号","设备名称","生产日期","下次到期日"]}}
  ],
  "flows": [
    {"flow_code":"checkin_review","steps":[{"role_code":"quality_supervisor","name":"打卡复核","mode":"any"}]},
    {"flow_code":"maint_review","steps":[{"role_code":"quality_supervisor","name":"维保审核","mode":"any"}]},
    {"flow_code":"workorder_review","steps":[{"role_code":"project_manager","name":"工单审核","mode":"any"}]},
    {"flow_code":"report_review","steps":[{"role_code":"quality_supervisor","name":"品质审核","mode":"any"},{"role_code":"project_manager","name":"项目审核","mode":"any"},{"role_code":"tenant_admin","name":"租户归档","mode":"any"}]},
    {"flow_code":"acceptance_review","steps":[{"role_code":"quality_supervisor","name":"承接查验审核","mode":"any"}]}
  ],
  "doc_templates": [
    {"code":"monthly_fire_baseline","name":"消防巡检月报基础样式","doc_kind":"monthly_report","engine":"ooxml","format":"docx","volume_code":"fire_equipment","system_filter":"","status":"enabled","layout":{"blocks":[{"type":"cover"},{"type":"summary_table"},{"type":"sign"}],"sign_slots":[{"code":"quality","label":"品质审核","step_index":0,"required":true},{"code":"manager","label":"项目审核","step_index":1,"required":true},{"code":"archive","label":"租户归档","step_index":2,"required":true}]}}
  ]
}', 'enabled');

-- +goose Down
SELECT 1;
