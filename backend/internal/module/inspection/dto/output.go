package dto

import "anxuncloud/internal/module/inspection/model"

type TemplateDefinitionItem struct {
	Code        string                `json:"code"`
	Name        string                `json:"name"`
	Standard    string                `json:"standard"`
	StandardNo  string                `json:"standard_no"`
	ValueType   string                `json:"value_type"`
	Unit        string                `json:"unit"`
	ValueMin    *float64              `json:"value_min"`
	ValueMax    *float64              `json:"value_max"`
	EnumOptions model.EnumOptionArray `json:"enum_options"`
	Tags        []string              `json:"tags"`
}

type TemplateDefinition struct {
	ID        string                   `json:"id"`
	Code      string                   `json:"code"`
	Name      string                   `json:"name"`
	PointType string                   `json:"point_type"`
	Purpose   string                   `json:"purpose"`
	Status    string                   `json:"status"`
	Items     []TemplateDefinitionItem `json:"items"`
}
