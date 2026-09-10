// Package service 设备台账模块业务逻辑：台账 CRUD / 维保登记确认链 / 到期提醒 / 导入导出。
// 租户隔离口径与巡检模块一致：List 走 ApplyCommunityFilter，Detail/Update/Delete 走 CheckCommunity，
// tenant_id 直挂行（维保流水）走 CheckTenantRow，租户上下文解析走 TenantScopeOrDefault。
package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// 到期状态（due_state）
const (
	DueNone    = "none"    // 无到期日（无规则/无日期数据，不判到期）
	DueNormal  = "normal"  // 正常
	DueWarning = "warning" // 临期（0 ≤ next_due - today ≤ warn_days）
	DueOverdue = "overdue" // 已逾期
	// v1.7 特殊态（筛选/展示优先于到期判定）
	DueScrap        = "scrap"         // 报废日已过
	DueLabelMissing = "label_missing" // 标签缺失（确认链打标，退出自动判定）
)

// 设备状态中文标签（导出/展示用）
var statusLabels = map[string]string{
	model.StatusInService:   "在用",
	model.StatusMaintaining: "维保中",
	model.StatusStopped:     "停用",
	model.StatusScrapped:    "报废",
}

// pointTypeExpected 点位类型 ↔ 设备类型软校验映射（§3.3：不匹配警告不拦截）。
var pointTypeExpected = map[string]string{
	"extinguisher": "fire_extinguisher", // 灭火器 ↔ 灭火器点位
	"hydrant":      "fire_cabinet",      // 消火栓 ↔ 消火栓点位
}

// TypeRule 设备类型维保规则（sys_dict_data(type_code='equipment_type') 的 attrs jsonb）。
// 零值规则（attrs 空）不自动判到期、不催办——导入自动扩充的字典项即此形态。
type TypeRule struct {
	FirstMonths int  // 首次维保月数（0=无首保概念，用周期月数）
	CycleMonths int  // 维保周期月数
	Remind      bool // 启用催办
	ScrapMonths int  // 报废月数（0=不计算报废日）
	SpotRatio   int  // 抽查比例 %（0=用全局 equipment.spotcheck_ratio；v1.7）
}

// ParseTypeRule 从字典 attrs 解析类型规则（纯函数；attrs 空 → 零值规则）。
func ParseTypeRule(attrs types.JSONMap) TypeRule {
	r := TypeRule{
		FirstMonths: attrs.Int("first_months"),
		CycleMonths: attrs.Int("cycle_months"),
		ScrapMonths: attrs.Int("scrap_months"),
		SpotRatio:   attrs.Int("spot_ratio"),
	}
	if b, ok := attrs["remind"].(bool); ok {
		r.Remind = b
	}
	return r
}

// HasCycle 规则是否可计算到期日（周期月数 > 0）。
func (r TypeRule) HasCycle() bool { return r.CycleMonths > 0 }

// DueStatus 到期判定统一纯函数（§3.4）：判定只看 next_due_date 一个字段，按日粒度比较。
func DueStatus(nextDue *time.Time, warnDays int, now time.Time) string {
	if nextDue == nil {
		return DueNone
	}
	today := truncateDay(now)
	due := truncateDay(*nextDue)
	days := int(due.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return DueOverdue
	case days <= warnDays:
		return DueWarning
	default:
		return DueNormal
	}
}

// CalcNextDueDate 按类型规则计算下次到期日（纯函数）：
// 有 lastMaint → +cycle_months；否则 manufacture + first_months（firstMonths=0 用 cycleMonths）；都无 → nil。
func CalcNextDueDate(manufacture, lastMaint *time.Time, firstMonths, cycleMonths int) *time.Time {
	if cycleMonths <= 0 {
		return nil
	}
	if lastMaint != nil {
		d := truncateDay(*lastMaint).AddDate(0, cycleMonths, 0)
		return &d
	}
	if manufacture != nil {
		fm := firstMonths
		if fm <= 0 {
			fm = cycleMonths
		}
		d := truncateDay(*manufacture).AddDate(0, fm, 0)
		return &d
	}
	return nil
}

// CalcScrapDate 报废日 = manufacture + scrap_months（scrapMonths<=0 或无出厂日期 → nil）。
func CalcScrapDate(manufacture *time.Time, scrapMonths int) *time.Time {
	if manufacture == nil || scrapMonths <= 0 {
		return nil
	}
	d := truncateDay(*manufacture).AddDate(0, scrapMonths, 0)
	return &d
}

