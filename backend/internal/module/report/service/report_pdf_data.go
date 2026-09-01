package service

import (
	"fmt"
	"strings"
	"time"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/pdf"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
)

// pdfData 由报告记录组装 PDF 版面数据（v2 模板）。
// 汇总表/分项明细/整改台账按期间实时查询（点位+打卡记录+异常打卡）；
// 签字信息取报告留痕（含签名图快照）；管理单位取 report 配置，公章取报告快照/签章资产表。
func (s *ReportService) pdfData(r *model.InspectionReport) pdf.MonthlyReportData {
	start, end, be := periodRange(r.Period)
	if be != nil {
		start, end = time.Now(), time.Now()
	}
	d := pdf.MonthlyReportData{
		CommunityName: s.commName(r.CommunityID),
		Period:        r.Period,
		TitleLine:     s.reportTitleLine(r),
		CompanyName:   s.cfgString("report.company_name"),
		Approved:      r.Status == model.StatusApproved,
		ImageLoader: func(key string) ([]byte, string, error) {
			data, tp, err := s.loadPhoto(key)
			if err != nil {
				logger.L.Warn("月报图片加载失败，已跳过", zap.String("file_key", key), zap.String("report_id", r.ID), zap.Error(err))
			}
			return data, tp, err
		},
	}
	if d.Approved {
		// 公章：优先用终审时固化的快照；存量报告无快照回退报告所属租户当前 active 公章资产
		d.SealKey = r.SealFileKey
		if d.SealKey == "" {
			d.SealKey = s.activeSealKey(r.CommunityID)
		}
		if r.ManagerAt != nil {
			d.ApproveDate = r.ManagerAt.Format("2006-01-02")
		}
	}

	// ===== 点位与类型 =====
	var points []insmodel.InspectionPoint
	s.db.Select("id", "name", "type", "qrcode_no", "template_id").
		Where("community_id = ?", r.CommunityID).Order("sort ASC, id ASC").Find(&points)
	// 专项报告：点位口径收拢到该类型期间任务实际覆盖的点位（应检/实检/漏检清单只列该专项的点位）
	if r.PatrolType != "" {
		covered := s.patrolPointIDs(r.CommunityID, start, end, r.PatrolType)
		filtered := make([]insmodel.InspectionPoint, 0, len(points))
		for _, pt := range points {
			if covered[pt.ID] {
				filtered = append(filtered, pt)
			}
		}
		points = filtered
	}
	pointByID := map[string]*insmodel.InspectionPoint{}
	typesPresent := map[string]bool{}
	for i := range points {
		pointByID[points[i].ID] = &points[i]
		if points[i].Type != "" {
			typesPresent[points[i].Type] = true
		}
	}
	typeNames := s.pointTypeNames(typesPresent)
	for _, code := range typeNames.ordered {
		d.TypeNames = append(d.TypeNames, typeNames.label(code)) // 封面设施类别用中文名
	}

	// ===== 当期打卡记录（过滤已被覆盖的旧记录，台账只取有效记录） =====
	var recs []insmodel.CheckinRecord
	scopeCheckinType(s.db, r.PatrolType).
		Select("id", "point_id", "inspector_id", "checkin_time", "result", "remark", "audit_status").
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND superseded_by IS NULL", r.CommunityID, start, end).
		Order("checkin_time ASC").Find(&recs)
	recsByPoint := map[string][]insmodel.CheckinRecord{}
	inspectorIDSet := map[string]bool{}
	recIDs := make([]string, 0, len(recs))
	for i := range recs {
		recsByPoint[recs[i].PointID] = append(recsByPoint[recs[i].PointID], recs[i])
		inspectorIDSet[recs[i].InspectorID] = true
		recIDs = append(recIDs, recs[i].ID)
	}
	// 逐项结果快照（v18 起独立表；record_id → 按 sort 升序的项行）
	itemsByRec := map[string][]insmodel.CheckinRecordItem{}
	if len(recIDs) > 0 {
		var recItems []insmodel.CheckinRecordItem
		s.db.Where("record_id IN ?", recIDs).Order("sort ASC").Find(&recItems)
		for _, it := range recItems {
			itemsByRec[it.RecordID] = append(itemsByRec[it.RecordID], it)
		}
	}

	// ===== 当期异常打卡记录（整改台账数据源；复核通过视为已闭环） =====
	abnormalRecs := make([]insmodel.CheckinRecord, 0)
	for i := range recs {
		if recs[i].Result == insmodel.ResultAbnormal {
			abnormalRecs = append(abnormalRecs, recs[i])
		}
	}
	userNames := s.userNamesOf(inspectorIDSet)

	// ===== 3.本月检查汇总表（按点位类型动态行） =====
	for _, t := range typeNames.ordered {
		var pts []*insmodel.InspectionPoint
		for i := range points {
			if points[i].Type == t {
				pts = append(pts, &points[i])
			}
		}
		row := pdf.SummaryRow{TypeName: typeNames.label(t), Total: len(pts)}
		checked := 0
		for _, pt := range pts {
			prs := recsByPoint[pt.ID]
			if len(prs) == 0 {
				continue
			}
			checked++
			// 「正常完好/存在问题」按点位口径统计（与"总数"单位一致）：
			// 点位有打卡且无异常记正常完好；有任一异常打卡记存在问题
			hasAbnormal := false
			for _, rec := range prs {
				if rec.Result == insmodel.ResultAbnormal {
					hasAbnormal = true
					break
				}
			}
			if hasAbnormal {
				row.Problems++
			} else {
				row.Normal++
			}
		}
		row.InspectRate = pct(int64(checked), int64(row.Total))
		// 该类型异常打卡：当期数（存在问题关联）与已复核数
		created := 0
		for i := range abnormalRecs {
			if pt, ok := pointByID[abnormalRecs[i].PointID]; ok && pt.Type == t {
				created++
				if abnormalRecs[i].AuditStatus == insmodel.AuditPass {
					row.Rectified++
				}
			}
		}
		row.RectifyRate = pct(int64(row.Rectified), int64(created))
		d.Summary = append(d.Summary, row)
	}

	// ===== 4.分项巡检明细（每个有点位的类型一张表） =====
	tplCache := map[string][]string{} // template_id → 检查项名
	for _, t := range typeNames.ordered {
		var pts []*insmodel.InspectionPoint
		for i := range points {
			if points[i].Type == t {
				pts = append(pts, &points[i])
			}
		}
		if len(pts) == 0 {
			continue
		}
		items := s.templateItems(pts, tplCache)
		dt := pdf.DetailTable{TypeName: typeNames.label(t), TypeCode: t, Items: items}
		if len(items) > 0 {
			dt.Note = "注：检查标准：" + strings.Join(items, "；") + "。"
		}
		// 行：每点位一行（取当期最新一次有效打卡；未巡点位保留空行，明细完整覆盖应巡清单）
		for _, pt := range pts {
			row := pdf.DetailRow{Location: pointLocation(pt)}
			prs := recsByPoint[pt.ID]
			if len(prs) == 0 {
				row.Marks = make([]string, len(items))
				row.Time = "—"
				dt.Rows = append(dt.Rows, row)
				continue
			}
			rec := prs[len(prs)-1] // recs 按 checkin_time 升序，末条即最新
			row.Inspector = userNames[rec.InspectorID]
			row.Time = rec.CheckinTime.Format("01-02 15:04")
			marks := make([]string, len(items))
			var failed []string
			for _, ci := range itemsByRec[rec.ID] {
				for j, name := range items {
					if ci.Name == name {
						if ci.Pass {
							marks[j] = "√"
						} else {
							marks[j] = "×"
						}
					}
				}
				if !ci.Pass {
					note := ci.Name
					if ci.Note != "" {
						note += "（" + ci.Note + "）"
					}
					failed = append(failed, note)
				}
			}
			row.Marks = marks
			if rec.Result == insmodel.ResultAbnormal {
				row.Problem = strings.Join(failed, "、")
				if rec.Remark != "" {
					if row.Problem != "" {
						row.Problem += "；"
					}
					row.Problem += rec.Remark
				}
			}
			dt.Rows = append(dt.Rows, row)
		}
		d.Details = append(d.Details, dt)
	}

	// ===== 附件：分项检查照片（v2：按设施类别分组，标注=点位名·日期[·检查项]） =====
	for _, t := range typeNames.ordered {
		group := pdf.PhotoGroup{Title: typeNames.label(t)}
		for i := range points {
			pt := points[i]
			if pt.Type != t {
				continue
			}
			for _, rec := range recsByPoint[pt.ID] {
				base := pt.Name + "·" + rec.CheckinTime.Format("01-02")
				for _, ci := range itemsByRec[rec.ID] {
					for _, ref := range ci.Photos {
						if f, err := uploadfile.ByID(s.db, ref); err == nil {
							group.Cells = append(group.Cells, pdf.PhotoCell{Label: base + "·" + ci.Name, Key: f.ID})
						}
					}
				}
			}
		}
		if len(group.Cells) > photoGroupMaxCells {
			group.Cells = group.Cells[:photoGroupMaxCells]
		}
		if len(group.Cells) > 0 {
			d.PhotoGroups = append(d.PhotoGroups, group)
		}
	}

	// ===== 5.问题清单及整改台账（v2 列：日期/故障问题+照片/处理情况+照片/检查人） =====
	for i := range abnormalRecs {
		rec := &abnormalRecs[i]
		row := pdf.LedgerRow{
			Date:      rec.CheckinTime.Format("2006-01-02"),
			Problem:   checkinProblem(*rec),
			Inspector: userNames[rec.InspectorID],
		}
		// 问题描述带上点位位置（v2 表无位置列，位置信息并入问题描述）
		if pt, ok := pointByID[rec.PointID]; ok {
			row.Problem = pointLocation(pt) + "：" + row.Problem
		}
		// 问题照片：逐项照片（v21 起照片唯一归属逐项）
		for _, ci := range itemsByRec[rec.ID] {
			for _, ref := range ci.Photos {
				if f, err := uploadfile.ByID(s.db, ref); err == nil {
					row.ProblemPhotos = append(row.ProblemPhotos, f.ID)
				}
			}
		}
		// 处理情况列：异常打卡的复核结论（无整改闭环流程后以此替代）
		row.FixText = auditStatusCN(rec.AuditStatus)
		d.Ledger = append(d.Ledger, row)
	}

	// ===== 2.三级签字栏（含签名图快照） =====
	signedBy := map[string]string{}
	sigBy := map[string]string{}
	proxyNameBy := map[string]string{}
	for _, e := range r.InspectorSigned {
		signedBy[e.UserID] = e.SignedAt
		sigBy[e.UserID] = e.SignatureKey
		if e.ProxyName != "" {
			proxyNameBy[e.UserID] = e.ProxyName
		}
	}
	for _, uid := range r.InspectorIDs {
		name := s.userName(uid)
		if pn, ok := proxyNameBy[uid]; ok {
			name += "（" + pn + "代签）" // 代签必须显式标注；原因留在系统留痕，不挤签字栏
		}
		d.InspectorSigns = append(d.InspectorSigns, pdf.SignInfo{
			Name: name, Time: signedBy[uid], SignatureKey: sigBy[uid],
		})
	}
	// 已签但不在应签名单的（如超管代签）追加展示
	for _, e := range r.InspectorSigned {
		if r.InspectorIDs.Contains(e.UserID) {
			continue
		}
		d.InspectorSigns = append(d.InspectorSigns, pdf.SignInfo{
			Name: e.Name, Time: e.SignedAt, SignatureKey: e.SignatureKey,
		})
	}
	if r.SupervisorBy != nil {
		d.Supervisor = pdf.SignInfo{
			Name: s.userName(*r.SupervisorBy), Time: timefmt.TP(r.SupervisorAt),
			Remark: r.SupervisorRemark, SignatureKey: r.SupervisorSignatureKey,
		}
	}
	if r.ManagerBy != nil {
		d.Manager = pdf.SignInfo{
			Name: s.userName(*r.ManagerBy), Time: timefmt.TP(r.ManagerAt),
			Remark: r.ManagerRemark, SignatureKey: r.ManagerSignatureKey,
		}
	}
	return d
}

