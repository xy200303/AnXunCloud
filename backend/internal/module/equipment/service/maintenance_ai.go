package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"anxuncloud/internal/module/equipment/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/uploadfile"

	"gorm.io/gorm"

	"go.uber.org/zap"
)

// 维保登记 AI 预检（v1.7）：提交后异步执行，输出写 ai_verdict（pass/review）+ ai_reason。
// 只标记不拦截——登记照样 pending 进确认链；AI 不可用/超时/解析失败一律留 NULL（"未预检"排后）。
// 预检内容：① 标签日期 vs 登记日期（年月容差）② 照片有效性分类（像不像维保标签/设备本体）
// ③ 钢印生产日期 vs 台账 manufacture_date（能读到才比）④ EXIF 拍摄时间偏差（suspectCheck 口径）。
// label_missing 登记只跑照片有效性（无标签可读日期）。
// v2.0 打卡融合：核验核心抽为 CheckMaintenanceLabel（Register 同步化与 mp 打卡链路共用），
// 可信自动确认的容差判定见 JudgeLabelTrust；本文件异步预检维持原"只标记不拦截"语义作兜底。

// labelDateRe 解析 AI reading 的紧凑格式：M2020-05|W2025-03 或 M无|W无（M=生产日期 W=维修日期，读不到为 无）。
var labelDateRe = regexp.MustCompile(`M(\d{4}-\d{2}|无)\|W(\d{4}-\d{2}|无)`)

// aiPreCheckPrompt 预检识别要点：读日期 + 照片有效性分类，reading 强制紧凑格式便于程序解析。
const aiPreCheckPrompt = "这是消防/物业设备的维保登记照片。任务：1) 判断照片像不像设备维保标签或设备本体（翻拍屏幕/全黑模糊/无关场景判不像）；2) 若能读到日期，读取瓶体钢印生产日期与维修标签日期（只需年月）。reading 严格输出：M{生产年月}|W{维修年月}，读不到写 无，如 M2020-05|W2025-03 或 M2020-05|W无 或 M无|W无；除 reading 外不要输出日期。"

// aiPreCheckPromptLabelMissing 标签缺失登记：只判定照片有效性（设备本体作证）。
const aiPreCheckPromptLabelMissing = "这是设备「标签缺失」登记照片（现场钢印磨损/铭牌缺失）。任务：判断照片是否为设备本体实拍（翻拍屏幕/全黑模糊/无关场景判不像）。reading 输出 M无|W无。"

