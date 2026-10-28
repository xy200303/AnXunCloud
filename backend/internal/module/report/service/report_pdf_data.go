package service

import (
	"fmt"
	"sort"
	"strings"

	equipmentsvc "anxuncloud/internal/module/equipment/service"
	inscontract "anxuncloud/internal/module/inspection/contract"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/module/report/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
)

// pdfData 由报告记录组装 PDF 版面数据（v2 模板）。
// 汇总表/分项明细/整改台账按期间实时查询（点位+打卡记录+异常打卡）；
// 签字信息取报告留痕（含签名图快照）；管理单位取 report 配置，公章取报告快照/签章资产表。
func (s *ReportService) pdfData(r *model.InspectionReport, docTemplate *model.DocTemplate) (model.MonthlyReportData, error) {
	start, end, be := periodRange(r.Period)
	if be != nil {
		return model.MonthlyReportData{}, be
	}
	if r.TenantID == nil || *r.TenantID == "" {
		return model.MonthlyReportData{}, fmt.Errorf("报告缺少租户")
	}
	effectiveTemplate := *docTemplate
	effectiveTemplate.TenantID = r.TenantID
	docTemplate = &effectiveTemplate
	d := model.MonthlyReportData{
		CommunityName:    s.commName(r.CommunityID),
		CommunityAddress: s.commAddress(r.CommunityID),
		Period:           r.Period,
		TitleLine:        s.reportTitleLine(r),
		CompanyName:      s.cfgString("report.company_name"),
		CompanyNameEn:    s.cfgString("report.company_name_en"),
		Contact: model.ContactInfo{
			Tel:     s.cfgString("site.contact_phone"),
			Email:   s.cfgString("site.contact_email"),
			Address: s.cfgString("site.address"),
		},
		Approved: r.Status == model.StatusApproved,
		ImageLoader: func(fileID string) ([]byte, string, error) {
			data, tp, err := s.loadPhoto(fileID)
			if err != nil {
				logger.L.Warn("月报图片加载失败，已跳过", zap.String("file_id", fileID), zap.String("report_id", r.ID), zap.Error(err))
			}
			return data, tp, err
		},
	}
	if d.CompanyName == "" {
		// 落款单位缺省时回落租户名
		d.CompanyName = s.tenantName(r.TenantID)
	}
	// 报告编号：暂无独立编号体系，用「小区名-YYYY-MM」
	d.ReportNo = d.CommunityName + "-" + r.Period
	if d.Approved {
		// 公章：优先用终审时固化的快照；存量报告无快照回退报告所属租户当前 active 公章资产
		d.SealFileID, _ = r.Stats["seal_file_id"].(string)
		if d.SealFileID == "" {
			d.SealFileID = s.activeSealID(r.CommunityID)
		}
	}

	var points []inscontract.PointFact
	var recs []inscontract.RecordFact
	var tasks []inscontract.TaskFact
	itemsByRec := map[string][]inscontract.ItemFact{}
	err := inspectionsvc.ReadApprovedFacts(s.db, inscontract.ApprovedFactQuery{TenantID: *r.TenantID, CommunityID: r.CommunityID, PatrolType: r.PatrolType, Start: start, End: end}, func(page inscontract.ApprovedFactPage) error {
		points = append(points, page.Points...)
		tasks = append(tasks, page.Tasks...)
		recs = append(recs, page.Records...)
		for _, item := range page.Items {
			itemsByRec[item.RecordID] = append(itemsByRec[item.RecordID], item)
		}
		return nil
	})
	if err != nil {
		return model.MonthlyReportData{}, err
	}
	if r.PatrolType != "" {
		covered := patrolPointIDs(tasks)
		filtered := make([]inscontract.PointFact, 0, len(points))
		for _, pt := range points {
			if covered[pt.ID] {
				filtered = append(filtered, pt)
			}
		}
		points = filtered
	}
	pointByID := map[string]*inscontract.PointFact{}
	typesPresent := map[string]bool{}
	for i := range points {
		pointByID[points[i].ID] = &points[i]
		if points[i].Type != "" {
			typesPresent[points[i].Type] = true
		}
	}
	typeNames, err := s.pointTypeNames(typesPresent)
	if err != nil {
		return model.MonthlyReportData{}, err
	}
	for _, code := range typeNames.ordered {
		d.TypeNames = append(d.TypeNames, typeNames.label(code))
	}

	recsByPoint := map[string][]inscontract.RecordFact{}
	inspectorIDSet := map[string]bool{}
	for i := range recs {
		recsByPoint[recs[i].PointID] = append(recsByPoint[recs[i].PointID], recs[i])
		inspectorIDSet[recs[i].InspectorID] = true
	}
	equipmentIDs := map[string]bool{}
	for _, items := range itemsByRec {
		for _, item := range items {
			if item.EquipmentID != nil {
				equipmentIDs[*item.EquipmentID] = true
			}
		}
	}
	equipmentNames := map[string]string{}
	if len(equipmentIDs) > 0 {
		ids := make([]string, 0, len(equipmentIDs))
		for id := range equipmentIDs {
			ids = append(ids, id)
		}
		if r.TenantID != nil {
			if names, err := equipmentsvc.EquipmentNames(s.db, *r.TenantID, ids); err == nil {
				equipmentNames = names
			}
		}
	}
	// ===== 当期异常打卡记录（整改台账数据源；复核通过视为已闭环） =====
	abnormalRecs := make([]inscontract.RecordFact, 0)
	for i := range recs {
		if recs[i].Result == inscontract.ResultAbnormal {
			abnormalRecs = append(abnormalRecs, recs[i])
		}
	}
	userNames, err := s.userNamesOf(*r.TenantID, inspectorIDSet)
	if err != nil {
		return model.MonthlyReportData{}, err
	}

	// ===== 3.本月检查汇总表（按点位类型动态行） =====
	for _, t := range typeNames.ordered {
		var pts []*inscontract.PointFact
		for i := range points {
			if points[i].Type == t {
				pts = append(pts, &points[i])
			}
		}
		row := model.SummaryRow{TypeName: typeNames.label(t), Total: len(pts)}
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
				if rec.Result == inscontract.ResultAbnormal {
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
				if abnormalRecs[i].AuditStatus == inscontract.AuditPass {
					row.Rectified++
				}
			}
		}
		row.RectifyRate = pct(int64(row.Rectified), int64(created))
		d.Summary = append(d.Summary, row)
	}
	// 行排序尽量贴合模板顺序：消火栓→灭火器→应急照明→指示牌→手报→烟感→其他
	sort.SliceStable(d.Summary, func(i, j int) bool {
		return summaryOrderKey(d.Summary[i].TypeName) < summaryOrderKey(d.Summary[j].TypeName)
	})

	// ===== 4.分项巡检明细（按输出模板列身份与行模式装配） =====
	for _, t := range typeNames.ordered {
		var pts []*inscontract.PointFact
		for i := range points {
			if points[i].Type == t {
				pts = append(pts, &points[i])
			}
		}
		if len(pts) == 0 {
			continue
		}
		columnSet := s.templateColumns(pts, docTemplate, itemsByRec)
		items := columnSet.Columns
		labels := make([]string, 0, len(items))
		keys := make([]string, 0, len(items))
		for _, c := range items {
			labels = append(labels, c.label)
			keys = append(keys, c.itemCode)
		}
		dt := model.DetailTable{TypeName: typeNames.label(t), TypeCode: t, Items: labels, ColumnKeys: keys}
		rowMode := outputRowMode(s, docTemplate, t)
		for _, pt := range pts {
			location := pointLocation(pt)
			rows := expandOutputRecords(rowMode, recsByPoint[pt.ID], itemsByRec, equipmentNames)
			if len(rows) == 0 {
				if r.DetailMode != "abnormal" {
					dt.Rows = append(dt.Rows, model.DetailRow{Location: location, Marks: make([]string, len(items)), Time: "—"})
				}
				continue
			}
			for _, entry := range rows {
				rec := entry.record
				if r.DetailMode == "abnormal" && rec.Result != inscontract.ResultAbnormal {
					continue
				}
				row := model.DetailRow{Location: location + entry.suffix, Inspector: userNames[rec.InspectorID], Time: rec.CheckinTime.Format("01-02 15:04")}
				if rec.Result == inscontract.ResultAbnormal {
					row.Problem = checkinProblem(rec, itemsByRec[rec.ID])
				}
				marks := make([]string, len(items))
				for _, ci := range itemsByRec[rec.ID] {
					if entry.equipmentID == nil && ci.EquipmentID != nil {
						continue
					}
					if entry.equipmentID != nil && (ci.EquipmentID == nil || *ci.EquipmentID != *entry.equipmentID) {
						continue
					}
					for _, ref := range ci.Photos {
						if f, err := uploadfile.ByID(s.db, ref); err == nil && len(row.PhotoIDs) < 3 {
							row.PhotoIDs = append(row.PhotoIDs, f.ID)
						}
					}
					for j, col := range items {
						if ci.TemplateID == nil || *ci.TemplateID != col.templateID || ci.ItemCode != col.itemCode {
							continue
						}
						if ci.Result == inscontract.ItemResultEscaped {
							marks[j] = "—"
							continue
						}
						if col.tag == "" {
							marks[j] = outputItemValue(ci, col)
							continue
						}
						abn := false
						for _, tag := range ci.AbnormalTags {
							if tag == col.tag {
								abn = true
								break
							}
						}
						if abn || (ci.Result == inscontract.ItemResultAbnormal && len(ci.AbnormalTags) == 0) {
							marks[j] = "×"
						} else {
							marks[j] = "√"
						}
					}
				}
				row.Marks = marks
				dt.Rows = append(dt.Rows, row)
			}
		}
		if r.DetailMode == "abnormal" && len(dt.Rows) == 0 {
			continue
		}
		d.Details = append(d.Details, dt)
	}

	// ===== 附件：分项检查照片（v2：按设施类别分组，标注=点位名·日期[·检查项]） =====
	for _, t := range typeNames.ordered {
		group := model.PhotoGroup{Title: typeNames.label(t)}
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
							group.Cells = append(group.Cells, model.PhotoCell{Label: base + "·" + ci.Name, FileID: f.ID})
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

	// ===== 5.问题清单及整改台账（新版模板列：类别/区域位置/问题说明+故障照片/整改情况+完结照片） =====
	for i := range abnormalRecs {
		rec := &abnormalRecs[i]
		row := model.LedgerRow{Date: rec.CheckinTime.Format("01-02"), Inspector: userNames[rec.InspectorID], Problem: checkinProblem(*rec, itemsByRec[rec.ID])}
		if pt, ok := pointByID[rec.PointID]; ok {
			row.Category = typeNames.label(pt.Type)
			row.Location = pointLocation(pt)
		}
		// 问题照片：逐项照片（v21 起照片唯一归属逐项）
		for _, ci := range itemsByRec[rec.ID] {
			for _, ref := range ci.Photos {
				if f, err := uploadfile.ByID(s.db, ref); err == nil {
					row.ProblemPhotoIDs = append(row.ProblemPhotoIDs, f.ID)
				}
			}
		}
		// 处理情况列：复核结论（处置是甲方线下的事，系统不再记录处置方式）
		row.FixText = auditStatusCN(rec.AuditStatus)
		d.Ledger = append(d.Ledger, row)
	}

	// ===== 2.动态审核链签字栏（含签名图快照） =====
	for _, step := range r.ReviewSteps {
		signs := make([]model.SignInfo, 0, len(step.CandidateIDs))
		for _, uid := range step.CandidateIDs {
			entry := types.SignEntry{}
			found := false
			for _, signed := range step.Signed {
				if signed.UserID == uid {
					entry, found = signed, true
					break
				}
			}
			name := s.userName(uid)
			if found && entry.ProxyName != "" {
				name += "（" + entry.ProxyName + "代签）"
			}
			signs = append(signs, model.SignInfo{Name: name, Time: entry.SignedAt, SignatureFileID: entry.SignatureFileID})
		}
		d.ReviewSigns = append(d.ReviewSigns, model.ReviewSignGroup{Name: step.Name, Signs: signs})
	}
	return d, nil
}

