package dto

type TemplateImportRow struct {
	Row           int    `json:"row"`
	PointType     string `json:"point_type"`
	TemplateName  string `json:"template_name"`
	Purpose       string `json:"purpose"`
	ItemCode      string `json:"item_code"`
	ItemName      string `json:"item_name"`
	ValueType     string `json:"value_type"`
	Standard      string `json:"standard"`
	PhotoRequired string `json:"photo_required"`
	Unit          string `json:"unit"`
	ValueMin      string `json:"value_min"`
	ValueMax      string `json:"value_max"`
	EnumOptions   string `json:"enum_options"`
	NoteRequired  string `json:"note_required"`
	AIPromptKind  string `json:"ai_prompt_kind"`
	AIEnabled     string `json:"ai_enabled"`
	Tags          string `json:"tags"`
	Guide         string `json:"guide"`
	StandardNo    string `json:"standard_no"`
	AIHint        string `json:"ai_hint"`
}

type TemplateImportApplyReq struct {
	Rows []TemplateImportRow `json:"rows" binding:"required,min=1,max=500"`
}

type TemplateImportFailureRow struct {
	TemplateImportRow
	Errors []string `json:"errors"`
}

type TemplateImportFailureExportReq struct {
	Rows []TemplateImportFailureRow `json:"rows" binding:"required,min=1,max=500"`
}
