package service

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

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
	// 批量预载：点位（名称/类型/楼栋）、楼栋名、小区名、巡检员名各一次 IN 查询（消除逐行 N+1，
	// 上限 5000 行 × 每行 3~4 查 ≈ 2 万 SQL）；查不到按原口径返回零值
	pointIDs, commIDs, userIDs := []string{}, []string{}, []string{}
	seenP, seenC, seenU := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := range rows {
		r := &rows[i]
		if !seenP[r.PointID] {
			seenP[r.PointID] = true
			pointIDs = append(pointIDs, r.PointID)
		}
		if !seenC[r.CommunityID] {
			seenC[r.CommunityID] = true
			commIDs = append(commIDs, r.CommunityID)
		}
		if !seenU[r.InspectorID] {
			seenU[r.InspectorID] = true
			userIDs = append(userIDs, r.InspectorID)
		}
	}
	points := map[string]checkinPointInfo{}
	if len(pointIDs) > 0 {
		var pts []model.InspectionPoint
		s.db.Select("id", "name", "type", "building_id").Where("id IN ?", pointIDs).Find(&pts)
		buildingIDs, seenB := []string{}, map[string]bool{}
		for i := range pts {
			points[pts[i].ID] = checkinPointInfo{name: pts[i].Name, ptype: pts[i].Type}
			if pts[i].BuildingID != nil && *pts[i].BuildingID != "" && !seenB[*pts[i].BuildingID] {
				seenB[*pts[i].BuildingID] = true
				buildingIDs = append(buildingIDs, *pts[i].BuildingID)
			}
		}
		if len(buildingIDs) > 0 {
			buildingNames := map[string]string{}
			var bs []model.Building
			s.db.Select("id", "name").Where("id IN ?", buildingIDs).Find(&bs)
			for _, b := range bs {
				buildingNames[b.ID] = b.Name
			}
			for i := range pts {
				if pts[i].BuildingID != nil {
					info := points[pts[i].ID]
					info.buildingName = buildingNames[*pts[i].BuildingID]
					points[pts[i].ID] = info
				}
			}
		}
	}
	commNames := map[string]string{}
	if len(commIDs) > 0 {
		var comms []sysmodel.Community
		s.db.Select("id", "name").Where("id IN ?", commIDs).Find(&comms)
		for i := range comms {
			commNames[comms[i].ID] = comms[i].Name
		}
	}
	userNames := map[string]string{}
	if len(userIDs) > 0 {
		var users []sysmodel.SysUser
		s.db.Select("id", "name").Where("id IN ?", userIDs).Find(&users)
		for i := range users {
			userNames[users[i].ID] = users[i].Name
		}
	}
	out := make([]excel.CheckinExportRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		pt := points[r.PointID]
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
			CommunityName: commNames[r.CommunityID],
			BuildingName:  pt.buildingName,
			PointName:     pt.name,
			PointType:     typeLabel,
			InspectorName: userNames[r.InspectorID],
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
