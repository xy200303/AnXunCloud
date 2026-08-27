package service

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
)

// 问题清单：异常打卡记录（result='abnormal'）的只读数据出口（列表 + Excel 导出）。

// issueExportLimit 导出上限，防止全量异常记录打爆内存。
const issueExportLimit = 5000

// applyIssueFilters 问题清单公共过滤条件（固定 result='abnormal'，含小区数据权限；
// 固定过滤 superseded_by 非空的旧记录，覆盖修改后仅最新记录计入问题清单）。
func (s *TaskService) applyIssueFilters(c *gin.Context, q *dto.IssueListQuery) (*gorm.DB, *errs.Error) {
	db := s.db.Model(&model.CheckinRecord{}).Where("result = ? AND superseded_by IS NULL", model.ResultAbnormal)
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.AuditStatus != "" {
		db = db.Where("audit_status = ?", q.AuditStatus)
	}
	if v, ok, be := bind.BoolFilter(q.ForceSubmit); be != nil {
		return nil, be
	} else if ok {
		db = db.Where("force_submit = ?", v)
	}
	if q.PointType != "" {
		db = db.Where("EXISTS (SELECT 1 FROM inspection_point p WHERE p.id = checkin_record.point_id AND p.deleted_at IS NULL AND p.type = ?)", q.PointType)
	}
	if q.PatrolType != "" {
		db = db.Where("EXISTS (SELECT 1 FROM inspection_task t WHERE t.id = checkin_record.task_id AND t.deleted_at IS NULL AND t.patrol_type = ?)", q.PatrolType)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("remark ILIKE ? OR EXISTS (SELECT 1 FROM inspection_point p WHERE p.id = checkin_record.point_id AND p.deleted_at IS NULL AND p.name ILIKE ?)", like, like)
	}
	if q.DateFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", q.DateFrom, time.Local)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("date_from 格式应为 YYYY-MM-DD")
		}
		db = db.Where("checkin_time >= ?", t)
	}
	if q.DateTo != "" {
		t, err := time.ParseInLocation("2006-01-02", q.DateTo, time.Local)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("date_to 格式应为 YYYY-MM-DD")
		}
		db = db.Where("checkin_time < ?", t.AddDate(0, 0, 1))
	}
	return middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id"), nil
}

// issuePointInfo 点位摘要（问题清单行展示/导出用）。
type issuePointInfo struct {
	name         string
	ptype        string
	buildingName string
}

// issuePoint 取点位名称/类型/楼栋名（查不到返回零值，同 pointName 风格）。
func issuePoint(db *gorm.DB, id string) issuePointInfo {
	var p model.InspectionPoint
	if db.Select("name", "type", "building_id").First(&p, "id = ?", id).Error != nil {
		return issuePointInfo{}
	}
	info := issuePointInfo{name: p.Name, ptype: p.Type}
	if p.BuildingID != nil {
		var b model.Building
		if db.Select("name").First(&b, "id = ?", *p.BuildingID).Error == nil {
			info.buildingName = b.Name
		}
	}
	return info
}

// issuePhotos 照片透出格式（同打卡记录详情：item/url/watermarked_url；v21 起从逐项照片聚合）。
func issuePhotos(db *gorm.DB, r *model.CheckinRecord) []gin.H {
	flat := RecordFlatPhotos(db, r.ID)
	photos := make([]gin.H, 0, len(flat))
	for _, p := range flat {
		photos = append(photos, gin.H{"item": p["item"], "url": p["url"], "watermarked_url": p["watermarked_url"]})
	}
	return photos
}

// IssueList 问题清单分页检索（checkin_time DESC）。
func (s *TaskService) IssueList(c *gin.Context, q *dto.IssueListQuery) (*response.Page, *errs.Error) {
	db, be := s.applyIssueFilters(c, q)
	if be != nil {
		return nil, be
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.CheckinRecord
	offset, limit := q.Normalize()
	if err := db.Order("checkin_time DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		pt := issuePoint(s.db, r.PointID)
		list = append(list, gin.H{
			"id": r.ID, "point_id": r.PointID,
			"point_name": pt.name, "building_name": pt.buildingName,
			"community_name": commName(s.db, r.CommunityID),
			"inspector_name": userName(s.db, r.InspectorID),
			"checkin_time":   timefmt.T(r.CheckinTime),
			"result":         r.Result, "remark": r.Remark,
			"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
			"audit_status": r.AuditStatus, "force_submit": r.ForceSubmit,
			"is_suspect": r.IsSuspect, "photos": issuePhotos(s.db, r),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// IssueExport 问题清单导出行（同列表过滤，不分页，上限 issueExportLimit 条）。
func (s *TaskService) IssueExport(c *gin.Context, q *dto.IssueListQuery) ([]excel.IssueExportRow, *errs.Error) {
	db, be := s.applyIssueFilters(c, q)
	if be != nil {
		return nil, be
	}
	var rows []model.CheckinRecord
	if err := db.Order("checkin_time DESC").Limit(issueExportLimit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 点位类型 value → 字典 label（一次取全量映射）
	typeLabels := map[string]string{}
	var dicts []sysmodel.SysDictData
	s.db.Select("value", "label").Where("type_code = 'point_type'").Find(&dicts)
	for _, d := range dicts {
		typeLabels[d.Value] = d.Label
	}
	out := make([]excel.IssueExportRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		pt := issuePoint(s.db, r.PointID)
		typeLabel := typeLabels[pt.ptype]
		if typeLabel == "" {
			typeLabel = pt.ptype
		}
		out = append(out, excel.IssueExportRow{
			CheckinTime:   timefmt.T(r.CheckinTime),
			CommunityName: commName(s.db, r.CommunityID),
			BuildingName:  pt.buildingName,
			PointName:     pt.name,
			PointType:     typeLabel,
			InspectorName: userName(s.db, r.InspectorID),
			Remark:        r.Remark,
			AIVerdict:     aiVerdictLabel(r.AIVerdict),
			AIReason:      r.AIReason,
			AuditStatus:   auditStatusLabel(r.AuditStatus),
			ForceSubmit:   yesNo(r.ForceSubmit),
			IsSuspect:     yesNo(r.IsSuspect),
		})
	}
	return out, nil
}

// aiVerdictLabel AI 结论中文化（pass/review/error → 通过/存疑/失败）。
func aiVerdictLabel(v string) string {
	switch v {
	case "pass":
		return "通过"
	case "review":
		return "存疑"
	case "error":
		return "失败"
	}
	return v
}

// auditStatusLabel 复核状态中文化。
func auditStatusLabel(s string) string {
	switch s {
	case model.AuditAutoPass:
		return "自动通过"
	case model.AuditPending:
		return "待复核"
	case model.AuditPass:
		return "复核通过"
	case model.AuditRejected:
		return "已驳回"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
