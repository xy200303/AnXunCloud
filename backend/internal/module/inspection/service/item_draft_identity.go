package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/pkg/errs"
)

func (s *CheckinService) templateItemForDraft(task *model.InspectionTask, point *model.InspectionPoint, ref dto.CheckItemRef) (model.CheckTemplateItem, *errs.Error) {
	if ref.TemplateID == "" && ref.ItemCode == "builtin_"+model.BuiltinEquipmentDateSpot && ref.EquipmentID != nil {
		_, spots, be := s.equipmentSynthetics(task, point)
		if be != nil {
			return model.CheckTemplateItem{}, be
		}
		judge, ok := spots[model.ItemInstanceKey("", ref.ItemCode, ref.EquipmentID)]
		if !ok {
			return model.CheckTemplateItem{}, errs.ErrParam.WithMsg("该标签抽查项当前未触发")
		}
		return model.CheckTemplateItem{
			Code: ref.ItemCode, Name: "设备日期标签抽查·" + judge.Name + "(" + judge.Code + ")",
			Standard:  "读取消防/物业设备标签或钢印上的日期。reading 输出 M{生产年月}|W{维修年月}，读不到写无",
			ValueType: model.ValueOK, PhotoRequired: "required", AIPromptKind: "label", AIEnabled: true,
			BuiltinKind: model.BuiltinEquipmentDateSpot,
		}, nil
	}
	if ref.TemplateID == "" {
		return model.CheckTemplateItem{}, errs.ErrParam.WithMsg("检查项缺少模板身份")
	}
	set := model.LoadTaskTemplateSets(s.db, task, []string{point.ID})[point.ID]
	if set == nil || len(set.TemplateIDs) == 0 {
		return model.CheckTemplateItem{}, errs.ErrParam.WithMsg("该任务未配置适用的检查项模板")
	}
	allowed := set.TemplateIDs
	var item model.CheckTemplateItem
	if err := s.db.Where("template_id = ? AND code = ? AND retired_at IS NULL", ref.TemplateID, ref.ItemCode).
		Where("template_id IN ?", allowed).First(&item).Error; err != nil {
		return model.CheckTemplateItem{}, errs.ErrParam.WithMsg("检查项不属于该点位模板或已退役")
	}
	return item, nil
}

func draftPhotoHash(fileIDs []string) string {
	sum := sha256.Sum256([]byte(strings.Join(fileIDs, ",")))
	return hex.EncodeToString(sum[:])
}

func upsertItemDraft(db *gorm.DB, draft *model.CheckinItemDraft) error {
	query := db.Model(&model.CheckinItemDraft{}).Where("task_id = ? AND point_id = ? AND item_code = ?", draft.TaskID, draft.PointID, draft.ItemCode)
	if draft.TemplateID == nil {
		query = query.Where("template_id IS NULL")
	} else {
		query = query.Where("template_id = ?", *draft.TemplateID)
	}
	if draft.EquipmentID == nil {
		query = query.Where("equipment_id IS NULL")
	} else {
		query = query.Where("equipment_id = ?", *draft.EquipmentID)
	}
	var existing model.CheckinItemDraft
	if err := query.Order("created_at DESC").First(&existing).Error; err == nil {
		return db.Model(&model.CheckinItemDraft{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"tenant_id": draft.TenantID, "inspector_id": draft.InspectorID, "community_id": draft.CommunityID,
			"value_num": draft.ValueNum, "value_date": draft.ValueDate, "value_text": draft.ValueText,
			"request_id": draft.RequestID, "retake_seq": draft.RetakeSeq, "photo_set_hash": draft.PhotoSetHash,
			"draft_kind": draft.DraftKind, "job_id": draft.JobID, "file_ids": draft.FileIDs,
			"ai_status": draft.AIStatus, "ai_verdict": draft.AIVerdict, "ai_reason": draft.AIReason, "ai_reading": draft.AIReading,
			"abnormal_tags": draft.AbnormalTags, "exception_type": draft.ExceptionType,
			"quality_pass": draft.QualityPass, "quality_issue": draft.QualityIssue,
			"manual_pass": draft.ManualPass, "manual_note": draft.ManualNote,
			"shoot_lng": draft.ShootLng, "shoot_lat": draft.ShootLat, "shoot_at": draft.ShootAt, "updated_at": time.Now(),
		}).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(draft).Error
}

func draftIdentityWhere(db *gorm.DB, templateID *string, itemCode string, equipmentID *string) *gorm.DB {
	query := db.Where("item_code = ?", itemCode)
	if templateID == nil || *templateID == "" {
		query = query.Where("template_id IS NULL")
	} else {
		query = query.Where("template_id = ?", *templateID)
	}
	if equipmentID == nil || *equipmentID == "" {
		query = query.Where("equipment_id IS NULL")
	} else {
		query = query.Where("equipment_id = ?", *equipmentID)
	}
	return query
}
