package service

import (
	"testing"
	"time"

	insmodel "anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/types"
)

func TestOutputRecordExpansionPreservesInstancesAndEquipment(t *testing.T) {
	now := time.Now()
	equipmentA, equipmentB := "equipment-a", "equipment-b"
	records := []insmodel.RecordFact{{CheckinTime: now}, {CheckinTime: now.Add(time.Hour)}}
	records[0].ID, records[1].ID = "record-a", "record-b"
	items := map[string][]insmodel.ItemFact{
		"record-a": {{RecordID: "record-a", EquipmentID: &equipmentA}},
		"record-b": {{RecordID: "record-b", EquipmentID: &equipmentA}, {RecordID: "record-b", EquipmentID: &equipmentB}},
	}
	if rows := expandOutputRecords("date", records, items, nil); len(rows) != 2 {
		t.Fatalf("同日多轮记录被吞掉: %d", len(rows))
	}
	rows := expandOutputRecords("equipment", records, items, map[string]string{equipmentA: "灭火器A", equipmentB: "灭火器B"})
	if len(rows) != 3 || rows[0].record.ID != "record-a" || rows[1].record.ID != "record-b" || rows[2].record.ID != "record-b" {
		t.Fatalf("设备实例展开错误: %+v", rows)
	}
	if rows[1].equipmentID == nil || *rows[1].equipmentID != equipmentA || rows[1].suffix != " · 设备 灭火器A" {
		t.Fatalf("设备身份或名称错误: %+v", rows[1])
	}
}

func TestOutputItemValue(t *testing.T) {
	number := 0.8
	text := "  正常  "
	date := time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local)
	column := detailColumn{unit: "MPa", standard: "压力在允许区间", standardNo: "2"}
	if got := outputItemValue(insmodel.ItemFact{ValueNum: &number}, column); got != "0.8MPa" {
		t.Fatalf("数值输出错误: %s", got)
	}
	if got := outputItemValue(insmodel.ItemFact{ValueText: &text}, detailColumn{}); got != "正常" {
		t.Fatalf("文本输出错误: %q", got)
	}
	if got := outputItemValue(insmodel.ItemFact{ValueDate: &date}, detailColumn{}); got != "2026-10-27" {
		t.Fatalf("日期输出错误: %s", got)
	}
}

func TestFilterVolumeDataDoesNotDuplicateReviewSigns(t *testing.T) {
	data := model.MonthlyReportData{ReviewSigns: []model.ReviewSignGroup{{Name: "现场确认"}, {Name: "主管审核"}, {Name: "月报审批"}}}
	template := model.DocTemplate{Layout: types.JSONMap{"sign_slots": []any{
		map[string]any{"step_index": float64(0)}, map[string]any{"step_index": float64(1)},
	}}}
	filtered := filterVolumeData(data, template, map[string]string{}, map[string]string{})
	if len(filtered.ReviewSigns) != 2 || filtered.ReviewSigns[0].Name != "现场确认" || filtered.ReviewSigns[1].Name != "主管审核" {
		t.Fatalf("签字位被重复或错误过滤: %+v", filtered.ReviewSigns)
	}
}
