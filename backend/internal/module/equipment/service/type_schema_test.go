package service

import (
	"testing"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/pkg/types"
)

func schemaTestItem() gin.H {
	return gin.H{
		"code":             "XCCT-MH-0001",
		"name":             "1栋3楼灭火器",
		"type":             "extinguisher",
		"type_label":       "灭火器设施",
		"community_name":   "某项目",
		"point_name":       "1栋3楼通道",
		"manufacture_date": "2023-05-01",
		"next_due_date":    "2026-05-01",
		"due_state":        DueWarning,
		"status_label":     "在用",
		"extra": types.JSONMap{
			"refill_date": "2024-03-15",
			"spec":        "MFZ/ABC4",
			"quantity":    "2",
			"pressure_ok": true,
			"weight":      4.5,
		},
	}
}

func TestResolveSchemaColumn(t *testing.T) {
	item := schemaTestItem()
	cases := []struct {
		name string
		col  SchemaColumn
		want string
	}{
		{"主字段直接取", SchemaColumn{Key: "code", Label: "设备编号"}, "XCCT-MH-0001"},
		{"关联名（项目名称）", SchemaColumn{Key: "community_name", Label: "项目名称"}, "某项目"},
		{"关联名（点位信息）", SchemaColumn{Key: "point_name", Label: "点位信息"}, "1栋3楼通道"},
		{"类型 label", SchemaColumn{Key: "type_label", Label: "设备分类"}, "灭火器设施"},
		{"固定文本列优先于取值", SchemaColumn{Key: "system", Label: "所属设备系统", Value: "消防设施（器材类）"}, "消防设施（器材类）"},
		{"extra 口袋取值", SchemaColumn{Key: "refill_date", Label: "换粉日期"}, "2024-03-15"},
		{"extra 数字转字符串去尾零", SchemaColumn{Key: "weight", Label: "重量"}, "4.5"},
		{"extra 布尔转 是/否", SchemaColumn{Key: "pressure_ok", Label: "压力正常"}, "是"},
		{"due_state 转中文标签", SchemaColumn{Key: "due_state", Label: "到期状态"}, "临期"},
		{"主字段缺失回落 extra", SchemaColumn{Key: "spec", Label: "规格型号"}, "MFZ/ABC4"},
		{"主字段与 extra 均无 → 空串", SchemaColumn{Key: "not_exist", Label: "不存在"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSchemaColumn(item, tc.col); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveSchemaColumn_NoExtra(t *testing.T) {
	item := gin.H{"code": "A-001"} // 无 extra 口袋
	if got := ResolveSchemaColumn(item, SchemaColumn{Key: "refill_date"}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	// extra 为 nil JSONMap
	item["extra"] = types.JSONMap(nil)
	if got := ResolveSchemaColumn(item, SchemaColumn{Key: "refill_date"}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestParseTypeSchemaConfig(t *testing.T) {
	cfg := ParseTypeSchemaConfig(types.JSONMap{
		"list_columns":   []any{map[string]any{"key": "code", "label": "设备编号", "width": float64(140)}},
		"export_columns": []any{map[string]any{"key": "system", "label": "所属设备系统", "value": "消防设施（器材类）"}},
		"form_fields":    []any{map[string]any{"key": "refill_date", "label": "换粉日期", "type": "date"}},
		"import_headers": []any{"项目名称", "设备编号"},
	})
	if len(cfg.ListColumns) != 1 || cfg.ListColumns[0].Width != 140 {
		t.Fatalf("list_columns 解析异常：%+v", cfg.ListColumns)
	}
	if len(cfg.ExportColumns) != 1 || cfg.ExportColumns[0].Value != "消防设施（器材类）" {
		t.Fatalf("export_columns 解析异常：%+v", cfg.ExportColumns)
	}
	if len(cfg.FormFields) != 1 || cfg.FormFields[0].Type != "date" {
		t.Fatalf("form_fields 解析异常：%+v", cfg.FormFields)
	}
	if len(cfg.ImportHeaders) != 2 || cfg.ImportHeaders[0] != "项目名称" {
		t.Fatalf("import_headers 解析异常：%+v", cfg.ImportHeaders)
	}
	// nil / 空 config → 空集
	if got := ParseTypeSchemaConfig(nil); len(got.ListColumns) != 0 || len(got.ExportColumns) != 0 {
		t.Fatalf("nil config 应为空集：%+v", got)
	}
}