// CheckMaintenanceLabel 维保标签照片核验核心（异步预检 / Register 同步化 / mp 打卡融合链路共用）：
// 大模型读标签日期（prompt 同源）+ 照片有效性分类；质量不达标、内容判不像、EXIF 拍摄时间偏差
// 超阈值（suspectCheck 口径）一律折算 review；日期比对不在此做（由调用方按场景口径判定）。
// 返回结论（pass/review）、理由、reading 原文（M/W 紧凑格式，可信判定用）、blocked——
// blocked=true 表示「明显不合格」（照片质量不达标 / 大模型判内容不像标签设备），调用方应硬拦截
// （与打卡 43107 质量拦截同口径），而不是生成待确认流水让烂照片进经理队列。
// 无可用照片/调用失败返回 err——调用方自行降级（保持 pending 兜底）。
func CheckMaintenanceLabel(ctx context.Context, aiCli *ai.Client, db *gorm.DB, pointName, pointType string, fileIDs []string, labelMissing bool) (verdict, reason, reading string, blocked bool, err error) {
	// 照片引用
	refs := make([]ai.PhotoRef, 0, len(fileIDs))
	var exifTimes []time.Time
	for _, fid := range fileIDs {
		f, ferr := uploadfile.ByID(db, fid)
		if ferr != nil {
			continue
		}
		refs = append(refs, ai.PhotoRef{URL: f.URL})
		if f.ExifTime != nil {
			exifTimes = append(exifTimes, *f.ExifTime)
		}
	}
	if len(refs) == 0 {
		return "", "", "", false, errors.New("无可用照片")
	}
	// ④ EXIF 拍摄时间偏差（复用 suspectCheck 口径：偏差超阈值记存疑点，不硬拦截——老照片补录场景存在）
	var issues []string
	limit := cfgInt(db, "inspection.exif_deviation_seconds", 300)
	for _, et := range exifTimes {
		dev := int(time.Since(et).Seconds())
		if dev < 0 {
			dev = -dev
		}
		if dev > limit {
			issues = append(issues, "照片拍摄时间与登记时间偏差过大")
			break
		}
	}
	// ①② 大模型识别（label_missing 只跑照片有效性）
	prompt := aiPreCheckPrompt
	if labelMissing {
		prompt = aiPreCheckPromptLabelMissing
	}
	res, err := aiCli.ReviewCheckin(ctx, ai.ReviewInput{
		PointName: pointName, PointType: pointType,
		CheckItems: []string{"维保标签"},
		ItemPhotos: []ai.ItemPhoto{{
			Name: "维保标签", Requirement: prompt, JudgeType: ai.JudgeLabel, Photos: refs,
		}},
	})
	if err != nil {
		return "", "", "", false, err
	}
	verdict, reason = res.Verdict, res.Reason
	rawItemVerdict := ""
	for _, iv := range res.Items {
		if iv.Name == "维保标签" {
			verdict, reason, reading = iv.Verdict, iv.Reason, iv.Reading
			rawItemVerdict = iv.Verdict
			break
		}
	}
	// 硬拦截判定：照片质量不达标（与打卡 43107 同源）或内容被判「不像」（翻拍屏幕/无关场景等）
	blocked = !res.Quality.Pass || rawItemVerdict == ai.VerdictAbnormal
	// 照片有效性：质量不达标或内容判不像 → review
	if !res.Quality.Pass {
		verdict = model.AIVerdictReview
		if res.Quality.Issue != "" {
			issues = append(issues, "照片质量："+res.Quality.Issue)
		}
	}
	if verdict == ai.VerdictAbnormal {
		verdict = model.AIVerdictReview // 核验只分 pass/review 两档
	}
	if len(issues) > 0 {
		verdict = model.AIVerdictReview
		if reason != "" {
			reason += "；"
		}
		reason += strings.Join(issues, "；")
	}
	if verdict != model.AIVerdictPass && verdict != model.AIVerdictReview {
		verdict = model.AIVerdictReview
		if reason == "" {
			reason = "AI 未给出明确结论"
		}
	}
	return verdict, reason, reading, blocked, nil
}

// JudgeLabelTrust AI 标签核验可信判定（纯函数；Register 同步化与打卡融合链路共用，方案决策 4）：
// 可信 = 结论 pass 且读出维修年月（W）与维保日期年月差 ≤1 个月 → 自动 confirmed + 回写台账；
// 读出生产年月（M）与台账出厂年月不符 → 不可信且标注「疑似设备更换」（转人工，replace 闭环二期）；
// 其余（review/W 读不出/解析失败/AI 未启用/超时）→ 兜底转人工。
func JudgeLabelTrust(verdict, reading string, maintDate time.Time, manufacture *time.Time) (trusted, suspectReplace bool, note string) {
	if verdict != model.AIVerdictPass {
		return false, false, ""
	}
	m := labelDateRe.FindStringSubmatch(strings.TrimSpace(reading))
	if m == nil || m[2] == "无" {
		return false, false, "" // 维修年月读不出 → 兜底人工
	}
	w, err := time.ParseInLocation("2006-01", m[2], time.Local)
	if err != nil || monthsDiff(w, maintDate) > 1 {
		return false, false, ""
	}
	// 钢印生产年月 vs 台账出厂年月（能读到且台账有数据才比）
	if m[1] != "无" && manufacture != nil {
		if t, err := time.ParseInLocation("2006-01", m[1], time.Local); err == nil &&
			(t.Year() != manufacture.Year() || t.Month() != manufacture.Month()) {
			return false, true, "疑似设备更换：钢印生产日期 " + m[1] + " 与台账出厂日期 " + manufacture.Format("2006-01") + " 不一致"
		}
	}
	return true, false, ""
}

