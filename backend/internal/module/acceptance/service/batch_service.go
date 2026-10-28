package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	communitysvc "anxuncloud/internal/module/community/service"
	filesvc "anxuncloud/internal/module/file/service"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/libreoffice"
	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

type BatchService struct {
	db              *gorm.DB
	store           *storage.Storage
	schema          *SchemaService
	buildings       func(string, string) ([]inspectionsvc.BuildingRef, error)
	verifyTemplates func(string) ([]inspectionsvc.VerifyTemplateRef, error)
	queue           *queuepkg.Queue
	pdfConverter    libreoffice.Converter
}

func (s *BatchService) BindPDFConverter(converter libreoffice.Converter) {
	s.pdfConverter = converter
}

func NewBatchService(db *gorm.DB, store *storage.Storage, schema *SchemaService, buildings func(string, string) ([]inspectionsvc.BuildingRef, error), verifyTemplates func(string) ([]inspectionsvc.VerifyTemplateRef, error)) *BatchService {
	return &BatchService{db: db, store: store, schema: schema, buildings: buildings, verifyTemplates: verifyTemplates}
}

type TargetReq struct {
	TargetType   string   `json:"target_type" binding:"required"`
	RefID        string   `json:"ref_id" binding:"required"`
	BuildingID   *string  `json:"building_id"`
	CategoryCode string   `json:"category_code" binding:"required"`
	TableCodes   []string `json:"table_codes" binding:"required"`
	Name         string   `json:"name"`
}

type MemberReq struct {
	UserID        string   `json:"user_id" binding:"required"`
	WorkScope     string   `json:"work_scope" binding:"required"`
	CategoryCodes []string `json:"category_codes"`
	BuildingIDs   []string `json:"building_ids"`
	ExternalName  string   `json:"external_name"`
}

