package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

const maintenanceBatchLimit = 500

type MaintenanceBatchItemReq struct {
	EquipmentID string   `json:"equipment_id"`
	Note        string   `json:"note"`
	FileIDs     []string `json:"file_ids"`
}

type MaintenanceBatchPreviewReq struct {
	CommunityID     string                    `json:"community_id"`
	EquipmentType   string                    `json:"equipment_type"`
	MaintenanceType string                    `json:"maintenance_type"`
	MaintenanceDate string                    `json:"maintenance_date"`
	IDs             []string                  `json:"ids"`
	Status          string                    `json:"status"`
	DueState        string                    `json:"due_state"`
	PointID         string                    `json:"point_id"`
	Keyword         string                    `json:"keyword"`
	ExcludedIDs     []string                  `json:"excluded_ids"`
	OperationID     string                    `json:"operation_id"`
	PayloadHash     string                    `json:"payload_hash"`
	OperatorName    string                    `json:"operator_name"`
	Vendor          string                    `json:"vendor"`
	Note            string                    `json:"note"`
	FileIDs         []string                  `json:"file_ids"`
	Items           []MaintenanceBatchItemReq `json:"items"`
}

func (s *MaintenanceService) PreviewMaintenanceBatch(c *gin.Context, req *MaintenanceBatchPreviewReq) (gin.H, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	if be := s.validateBatchRequest(c, req); be != nil {
		return nil, be
	}
	ids, be := s.batchCandidateIDs(c, req)
	if be != nil {
		return nil, be
	}
	candidates := map[string]bool{}
	for _, id := range ids {
		candidates[id] = true
	}
	excluded := map[string]bool{}
	for _, id := range req.ExcludedIDs {
		if !candidates[id] {
			return nil, errs.ErrParam.WithMsg("excluded_ids 包含不属于快照的设备")
		}
		excluded[id] = true
	}
	selected := make([]string, 0, len(ids))
	for _, id := range ids {
		if !excluded[id] {
			selected = append(selected, id)
		}
	}
	if len(selected) == 0 || len(selected) > maintenanceBatchLimit {
		return nil, errs.ErrParam.WithMsg("批量维保须选择 1-500 台设备")
	}
	hash := maintenanceBatchHash(req)
	if req.PayloadHash != hash {
		return nil, errs.ErrParam.WithMsg("payload_hash 与当前批量请求不一致")
	}
	var existing model.MaintenanceSelection
	err := s.db.Where("tenant_id = ? AND actor_id = ? AND action = ? AND operation_id = ?", identity.TenantID, identity.UserID, "maintenance_batch_register", req.OperationID).First(&existing).Error
	if err == nil {
		if existing.PayloadHash != hash {
			return nil, errs.ErrConflict.WithMsg("operation_id 已用于其他批量请求")
		}
		return gin.H{"selection_token": existing.Token, "count": len(selected), "payload_hash": hash, "expires_at": existing.ExpiresAt}, nil
	}
	token := types.NewID()
	selection := model.MaintenanceSelection{Token: token, TenantID: identity.TenantID, ActorID: identity.UserID,
		Action: "maintenance_batch_register", OperationID: req.OperationID, PayloadHash: hash,
		EquipmentIDs: types.StringArray(selected), ExcludedIDs: types.StringArray(req.ExcludedIDs),
		Filter: batchRequestMap(req), ExpiresAt: time.Now().Add(10 * time.Minute)}
	if err := s.db.Create(&selection).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"selection_token": token, "count": len(selected), "payload_hash": hash, "expires_at": selection.ExpiresAt}, nil
}

func (s *MaintenanceService) validateBatchRequest(c *gin.Context, req *MaintenanceBatchPreviewReq) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil || identity.TenantID == "" || req.CommunityID == "" || req.EquipmentType == "" {
		return errs.ErrDataScope
	}
	if req.MaintenanceType != model.MaintenanceRefill && req.MaintenanceType != model.MaintenanceHydroTest {
		return errs.ErrParam.WithMsg("批量登记仅支持换粉或水压试验")
	}
	if req.OperationID == "" || req.PayloadHash == "" || req.MaintenanceDate == "" {
		return errs.ErrParam.WithMsg("operation_id、payload_hash 和施工日期不能为空")
	}
	if _, be := parseDate(req.MaintenanceDate); be != nil {
		return be
	}
	if len(req.IDs) > 0 && (req.Status != "" || req.DueState != "" || req.PointID != "" || req.Keyword != "") {
		return errs.ErrParam.WithMsg("手选 ids 与筛选全选不能混用")
	}
	if !identity.ScopeSelf {
		return middleware.CheckCommunity(s.db, c, req.CommunityID)
	}
	return nil
}