// photoGroupMaxCells 单个设施类别照片上限（防单组撑爆版面）。
const photoGroupMaxCells = 48

func patrolPointIDs(tasks []inscontract.TaskFact) map[string]bool {
	set := map[string]bool{}
	for i := range tasks {
		for _, pid := range tasks[i].PointIDs {
			set[pid] = true
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
func (s *ReportService) pointTypeNames(present map[string]bool) (pointTypeNames, error) {
	rows, err := systemsvc.EnabledDictionary(s.db, "point_type")
	if err != nil {
		return pointTypeNames{}, err
	}
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
	return out, nil
}

type detailColumn struct {
	label, item, tag     string
	templateID           string
	itemCode             string
	unit                 string
	standard, standardNo string
}

func outputItemValue(item inscontract.ItemFact, column detailColumn) string {
	if item.ValueNum != nil {
		return fmt.Sprint(*item.ValueNum) + column.unit
	}
	if item.ValueDate != nil {
		return item.ValueDate.Format("2006-01-02")
	}
	if item.ValueText != nil && strings.TrimSpace(*item.ValueText) != "" {
		return strings.TrimSpace(*item.ValueText)
	}
	if item.Result == inscontract.ItemResultNormal {
		return "√"
	}
	return "×"
}

func (s *ReportService) templateColumns(pts []*inscontract.PointFact, docTemplate *model.DocTemplate, itemsByRec map[string][]inscontract.ItemFact) detailColumnSet {
	return outputTemplateColumns(s, pts, docTemplate, itemsByRec)
}

// pointLocation 位置/编号：点位名称 + 编码（qrcode_no 或 NFC 卡号）。
func pointLocation(pt *inscontract.PointFact) string {
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

// checkinProblem 异常打卡问题描述：优先异常备注；空则逐项兜底——异常项取 note，
// 逃生项「无法检查·类型」（escaped=没检成，与 abnormal 区分展示）。
func checkinProblem(rec inscontract.RecordFact, items []inscontract.ItemFact) string {
	if rec.Remark != "" {
		return rec.Remark
	}
	texts := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		var s string
		switch {
		case it.Result == inscontract.ItemResultEscaped:
			s = "无法检查·" + exceptionTypeCN(it.ExceptionType)
		case it.Result == inscontract.ItemResultAbnormal:
			s = strings.TrimSpace(it.Note)
		}
		if s != "" && !seen[s] {
			seen[s] = true
			texts = append(texts, s)
		}
	}
	if len(texts) == 0 {
		return "打卡异常"
	}
	return strings.Join(texts, "；")
}

// exceptionTypeCN 逃生类型中文（逐项「无法检查」状态文案用）。
func exceptionTypeCN(t string) string {
	switch t {
	case "device_missing":
		return "设备不存在"
	case "unable_to_capture":
		return "无法拍摄"
	case "camera_broken":
		return "相机故障"
	case "ai_failed":
		return "AI 识别失败（人工现场确认）"
	case "label_missing":
		return "标签磨损无法辨认"
	}
	return t
}

// summaryOrderKeywords 汇总表行排序关键词（甲方模板顺序），命中首个关键词的下标，未命中置后。
var summaryOrderKeywords = []string{"消火栓", "灭火器", "应急照明", "指示", "手动报警", "烟感", "温感"}

func summaryOrderKey(typeName string) int {
	for i, kw := range summaryOrderKeywords {
		if strings.Contains(typeName, kw) {
			return i
		}
	}
	return len(summaryOrderKeywords)
}

// auditStatusCN 打卡复核状态中文（台账「处理情况」列回落口径）。
func auditStatusCN(status string) string {
	switch status {
	case inscontract.AuditPending:
		return "待复核"
	case inscontract.AuditPass:
		return "复核通过"
	case inscontract.AuditRejected:
		return "复核驳回"
	case inscontract.AuditAutoPass:
		return "自动通过"
	}
	return status
}

// userNamesOf 批量反查用户姓名。
func (s *ReportService) userNamesOf(tenantID string, idSet map[string]bool) (map[string]string, error) {
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return systemsvc.UserDisplayNames(s.db, tenantID, ids)
}