// photoGroupMaxCells 单个设施类别照片上限（防单组撑爆版面）。
const photoGroupMaxCells = 48

// patrolPointIDs 该小区该期间内指定巡查类型任务覆盖的点位集合（任务点位快照优先，空快照回落计划名单，兼容存量任务）。
func (s *ReportService) patrolPointIDs(communityID string, start, end time.Time, patrolType string) map[string]bool {
	var tasks []insmodel.InspectionTask
	s.db.Select("id", "plan_id", "point_ids").
		Where("community_id = ? AND task_date >= ? AND task_date < ? AND patrol_type = ?",
			communityID, start.Format("2006-01-02"), end.Format("2006-01-02"), patrolType).
		Find(&tasks)
	set := map[string]bool{}
	planIDSet := map[string]bool{}
	for i := range tasks {
		if len(tasks[i].PointIDs) > 0 {
			for _, pid := range tasks[i].PointIDs {
				set[pid] = true
			}
		} else {
			planIDSet[tasks[i].PlanID] = true
		}
	}
	if len(planIDSet) > 0 {
		ids := make([]string, 0, len(planIDSet))
		for id := range planIDSet {
			ids = append(ids, id)
		}
		var plans []insmodel.InspectionPlan
		s.db.Select("id", "point_ids").Where("id IN ?", ids).Find(&plans)
		for _, p := range plans {
			for _, pid := range p.PointIDs {
				set[pid] = true
			}
		}
	}
	return set
}

