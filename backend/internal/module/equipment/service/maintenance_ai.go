package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"anxuncloud/internal/module/equipment/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
)

// 维保登记 AI 预检（v1.7）：提交后异步执行，输出写 ai_verdict（pass/review）+ ai_reason。
// 只标记不拦截——登记照样 pending 进确认链；AI 不可用/超时/解析失败一律留 NULL（"未预检"排后）。
// 预检内容：① 标签日期 vs 登记日期（年月容差）② 照片有效性分类（像不像维保标签/设备本体）
// ③ 钢印生产日期 vs 台账 manufacture_date（能读到才比）④ EXIF 拍摄时间偏差（suspectCheck 口径）。
// label_missing 登记只跑照片有效性（无标签可读日期）。

// labelDateRe 解析 AI reading 的紧凑格式：M2020-05|W2025-03 或 M2020-05|W无（M=生产日期 W=维修日期）。
var labelDateRe = regexp.MustCompile(`M(\d{4}-\d{2})\|W(\d{4}-\d{2}|无)`)

// aiPreCheckPrompt 预检识别要点：读日期 + 照片有效性分类，reading 强制紧凑格式便于程序解析。
const aiPreCheckPrompt = "这是消防/物业设备的维保登记照片。任务：1) 判断照片像不像设备维保标签或设备本体（翻拍屏幕/全黑模糊/无关场景判不像）；2) 若能读到日期，读取瓶体钢印生产日期与维修标签日期（只需年月）。reading 严格输出：M{生产年月}|W{维修年月}，读不到写 无，如 M2020-05|W2025-03 或 M2020-05|W无 或 M无|W无；除 reading 外不要输出日期。"

// aiPreCheckPromptLabelMissing 标签缺失登记：只判定照片有效性（设备本体作证）。
const aiPreCheckPromptLabelMissing = "这是设备「标签缺失」登记照片（现场钢印磨损/铭牌缺失）。任务：判断照片是否为设备本体实拍（翻拍屏幕/全黑模糊/无关场景判不像）。reading 输出 M无|W无。"

// aiPreCheck 异步预检入口（Register 提交后 go 调用；任何失败仅记日志，绝不影响登记）。
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
	// 照片引用
	refs := make([]ai.PhotoRef, 0, len(m.FileIDs))
	var exifTimes []time.Time
	for _, fid := range m.FileIDs {
		f, err := uploadfile.ByID(s.db, fid)
		if err != nil {
			continue
		}
		refs = append(refs, ai.PhotoRef{URL: f.URL})
		if f.ExifTime != nil {
			exifTimes = append(exifTimes, *f.ExifTime)
		}
	}
	if len(refs) == 0 {
		return
	}
	// ④ EXIF 拍摄时间偏差（复用 suspectCheck 口径：偏差超阈值记存疑点）
	var issues []string
	limit := cfgInt(s.db, "inspection.exif_deviation_seconds", 300)
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
	// ①②③ 大模型识别（label_missing 只跑照片有效性）
	prompt := aiPreCheckPrompt
	if m.LabelMissing {
		prompt = aiPreCheckPromptLabelMissing
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := s.aiCli.ReviewCheckin(ctx, ai.ReviewInput{
		PointName: e.Name, PointType: e.Type,
		CheckItems: []string{"维保标签"},
		ItemPhotos: []ai.ItemPhoto{{
			Name: "维保标签", Requirement: prompt, JudgeType: ai.JudgeLabel, Photos: refs,
		}},
	})
	if err != nil {
		logger.L.Warn("维保登记 AI 预检调用失败，留 NULL", zap.String("rec_id", recID), zap.Error(err))
		return
	}
	verdict, reason, reading := res.Verdict, res.Reason, ""
	for _, iv := range res.Items {
		if iv.Name == "维保标签" {
			verdict, reason, reading = iv.Verdict, iv.Reason, iv.Reading
			break
		}
	}
	// 照片有效性：质量不达标或内容判不像 → review
	if !res.Quality.Pass {
		verdict = model.AIVerdictReview
		if res.Quality.Issue != "" {
			issues = append(issues, "照片质量："+res.Quality.Issue)
		}
	}
	if verdict == ai.VerdictAbnormal {
		verdict = model.AIVerdictReview // 预检只分 pass/review 两档
	}
	if !m.LabelMissing {
		if m2 := labelDateRe.FindStringSubmatch(strings.TrimSpace(reading)); m2 != nil {
			// ① 标签维修日期 vs 登记日期（年月容差）
			if m2[2] != "无" {
				if t, err := time.ParseInLocation("2006-01", m2[2], time.Local); err == nil &&
					(t.Year() != m.MaintenanceDate.Year() || t.Month() != m.MaintenanceDate.Month()) {
					verdict = model.AIVerdictReview
					issues = append(issues, "标签维修日期 "+m2[2]+" 与登记日期 "+m.MaintenanceDate.Format("2006-01")+" 不一致")
				}
			}
			// ③ 钢印生产日期 vs 台账 manufacture_date（能读到且台账有数据才比）
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
	}
	if len(issues) > 0 {
		verdict = model.AIVerdictReview
		if reason != "" {
			reason = reason + "；"
		}
		reason += strings.Join(issues, "；")
	}
	if verdict != model.AIVerdictPass && verdict != model.AIVerdictReview {
		verdict = model.AIVerdictReview
		if reason == "" {
			reason = "AI 未给出明确结论"
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
