package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/community/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

var presetPackCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

type PresetPackReq struct {
	Name        string        `json:"name" binding:"required"`
	Description string        `json:"description"`
	Payload     types.JSONMap `json:"payload" binding:"required"`
	Status      string        `json:"status" binding:"required"`
}

func (s *CommunityService) PresetPacks() ([]model.CommunityPresetPack, *errs.Error) {
	var rows []model.CommunityPresetPack
	if err := s.db.Select("id", "code", "name", "description", "status").Where("status = ?", "enabled").Order("code").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}

func (s *CommunityService) PresetPack(code string) (gin.H, *errs.Error) {
	var row model.CommunityPresetPack
	if err := s.db.First(&row, "code = ? AND status = ?", code, "enabled").Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return presetPackView(row), nil
}

func (s *CommunityService) PlatformPresetPacks() ([]model.CommunityPresetPack, *errs.Error) {
	var rows []model.CommunityPresetPack
	if err := s.db.Select("id", "code", "name", "description", "status", "created_at", "updated_at").Order("code").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}

func (s *CommunityService) PlatformPresetPack(code string) (gin.H, *errs.Error) {
	var row model.CommunityPresetPack
	if err := s.db.First(&row, "code = ?", code).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return presetPackView(row), nil
}

func presetPackView(row model.CommunityPresetPack) gin.H {
	return gin.H{"code": row.Code, "name": row.Name, "description": row.Description, "payload": row.Payload, "status": row.Status, "updated_at": row.UpdatedAt}
}

func (s *CommunityService) SavePresetPack(code string, req PresetPackReq) *errs.Error {
	code = strings.TrimSpace(code)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if !presetPackCodePattern.MatchString(code) || req.Name == "" || len([]rune(req.Name)) > 128 || len([]rune(req.Description)) > 500 {
		return errs.ErrParam.WithMsg("预置包编码、名称或说明格式无效")
	}
	if req.Status != "enabled" && req.Status != "disabled" {
		return errs.ErrParam.WithMsg("预置包状态无效")
	}
	if be := validatePresetPackPayload(req.Payload); be != nil {
		return be
	}
	row := model.CommunityPresetPack{Code: code, Name: req.Name, Description: req.Description, Payload: req.Payload, Status: req.Status}
	var existing model.CommunityPresetPack
	err := s.db.First(&existing, "code = ?", code).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.ErrInternal
	}
	if err == nil {
		if err := s.db.Model(&existing).Select("name", "description", "payload", "status", "updated_at").Updates(row).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Create(&row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func validatePresetPackPayload(payload types.JSONMap) *errs.Error {
	data, err := json.Marshal(payload)
	if err != nil || len(data) > 2<<20 {
		return errs.ErrParam.WithMsg("预置包内容必须是小于 2MB 的有效 JSON 对象")
	}
	count := 0
	for _, key := range []string{"point_types", "templates", "equipment_schemas", "flows", "doc_templates"} {
		value, ok := payload[key].([]any)
		if !ok {
			return errs.ErrParam.WithMsg(fmt.Sprintf("预置包缺少 %s 数组", key))
		}
		count += len(value)
		for _, raw := range value {
			if _, ok := raw.(map[string]any); !ok {
				return errs.ErrParam.WithMsg(fmt.Sprintf("%s 的每一项都必须是对象", key))
			}
		}
	}
	if count == 0 || count > 10000 {
		return errs.ErrParam.WithMsg("预置包内容不能为空，且总条目不能超过 10000")
	}
	flows := map[string]bool{}
	for _, raw := range payload["flows"].([]any) {
		code, _ := raw.(map[string]any)["flow_code"].(string)
		flows[code] = true
	}
	for _, code := range []string{"checkin_review", "maint_review", "workorder_review", "report_review", "acceptance_review"} {
		if !flows[code] {
			return errs.ErrParam.WithMsg("预置包必须包含五类审批流程")
		}
	}
	return nil
}
