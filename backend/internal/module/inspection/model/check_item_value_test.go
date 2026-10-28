package model

import (
	"testing"
	"time"
)

func number(v float64) *float64 { return &v }

func TestCheckItemDefinitionValidation(t *testing.T) {
	valid := CheckTemplateItem{Code: "pressure", Name: "压力", ValueType: ValueNumber, AIPromptKind: "metric", ValueMin: number(0), ValueMax: number(1)}
	if err := valid.ValidateDefinition(); err != nil {
		t.Fatalf("有效数值项被拒绝: %v", err)
	}
	invalid := valid
	invalid.ValueMin, invalid.ValueMax = number(2), number(1)
	if err := invalid.ValidateDefinition(); err == nil {
		t.Fatal("下限大于上限未被拒绝")
	}
	enum := CheckTemplateItem{Code: "intercom", Name: "对讲", ValueType: ValueEnum, AIPromptKind: "general", EnumOptions: EnumOptionArray{{Value: "ok", Label: "正常", Result: ItemResultNormal}, {Value: "bad", Label: "异常", Result: ItemResultAbnormal}}}
	if err := enum.ValidateDefinition(); err != nil {
		t.Fatalf("有效枚举项被拒绝: %v", err)
	}
	enum.EnumOptions[1].Result = "pass"
	if err := enum.ValidateDefinition(); err == nil {
		t.Fatal("非法枚举结论未被拒绝")
	}
}

func TestResultForValueUsesServerDefinition(t *testing.T) {
	numeric := CheckTemplateItem{ValueType: ValueNumber, ValueMin: number(0), ValueMax: number(10)}
	for _, tc := range []struct {
		value float64
		want  string
	}{{5, ItemResultNormal}, {-1, ItemResultAbnormal}, {11, ItemResultAbnormal}} {
		got, err := numeric.ResultForValue(ItemResultNormal, &tc.value, nil, "")
		if err != nil || got != tc.want {
			t.Fatalf("数值 %v: got %q, %v; want %q", tc.value, got, err, tc.want)
		}
	}
	enum := CheckTemplateItem{ValueType: ValueEnum, EnumOptions: EnumOptionArray{{Value: "yes", Label: "正常", Result: ItemResultNormal}, {Value: "no", Label: "异常", Result: ItemResultAbnormal}}}
	if got, err := enum.ResultForValue(ItemResultNormal, nil, nil, "no"); err != nil || got != ItemResultAbnormal {
		t.Fatalf("枚举结论未按定义归类: %q, %v", got, err)
	}
	if _, err := enum.ResultForValue(ItemResultNormal, nil, nil, "forged"); err == nil {
		t.Fatal("伪造枚举值未被拒绝")
	}
	date := CheckTemplateItem{ValueType: ValueDate}
	now := time.Now()
	if got, err := date.ResultForValue(ItemResultAbnormal, nil, &now, ""); err != nil || got != ItemResultNormal {
		t.Fatalf("日期项不应判定异常: %q, %v", got, err)
	}
}

func TestItemInstanceKeySeparatesEquipment(t *testing.T) {
	equipmentA, equipmentB := "equipment-a", "equipment-b"
	place := ItemInstanceKey("template", "item", nil)
	if place == ItemInstanceKey("template", "item", &equipmentA) || ItemInstanceKey("template", "item", &equipmentA) == ItemInstanceKey("template", "item", &equipmentB) {
		t.Fatal("场所与不同设备的项实例键发生冲突")
	}
}
