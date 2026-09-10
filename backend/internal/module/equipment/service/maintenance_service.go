package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

// 站内消息类型
const (
	MsgTypeMaintReject = "equipment_maint_reject" // 维保登记驳回
	MsgTypeExpire      = "equipment_expire"       // 设备到期提醒
	MsgTypeScrap       = "equipment_scrap"        // 设备报废提醒（v1.7 报废催办链）
)

// 台账补录（ledger_fix）随单补录日期存 payload 列：{manufacture_date, last_maintenance_date}（YYYY-MM-DD），
// 确认时解析回写台账；与 AI 预检的 ai_verdict/ai_reason 互不干扰。
const (
	ledgerKeyManufacture = "manufacture_date"
	ledgerKeyLastMaint   = "last_maintenance_date"
)

// 合法维保类型
var maintenanceTypes = map[string]bool{
	model.MaintenanceRepair:  true,
	model.MaintenanceKeep:    true,
	model.MaintenanceInspect: true,
	model.MaintenanceReplace: true,
	model.MaintenanceLedger:  true,
}

// MaintenanceService 维保登记与确认链服务（台账唯一写入口：confirmed 才回写）。
type MaintenanceService struct {
	db       *gorm.DB
	notifier *notify.Notifier
	aiCli    *ai.Client // AI 预检（可空：未装配/未启用时 ai_verdict 留 NULL）
}

func NewMaintenanceService(db *gorm.DB, notifier *notify.Notifier, aiCli *ai.Client) *MaintenanceService {
	return &MaintenanceService{db: db, notifier: notifier, aiCli: aiCli}
}

// checkRegisterAccess 登记侧访问校验：field_staff 为 self 档（CheckCommunity 一律拒绝），
// 登记是本人动作，退化为租户边界校验（设备租户 = 本人租户）；管理档照常走 CheckCommunity。
func checkRegisterAccess(db *gorm.DB, c *gin.Context, e *model.Equipment) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil
	}
	if identity.ScopeSelf && !identity.SuperAdmin {
		if e.TenantID == nil || *e.TenantID != identity.TenantID {
			return errs.ErrDataScope
		}
		return nil
	}
	return middleware.CheckCommunity(db, c, e.CommunityID)
}

// Register 维保登记（一键+一拍）：照片必传且归属本人；confirm_status=pending；
// ai_verdict 留 NULL（AI 预检另一任务填充）；台账不更新，等经理确认后回写。
func (s *MaintenanceService) Register(c *gin.Context, req *dto.MaintenanceRegisterReq) (string, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return "", errs.ErrUnauthorized
	}
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", req.EquipmentID).Error; err != nil {
		return "", errs.ErrNotFound
	}
	if be := checkRegisterAccess(s.db, c, &e); be != nil {
		return "", be
	}
	if e.Status == model.StatusScrapped {
		return "", errs.ErrConflict.WithMsg("设备已报废，不可登记维保")
	}
	maintType := req.MaintenanceType
	if maintType == "" {
		maintType = model.MaintenanceRepair
	}
	if !maintenanceTypes[maintType] {
		return "", errs.ErrParam.WithMsg("maintenance_type 取值非法（repair/maintain/inspect/replace/ledger_fix）")
	}
	// 维保日期：缺省今天（巡检员可改为标签上的实际日期）
	maintDate := truncateDay(time.Now())
	if strings.TrimSpace(req.MaintenanceDate) != "" {
		d, be := parseDate(req.MaintenanceDate)
		if be != nil {
			return "", be
		}
		maintDate = *d
	}
	// 照片：至少 1 张，逐一上传确认（存在且归属本人，与 checkin_service 同口径）
	if len(req.FileIDs) > 9 {
		return "", errs.ErrParam.WithMsg("照片最多 9 张")
	}
	for _, fid := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, fid)
		if err != nil || f.UserID != identity.UserID {
			return "", errs.ErrPhotoNotUploaded
		}
	}
	operatorName := strings.TrimSpace(req.OperatorName)
	if operatorName == "" {
		operatorName = identity.Name
	}
	m := model.EquipmentMaintenance{
		TenantID:        e.TenantID,
		EquipmentID:     e.ID,
		MaintenanceType: maintType,
		MaintenanceDate: maintDate,
		OperatorName:    operatorName,
		Note:            strings.TrimSpace(req.Note),
		FileIDs:         types.IDArray(req.FileIDs),
		LabelMissing:    req.LabelMissing, // 标签缺失登记：照片拍设备本体作证，日期免填
		ConfirmStatus:   model.ConfirmPending,
		CreatedBy:       identity.UserID,
	}
	if v := strings.TrimSpace(req.Vendor); v != "" {
		m.Vendor = &v
	}
	// 台账补录：允许随单提交出厂/最近维保日期（确认时一并回写台账），暂存 payload 列
	if maintType == model.MaintenanceLedger {
		manufacture, be := parseDate(req.ManufactureDate)
		if be != nil {
			return "", be
		}
		lastMaint, be := parseDate(req.LastMaintenanceDate)
		if be != nil {
			return "", be
		}
		if manufacture != nil || lastMaint != nil {
			m.Payload = types.JSONMap{
				ledgerKeyManufacture: dateStr(manufacture),
				ledgerKeyLastMaint:   dateStr(lastMaint),
			}
		}
	}
	if err := s.db.Create(&m).Error; err != nil {
		return "", errs.ErrInternal
	}
	// AI 预检异步执行（v1.7：读日期比对/照片有效性/EXIF 偏差；只标记不拦截，失败留 NULL 不阻塞登记）
	if s.aiCli != nil && s.aiCli.Enabled() {
		go s.aiPreCheck(m.ID)
	}
	return m.ID, nil
}