// monthsDiff 年月差绝对值（按月计，忽略日）。
func monthsDiff(a, b time.Time) int {
	d := (a.Year()-b.Year())*12 + int(a.Month()) - int(b.Month())
	if d < 0 {
		return -d
	}
	return d
}

// aiPreCheck 异步预检入口（同步核验失败/标签缺失登记时的兜底；任何失败仅记日志，绝不影响登记）。
func (s *MaintenanceService) aiPreCheck(recID string) {
	if s.aiCli == nil || !s.aiCli.Enabled() {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("维保登记 AI 预检 panic", zap.String("rec_id", recID), zap.Any("panic", r))
		}
	}()
	var m model.EquipmentMaintenance
	if err := s.db.First(&m, "id = ?", recID).Error; err != nil {
		return
	}
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", m.EquipmentID).Error; err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	verdict, reason, reading, _, err := CheckMaintenanceLabel(ctx, s.aiCli, s.db, e.Name, e.Type, m.FileIDs, m.LabelMissing)
	if err != nil {
		logger.L.Warn("维保登记 AI 预检调用失败，留 NULL", zap.String("rec_id", recID), zap.Error(err))
		return
	}
	// ①③ 标签维修日期 vs 登记日期、钢印生产日期 vs 台账出厂日期（年月精确口径，只标记不拦截；
	// 可信自动确认的 ≤1 个月容差判定是另一链路，见 JudgeLabelTrust）
	if !m.LabelMissing {
		var issues []string
		if m2 := labelDateRe.FindStringSubmatch(strings.TrimSpace(reading)); m2 != nil {
			if m2[2] != "无" {
				if t, err := time.ParseInLocation("2006-01", m2[2], time.Local); err == nil &&
					(t.Year() != m.MaintenanceDate.Year() || t.Month() != m.MaintenanceDate.Month()) {
					verdict = model.AIVerdictReview
					issues = append(issues, "标签维修日期 "+m2[2]+" 与登记日期 "+m.MaintenanceDate.Format("2006-01")+" 不一致")
				}
			}
			if m2[1] != "无" && e.ManufactureDate != nil {
				if t, err := time.ParseInLocation("2006-01", m2[1], time.Local); err == nil &&
					(t.Year() != e.ManufactureDate.Year() || t.Month() != e.ManufactureDate.Month()) {
					verdict = model.AIVerdictReview
					issues = append(issues, "钢印生产日期 "+m2[1]+" 与台账出厂日期 "+e.ManufactureDate.Format("2006-01")+" 不一致（防换设备）")
				}
			}
		} else if verdict != model.AIVerdictReview {
			// 读不到日期格式：不因此判 review（钢印格式五花八门），仅记录
			if strings.TrimSpace(reading) != "" {
				issues = append(issues, "日期识别结果："+truncateStr2(reading, 60))
			}
		}
		if len(issues) > 0 {
			verdict = model.AIVerdictReview
			if reason != "" {
				reason += "；"
			}
			reason += strings.Join(issues, "；")
		}
	}
	if err := s.db.Model(&m).Updates(map[string]any{
		"ai_verdict": verdict,
		"ai_reason":  truncateStr2(reason, 500),
	}).Error; err != nil {
		logger.L.Warn("维保登记 AI 预检回写失败", zap.String("rec_id", recID), zap.Error(err))
	}
}

// truncateStr2 截断字符串（本包内工具，避免依赖 mp 包）。
func truncateStr2(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n])
}