func (s *MaintenanceService) batchCandidateIDs(c *gin.Context, req *MaintenanceBatchPreviewReq) ([]string, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	query := s.db.Model(&model.Equipment{}).Where("tenant_id = ? AND community_id = ? AND type = ? AND deleted_at IS NULL", identity.TenantID, req.CommunityID, req.EquipmentType)
	if len(req.IDs) > 0 {
		query = query.Where("id IN ?", req.IDs)
	} else {
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		if req.DueState != "" {
			query = query.Where("next_due_date IS NOT NULL")
			if req.DueState == "overdue" {
				query = query.Where("next_due_date < CURRENT_DATE")
			} else if req.DueState == "warning" {
				query = query.Where("next_due_date >= CURRENT_DATE AND next_due_date <= CURRENT_DATE + 30")
			} else if req.DueState == "normal" {
				query = query.Where("next_due_date > CURRENT_DATE + 30")
			}
		}
		if req.PointID != "" {
			query = query.Where("point_id = ?", req.PointID)
		}
		if req.Keyword != "" {
			query = query.Where("code ILIKE ? OR name ILIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
		}
	}
	var ids []string
	if err := query.Order("id").Limit(maintenanceBatchLimit+1).Pluck("id", &ids).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(ids) > maintenanceBatchLimit {
		return nil, errs.ErrParam.WithMsg("筛选结果超过 500 台，请缩小范围")
	}
	return ids, nil
}

func (s *MaintenanceService) CreateMaintenanceBatch(c *gin.Context, selectionToken string) (gin.H, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	var selection model.MaintenanceSelection
	if err := s.db.First(&selection, "token = ? AND actor_id = ? AND tenant_id = ? AND action = ?", selectionToken, identity.UserID, identity.TenantID, "maintenance_batch_register").Error; err != nil {
		return nil, errs.ErrDataScope
	}
	if time.Now().After(selection.ExpiresAt) {
		return nil, errs.ErrConflict.WithMsg("selection_token 已过期，请重新预览")
	}
	var req MaintenanceBatchPreviewReq
	if err := json.Unmarshal(batchRequestMapBytes(selection.Filter), &req); err != nil {
		return nil, errs.ErrInternal
	}
	if maintenanceBatchHash(&req) != selection.PayloadHash {
		return nil, errs.ErrConflict.WithMsg("批量请求内容已变化，请重新预览")
	}
	if be := s.validateBatchRequest(c, &req); be != nil {
		return nil, be
	}
	var batch model.MaintenanceBatch
	if err := s.db.Where("tenant_id = ? AND created_by = ? AND operation_id = ?", identity.TenantID, identity.UserID, req.OperationID).First(&batch).Error; err == nil {
		if batch.PayloadHash != selection.PayloadHash {
			return nil, errs.ErrConflict.WithMsg("operation_id 已用于其他登记单")
		}
		return gin.H{"id": batch.ID, "status": batch.Status, "count": len(selection.EquipmentIDs)}, nil
	}
	equipments, be := s.recheckBatchEquipment(c, &req, selection.EquipmentIDs)
	if be != nil {
		return nil, be
	}
	maintDate, _ := parseDate(req.MaintenanceDate)
	var duplicate int64
	if err := s.db.Model(&model.MaintenanceBatchItem{}).
		Joins("JOIN maintenance_batch b ON b.id = maintenance_batch_item.batch_id").
		Where("maintenance_batch_item.equipment_id IN ? AND b.maintenance_type = ? AND b.maintenance_date = ? AND b.status IN ?", []string(selection.EquipmentIDs), req.MaintenanceType, *maintDate, []string{model.BatchPending, model.BatchConfirmed}).Count(&duplicate).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if duplicate > 0 {
		return nil, errs.ErrConflict.WithMsg("部分设备已有同日期同动作的未关闭或已生效维保，请重新预览")
	}
	flow := communitysvc.ResolveFlow(s.db, req.CommunityID, sysmodel.FlowMaintReview)
	if len(flow) == 0 {
		return nil, errs.ErrConflict.WithMsg("维保审批链未配置")
	}
	candidates := communitysvc.RoleCandidateUserIDsForCommunity(s.db, req.CommunityID, flow[0].RoleCode)
	hasOther := false
	for _, candidate := range candidates {
		if !communitysvc.SameNaturalPerson(s.db, identity.UserID, candidate) {
			hasOther = true
			break
		}
	}
	if !hasOther {
		return nil, errs.ErrConflict.WithMsg("维保审批链没有非登记人候选人，不能创建批量登记单")
	}
	operator := strings.TrimSpace(req.OperatorName)
	if operator == "" {
		operator = identity.Name
	}
	batch = model.MaintenanceBatch{TenantID: identity.TenantID, CommunityID: req.CommunityID,
		MaintenanceType: req.MaintenanceType, MaintenanceDate: *maintDate, Status: model.BatchPending,
		ReviewFlow: flow, OperationID: req.OperationID, PayloadHash: selection.PayloadHash,
		OperatorName: operator, Note: strings.TrimSpace(req.Note), FileIDs: types.StringArray(req.FileIDs), CreatedBy: identity.UserID}
	if req.Vendor != "" {
		batch.Vendor = &req.Vendor
	}
	itemInputs := map[string]MaintenanceBatchItemReq{}
	for _, item := range req.Items {
		itemInputs[item.EquipmentID] = item
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		for _, equipment := range equipments {
			input := itemInputs[equipment.ID]
			if err := validateBatchFiles(tx, identity.UserID, identity.TenantID, append(append([]string(nil), batch.FileIDs...), input.FileIDs...)); err != nil {
				return err
			}
			item := model.MaintenanceBatchItem{TenantID: batch.TenantID, BatchID: batch.ID, EquipmentID: equipment.ID,
				Status: model.BatchPending, Note: strings.TrimSpace(input.Note), FileIDs: types.StringArray(input.FileIDs)}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"id": batch.ID, "status": batch.Status, "count": len(equipments)}, nil
}

func (s *MaintenanceService) recheckBatchEquipment(c *gin.Context, req *MaintenanceBatchPreviewReq, ids []string) ([]model.Equipment, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	var equipments []model.Equipment
	if err := s.db.Where("tenant_id = ? AND community_id = ? AND type = ? AND id IN ? AND deleted_at IS NULL", identity.TenantID, req.CommunityID, req.EquipmentType, ids).Order("id").Find(&equipments).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(equipments) != len(ids) {
		return nil, errs.ErrConflict.WithMsg("部分设备已失效或不属于当前范围，请重新预览")
	}
	for _, equipment := range equipments {
		if equipment.Status == model.StatusScrapped {
			return nil, errs.ErrConflict.WithMsg("设备「" + equipment.Name + "」已报废，请重新预览")
		}
		if !identity.ScopeSelf {
			if be := middleware.CheckCommunity(s.db, c, equipment.CommunityID); be != nil {
				return nil, be
			}
		}
	}
	return equipments, nil
}

func validateBatchFiles(db *gorm.DB, userID, tenantID string, ids []string) error {
	for _, id := range ids {
		file, err := uploadfile.ByID(db, id)
		if err != nil || file.UserID != userID || file.TenantID == nil || *file.TenantID != tenantID {
			return fmt.Errorf("维保证据不属于当前用户或租户")
		}
	}
	return nil
}

func (s *MaintenanceService) ConfirmMaintenanceBatch(c *gin.Context, batchID string) (gin.H, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	result := struct {
		Confirmed int
		Status    string
	}{Status: model.BatchPending}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var batch model.MaintenanceBatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&batch, "id = ? AND tenant_id = ?", batchID, identity.TenantID).Error; err != nil {
			return errs.ErrNotFound
		}
		if batch.Status == model.BatchConfirmed {
			return nil
		}
		if batch.Status != model.BatchPending {
			return errs.ErrConflict.WithMsg("登记单当前不可确认")
		}
		if communitysvc.SameNaturalPerson(tx, identity.UserID, batch.CreatedBy) {
			return errs.ErrNoPerm.WithMsg("不能确认本人提交的批量登记单")
		}
		if batch.ConfirmStep >= len(batch.ReviewFlow) {
			return errs.ErrConflict.WithMsg("维保审批链无效")
		}
		step := batch.ReviewFlow[batch.ConfirmStep]
		if step.Kind == sysmodel.FlowStepKindAI || !communitysvc.RoleCandidateAuthorized(tx, []string{batch.CommunityID}, step.RoleCode, identity) {
			return errs.ErrNotInSlot
		}
		if batch.ConfirmStep+1 < len(batch.ReviewFlow) {
			return tx.Model(&batch).Updates(map[string]any{"confirm_step": batch.ConfirmStep + 1, "revision": batch.Revision + 1}).Error
		}
		var items []model.MaintenanceBatchItem
		if err := tx.Where("batch_id = ? AND status = ?", batch.ID, model.BatchPending).Order("equipment_id").Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 || len(items) > maintenanceBatchLimit {
			return errs.ErrConflict.WithMsg("批量登记成员为空或超过上限")
		}
		rules, _ := NewEquipmentService(tx).typeRules()
		now := time.Now()
		for _, item := range items {
			var equipment model.Equipment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&equipment, "id = ? AND tenant_id = ? AND community_id = ? AND deleted_at IS NULL", item.EquipmentID, batch.TenantID, batch.CommunityID).Error; err != nil {
				return errs.ErrConflict.WithMsg("设备已失效，整单保持待确认")
			}
			if equipment.Status == model.StatusScrapped {
				return errs.ErrConflict.WithMsg("设备「" + equipment.Name + "」已报废，整单保持待确认")
			}
			record := model.EquipmentMaintenance{TenantID: &batch.TenantID, EquipmentID: equipment.ID, CommunityID: batch.CommunityID,
				BatchRegistrationID: &batch.ID, BatchItemID: &item.ID, MaintenanceType: batch.MaintenanceType,
				MaintenanceDate: batch.MaintenanceDate, Vendor: batch.Vendor, OperatorName: batch.OperatorName,
				Note: strings.TrimSpace(batch.Note + " " + item.Note), FileIDs: types.IDArray(append(append([]string(nil), batch.FileIDs...), item.FileIDs...)),
				ConfirmStatus: model.ConfirmConfirmed, ConfirmedBy: &identity.UserID, ConfirmedAt: &now, ConfirmMode: model.ConfirmModeManual,
				Source: model.SourceForm, FlowSnapshot: batch.ReviewFlow, CreatedBy: batch.CreatedBy}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			if _, err := ApplyLedgerWriteback(tx, &record, &equipment, rules[equipment.Type]); err != nil {
				return errs.ErrConflict.WithMsg("设备「" + equipment.Name + "」台账日期冲突，整单已回滚")
			}
			if err := tx.Model(&item).Update("status", model.BatchConfirmed).Error; err != nil {
				return err
			}
			result.Confirmed++
		}
		result.Status = model.BatchConfirmed
		return tx.Model(&batch).Updates(map[string]any{"status": model.BatchConfirmed, "confirmed_at": now, "revision": batch.Revision + 1}).Error
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return nil, be
		}
		return nil, errs.ErrInternal
	}
	return gin.H{"status": result.Status, "confirmed": result.Confirmed}, nil
}