type BatchReq struct {
	CommunityID string      `json:"community_id" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	LedgerType  string      `json:"ledger_type" binding:"required"`
	PeriodStart string      `json:"period_start" binding:"required"`
	PeriodEnd   string      `json:"period_end" binding:"required"`
	ManagerID   string      `json:"manager_id"`
	Targets     []TargetReq `json:"targets" binding:"required"`
	Members     []MemberReq `json:"members"`
}

type BatchQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Name        string `form:"name"`
	Status      string `form:"status"`
}

func (s *BatchService) List(c *gin.Context, q *BatchQuery) (*response.Page, *errs.Error) {
	query := s.db.Model(&model.Batch{})
	query = middleware.ApplyCommunityFilter(query, c, "acceptance_batch.community_id")
	if identity := middleware.CurrentIdentity(c); identity != nil && identity.ScopeSelf {
		query = query.Where("acceptance_batch.manager_id = ? OR acceptance_batch.id IN (?)", identity.UserID, s.db.Model(&model.Member{}).Select("batch_id").Where("user_id = ?", identity.UserID))
	}
	if q.CommunityID != "" {
		query = query.Where("community_id = ?", q.CommunityID)
	}
	if q.Name != "" {
		query = query.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var batches []model.Batch
	offset, limit := q.Normalize()
	if err := query.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&batches).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(batches))
	for _, batch := range batches {
		var targetCount, coveredCount, problemCount int64
		s.db.Model(&model.Target{}).Where("batch_id = ?", batch.ID).Count(&targetCount)
		s.db.Model(&model.Target{}).Where("batch_id = ? AND coverage_confirmed = true", batch.ID).Count(&coveredCount)
		s.db.Model(&model.Problem{}).Where("batch_id = ?", batch.ID).Count(&problemCount)
		names, _ := systemsvc.UserDisplayNames(s.db, batch.TenantID, []string{batch.ManagerID})
		items = append(items, gin.H{"id": batch.ID, "name": batch.Name, "community_id": batch.CommunityID, "ledger_type": batch.LedgerType, "status": batch.Status, "revision": batch.Revision, "manager_id": batch.ManagerID, "manager_name": names[batch.ManagerID], "target_count": targetCount, "covered_count": coveredCount, "problem_count": problemCount, "period_start": batch.PeriodStart, "period_end": batch.PeriodEnd, "updated_at": batch.UpdatedAt})
	}
	return &response.Page{List: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *BatchService) Create(c *gin.Context, req BatchReq) (string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return "", be
	}
	if req.LedgerType != "institution_change" && req.LedgerType != "unit_acceptance" {
		return "", errs.ErrParam.WithMsg("当前只支持机构更迭或分户验收")
	}
	start, err := time.ParseInLocation("2006-01-02", req.PeriodStart, time.Local)
	if err != nil {
		return "", errs.ErrParam.WithMsg("开始日期格式无效")
	}
	end, err := time.ParseInLocation("2006-01-02", req.PeriodEnd, time.Local)
	if err != nil || !end.After(start) {
		return "", errs.ErrParam.WithMsg("结束日期必须晚于开始日期")
	}
	managerID := req.ManagerID
	if managerID == "" {
		managerID = middleware.CurrentUserID(c)
	}
	identity := middleware.CurrentIdentity(c)
	managerIdentity := &middleware.Identity{UserID: managerID}
	if !communitysvc.RoleCandidateAuthorized(s.db, []string{req.CommunityID}, sysmodel.ProjectManagerCode, managerIdentity) && !communitysvc.RoleCandidateAuthorized(s.db, []string{req.CommunityID}, sysmodel.QualitySupervisorCode, managerIdentity) && (identity == nil || !identity.SuperAdmin) {
		return "", errs.ErrParam.WithMsg("负责人须为该小区项目经理或品质主管")
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return "", errs.ErrParam.WithMsg("小区租户不存在")
	}
	schemas, be := s.schema.List(ConfigScope{Name: model.ScopeTenant, ID: *tenantID})
	if be != nil {
		return "", be
	}
	enabledSchemas := make([]gin.H, 0, len(schemas))
	for _, schema := range schemas {
		if schema["status"] == model.Enabled {
			enabledSchemas = append(enabledSchemas, schema)
		}
	}
	if len(enabledSchemas) == 0 {
		return "", errs.ErrConflict.WithMsg("尚未配置启用的承接查验内容模板")
	}
	templates, err := s.verifyTemplates(*tenantID)
	if err != nil {
		return "", errs.ErrInternal
	}
	flow := communitysvc.ResolveFlow(s.db, req.CommunityID, sysmodel.FlowAcceptanceReview)
	if len(flow) == 0 {
		return "", errs.ErrConflict.WithMsg("承接查验审批链未配置")
	}
	buildingRefs, err := s.buildings(*tenantID, req.CommunityID)
	if err != nil {
		return "", errs.ErrInternal
	}
	buildingMap := map[string]inspectionsvc.BuildingRef{}
	for _, building := range buildingRefs {
		buildingMap[building.ID] = building
	}
	batch := model.Batch{TenantID: *tenantID, CommunityID: req.CommunityID, Name: strings.TrimSpace(req.Name), LedgerType: req.LedgerType, PeriodStart: start, PeriodEnd: end, ManagerID: managerID, Status: model.StatusDraft, Revision: 1, ReviewFlow: flow, TemplateSnapshot: types.JSONMap{"schemas": enabledSchemas, "verify_templates": templates}}
	if batch.Name == "" || len(req.Targets) == 0 {
		return "", errs.ErrParam.WithMsg("批次名称与查验对象不能为空")
	}
	available := map[string]string{}
	for _, schema := range enabledSchemas {
		code, _ := schema["code"].(string)
		category, _ := schema["category"].(string)
		available[code] = category
	}
	targets := make([]model.Target, 0, len(req.Targets))
	seen, assigned := map[string]bool{}, map[string]bool{}
	for _, item := range req.Targets {
		key := item.TargetType + "\x00" + item.RefID + "\x00" + item.CategoryCode
		if seen[key] {
			return "", errs.ErrParam.WithMsg("同一对象和专项不能重复")
		}
		seen[key] = true
		if len(item.TableCodes) == 0 {
			return "", errs.ErrParam.WithMsg("每个查验对象必须明确选择专项")
		}
		for _, code := range item.TableCodes {
			if available[code] != item.CategoryCode || assigned[code] {
				return "", errs.ErrParam.WithMsg("专项不属于该对象类别，或已分配给其他对象：" + code)
			}
			assigned[code] = true
		}
		target := model.Target{TenantID: *tenantID, TargetType: item.TargetType, RefID: item.RefID, CategoryCode: item.CategoryCode, TableCodes: types.StringArray(item.TableCodes), Name: strings.TrimSpace(item.Name)}
		switch item.TargetType {
		case "building":
			building, ok := buildingMap[item.RefID]
			if !ok {
				return "", errs.ErrParam.WithMsg("查验楼栋不属于该小区")
			}
			target.BuildingID = &building.ID
			target.Name, target.Location = building.Name, building.Location
		case "unit":
			if target.Name == "" || item.BuildingID == nil {
				return "", errs.ErrParam.WithMsg("分户查验必须填写房号并选择所属楼栋")
			}
			building, ok := buildingMap[*item.BuildingID]
			if !ok {
				return "", errs.ErrParam.WithMsg("分户所属楼栋不属于该小区")
			}
			target.BuildingID = &building.ID
			target.Location = building.Name
		case "documents", "area", "equipment":
			if target.Name == "" {
				return "", errs.ErrParam.WithMsg("请填写查验对象名称")
			}
		default:
			return "", errs.ErrParam.WithMsg("查验对象类型无效")
		}
		targets = append(targets, target)
	}
	members := req.Members
	if len(members) == 0 {
		members = []MemberReq{{UserID: managerID, WorkScope: "docs"}, {UserID: managerID, WorkScope: "site"}}
	}
	memberRows := make([]model.Member, 0, len(members))
	for _, member := range members {
		if member.WorkScope != "docs" && member.WorkScope != "site" {
			return "", errs.ErrParam.WithMsg("成员分工无效")
		}
		if !middleware.UserCanSeeCommunity(s.db, member.UserID, req.CommunityID) && strings.TrimSpace(member.ExternalName) == "" {
			return "", errs.ErrParam.WithMsg("成员不在该小区范围内；外聘人员须登记姓名")
		}
		memberRows = append(memberRows, model.Member{TenantID: *tenantID, UserID: member.UserID, WorkScope: member.WorkScope, CategoryCodes: types.StringArray(member.CategoryCodes), BuildingIDs: types.StringArray(member.BuildingIDs), ExternalName: strings.TrimSpace(member.ExternalName)})
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		for i := range targets {
			targets[i].BatchID = batch.ID
			if err := tx.Create(&targets[i]).Error; err != nil {
				return err
			}
		}
		for i := range memberRows {
			memberRows[i].BatchID = batch.ID
			if err := tx.Create(&memberRows[i]).Error; err != nil {
				return err
			}
		}
		subjects := []model.SignSubject{
			{TenantID: batch.TenantID, BatchID: batch.ID, Code: "inspector", Name: "查验人", Required: true, Sort: 1},
			{TenantID: batch.TenantID, BatchID: batch.ID, Code: "reviewer", Name: "审核人", RoleCode: sysmodel.QualitySupervisorCode, Required: true, Sort: 2},
			{TenantID: batch.TenantID, BatchID: batch.ID, Code: "transferor", Name: "移交方", External: true, Required: true, Sort: 3},
			{TenantID: batch.TenantID, BatchID: batch.ID, Code: "receiver", Name: "接收方", External: true, Required: true, Sort: 4},
		}
		for _, subject := range subjects {
			if err := tx.Create(&subject).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	return batch.ID, nil
}

func (s *BatchService) load(c *gin.Context, id string) (*model.Batch, *errs.Error) {
	var batch model.Batch
	if err := s.db.First(&batch, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, batch.CommunityID); be != nil {
		return nil, be
	}
	identity := middleware.CurrentIdentity(c)
	if identity != nil && identity.ScopeSelf && batch.ManagerID != identity.UserID {
		var count int64
		s.db.Model(&model.Member{}).Where("batch_id = ? AND user_id = ?", id, identity.UserID).Count(&count)
		if count == 0 {
			return nil, errs.ErrDataScope
		}
	}
	return &batch, nil
}

func (s *BatchService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	batch, be := s.load(c, id)
	if be != nil {
		return nil, be
	}
	var targets []model.Target
	var members []model.Member
	var records []model.Record
	var problems []model.Problem
	var subjects []model.SignSubject
	var signatures []model.Signature
	var votes []model.ReviewVote
	var files []model.ReportFile
	var logs []model.HandoverLog
	var reopenRequests []model.ReopenRequest
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&targets)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&members)
	s.db.Where("batch_id = ?", id).Order("updated_at DESC").Find(&records)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&problems)
	s.db.Where("batch_id = ?", id).Order("sort, code").Find(&subjects)
	s.db.Where("batch_id = ?", id).Order("signed_at, id").Find(&signatures)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&votes)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&files)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&logs)
	s.db.Where("batch_id = ?", id).Order("created_at, id").Find(&reopenRequests)
	return gin.H{"batch": batch, "targets": targets, "members": members, "records": records, "problems": problems, "subjects": subjects, "signatures": signatures, "votes": votes, "files": files, "logs": logs, "reopen_requests": reopenRequests}, nil
}

func (s *BatchService) Start(c *gin.Context, id string) *errs.Error {
	batch, be := s.load(c, id)
	if be != nil {
		return be
	}
	if be := s.requireManager(c, batch); be != nil {
		return be
	}
	if batch.Status != model.StatusDraft {
		return errs.ErrConflict.WithMsg("只有草稿批次可以开始查验")
	}
	return s.transition(batch, model.StatusInspecting)
}

func (s *BatchService) requireManager(c *gin.Context, batch *model.Batch) *errs.Error {
	if middleware.CurrentUserID(c) != batch.ManagerID {
		return errs.ErrNoPerm.WithMsg("只有批次负责人可以执行此操作")
	}
	return nil
}

func (s *BatchService) transition(batch *model.Batch, status string) *errs.Error {
	result := s.db.Model(&model.Batch{}).Where("id = ? AND status = ? AND revision = ?", batch.ID, batch.Status, batch.Revision).Updates(map[string]any{"status": status, "revision": batch.Revision + 1})
	if result.Error != nil {
		return errs.ErrInternal
	}
	if result.RowsAffected != 1 {
		return errs.ErrConflict.WithMsg("批次状态已被其他人更新")
	}
	return nil
}

type RecordReq struct {
	TargetID  string        `json:"target_id" binding:"required"`
	TableCode string        `json:"table_code" binding:"required"`
	ItemCode  string        `json:"item_code" binding:"required"`
	Result    string        `json:"result" binding:"required"`
	ValueText string        `json:"value_text"`
	Values    types.JSONMap `json:"values"`
	Note      string        `json:"note"`
	Photos    []string      `json:"photos"`
}

func (s *BatchService) SaveRecord(c *gin.Context, batchID string, req RecordReq) (string, *errs.Error) {
	batch, be := s.load(c, batchID)
	if be != nil {
		return "", be
	}
	if batch.Status != model.StatusInspecting {
		return "", errs.ErrConflict.WithMsg("批次当前不在查验阶段")
	}
	var target model.Target
	if err := s.db.First(&target, "id = ? AND batch_id = ?", req.TargetID, batchID).Error; err != nil {
		return "", errs.ErrNotFound
	}
	workScope := "site"
	if target.TargetType == "documents" {
		workScope = "docs"
	}
	member, be := s.memberFor(c, batch, workScope)
	if be != nil {
		return "", be
	}
	if len(member.CategoryCodes) > 0 && !types.IDArray(member.CategoryCodes).Contains(target.CategoryCode) {
		return "", errs.ErrDataScope
	}
	if len(member.BuildingIDs) > 0 && !types.IDArray(member.BuildingIDs).Contains(target.RefID) {
		return "", errs.ErrDataScope
	}
	if req.Result != "missing" && req.Result != "problem" {
		return "", errs.ErrParam.WithMsg("只登记缺项或问题事实")
	}
	if strings.TrimSpace(req.Note) == "" {
		return "", errs.ErrParam.WithMsg("请填写问题事实")
	}
	if target.TargetType != "documents" && len(req.Photos) == 0 {
		return "", errs.ErrParam.WithMsg("现场问题必须上传照片")
	}
	if be := s.validateSnapshotItem(batch, &target, req.TableCode, req.ItemCode); be != nil {
		return "", be
	}
	files := uploadfile.ByIDs(s.db, req.Photos)
	actorID := middleware.CurrentUserID(c)
	for _, photo := range req.Photos {
		file, ok := files[photo]
		if !ok || file.Scene != "acceptance" || file.UserID != actorID || file.TenantID == nil || *file.TenantID != batch.TenantID {
			return "", errs.ErrParam.WithMsg("问题照片不属于当前用户或租户")
		}
	}
	var record model.Record
	err := s.db.Where("batch_id = ? AND target_id = ? AND table_code = ? AND item_code = ?", batchID, target.ID, req.TableCode, req.ItemCode).First(&record).Error
	if err != nil {
		record = model.Record{TenantID: batch.TenantID, BatchID: batch.ID, TargetID: target.ID, TableCode: req.TableCode, ItemCode: req.ItemCode, CategoryCode: target.CategoryCode, Revision: 1, CreatedBy: actorID}
	} else {
		record.Revision++
	}
	record.Result, record.ValueText, record.Values, record.Note, record.Photos = req.Result, strings.TrimSpace(req.ValueText), req.Values, strings.TrimSpace(req.Note), types.StringArray(req.Photos)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if record.ID == "" {
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else {
			result := tx.Model(&model.Record{}).Where("id = ? AND revision = ?", record.ID, record.Revision-1).Updates(map[string]any{
				"result": record.Result, "value_text": record.ValueText, "value_map": record.Values, "note": record.Note, "photos": record.Photos, "revision": record.Revision,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errs.ErrConflict.WithMsg("问题记录已被其他人修订")
			}
		}
		problem := model.Problem{TenantID: batch.TenantID, BatchID: batch.ID, RecordID: record.ID, Fact: record.Note, Location: target.Location, Photos: record.Photos}
		if err := tx.Where("record_id = ?", record.ID).Assign(problem).FirstOrCreate(&problem).Error; err != nil {
			return err
		}
		if err := tx.Model(&problem).Updates(map[string]any{"fact": problem.Fact, "location": problem.Location, "photos": problem.Photos}).Error; err != nil {
			return err
		}
		return filesvc.LinkObject(tx, batch.TenantID, batch.CommunityID, "acceptance_record", record.ID, req.Photos)
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return "", be
		}
		return "", errs.ErrInternal
	}
	return record.ID, nil
}

func (s *BatchService) validateSnapshotItem(batch *model.Batch, target *model.Target, tableCode, itemCode string) *errs.Error {
	if !types.IDArray(target.TableCodes).Contains(tableCode) {
		return errs.ErrParam.WithMsg("检查专项未分配给该查验对象")
	}
	data, _ := json.Marshal(batch.TemplateSnapshot["schemas"])
	var schemas []struct {
		Code       string          `json:"code"`
		Category   string          `json:"category"`
		TableKind  string          `json:"table_kind"`
		TemplateID *string         `json:"template_id"`
		Rows       []model.ListRow `json:"rows"`
	}
	if json.Unmarshal(data, &schemas) != nil {
		return errs.ErrInternal
	}
	for _, schema := range schemas {
		if schema.Code != tableCode || schema.Category != target.CategoryCode {
			continue
		}
		if schema.TableKind == "matrix" {
			data, _ = json.Marshal(batch.TemplateSnapshot["verify_templates"])
			var templates []inspectionsvc.VerifyTemplateRef
			if json.Unmarshal(data, &templates) != nil {
				return errs.ErrInternal
			}
			for _, template := range templates {
				if schema.TemplateID == nil || template.ID != *schema.TemplateID {
					continue
				}
				for _, item := range template.Items {
					if item.Code == itemCode {
						return nil
					}
				}
			}
		} else {
			for _, row := range schema.Rows {
				if row.Code == itemCode {
					return nil
				}
			}
		}
	}
	return errs.ErrParam.WithMsg("检查项不在本批次冻结快照中")
}

func (s *BatchService) memberFor(c *gin.Context, batch *model.Batch, workScope string) (*model.Member, *errs.Error) {
	var member model.Member
	query := s.db.Where("batch_id = ? AND user_id = ?", batch.ID, middleware.CurrentUserID(c))
	if workScope != "" {
		query = query.Where("work_scope = ?", workScope)
	}
	if err := query.First(&member).Error; err != nil {
		return nil, errs.ErrNoPerm.WithMsg("你不在本批次分工范围内")
	}
	return &member, nil
}

func (s *BatchService) ConfirmCoverage(c *gin.Context, batchID, targetID string) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if batch.Status != model.StatusInspecting {
		return errs.ErrConflict.WithMsg("批次当前不在查验阶段")
	}
	var target model.Target
	if err := s.db.First(&target, "id = ? AND batch_id = ?", targetID, batchID).Error; err != nil {
		return errs.ErrNotFound
	}
	workScope := "site"
	if target.TargetType == "documents" {
		workScope = "docs"
	}
	member, be := s.memberFor(c, batch, workScope)
	if be != nil {
		return be
	}
	if len(member.CategoryCodes) > 0 && !types.IDArray(member.CategoryCodes).Contains(target.CategoryCode) {
		return errs.ErrDataScope
	}
	if len(member.BuildingIDs) > 0 && !types.IDArray(member.BuildingIDs).Contains(target.RefID) {
		return errs.ErrDataScope
	}
	now, actor := time.Now(), middleware.CurrentUserID(c)
	result := s.db.Model(&model.Target{}).Where("id = ? AND coverage_confirmed = false", target.ID).Updates(map[string]any{"coverage_confirmed": true, "covered_by": actor, "covered_at": now})
	if result.Error != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *BatchService) Submit(c *gin.Context, batchID string) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if be := s.requireManager(c, batch); be != nil {
		return be
	}
	if batch.Status != model.StatusInspecting {
		return errs.ErrConflict.WithMsg("只有查验中的批次可以送审")
	}
	var uncovered int64
	s.db.Model(&model.Target{}).Where("batch_id = ? AND coverage_confirmed = false", batchID).Count(&uncovered)
	if uncovered > 0 {
		return errs.ErrConflict.WithMsg("仍有查验对象未确认覆盖")
	}
	for _, step := range batch.ReviewFlow {
		candidates := communitysvc.RoleCandidateUserIDsForCommunity(s.db, batch.CommunityID, step.RoleCode)
		allowed := false
		for _, candidate := range candidates {
			if !communitysvc.SameNaturalPerson(s.db, batch.ManagerID, candidate) {
				allowed = true
				break
			}
		}
		if !allowed {
			return errs.ErrConflict.WithMsg("审批节点「" + step.Name + "」没有非提交人候选人，批次保持挂起")
		}
	}
	return s.transition(batch, model.StatusReviewing)
}
