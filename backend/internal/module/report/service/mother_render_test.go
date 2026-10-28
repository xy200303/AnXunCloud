package service

import (
	"strings"
	"testing"

	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/ooxml"
)

func TestFillMotherRecordsCombinesPointAndLocationNote(t *testing.T) {
	raw, _, ok := ooxml.BuiltinTemplate("fire_equipment")
	if !ok {
		t.Fatal("缺少器材类母版")
	}
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	profile := motherRecordProfiles["fire_equipment"][2]
	detail := model.DetailTable{
		ColumnKeys: []string{"location_note"},
		Rows:       []model.DetailRow{{Location: "1栋消火栓", Marks: []string{"一层东侧"}, Inspector: "inspector", Time: "10-28 10:00"}},
	}
	if err := fillMotherRecords(mother, profile, detail, nil); err != nil {
		t.Fatal(err)
	}
	location, err := mother.CellText(profile.table, 1, profile.locationColumn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(location, "1栋消火栓") || !strings.Contains(location, "一层东侧") {
		t.Fatalf("位置事实被覆盖: %q", location)
	}
}
