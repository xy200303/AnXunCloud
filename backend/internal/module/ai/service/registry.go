package service

import (
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

type Scope struct {
	Name string
	ID   string
}

type ProviderReq struct {
	Code          string `json:"code"`
	Name          string `json:"name" binding:"required"`
	Protocol      string `json:"protocol" binding:"required"`
	BaseURL       string `json:"base_url" binding:"required"`
	CredentialRef string `json:"credential_ref" binding:"required"`
	Status        string `json:"status"`
}

type ModelReq struct {
	DisplayName   string   `json:"display_name" binding:"required"`
	ContextWindow int      `json:"context_window" binding:"required"`
	MaxOutput     int      `json:"max_output" binding:"required"`
	Capabilities  []string `json:"capabilities" binding:"required"`
	Status        string   `json:"status"`
}

type BindingReq struct {
	ProviderCode   string `json:"provider_code" binding:"required"`
	ModelID        string `json:"model_id" binding:"required"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	MaxRetries     int    `json:"max_retries"`
	CustomPrompt   string `json:"custom_prompt"`
	Status         string `json:"status"`
}

type RegistryService struct{ db *gorm.DB }

func NewRegistryService(db *gorm.DB) *RegistryService { return &RegistryService{db: db} }

var businessCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
var credentialPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`)
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func validStatus(value string) bool { return value == model.Enabled || value == model.Disabled }

func (s *RegistryService) Providers(scope Scope) ([]gin.H, *errs.Error) {
	rows, err := resolveRows[model.Provider](s.db, scope, "code")
	if err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(rows))
	for _, p := range rows {
		items = append(items, gin.H{"code": p.Code, "name": p.Name, "protocol": p.Protocol, "base_url": p.BaseURL, "credential_ref": p.CredentialRef, "status": p.Status, "source": p.Scope})
	}
	return items, nil
}

func (s *RegistryService) Models(scope Scope) ([]gin.H, *errs.Error) {
	rows, err := resolveRows[model.Model](s.db, scope, "provider_code, model_id")
	if err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(rows))
	for _, m := range rows {
		items = append(items, gin.H{"provider_code": m.ProviderCode, "model_id": m.ModelID, "display_name": m.DisplayName, "context_window": m.ContextWindow, "max_output": m.MaxOutput, "capabilities": m.Capabilities, "status": m.Status, "source": m.Scope})
	}
	return items, nil
}

func (s *RegistryService) Bindings(scope Scope) ([]gin.H, *errs.Error) {
	rows, err := resolveRows[model.TaskBinding](s.db, scope, "task_type")
	if err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(rows))
	for _, b := range rows {
		items = append(items, gin.H{"task_type": b.TaskType, "provider_code": b.ProviderCode, "model_id": b.ModelID, "timeout_seconds": b.TimeoutSeconds, "max_retries": b.MaxRetries, "custom_prompt": b.CustomPrompt, "status": b.Status, "source": b.Scope})
	}
	return items, nil
}