// truncateDay 按本地时区截断到日（到期判定/日期入库统一日粒度）。
func truncateDay(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// parseDate 解析 YYYY-MM-DD（空串返回 nil；非法返回错误）。
func parseDate(s string) (*time.Time, *errs.Error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return nil, errs.ErrParam.WithMsg("日期格式应为 YYYY-MM-DD：" + s)
	}
	return &t, nil
}

// CfgInt 读取系统参数整数值（导出供 mp 模块用；缺失/非法回退默认值）。
func CfgInt(db *gorm.DB, key string, def int) int { return cfgInt(db, key, def) }

// cfgInt 读取系统参数整数值（缺失/非法回退默认值；与 task_service.cfgInt 同口径直读 sys_config）。
func cfgInt(db *gorm.DB, key string, def int) int {
	var v string
	if err := db.Model(&sysmodel.SysConfig{}).Where("key = ?", key).Select("value").Scan(&v).Error; err == nil {
		if n, err2 := strconv.Atoi(strings.TrimSpace(v)); err2 == nil {
			return n
		}
	}
	return def
}

// EquipmentService 设备台账服务。
type EquipmentService struct {
	db *gorm.DB
}

func NewEquipmentService(db *gorm.DB) *EquipmentService {
	return &EquipmentService{db: db}
}

// TypeRules 加载 equipment_type 字典规则（value → 规则；导出供 mp 模块抽查触发用）。
func (s *EquipmentService) TypeRules() map[string]TypeRule {
	rules, _ := s.typeRules()
	return rules
}

// typeRules 加载 equipment_type 字典：value → (label, 规则)。
func (s *EquipmentService) typeRules() (map[string]TypeRule, map[string]string) {
	rules := map[string]TypeRule{}
	labels := map[string]string{}
	var dds []sysmodel.SysDictData
	s.db.Select("value", "label", "attrs").Where("type_code = 'equipment_type'").Find(&dds)
	for _, dd := range dds {
		rules[dd.Value] = ParseTypeRule(dd.Attrs)
		labels[dd.Value] = dd.Label
	}
	return rules, labels
}

// warnDaysOf 临期阈值优先级：设备 warn_days > 全局 equipment.expire_warn_days（默认 30）。
func (s *EquipmentService) warnDaysOf(e *model.Equipment) int {
	if e.WarnDays != nil && *e.WarnDays > 0 {
		return *e.WarnDays
	}
	return cfgInt(s.db, "equipment.expire_warn_days", 30)
}

