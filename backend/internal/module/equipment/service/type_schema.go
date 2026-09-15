package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// ========== 类型化台账（《巡检与台账优化方案（9.14实测反馈）》§六）==========
// 一套数据（固定主字段 + extra 口袋），多套视图：按设备类型配置 列表列/导出列/表单字段/导入列头。
// 租户级方案整体覆盖平台默认；无方案的类型走通用默认集（业务默认视图，非兼容债）。

// SchemaColumn 方案列定义（list_columns/export_columns 共用）。
type SchemaColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Width int    `json:"width,omitempty"`
	// Value 固定文本列（如灭火器导出的「所属设备系统=消防设施（器材类）」）；非空时输出固定文本，不再取值
	Value string `json:"value,omitempty"`
}

// SchemaFormField 方案表单字段定义（type：text/date/number/select 等，前端按类型渲染）。
type SchemaFormField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// TypeSchemaConfig 类型字段方案 config 结构。
type TypeSchemaConfig struct {
	ListColumns   []SchemaColumn    `json:"list_columns"`
	ExportColumns []SchemaColumn    `json:"export_columns"`
	FormFields    []SchemaFormField `json:"form_fields"`
	ImportHeaders []string          `json:"import_headers"`
}

// ParseTypeSchemaConfig 从 config jsonb 解析方案结构（纯函数；字段缺失按空集）。
func ParseTypeSchemaConfig(config types.JSONMap) TypeSchemaConfig {
	var cfg TypeSchemaConfig
	if config == nil {
		return cfg
	}
	b, err := json.Marshal(config)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

// dueStateLabels 到期状态中文标签（方案导出 due_state 列转展示文案用）。
var dueStateLabels = map[string]string{
	DueNone: "无到期日", DueNormal: "正常", DueWarning: "临期",
	DueOverdue: "已逾期", DueScrap: "应报废", DueLabelMissing: "标签缺失",
}

// ResolveSchemaColumn 方案列取值（纯函数，按方案导出用）：
// 固定文本列（Value 非空）> 主字段/装配字段（toItems 已装配的 code/name/type_label/community_name/point_name 等；
// due_state 转中文标签）> extra 口袋（jsonb 读回为 any，统一转字符串）。
func ResolveSchemaColumn(item gin.H, col SchemaColumn) string {
	if col.Value != "" {
		return col.Value
	}
	if v, ok := item[col.Key]; ok && v != nil {
		if s, ok := v.(string); ok {
			if col.Key == "due_state" {
				return dueStateLabels[s]
			}
			return s
		}
		return schemaValueString(v)
	}
	extra, ok := item["extra"].(types.JSONMap)
	if !ok || extra == nil {
		return ""
	}
	return schemaValueString(extra[col.Key])
}

// schemaValueString extra 口袋值统一转字符串（整型浮点去小数尾零，布尔转 是/否）。
func schemaValueString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "是"
		}
		return "否"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// effectiveTypeSchema 取生效方案：租户级优先，缺省回落平台默认（仅 status=enabled；无方案返回 false）。
func (s *EquipmentService) effectiveTypeSchema(tenantID, typeValue string) (*model.EquipmentTypeSchema, bool) {
	if tenantID != "" {
		var row model.EquipmentTypeSchema
		if err := s.db.Where("tenant_id = ? AND type = ? AND status = ?", tenantID, typeValue, sysmodel.StatusEnabled).
			First(&row).Error; err == nil {
			return &row, true
		}
	}
	var row model.EquipmentTypeSchema
	if err := s.db.Where("tenant_id IS NULL AND type = ? AND status = ?", typeValue, sysmodel.StatusEnabled).
		First(&row).Error; err == nil {
		return &row, true
	}
	return nil, false
}

// schemaItem 方案响应装配（source：tenant=租户级 / platform=平台默认）。
func schemaItem(row *model.EquipmentTypeSchema, source string) gin.H {
	return gin.H{
		"id": row.ID, "type": row.Type, "tenant_id": row.TenantID,
		"source": source, "config": ParseTypeSchemaConfig(row.Config),
		"status": row.Status, "updated_at": timefmt.T(row.UpdatedAt),
	}
}