// ledgerFixDates 从 payload 列解析台账补录随单的补录日期。
func ledgerFixDates(m *model.EquipmentMaintenance) (manufacture, lastMaint *time.Time) {
	if len(m.Payload) == 0 {
		return nil, nil
	}
	if v, ok := m.Payload[ledgerKeyManufacture].(string); ok && v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			manufacture = &t
		}
	}
	if v, ok := m.Payload[ledgerKeyLastMaint].(string); ok && v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			lastMaint = &t
		}
	}
	return manufacture, lastMaint
}

// ConfirmResult 批量确认结果。
type ConfirmResult struct {
	Confirmed int      `json:"confirmed"`
	Skipped   int      `json:"skipped"` // 非 pending（已确认/已驳回）幂等跳过
	NotFound  []string `json:"not_found"`
}

// Confirm 批量确认：事务内逐条 pending→confirmed 并回写台账（last_maintenance_date、
// ledger_fix 时同时回写 manufacture_date、按类型规则重算 next_due_date 与 scrap_date）。
// 跨租户记录整条拒绝（不暴露存在性）；已 confirmed 跳过（幂等）。
func (s *MaintenanceService) Confirm(c *gin.Context, req *dto.ConfirmReq) (*ConfirmResult, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	result := &ConfirmResult{NotFound: []string{}}
	rules, _ := NewEquipmentService(s.db).typeRules()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range req.IDs {
			var m model.EquipmentMaintenance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", id).Error; err != nil {
				result.NotFound = append(result.NotFound, id)
				continue
			}
			// 跨租户记录：整条批次拒绝（40302，不暴露存在性）
			if be := middleware.CheckTenantRow(c, tx, m.TenantID); be != nil {
				return be
			}
			if m.ConfirmStatus != model.ConfirmPending {
				result.Skipped++
				continue
			}
			var e model.Equipment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, "id = ?", m.EquipmentID).Error; err != nil {
				result.NotFound = append(result.NotFound, id)
				continue
			}
			now := time.Now()
			if err := tx.Model(&m).Updates(map[string]any{
				"confirm_status": model.ConfirmConfirmed,
				"confirmed_by":   identity.UserID,
				"confirmed_at":   now,
			}).Error; err != nil {
				return errs.ErrInternal
			}
			// 回写台账（confirmed 流水是台账唯一写入口）
			lastMaint := &m.MaintenanceDate
			updates := map[string]any{"last_maintenance_date": m.MaintenanceDate}
			if m.MaintenanceType == model.MaintenanceLedger {
				manufacture, fixLast := ledgerFixDates(&m)
				if fixLast != nil {
					lastMaint = fixLast
					updates["last_maintenance_date"] = *fixLast
				}
				if manufacture != nil {
					updates["manufacture_date"] = *manufacture
					e.ManufactureDate = manufacture
				}
			}
			rule := rules[e.Type]
			if nd := CalcNextDueDate(e.ManufactureDate, lastMaint, rule.FirstMonths, rule.CycleMonths); nd != nil {
				updates["next_due_date"] = *nd
			}
			updates["scrap_date"] = CalcScrapDate(e.ManufactureDate, rule.ScrapMonths)
			// 标签缺失标记随确认链流转：标签缺失单确认后打标（退出自动判定）；
			// 正常登记/补录确认后清除（覆盖设备换过标签的场景）
			updates["label_missing"] = m.LabelMissing
			// 确认后恢复全绿：清掉防打扰标记，下一周期重新提醒
			updates["last_notified_at"] = nil
			if err := tx.Model(&e).Updates(updates).Error; err != nil {
				return errs.ErrInternal
			}
			result.Confirmed++
		}
		return nil
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return nil, be
		}
		return nil, errs.ErrInternal
	}
	return result, nil
}