// List 台账分页列表（租户隔离 + 类型/状态/到期状态/关键字筛选）。
func (s *EquipmentService) List(c *gin.Context, q *dto.ListQuery) (*response.Page, *errs.Error) {
	db := s.filtered(c, q)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.Equipment
	offset, limit := q.Normalize()
	if err := db.Order("next_due_date ASC NULLS LAST, created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// filtered 列表/导出共用查询构造（含租户隔离与 due_state 筛选）。
func (s *EquipmentService) filtered(c *gin.Context, q *dto.ListQuery) *gorm.DB {
	db := s.db.Model(&model.Equipment{})
	if q.Type != "" {
		db = db.Where("type = ?", q.Type)
	}
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.PointID != "" {
		db = db.Where("point_id = ?", q.PointID)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		db = db.Where("code LIKE ? OR name LIKE ?", kw, kw)
	}
	// due_state 筛选：warn_days 逐设备取 COALESCE(warn_days, 全局配置)；
	// scrap=报废日已过 / label_missing=标签缺失（两个 v1.7 特殊态优先于到期判定）
	if q.DueState != "" {
		globalWarn := cfgInt(s.db, "equipment.expire_warn_days", 30)
		today := truncateDay(time.Now()).Format("2006-01-02")
		warnExpr := "COALESCE(warn_days, ?)"
		switch q.DueState {
		case DueLabelMissing:
			db = db.Where("label_missing = ?", true)
		case DueScrap:
			db = db.Where("scrap_date IS NOT NULL AND scrap_date <= ? AND status = ?", today, model.StatusInService)
		case DueNone:
			db = db.Where("next_due_date IS NULL")
		case DueOverdue:
			db = db.Where("next_due_date IS NOT NULL AND next_due_date < ?", today)
		case DueWarning:
			db = db.Where("next_due_date >= ? AND next_due_date <= ?::date + ("+warnExpr+")", today, today, globalWarn)
		case DueNormal:
			db = db.Where("next_due_date > ?::date + ("+warnExpr+")", today, globalWarn)
		}
	}
	return middleware.ApplyCommunityFilter(db, c, "community_id")
}

// toItems 列表项装配：小区/楼栋/点位名、类型 label、due_state。
func (s *EquipmentService) toItems(rows []model.Equipment) []gin.H {
	_, typeLabels := s.typeRules()
	commNames, buildingNames, pointNames := s.resolveNames(rows)
	now := time.Now()
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		e := &rows[i]
		warn := s.warnDaysOf(e)
		// 特殊态优先：标签缺失 > 报废日已过 > 到期日判定（与 JudgeDevice 同口径）
		dueState := DueStatus(e.NextDueDate, warn, now)
		if e.LabelMissing {
			dueState = DueLabelMissing
		} else if e.ScrapDate != nil && !truncateDay(*e.ScrapDate).After(truncateDay(now)) && e.Status == model.StatusInService {
			dueState = DueScrap
		}
		item := gin.H{
			"id": e.ID, "community_id": e.CommunityID, "community_name": commNames[e.CommunityID],
			"building_id": e.BuildingID, "point_id": e.PointID,
			"type": e.Type, "type_label": typeLabels[e.Type],
			"code": e.Code, "name": e.Name,
			"manufacture_date":      dateStr(e.ManufactureDate),
			"last_maintenance_date": dateStr(e.LastMaintenanceDate),
			"next_due_date":         dateStr(e.NextDueDate),
			"scrap_date":            dateStr(e.ScrapDate),
			"due_state":             dueState,
			"label_missing":         e.LabelMissing,
			"warn_days":             e.WarnDays,
			"status":                e.Status, "status_label": statusLabels[e.Status],
			"extra": e.Extra, "remark": e.Remark,
			"created_at": timefmt.T(e.CreatedAt),
		}
		if e.BuildingID != nil {
			item["building_name"] = buildingNames[*e.BuildingID]
		}
		if e.PointID != nil {
			item["point_name"] = pointNames[*e.PointID]
		}
		list = append(list, item)
	}
	return list
}

// resolveNames 批量解析小区/楼栋/点位名称（消除 N+1）。
func (s *EquipmentService) resolveNames(rows []model.Equipment) (map[string]string, map[string]string, map[string]string) {
	commIDs, buildingIDs, pointIDs := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for i := range rows {
		commIDs[rows[i].CommunityID] = struct{}{}
		if rows[i].BuildingID != nil && *rows[i].BuildingID != "" {
			buildingIDs[*rows[i].BuildingID] = struct{}{}
		}
		if rows[i].PointID != nil && *rows[i].PointID != "" {
			pointIDs[*rows[i].PointID] = struct{}{}
		}
	}
	names := func(table any, ids map[string]struct{}) map[string]string {
		out := map[string]string{}
		if len(ids) == 0 {
			return out
		}
		list := make([]string, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		var rs []struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		s.db.Model(table).Select("id", "name").Where("id IN ?", list).Scan(&rs)
		for _, r := range rs {
			out[r.ID] = r.Name
		}
		return out
	}
	return names(&sysmodel.Community{}, commIDs),
		names(&insmodel.Building{}, buildingIDs),
		names(&insmodel.InspectionPoint{}, pointIDs)
}

// dateStr 日期指针转 YYYY-MM-DD（nil/零值返回空串）。
func dateStr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// Detail 设备详情（带维保规则与到期状态）。
func (s *EquipmentService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, e.CommunityID); be != nil {
		return nil, be
	}
	items := s.toItems([]model.Equipment{e})
	item := items[0]
	item["updated_at"] = timefmt.T(e.UpdatedAt)
	item["last_notified_at"] = timefmt.TP(e.LastNotifiedAt)
	rules, _ := s.typeRules()
	if r, ok := rules[e.Type]; ok {
		item["type_rule"] = gin.H{
			"first_months": r.FirstMonths, "cycle_months": r.CycleMonths,
			"remind": r.Remind, "scrap_months": r.ScrapMonths,
		}
	}
	return item, nil
}

// Create 新增设备；code 租户内唯一（友好报错）；返回点位类型软校验警告（空串=无）。
func (s *EquipmentService) Create(c *gin.Context, req *dto.SaveReq) (string, string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return "", "", be
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return "", "", errs.ErrCommunityNotExist
	}
	warning, be := s.validate(req, "")
	if be != nil {
		return "", "", be
	}
	manufacture, be := parseDate(req.ManufactureDate)
	if be != nil {
		return "", "", be
	}
	nextDue, be := parseDate(req.NextDueDate)
	if be != nil {
		return "", "", be
	}
	rules, _ := s.typeRules()
	rule := rules[req.Type]
	// 到期日：人工覆盖优先，否则按类型规则计算默认值（无规则/无日期数据 → NULL 不判到期）
	if nextDue == nil {
		nextDue = CalcNextDueDate(manufacture, nil, rule.FirstMonths, rule.CycleMonths)
	}
	status := req.Status
	if status == "" {
		status = model.StatusInService
	}
	e := model.Equipment{
		TenantID:        tenantID, // 冗余列（=所属小区租户）
		CommunityID:     req.CommunityID,
		BuildingID:      emptyToNil(req.BuildingID),
		PointID:         emptyToNil(req.PointID),
		Type:            req.Type,
		Code:            strings.TrimSpace(req.Code),
		Name:            strings.TrimSpace(req.Name),
		ManufactureDate: manufacture,
		NextDueDate:     nextDue,
		ScrapDate:       CalcScrapDate(manufacture, rule.ScrapMonths),
		WarnDays:        req.WarnDays,
		Status:          status,
		Extra:           types.JSONMap(req.Extra),
		Remark:          req.Remark,
	}
	if err := s.db.Create(&e).Error; err != nil {
		if isUniqueViolation(err) {
			return "", "", errs.ErrConflict.WithMsg("设备编号「" + e.Code + "」在该租户内已存在")
		}
		return "", "", errs.ErrInternal
	}
	return e.ID, warning, nil
}

