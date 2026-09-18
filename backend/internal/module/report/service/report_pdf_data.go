package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/pdf"
	"anxuncloud/internal/pkg/types"
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
		CommunityName:    s.commName(r.CommunityID),
		CommunityAddress: s.commAddress(r.CommunityID),
		Period:           r.Period,
		TitleLine:        s.reportTitleLine(r),
		CompanyName:      s.cfgString("report.company_name"),
		CompanyNameEn:    s.cfgString("report.company_name_en"),
		Contact: pdf.ContactInfo{
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
		d.SealFileID = r.SealFileID
		if d.SealFileID == "" {
			d.SealFileID = s.activeSealID(r.CommunityID)
		}
	}

	// ===== 点位与类型 =====
	var points []insmodel.InspectionPoint
	s.db.Select("id", "name", "type", "qrcode_no").
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
	// 行排序尽量贴合模板顺序：消火栓→灭火器→应急照明→指示牌→手报→烟感→其他
	sort.SliceStable(d.Summary, func(i, j int) bool {
		return summaryOrderKey(d.Summary[i].TypeName) < summaryOrderKey(d.Summary[j].TypeName)
	})

	// ===== 4.分项巡检明细（每个有点位的类型一张表） =====
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
		items := s.templateColumns(pts)
		labels := make([]string, 0, len(items))
		for _, c := range items {
			labels = append(labels, c.label)
		}
		dt := pdf.DetailTable{TypeName: typeNames.label(t), TypeCode: t, Items: labels}
		if len(labels) > 0 {
			dt.Note = "注：检查标准：" + strings.Join(labels, "；") + "。"
		}
		// 行：每点位一行（取当期最新一次有效打卡；未巡点位保留空行，明细完整覆盖应巡清单）
		// detail_mode=abnormal 时只保留异常点位行（报告厚度控制；汇总表仍为全量口径）
		for _, pt := range pts {
			row := pdf.DetailRow{Location: pointLocation(pt)}
			prs := recsByPoint[pt.ID]
			if len(prs) == 0 {
				if r.DetailMode == "abnormal" {
					continue
				}
				row.Marks = make([]string, len(items))
				row.Time = "—"
				dt.Rows = append(dt.Rows, row)
				continue
			}
			rec := prs[len(prs)-1] // recs 按 checkin_time 升序，末条即最新
			if r.DetailMode == "abnormal" && rec.Result != insmodel.ResultAbnormal {
				continue
			}
			row.Inspector = userNames[rec.InspectorID]
			row.Time = rec.CheckinTime.Format("01-02 15:04")
			marks := make([]string, len(items))
			for _, ci := range itemsByRec[rec.ID] {
				for j, col := range items {
					if ci.Name != col.item {
						continue
					}
					// 逃生态（没检成）统一标「—」，不计异常（异常只算 abnormal）
					if ci.Result == insmodel.ItemResultEscaped {
						marks[j] = "—"
						continue
					}
					if col.tag == "" {
						// 无 tag 传统项：按项结果判
						if ci.Result == insmodel.ItemResultNormal {
							marks[j] = "√"
						} else {
							marks[j] = "×"
						}
						continue
					}
					// tag 列：勾选异常 → ×；项整体异常但无 tag 明细 → 该项全部 tag 列 ×；其余 √
					abn := false
					for _, t := range ci.AbnormalTags {
						if t == col.tag {
							abn = true
							break
						}
					}
					if abn || (ci.Result == insmodel.ItemResultAbnormal && len(ci.AbnormalTags) == 0) {
						marks[j] = "×"
					} else {
						marks[j] = "√"
					}
				}
			}
			row.Marks = marks
			dt.Rows = append(dt.Rows, row)
		}
		if r.DetailMode == "abnormal" && len(dt.Rows) == 0 {
			continue // 仅异常明细模式下该类型无异常点位，不出空表
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
							group.Cells = append(group.Cells, pdf.PhotoCell{Label: base + "·" + ci.Name, FileID: f.ID})
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
		row := pdf.LedgerRow{Problem: checkinProblem(*rec, itemsByRec[rec.ID])}
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
		signs := make([]pdf.SignInfo, 0, len(step.CandidateIDs))
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
			signs = append(signs, pdf.SignInfo{Name: name, Time: entry.SignedAt, SignatureFileID: entry.SignatureFileID})
		}
		d.ReviewSigns = append(d.ReviewSigns, pdf.ReviewSignGroup{Name: step.Name, Signs: signs})
	}
	return d
}

// photoGroupMaxCells 单个设施类别照片上限（防单组撑爆版面）。
const photoGroupMaxCells = 48

// patrolPointIDs 该小区该期间内指定巡查类型任务覆盖的点位集合（以任务点位快照为准）。
func (s *ReportService) patrolPointIDs(communityID string, start, end time.Time, patrolType string) map[string]bool {
	var tasks []insmodel.InspectionTask
	s.db.Select("id", "plan_id", "point_ids").
		Where("community_id = ? AND task_date >= ? AND task_date < ? AND patrol_type = ?",
			communityID, start.Format("2006-01-02"), end.Format("2006-01-02"), patrolType).
		Find(&tasks)
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

// detailColumn 明细列定义：tag 展开的列（item+tag 定位勾选结果；tag 空=无 tag 传统项按 pass 判）。
type detailColumn struct{ label, item, tag string }

// templateColumns 明细列：第一个有模板项的点位的模板项展开——带 tags 的项逐 tag 一列（官方月报明细列口径），
// 无 tags 的项保持一项一列。
func (s *ReportService) templateColumns(pts []*insmodel.InspectionPoint) []detailColumn {
	ids := make([]string, 0, len(pts))
	for _, pt := range pts {
		ids = append(ids, pt.ID)
	}
	sets := insmodel.LoadPointTemplateSets(s.db, ids)
	for _, pt := range pts {
		set := sets[pt.ID]
		if set == nil || len(set.Items) == 0 {
			continue
		}
		var cols []detailColumn
		for _, it := range set.Items {
			if len(it.Tags) > 0 {
				for _, tag := range it.Tags {
					cols = append(cols, detailColumn{label: tag, item: it.Name, tag: tag})
				}
			} else {
				cols = append(cols, detailColumn{label: it.Name, item: it.Name})
			}
		}
		return cols
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

// checkinProblem 异常打卡问题描述：优先异常备注；空则逐项兜底——异常项取 note，
// 逃生项「无法检查·类型」（escaped=没检成，与 abnormal 区分展示）。
func checkinProblem(rec insmodel.CheckinRecord, items []insmodel.CheckinRecordItem) string {
	if rec.Remark != "" {
		return rec.Remark
	}
	texts := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		var s string
		switch {
		case it.Result == insmodel.ItemResultEscaped:
			s = "无法检查·" + exceptionTypeCN(it.ExceptionType)
		case it.Result == insmodel.ItemResultAbnormal:
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