// Reject 驳回登记：pending→rejected + 理由，通知登记人重新登记（台账不变）。
func (s *MaintenanceService) Reject(c *gin.Context, req *dto.RejectReq) *errs.Error {
	var m model.EquipmentMaintenance
	if err := s.db.First(&m, "id = ?", req.ID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckTenantRow(c, s.db, m.TenantID); be != nil {
		return be
	}
	if m.ConfirmStatus != model.ConfirmPending {
		return errs.ErrConflict.WithMsg("仅待确认记录可驳回")
	}
	if err := s.db.Model(&m).Updates(map[string]any{
		"confirm_status": model.ConfirmRejected,
		"reject_reason":  strings.TrimSpace(req.Reason),
	}).Error; err != nil {
		return errs.ErrInternal
	}
	// 通知登记人（附设备编号/名称与驳回理由）
	var e model.Equipment
	if s.db.Select("code", "name").First(&e, "id = ?", m.EquipmentID).Error == nil {
		title := "维保登记被驳回"
		content := fmt.Sprintf("设备「%s（%s）」的维保登记被驳回：%s。请重新登记。", e.Name, e.Code, strings.TrimSpace(req.Reason))
		if err := s.notifier.Send(m.CreatedBy, MsgTypeMaintReject, title, content, &m.ID); err != nil {
			return errs.ErrInternal
		}
	}
	return nil
}

// ConfirmList 待确认分页：ai_verdict='review' 置顶（NULL 视为 pass 排后），
// 带设备编号/名称/点位名/登记人姓名/照片 url。
func (s *MaintenanceService) ConfirmList(c *gin.Context, q *dto.ConfirmListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.EquipmentMaintenance{}).Where("confirm_status = ?", model.ConfirmPending)
	if tid, be := middleware.TenantScopeOrDefault(c, s.db); be != nil {
		return nil, be
	} else if tid != "" {
		db = db.Where("tenant_id = ?", tid)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.EquipmentMaintenance
	offset, limit := q.Normalize()
	if err := db.Order("CASE WHEN ai_verdict = 'review' THEN 0 ELSE 1 END, created_at ASC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toMaintenanceItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// History 某设备维保历史（含确认状态与照片凭证）。
func (s *MaintenanceService) History(c *gin.Context, equipmentID string, q *response.PageQuery) (*response.Page, *errs.Error) {
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", equipmentID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := checkRegisterAccess(s.db, c, &e); be != nil {
		return nil, be
	}
	db := s.db.Model(&model.EquipmentMaintenance{}).Where("equipment_id = ?", equipmentID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.EquipmentMaintenance
	offset, limit := q.Normalize()
	if err := db.Order("maintenance_date DESC, created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toMaintenanceItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// toMaintenanceItems 维保流水装配：设备编号/名称/点位名/登记人姓名/照片 url。
func (s *MaintenanceService) toMaintenanceItems(rows []model.EquipmentMaintenance) []gin.H {
	eqByID := map[string]model.Equipment{}
	{
		ids := make([]string, 0, len(rows))
		for i := range rows {
			ids = append(ids, rows[i].EquipmentID)
		}
		var es []model.Equipment
		s.db.Select("id", "code", "name", "point_id").Where("id IN ?", ids).Find(&es)
		for i := range es {
			eqByID[es[i].ID] = es[i]
		}
	}
	pointNames := map[string]string{}
	{
		ids := []string{}
		for _, e := range eqByID {
			if e.PointID != nil && *e.PointID != "" {
				ids = append(ids, *e.PointID)
			}
		}
		if len(ids) > 0 {
			var ps []insmodel.InspectionPoint
			s.db.Select("id", "name").Where("id IN ?", ids).Find(&ps)
			for _, p := range ps {
				pointNames[p.ID] = p.Name
			}
		}
	}
	userNames := map[string]string{}
	{
		ids := []string{}
		for i := range rows {
			ids = append(ids, rows[i].CreatedBy)
			if rows[i].ConfirmedBy != nil {
				ids = append(ids, *rows[i].ConfirmedBy)
			}
		}
		var us []sysmodel.SysUser
		s.db.Select("id", "name").Where("id IN ?", ids).Find(&us)
		for _, u := range us {
			userNames[u.ID] = u.Name
		}
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		m := &rows[i]
		e := eqByID[m.EquipmentID]
		photos := make([]gin.H, 0, len(m.FileIDs))
		for fid, f := range uploadfile.ByIDs(s.db, m.FileIDs) {
			photos = append(photos, gin.H{"file_id": fid, "url": f.URL})
		}
		item := gin.H{
			"id": m.ID, "equipment_id": m.EquipmentID,
			"equipment_code": e.Code, "equipment_name": e.Name,
			"maintenance_type": m.MaintenanceType,
			"maintenance_date": dateStr(&m.MaintenanceDate),
			"vendor":           m.Vendor, "operator_name": m.OperatorName, "note": m.Note,
			"photos":         photos,
			"confirm_status": m.ConfirmStatus, "reject_reason": m.RejectReason,
			"label_missing": m.LabelMissing,
			"ai_verdict": m.AIVerdict, "ai_reason": m.AIReason,
			"created_by": m.CreatedBy, "created_by_name": userNames[m.CreatedBy],
			"created_at": timefmt.T(m.CreatedAt),
		}
		if e.PointID != nil {
			item["point_id"] = *e.PointID
			item["point_name"] = pointNames[*e.PointID]
		}
		if m.ConfirmedBy != nil {
			item["confirmed_by"] = *m.ConfirmedBy
			item["confirmed_by_name"] = userNames[*m.ConfirmedBy]
			item["confirmed_at"] = timefmt.TP(m.ConfirmedAt)
		}
		// 台账补录：补录日期显式透出（确认页核对用）
		if m.MaintenanceType == model.MaintenanceLedger {
			manufacture, lastMaint := ledgerFixDates(m)
			item["fix_manufacture_date"] = dateStr(manufacture)
			item["fix_last_maintenance_date"] = dateStr(lastMaint)
		}
		list = append(list, item)
	}
	return list
}

// DueDevices mp 端待维保设备列表：本租户临期+逾期+应报废在用设备（带点位/小区名，逾期/报废在前）。
// 标签缺失设备不出现（已退出自动判定，走经理处置通道）。
func (s *MaintenanceService) DueDevices(c *gin.Context) ([]gin.H, *errs.Error) {
	tid, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return nil, be
	}
	globalWarn := cfgInt(s.db, "equipment.expire_warn_days", 30)
	today := truncateDay(time.Now())
	db := s.db.Model(&model.Equipment{}).
		Where("status = ? AND label_missing = ?", model.StatusInService, false).
		Where("(next_due_date IS NOT NULL AND next_due_date <= ?::date + COALESCE(warn_days, ?)) OR (scrap_date IS NOT NULL AND scrap_date <= ?)",
			today.Format("2006-01-02"), globalWarn, today.Format("2006-01-02")).
		Where("NOT EXISTS (SELECT 1 FROM equipment_maintenance m WHERE m.equipment_id = equipment.id AND m.confirm_status = ?)", model.ConfirmPending)
	if tid != "" {
		db = db.Where("tenant_id = ?", tid)
	}
	// 只列启用催办的类型（电梯等外包合同驱动型不催办）；报废设备不受类型催办开关限制（违规使用风险）
	rules, _ := NewEquipmentService(s.db).typeRules()
	remindTypes := make([]string, 0, len(rules))
	for t, r := range rules {
		if r.Remind {
			remindTypes = append(remindTypes, t)
		}
	}
	if len(remindTypes) == 0 {
		return []gin.H{}, nil
	}
	var rows []model.Equipment
	if err := db.Where("type IN ? OR scrap_date <= ?", remindTypes, today.Format("2006-01-02")).
		Order("next_due_date ASC NULLS FIRST").Limit(200).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := NewEquipmentService(s.db).toItems(rows)
	for i := range rows {
		if rows[i].NextDueDate != nil {
			items[i]["overdue_days"] = int(today.Sub(truncateDay(*rows[i].NextDueDate)).Hours() / 24)
		}
	}
	return items, nil
}
