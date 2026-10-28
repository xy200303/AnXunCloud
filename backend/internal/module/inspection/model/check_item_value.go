package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ValueOK     = "ok"
	ValueNumber = "number"
	ValueDate   = "date"
	ValueText   = "text"
	ValueEnum   = "enum"

	BuiltinEquipmentValidity = "equipment_validity"
	BuiltinEquipmentDateSpot = "equipment_date_spot"
)

type EnumOption struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Result string `json:"result"`
}

type EnumOptionArray []EnumOption

func (a EnumOptionArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	data, err := json.Marshal(a)
	return string(data), err
}

func (a *EnumOptionArray) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case nil:
		*a = EnumOptionArray{}
		return nil
	default:
		return fmt.Errorf("enum_options: unsupported source %T", src)
	}
	return json.Unmarshal(data, a)
}

var checkItemCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func ValidCheckItemCode(code string) bool {
	return checkItemCodePattern.MatchString(code)
}

func ValidAIPromptKind(kind string) bool {
	switch kind {
	case "general", "presence", "damage", "metric", "state", "label", "passage", "leak", "indicator", "tidiness", "baseline":
		return true
	}
	return false
}

func (i *CheckTemplateItem) ValidateDefinition() error {
	if !ValidCheckItemCode(i.Code) {
		return fmt.Errorf("检查项编码须为 3~64 位小写字母、数字或下划线，且以字母开头")
	}
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("检查项名称不能为空")
	}
	if i.BuiltinKind != "" {
		return fmt.Errorf("系统内置检查项不能通过模板接口配置")
	}
	if !ValidAIPromptKind(i.AIPromptKind) {
		return fmt.Errorf("AI 提示类型无效")
	}
	switch i.ValueType {
	case ValueNumber:
		if i.ValueMin != nil && i.ValueMax != nil && *i.ValueMin > *i.ValueMax {
			return fmt.Errorf("数值下限不能大于上限")
		}
	case ValueEnum:
		if len(i.EnumOptions) < 2 {
			return fmt.Errorf("枚举检查项至少需要两个选项")
		}
		seen := map[string]bool{}
		for _, option := range i.EnumOptions {
			option.Value = strings.TrimSpace(option.Value)
			option.Label = strings.TrimSpace(option.Label)
			if option.Value == "" || option.Label == "" || seen[option.Value] {
				return fmt.Errorf("枚举选项值不能为空或重复")
			}
			if option.Result != ItemResultNormal && option.Result != ItemResultAbnormal {
				return fmt.Errorf("枚举选项结论只能是 normal 或 abnormal")
			}
			seen[option.Value] = true
		}
	case ValueOK, ValueDate, ValueText:
		if i.Unit != "" || i.ValueMin != nil || i.ValueMax != nil || len(i.EnumOptions) != 0 {
			return fmt.Errorf("当前值类型不能配置单位、数值区间或枚举选项")
		}
	default:
		return fmt.Errorf("值类型只能是 ok、number、date、text 或 enum")
	}
	if i.ValueType != ValueEnum && len(i.EnumOptions) != 0 {
		return fmt.Errorf("仅枚举检查项可配置选项")
	}
	if i.ValueType != ValueOK && i.ValueType != ValueEnum && len(i.Tags) != 0 {
		return fmt.Errorf("观察点仅适用于 ok 或 enum 检查项")
	}
	return nil
}

func (i *CheckTemplateItem) ResultForValue(result string, valueNum *float64, valueDate *time.Time, valueText string) (string, error) {
	if result == ItemResultEscaped {
		return result, nil
	}
	switch i.ValueType {
	case ValueOK:
		if result != ItemResultNormal && result != ItemResultAbnormal {
			return "", fmt.Errorf("请选择正常或异常")
		}
		return result, nil
	case ValueNumber:
		if valueNum == nil {
			return "", fmt.Errorf("请填写数值")
		}
		if (i.ValueMin != nil && *valueNum < *i.ValueMin) || (i.ValueMax != nil && *valueNum > *i.ValueMax) {
			return ItemResultAbnormal, nil
		}
		return ItemResultNormal, nil
	case ValueDate:
		if valueDate == nil {
			return "", fmt.Errorf("请选择日期")
		}
		return ItemResultNormal, nil
	case ValueText:
		if strings.TrimSpace(valueText) == "" {
			return "", fmt.Errorf("请填写文本")
		}
		return ItemResultNormal, nil
	case ValueEnum:
		for _, option := range i.EnumOptions {
			if option.Value == valueText {
				return option.Result, nil
			}
		}
		return "", fmt.Errorf("请选择有效的枚举选项")
	default:
		return "", fmt.Errorf("检查项值类型无效")
	}
}

func ItemInstanceKey(templateID, itemCode string, equipmentID *string) string {
	key := templateID + ":" + itemCode + ":"
	if equipmentID != nil {
		key += *equipmentID
	}
	return key
}
