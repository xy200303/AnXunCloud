// Package types 提供 GORM JSONB 字段的自定义类型与 UUIDv7 主键基座。
package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewID 生成 UUIDv7 字符串（应用层主键，分布式/客户端生成预留；PG 15 无 uuidv7()，统一走应用层）。
func NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// IsUUIDv7 校验字符串为合法 UUIDv7（客户端打卡幂等 ID 用）。
func IsUUIDv7(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u.Version() == 7
}

// UUIDModel 主键基座：嵌入即获得 uuid 主键 + 应用层 UUIDv7 赋值（BeforeCreate）。
type UUIDModel struct {
	ID string `gorm:"type:uuid;primaryKey" json:"id"`
}

// BeforeCreate GORM 钩子：未指定 ID 时生成 UUIDv7；客户端已带入的 ID（离线补传幂等）原样保留。
func (m *UUIDModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = NewID()
	}
	return nil
}

// IDArray 映射 jsonb 的 UUID 字符串数组（如 role_ids、point_ids）。
type IDArray []string

// Value 实现 driver.Valuer。
func (a IDArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

// Scan 实现 sql.Scanner。
func (a *IDArray) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*a = IDArray{}
		return nil
	}
	return json.Unmarshal(data, a)
}

// Contains 判断数组是否包含指定值。
func (a IDArray) Contains(v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

// StringArray 映射 jsonb 字符串数组（如 required_photo_items）。
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *StringArray) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*a = StringArray{}
		return nil
	}
	return json.Unmarshal(data, a)
}

// JSONMap 映射 jsonb 对象（如 cycle_config）。
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *JSONMap) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*m = JSONMap{}
		return nil
	}
	return json.Unmarshal(data, m)
}

// Ints 从 JSONMap 读取整数数组字段（如 weekdays/days）。
func (m JSONMap) Ints(key string) []int {
	v, ok := m[key]
	if !ok {
		return nil
	}
	// 兼容两种来源：DB 读回（[]any/float64）与进程内直接构造（[]int，如演示种子/测试）
	if ints, ok := v.([]int); ok {
		return ints
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(arr))
	for _, x := range arr {
		if f, ok := x.(float64); ok {
			out = append(out, int(f))
		}
	}
	return out
}

// Int 从 JSONMap 读取整数字段。
func (m JSONMap) Int(key string) int {
	if f, ok := m[key].(float64); ok {
		return int(f)
	}
	return 0
}

// Float 从 JSONMap 读取浮点字段。
func (m JSONMap) Float(key string) float64 {
	if f, ok := m[key].(float64); ok {
		return f
	}
	return 0
}

// 模板项拍照要求取值（check_template_item / checkin_record_item 的 photo_required 列）。
const (
	PhotoReqNone     = "none"     // 不要求拍照
	PhotoReqOptional = "optional" // 可拍可不拍
	PhotoReqRequired = "required" // 必拍（合格也须 ≥1 张该项照片）
)

// SignEntry 报告签字留痕（inspection_report.inspector_signed JSONB）。
type SignEntry struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	SignedAt string `json:"signed_at"`
	// SignatureFileID 签字时的手写签名图快照（空=未设置签名，PDF 回退打印姓名）
	SignatureFileID string `json:"signature_file_id,omitempty"`
	// AssetID 签字时的签章资产 id（v16 起；可空，便于法律追溯定位版本）
	AssetID string `json:"asset_id,omitempty"`
	// 代签留痕（三字段同时非空表示代签：UserID/Name 为被代签巡检员，签名图取代签人本人资产——代签人对该次确认负责）
	ProxyBy     string `json:"proxy_by,omitempty"`
	ProxyName   string `json:"proxy_name,omitempty"`
	ProxyReason string `json:"proxy_reason,omitempty"`
}

// SignArray 映射 jsonb 签字记录数组。
type SignArray []SignEntry

func (a SignArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *SignArray) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*a = SignArray{}
		return nil
	}
	return json.Unmarshal(data, a)
}

// AttachmentItem 公告附件元素（sys_notice.attachments JSONB）。
type AttachmentItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// AttachmentArray 映射 jsonb 附件数组（sys_notice.attachments）。
type AttachmentArray []AttachmentItem

func (a AttachmentArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *AttachmentArray) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*a = AttachmentArray{}
		return nil
	}
	return json.Unmarshal(data, a)
}

func toBytes(src any) ([]byte, error) {
	if src == nil {
		return nil, nil
	}
	switch v := src.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, errors.New("types: unsupported source type")
	}
}

// FlowStep 审批链环节（approval_flow.steps JSONB；slot 引用职责槽位 code，名单解析复用槽位体系）。
type FlowStep struct {
	Slot string `json:"slot"`
	Name string `json:"name"`
}

// FlowStepArray 映射 jsonb 审批链环节数组。
type FlowStepArray []FlowStep

func (a FlowStepArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *FlowStepArray) Scan(src any) error {
	data, err := toBytes(src)
	if err != nil {
		return err
	}
	if data == nil {
		*a = FlowStepArray{}
		return nil
	}
	return json.Unmarshal(data, a)
}