// pointTypeNames 点位类型中文名表（按字典 point_type 排序；字典外类型置后用原 code）。
type pointTypeNames struct {
	ordered []string
	labels  map[string]string
}

func (n pointTypeNames) label(code string) string {
	if v, ok := n.labels[code]; ok {
		return v
	}
	return code
}

// pointTypeNames 读取字典 point_type，输出有点位类型的有序中文名表。
func (s *ReportService) pointTypeNames(present map[string]bool) pointTypeNames {
	type dictRow struct {
		Label string
		Value string
	}
	var rows []dictRow
	s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND status = ?", "point_type", sysmodel.StatusEnabled).
		Order("sort ASC").Select("label", "value").Scan(&rows)
	out := pointTypeNames{labels: map[string]string{}}
	seen := map[string]bool{}
	for _, r := range rows {
		out.labels[r.Value] = r.Label
		if present[r.Value] {
			out.ordered = append(out.ordered, r.Value)
			seen[r.Value] = true
		}
	}
	for code := range present {
		if !seen[code] {
			out.ordered = append(out.ordered, code)
		}
	}
	return out
}

// templateItems 取该类型点位的检查项模板项名（首个配置了模板的点位；v18 起读 check_template_item）。
func (s *ReportService) templateItems(pts []*insmodel.InspectionPoint, cache map[string][]string) []string {
	for _, pt := range pts {
		if pt.TemplateID == nil {
			continue
		}
		if items, ok := cache[*pt.TemplateID]; ok {
			return items
		}
		var rows []insmodel.CheckTemplateItem
		s.db.Select("name").Where("template_id = ?", *pt.TemplateID).Order("sort ASC").Find(&rows)
		if len(rows) == 0 {
			continue
		}
		items := make([]string, 0, len(rows))
		for _, it := range rows {
			items = append(items, it.Name)
		}
		cache[*pt.TemplateID] = items
		return items
	}
	return nil
}