func resolveRows[T any](db *gorm.DB, scope Scope, order string) ([]T, error) {
	query := db.Model(new(T))
	if scope.Name == model.ScopeTenant {
		query = query.Where("scope = ? AND scope_id = ?", model.ScopeTenant, scope.ID)
	} else {
		query = query.Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID)
	}
	var rows []T
	if err := query.Order(order).Find(&rows).Error; err != nil {
		return nil, err
	}
	if scope.Name == model.ScopePlatform {
		return rows, nil
	}
	keyOf := func(value T) string {
		switch row := any(value).(type) {
		case model.Provider:
			return row.Code
		case model.Model:
			return row.ProviderCode + "\x00" + row.ModelID
		case model.TaskBinding:
			return row.TaskType
		}
		return ""
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		seen[keyOf(row)] = true
	}
	var inherited []T
	if err := db.Model(new(T)).Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID).Order(order).Find(&inherited).Error; err != nil {
		return nil, err
	}
	for _, row := range inherited {
		if !seen[keyOf(row)] {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (s *RegistryService) SaveProvider(scope Scope, code string, req ProviderReq) *errs.Error {
	if code == "" {
		code = req.Code
	}
	if !businessCodePattern.MatchString(code) || strings.TrimSpace(req.Name) == "" || !credentialPattern.MatchString(req.CredentialRef) || !validStatus(req.Status) {
		return errs.ErrParam.WithMsg("供应商编码、名称、状态或环境变量引用无效")
	}
	switch req.Protocol {
	case "openai_chat", "openai_responses", "gemini", "claude":
	default:
		return errs.ErrParam.WithMsg("AI 接口协议无效")
	}
	parsed, err := url.ParseRequestURI(req.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errs.ErrParam.WithMsg("接口地址必须是有效的 HTTP(S) 地址")
	}
	row := model.Provider{Scope: scope.Name, ScopeID: scope.ID, Code: code, Name: strings.TrimSpace(req.Name), Protocol: req.Protocol, BaseURL: strings.TrimRight(req.BaseURL, "/"), CredentialRef: req.CredentialRef, Status: req.Status}
	return saveRow(s.db, &row, "scope = ? AND scope_id = ? AND code = ?", scope.Name, scope.ID, code)
}

func (s *RegistryService) SaveModel(scope Scope, providerCode, modelID string, req ModelReq) *errs.Error {
	if !businessCodePattern.MatchString(providerCode) || !modelIDPattern.MatchString(modelID) || req.ContextWindow <= 0 || req.MaxOutput <= 0 || !validStatus(req.Status) {
		return errs.ErrParam.WithMsg("模型标识、能力参数或状态无效")
	}
	providers, be := s.Providers(scope)
	if be != nil {
		return be
	}
	found := false
	for _, item := range providers {
		if item["code"] == providerCode {
			found = true
			break
		}
	}
	if !found {
		return errs.ErrParam.WithMsg("供应商不存在，请先注册供应商")
	}
	allowed := map[string]bool{"vision": true, "text": true, "tools": true}
	seen := map[string]bool{}
	capabilities := types.StringArray{}
	for _, capability := range req.Capabilities {
		if !allowed[capability] || seen[capability] {
			return errs.ErrParam.WithMsg("模型能力标记无效")
		}
		seen[capability] = true
		capabilities = append(capabilities, capability)
	}
	if len(capabilities) == 0 {
		return errs.ErrParam.WithMsg("至少选择一种模型能力")
	}
	row := model.Model{Scope: scope.Name, ScopeID: scope.ID, ProviderCode: providerCode, ModelID: modelID, DisplayName: strings.TrimSpace(req.DisplayName), ContextWindow: req.ContextWindow, MaxOutput: req.MaxOutput, Capabilities: capabilities, Status: req.Status}
	return saveRow(s.db, &row, "scope = ? AND scope_id = ? AND provider_code = ? AND model_id = ?", scope.Name, scope.ID, providerCode, modelID)
}

func (s *RegistryService) SaveBinding(scope Scope, taskType string, req BindingReq) *errs.Error {
	required, ok := taskCapabilities()[taskType]
	if !ok || req.TimeoutSeconds < 1 || req.TimeoutSeconds > 300 || req.MaxRetries < 0 || req.MaxRetries > 3 || len(req.CustomPrompt) > 8000 || !validStatus(req.Status) {
		return errs.ErrParam.WithMsg("任务类型、超时、重试次数或状态无效")
	}
	providers, be := s.Providers(scope)
	if be != nil {
		return be
	}
	providerEnabled := false
	for _, provider := range providers {
		if provider["code"] == req.ProviderCode && provider["status"] == model.Enabled {
			providerEnabled = true
			break
		}
	}
	if !providerEnabled {
		return errs.ErrParam.WithMsg("所选供应商不存在或已停用")
	}
	models, be := s.Models(scope)
	if be != nil {
		return be
	}
	found := false
	for _, item := range models {
		if item["provider_code"] != req.ProviderCode || item["model_id"] != req.ModelID || item["status"] != model.Enabled {
			continue
		}
		capabilities, _ := item["capabilities"].(types.StringArray)
		for _, capability := range required {
			if !types.IDArray(capabilities).Contains(capability) {
				return errs.ErrParam.WithMsg("所选模型不具备该任务必需的 " + capability + " 能力")
			}
		}
		found = true
		break
	}
	if !found {
		return errs.ErrParam.WithMsg("所选模型不存在或已停用")
	}
	row := model.TaskBinding{Scope: scope.Name, ScopeID: scope.ID, TaskType: taskType, ProviderCode: req.ProviderCode, ModelID: req.ModelID, TimeoutSeconds: req.TimeoutSeconds, MaxRetries: req.MaxRetries, CustomPrompt: strings.TrimSpace(req.CustomPrompt), Status: req.Status}
	return saveRow(s.db, &row, "scope = ? AND scope_id = ? AND task_type = ?", scope.Name, scope.ID, taskType)
}

func saveRow(db *gorm.DB, row any, query string, args ...any) *errs.Error {
	var count int64
	if err := db.Model(row).Where(query, args...).Count(&count).Error; err != nil {
		return errs.ErrInternal
	}
	if count > 0 {
		if err := db.Model(row).Where(query, args...).Select("*").Updates(row).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := db.Create(row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func taskCapabilities() map[string][]string {
	return map[string][]string{
		"image_inspection": {"vision", "text"}, "meter_reading": {"vision", "text"},
		"label_date_ocr": {"vision", "text"}, "doc_scan_extract": {"vision", "text"},
		"assistant": {"text", "tools"}, "agent_review": {"text", "tools"},
	}
}

func TaskCatalog() []gin.H {
	labels := map[string]string{"image_inspection": "照片检查", "meter_reading": "表计读数", "label_date_ocr": "标签日期识别", "doc_scan_extract": "资料扫描提取", "assistant": "智能助手", "agent_review": "Agent 预审"}
	items := make([]gin.H, 0, len(taskCapabilities()))
	for task, capabilities := range taskCapabilities() {
		items = append(items, gin.H{"task_type": task, "name": labels[task], "required_capabilities": capabilities})
	}
	return items
}

type ResolvedTask struct {
	Provider model.Provider
	Model    model.Model
	Binding  model.TaskBinding
	APIKey   string
}

func (s *RegistryService) ResolveTask(scope Scope, taskType string) (*ResolvedTask, *errs.Error) {
	bindings, err := resolveRows[model.TaskBinding](s.db, scope, "task_type")
	if err != nil {
		return nil, errs.ErrInternal
	}
	var selected *model.TaskBinding
	for i := range bindings {
		if bindings[i].TaskType == taskType && bindings[i].Status == model.Enabled {
			selected = &bindings[i]
			break
		}
	}
	if selected == nil {
		return nil, errs.ErrParam.WithMsg("该 AI 能力未配置或未启用")
	}
	providers, err := resolveRows[model.Provider](s.db, scope, "code")
	if err != nil {
		return nil, errs.ErrInternal
	}
	models, err := resolveRows[model.Model](s.db, scope, "provider_code, model_id")
	if err != nil {
		return nil, errs.ErrInternal
	}
	resolved := &ResolvedTask{Binding: *selected}
	for _, provider := range providers {
		if provider.Code == selected.ProviderCode && provider.Status == model.Enabled {
			resolved.Provider = provider
			break
		}
	}
	for _, item := range models {
		if item.ProviderCode == selected.ProviderCode && item.ModelID == selected.ModelID && item.Status == model.Enabled {
			resolved.Model = item
			break
		}
	}
	if resolved.Provider.ID == "" || resolved.Model.ID == "" {
		return nil, errs.ErrParam.WithMsg("绑定的供应商或模型不可用")
	}
	for _, capability := range taskCapabilities()[taskType] {
		if !types.IDArray(resolved.Model.Capabilities).Contains(capability) {
			return nil, errs.ErrParam.WithMsg("绑定模型缺少必需能力")
		}
	}
	resolved.APIKey = os.Getenv(resolved.Provider.CredentialRef)
	if resolved.APIKey == "" {
		return nil, errs.ErrParam.WithMsg("模型密钥环境变量未配置")
	}
	return resolved, nil
}
