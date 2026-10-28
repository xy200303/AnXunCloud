package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

type DocTemplateScope struct {
	Platform bool
	TenantID string
}

type DocTemplateReq struct {
	Code           string        `json:"code"`
	Name           string        `json:"name" binding:"required"`
	DocKind        string        `json:"doc_kind" binding:"required"`
	Engine         string        `json:"engine" binding:"required"`
	Format         string        `json:"format" binding:"required"`
	Layout         types.JSONMap `json:"layout"`
	StylePackID    *string       `json:"style_pack_id"`
	SkeletonFileID *string       `json:"skeleton_file_id"`
	VolumeCode     string        `json:"volume_code" binding:"required"`
	SystemFilter   string        `json:"system_filter"`
	Status         string        `json:"status"`
}

type DocTemplateService struct{ db *gorm.DB }

func NewDocTemplateService(db *gorm.DB) *DocTemplateService { return &DocTemplateService{db: db} }

var docCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func (s *DocTemplateService) List(scope DocTemplateScope) ([]gin.H, *errs.Error) {
	var tenantRows []model.DocTemplate
	if !scope.Platform {
		if err := s.db.Where("tenant_id = ?", scope.TenantID).Order("code").Find(&tenantRows).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	var platformRows []model.DocTemplate
	if err := s.db.Where("tenant_id IS NULL").Order("code").Find(&platformRows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	rows := append(tenantRows, platformRows...)
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		source := "tenant"
		if row.TenantID == nil {
			source = "platform"
		}
		items = append(items, gin.H{"id": row.ID, "code": row.Code, "name": row.Name, "doc_kind": row.DocKind, "engine": row.Engine, "format": row.Format, "layout": row.Layout, "style_pack_id": row.StylePackID, "skeleton_file_id": row.SkeletonFileID, "volume_code": row.VolumeCode, "system_filter": row.SystemFilter, "status": row.Status, "source": source, "published_at": row.PublishedAt})
	}
	return items, nil
}

func (s *DocTemplateService) Save(scope DocTemplateScope, code string, req DocTemplateReq) *errs.Error {
	if code == "" {
		code = req.Code
	}
	if !docCodePattern.MatchString(code) || strings.TrimSpace(req.Name) == "" || !docCodePattern.MatchString(req.VolumeCode) {
		return errs.ErrParam.WithMsg("模板编码、名称或册别编码无效")
	}
	if req.DocKind != "monthly_report" && req.DocKind != "acceptance" && req.DocKind != "maintenance_ledger" {
		return errs.ErrParam.WithMsg("输出文档类别无效")
	}
	if req.Engine != "ooxml" && req.Engine != "xlsx_template" {
		return errs.ErrParam.WithMsg("输出引擎无效")
	}
	if req.Format != "pdf" && req.Format != "docx" && req.Format != "xlsx" {
		return errs.ErrParam.WithMsg("输出格式无效")
	}
	if req.Status != "draft" && req.Status != "enabled" {
		return errs.ErrParam.WithMsg("模板状态只能是草稿或启用")
	}
	if req.Layout == nil {
		req.Layout = types.JSONMap{}
	}
	if be := validateLayout(req.Layout); be != nil {
		return be
	}
	if req.DocKind == "monthly_report" {
		mother, _ := req.Layout["mother"].(map[string]any)
		profile, _ := mother["profile"].(string)
		if req.SkeletonFileID == nil || *req.SkeletonFileID == "" || profile == "" {
			return errs.ErrParam.WithMsg("月报必须绑定甲方母版及母版 profile")
		}
	}
	var tenantID *string
	if !scope.Platform {
		tenantID = &scope.TenantID
	}
	row := model.DocTemplate{TenantID: tenantID, Code: code, Name: strings.TrimSpace(req.Name), DocKind: req.DocKind, Engine: req.Engine, Format: req.Format, Layout: req.Layout, StylePackID: req.StylePackID, SkeletonFileID: req.SkeletonFileID, VolumeCode: req.VolumeCode, SystemFilter: strings.TrimSpace(req.SystemFilter), Status: req.Status}
	if req.Status == "enabled" {
		now := time.Now()
		row.PublishedAt = &now
	}
	query := s.db.Where("code = ?", code)
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	var existing model.DocTemplate
	err := query.First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.ErrInternal
	}
	if err == nil {
		updates := map[string]any{
			"name": row.Name, "doc_kind": row.DocKind, "engine": row.Engine, "format": row.Format, "layout": row.Layout,
			"style_pack_id": row.StylePackID, "skeleton_file_id": row.SkeletonFileID, "volume_code": row.VolumeCode,
			"system_filter": row.SystemFilter, "status": row.Status, "published_at": row.PublishedAt,
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Create(&row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func validateLayout(layout types.JSONMap) *errs.Error {
	if _, exists := layout["sign_slots"]; !exists {
		return nil
	}
	data, _ := json.Marshal(layout["sign_slots"])
	var slots []struct {
		Code      string `json:"code"`
		Label     string `json:"label"`
		StepIndex *int   `json:"step_index"`
		Required  *bool  `json:"required"`
	}
	if json.Unmarshal(data, &slots) != nil || len(slots) == 0 {
		return errs.ErrParam.WithMsg("签字位必须绑定审批环节")
	}
	seen := map[string]bool{}
	for _, slot := range slots {
		if strings.TrimSpace(slot.Code) == "" || strings.TrimSpace(slot.Label) == "" || slot.StepIndex == nil || *slot.StepIndex < 0 || *slot.StepIndex > 4 || seen[slot.Code] {
			return errs.ErrParam.WithMsg("签字位编码、名称或审批环节无效")
		}
		seen[slot.Code] = true
	}
	return nil
}

func (s *DocTemplateService) EffectiveMonthly(tenantID string) ([]model.DocTemplate, error) {
	var tenantRows []model.DocTemplate
	if err := s.db.Where("tenant_id = ? AND doc_kind = ? AND status = ?", tenantID, "monthly_report", "enabled").Order("system_filter, code").Find(&tenantRows).Error; err != nil {
		return nil, err
	}
	var rows []model.DocTemplate
	if err := s.db.Where("tenant_id IS NULL AND doc_kind = ? AND status = ?", "monthly_report", "enabled").Order("system_filter, code").Find(&rows).Error; err != nil {
		return nil, err
	}
	indexByCode := map[string]int{}
	for i := range rows {
		indexByCode[rows[i].Code] = i
	}
	for _, row := range tenantRows {
		if index, ok := indexByCode[row.Code]; ok {
			rows[index] = row
		} else {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