func (s *MaintenanceService) RejectMaintenanceBatch(c *gin.Context, batchID, reason string) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	if strings.TrimSpace(reason) == "" {
		return errs.ErrParam.WithMsg("驳回必须填写原因")
	}
	var batch model.MaintenanceBatch
	if err := s.db.First(&batch, "id = ? AND tenant_id = ? AND status = ?", batchID, identity.TenantID, model.BatchPending).Error; err != nil {
		return errs.ErrNotFound
	}
	if communitysvc.SameNaturalPerson(s.db, identity.UserID, batch.CreatedBy) {
		return errs.ErrNoPerm.WithMsg("不能审核本人提交的批量登记单")
	}
	if batch.ConfirmStep >= len(batch.ReviewFlow) {
		return errs.ErrConflict.WithMsg("维保审批链无效")
	}
	step := batch.ReviewFlow[batch.ConfirmStep]
	if !communitysvc.RoleCandidateAuthorized(s.db, []string{batch.CommunityID}, step.RoleCode, identity) {
		return errs.ErrNotInSlot
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&batch).Updates(map[string]any{"status": model.BatchRejected, "reject_reason": reason, "revision": batch.Revision + 1}).Error; err != nil {
			return err
		}
		return tx.Model(&model.MaintenanceBatchItem{}).Where("batch_id = ?", batch.ID).Update("status", model.BatchRejected).Error
	}); err != nil {
		return errs.ErrInternal
	}
	return nil
}

func maintenanceBatchHash(req *MaintenanceBatchPreviewReq) string {
	copyReq := *req
	copyReq.PayloadHash = ""
	data, _ := json.Marshal(copyReq)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func batchRequestMap(req *MaintenanceBatchPreviewReq) types.JSONMap {
	data, _ := json.Marshal(req)
	var out types.JSONMap
	_ = json.Unmarshal(data, &out)
	return out
}

func batchRequestMapBytes(value types.JSONMap) []byte {
	data, _ := json.Marshal(value)
	return data
}