// Update 修改设备（next_due_date 留空保持原值，非空为人工覆盖）。
func (s *EquipmentService) Update(c *gin.Context, id string, req *dto.SaveReq) (string, *errs.Error) {
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", id).Error; err != nil {
		return "", errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, e.CommunityID); be != nil {
		return "", be
	}
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return "", be
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return "", errs.ErrCommunityNotExist
	}
	warning, be := s.validate(req, id)
	if be != nil {
		return "", be
	}
	manufacture, be := parseDate(req.ManufactureDate)
	if be != nil {
		return "", be
	}
	nextDue, be := parseDate(req.NextDueDate)
	if be != nil {
		return "", be
	}
	rules, _ := s.typeRules()
	rule := rules[req.Type]
	updates := map[string]any{
		"tenant_id": tenantID, "community_id": req.CommunityID,
		"building_id": emptyToNil(req.BuildingID), "point_id": emptyToNil(req.PointID),
		"type": req.Type, "code": strings.TrimSpace(req.Code), "name": strings.TrimSpace(req.Name),
		"manufacture_date": manufacture,
		"scrap_date":       CalcScrapDate(manufacture, rule.ScrapMonths),
		"warn_days":        req.WarnDays,
		"extra":            types.JSONMap(req.Extra), "remark": req.Remark,
	}
	// 到期日：非空=人工覆盖；空=保持原值（不因编辑其他字段被规则重算冲掉人工覆盖）
	if nextDue != nil {
		updates["next_due_date"] = nextDue
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := s.db.Model(&e).Updates(updates).Error; err != nil {
		if isUniqueViolation(err) {
			return "", errs.ErrConflict.WithMsg("设备编号「" + req.Code + "」在该租户内已存在")
		}
		return "", errs.ErrInternal
	}
	return warning, nil
}

// Delete 软删除（台账保留历史，维保流水不受影响）。
func (s *EquipmentService) Delete(c *gin.Context, id string) *errs.Error {
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, e.CommunityID); be != nil {
		return be
	}
	if err := s.db.Delete(&e).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// BatchDelete 批量软删除。ids 模式逐台校验小区数据权限；all 模式按筛选条件（filtered 自带租户隔离），上限 2000。