// ListTypeSchemas 类型方案查询：带 type 单查生效方案（无方案返回 null）；不带 type 列全部生效方案
// （租户级覆盖平台默认，按类型合并）。
func (s *EquipmentService) ListTypeSchemas(c *gin.Context, typeValue string) (any, *errs.Error) {
	tenantID, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return nil, be
	}
	if typeValue != "" {
		if row, ok := s.effectiveTypeSchema(tenantID, typeValue); ok {
			source := "platform"
			if row.TenantID != nil {
				source = "tenant"
			}
			return schemaItem(row, source), nil
		}
		return nil, nil // 无方案：前端走通用默认集
	}
	var rows []model.EquipmentTypeSchema
	if err := s.db.Where("status = ? AND (tenant_id IS NULL OR tenant_id = ?)", sysmodel.StatusEnabled, tenantID).
		Order("type ASC, tenant_id ASC NULLS FIRST").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	merged := map[string]gin.H{}
	for i := range rows {
		source := "platform"
		if rows[i].TenantID != nil {
			source = "tenant"
		}
		// NULLS FIRST 排序保证平台行先入，租户行后入覆盖
		merged[rows[i].Type] = schemaItem(&rows[i], source)
	}
	list := make([]gin.H, 0, len(merged))
	for _, item := range merged {
		list = append(list, item)
	}
	return list, nil
}

// SaveTypeSchema 租户级保存（upsert：整体替换 config；租户级没有就插，有就改）。
// 平台默认方案（tenant_id NULL）不可写——超管保存的也是其上下文租户的租户级方案。
func (s *EquipmentService) SaveTypeSchema(c *gin.Context, typeValue string, req *dto.TypeSchemaSaveReq) (gin.H, *errs.Error) {
	tenantID, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return nil, be
	}
	if tenantID == "" {
		return nil, errs.ErrParam.WithMsg("无法解析租户上下文")
	}
	// 类型须为 equipment_type 字典启用项
	var count int64
	s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = 'equipment_type' AND value = ? AND status = ?", typeValue, sysmodel.StatusEnabled).
		Count(&count)
	if count == 0 {
		return nil, errs.ErrParam.WithMsg("type 须为字典 equipment_type 的启用项")
	}
	cfg := ParseTypeSchemaConfig(types.JSONMap(req.Config))
	if be := validateTypeSchemaConfig(&cfg); be != nil {
		return nil, be
	}
	config := types.JSONMap(req.Config)
	if config == nil {
		config = types.JSONMap{}
	}
	var row model.EquipmentTypeSchema
	err := s.db.Where("tenant_id = ? AND type = ?", tenantID, typeValue).First(&row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row = model.EquipmentTypeSchema{TenantID: &tenantID, Type: typeValue, Config: config, Status: sysmodel.StatusEnabled}
		if err := s.db.Create(&row).Error; err != nil {
			return nil, errs.ErrInternal
		}
	case err != nil:
		return nil, errs.ErrInternal
	default:
		if err := s.db.Model(&row).Updates(map[string]any{"config": config, "status": sysmodel.StatusEnabled}).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	return schemaItem(&row, "tenant"), nil
}

// validateTypeSchemaConfig 方案结构校验（列/字段的 key、label 必填）。
func validateTypeSchemaConfig(cfg *TypeSchemaConfig) *errs.Error {
	checkCols := func(cols []SchemaColumn, field string) *errs.Error {
		for i, col := range cols {
			if col.Key == "" || col.Label == "" {
				return errs.ErrParam.WithMsg(fmt.Sprintf("config.%s[%d] 的 key/label 必填", field, i))
			}
		}
		return nil
	}
	if be := checkCols(cfg.ListColumns, "list_columns"); be != nil {
		return be
	}
	if be := checkCols(cfg.ExportColumns, "export_columns"); be != nil {
		return be
	}
	for i, f := range cfg.FormFields {
		if f.Key == "" || f.Label == "" {
			return errs.ErrParam.WithMsg(fmt.Sprintf("config.form_fields[%d] 的 key/label 必填", i))
		}
	}
	return nil
}
