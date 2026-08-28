package service

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
	"anxuncloud/internal/pkg/timefmt"
)

// 巡检记录导出（列表同过滤，不分页）。问题清单已并入巡检记录（结果筛选 + 本导出）。

// checkinExportLimit 导出上限，防止全量记录打爆内存。
const checkinExportLimit = 5000

// checkinPointInfo 点位摘要（导出行展示用）。
type checkinPointInfo struct {
	name         string
	ptype        string
	buildingName string
}

// checkinPoint 取点位名称/类型/楼栋名（查不到返回零值，同 pointName 风格）。
func checkinPoint(db *gorm.DB, id string) checkinPointInfo {
	var p model.InspectionPoint
	if db.Select("name", "type", "building_id").First(&p, "id = ?", id).Error != nil {
		return checkinPointInfo{}
	}
	info := checkinPointInfo{name: p.Name, ptype: p.Type}
	if p.BuildingID != nil {
		var b model.Building
		if db.Select("name").First(&b, "id = ?", *p.BuildingID).Error == nil {
			info.buildingName = b.Name
		}
	}
	return info
}

// CheckinExport 巡检记录导出行（与列表同一套过滤含审核状态，上限 checkinExportLimit 条）。
func (s *TaskService) CheckinExport(c *gin.Context, q *dto.CheckinListQuery) ([]excel.CheckinExportRow, *errs.Error) {
	db, be := s.applyCheckinFilters(c, q)
	if be != nil {
		return nil, be
	}
	if q.AuditStatus != "" {
		// 支持逗号多值（如 pass,rejected 查"已审核"合集），与列表口径一致
		if statuses := strings.Split(q.AuditStatus, ","); len(statuses) > 1 {
			db = db.Where("audit_status IN ?", statuses)
		} else {
			db = db.Where("audit_status = ?", q.AuditStatus)
		}
	}
	var rows []model.CheckinRecord
	if err := db.Order("checkin_time DESC").Limit(checkinExportLimit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 点位类型 value → 字典 label（一次取全量映射）
	typeLabels := map[string]string{}
	var dicts []sysmodel.SysDictData
	s.db.Select("value", "label").Where("type_code = 'point_type'").Find(&dicts)
	for _, d := range dicts {
		typeLabels[d.Value] = d.Label
	}
	out := make([]excel.CheckinExportRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		pt := checkinPoint(s.db, r.PointID)
		typeLabel := typeLabels[pt.ptype]
		if typeLabel == "" {
			typeLabel = pt.ptype
		}
		distance := ""
		if r.DistanceToPoint != nil {
			distance = strconv.Itoa(int(*r.DistanceToPoint))
		}
		out = append(out, excel.CheckinExportRow{
			CheckinTime:   timefmt.T(r.CheckinTime),
			CommunityName: commName(s.db, r.CommunityID),
			BuildingName:  pt.buildingName,
			PointName:     pt.name,
			PointType:     typeLabel,
			InspectorName: userName(s.db, r.InspectorID),
			Result:        resultLabel(r.Result),
			Remark:        r.Remark,
			CheckinType:   checkinTypeLabel(r.CheckinType),
			Distance:      distance,
			AIVerdict:     aiVerdictLabel(r.AIVerdict),
			AIReason:      r.AIReason,
			AuditStatus:   auditStatusLabel(r.AuditStatus),
			ForceSubmit:   yesNo(r.ForceSubmit),
			IsSuspect:     yesNo(r.IsSuspect),
		})
	}
	return out, nil
}

// resultLabel 打卡结果中文化。
func resultLabel(s string) string {
	if s == model.ResultAbnormal {
		return "异常"
	}
	return "正常"
}

// checkinTypeLabel 打卡方式中文化（qrcode/fence/nfc/offline）。
func checkinTypeLabel(s string) string {
	switch s {
	case "qrcode":
		return "扫码"
	case "nfc":
		return "NFC"
	case "offline":
		return "离线补传"
	}
	return "围栏"
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