// pointLocation 位置/编号：点位名称 + 编码（qrcode_no 或 NFC 卡号）。
func pointLocation(pt *insmodel.InspectionPoint) string {
	if pt == nil {
		return "-"
	}
	code := pt.QRCodeNo
	if code == "" {
		code = pt.NfcID
	}
	if code == "" {
		return pt.Name
	}
	return fmt.Sprintf("%s %s", pt.Name, code)
}

// checkinProblem 异常打卡问题描述：优先异常备注，空则兜底。
func checkinProblem(rec insmodel.CheckinRecord) string {
	if rec.Remark != "" {
		return rec.Remark
	}
	return "打卡异常"
}

// auditStatusCN 打卡复核状态中文（台账「处理情况」列）。
func auditStatusCN(status string) string {
	switch status {
	case insmodel.AuditPending:
		return "待复核"
	case insmodel.AuditPass:
		return "复核通过"
	case insmodel.AuditRejected:
		return "复核驳回"
	case insmodel.AuditAutoPass:
		return "自动通过"
	}
	return status
}

// userNamesOf 批量反查用户姓名。
func (s *ReportService) userNamesOf(idSet map[string]bool) map[string]string {
	out := map[string]string{}
	if len(idSet) == 0 {
		return out
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var users []sysmodel.SysUser
	s.db.Select("id", "name").Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		out[u.ID] = u.Name
	}
	return out
}