// 不存在的 id 忽略，返回实际删除数。
func (s *EquipmentService) BatchDelete(c *gin.Context, req *dto.BatchDeleteReq) (int, *errs.Error) {
	if req.All {
		q := &dto.ListQuery{
			Type: req.Type, CommunityID: req.CommunityID, PointID: req.PointID,
			Status: req.Status, DueState: req.DueState, Keyword: req.Keyword,
		}
		var ids []string
		if err := s.filtered(c, q).Limit(2001).Pluck("id", &ids).Error; err != nil {
			return 0, errs.ErrInternal
		}
		if len(ids) == 0 {
			return 0, errs.ErrNotFound
		}
		if len(ids) > 2000 {
			return 0, errs.ErrParam.WithMsg("筛选结果超过 2000 台，请缩小范围后分批删除")
		}
		if err := s.db.Where("id IN ?", ids).Delete(&model.Equipment{}).Error; err != nil {
			return 0, errs.ErrInternal
		}
		return len(ids), nil
	}
	ids := req.IDs
	if len(ids) == 0 {
		return 0, errs.ErrParam.WithMsg("ids 必填")
	}
	if len(ids) > 500 {
		return 0, errs.ErrParam.WithMsg("单次最多删除 500 台")
	}
	var rows []model.Equipment
	if err := s.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, errs.ErrInternal
	}
	if len(rows) == 0 {
		return 0, errs.ErrNotFound
	}
	checked := map[string]bool{}
	found := make([]string, 0, len(rows))
	for i := range rows {
		if !checked[rows[i].CommunityID] {
			if be := middleware.CheckCommunity(s.db, c, rows[i].CommunityID); be != nil {
				return 0, be
			}
			checked[rows[i].CommunityID] = true
		}
		found = append(found, rows[i].ID)
	}
	if err := s.db.Where("id IN ?", found).Delete(&model.Equipment{}).Error; err != nil {
		return 0, errs.ErrInternal
	}
	return len(found), nil
}

// validate 设备参数校验（excludeID 为修改时排除自身的编号查重）。
func (s *EquipmentService) validate(req *dto.SaveReq, excludeID string) (string, *errs.Error) {
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return "", errs.ErrCommunityNotExist
	}
	var count int64
	// 设备类型须为 equipment_type 字典启用项
	s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = 'equipment_type' AND value = ? AND status = ?", req.Type, sysmodel.StatusEnabled).
		Count(&count)
	if count == 0 {
		return "", errs.ErrParam.WithMsg("type 须为字典 equipment_type 的启用项")
	}
	// 编号租户内唯一（软删行不占位）
	dup := s.db.Model(&model.Equipment{}).Where("tenant_id = ? AND code = ?", *tenantID, strings.TrimSpace(req.Code))
	if excludeID != "" {
		dup = dup.Where("id <> ?", excludeID)
	}
	if dup.Count(&count); count > 0 {
		return "", errs.ErrConflict.WithMsg("设备编号「" + req.Code + "」在该租户内已存在")
	}
	// 楼栋须属于该小区
	if req.BuildingID != nil && *req.BuildingID != "" {
		s.db.Model(&insmodel.Building{}).Where("id = ? AND community_id = ?", *req.BuildingID, req.CommunityID).Count(&count)
		if count == 0 {
			return "", errs.ErrParam.WithMsg("building_id 不存在或不属于该小区")
		}
	}
	// 点位绑定校验：存在且同属一个小区；类型不匹配仅警告（现实混挂常见，§3.3）
	warning := ""
	if req.PointID != nil && *req.PointID != "" {
		var p insmodel.InspectionPoint
		if err := s.db.First(&p, "id = ?", *req.PointID).Error; err != nil || p.CommunityID != req.CommunityID {
			return "", errs.ErrParam.WithMsg("point_id 不存在或不属于该小区")
		}
		if expected, ok := pointTypeExpected[req.Type]; ok && p.Type != expected {
			warning = fmt.Sprintf("点位类型「%s」与设备类型「%s」不匹配（期望点位类型 %s），已按绑定保存", p.Type, req.Type, expected)
		}
	}
	if req.Status != "" {
		if _, ok := statusLabels[req.Status]; !ok {
			return "", errs.ErrParam.WithMsg("status 取值非法（in_service/maintaining/stopped/scrapped）")
		}
	}
	if req.WarnDays != nil && (*req.WarnDays <= 0 || *req.WarnDays > 365) {
		return "", errs.ErrParam.WithMsg("warn_days 取值须为 1–365")
	}
	return warning, nil
}

// emptyToNil 空字符串指针转 NULL。
func emptyToNil(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// isUniqueViolation 判断 PG 唯一约束冲突（23505）。
func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key"))
}
