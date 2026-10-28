package service

import (
	"testing"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/pkg/ooxml"
)

func TestCoveredNormalUsesAssignedTableAndProblemWins(t *testing.T) {
	raw, _, _ := ooxml.BuiltinTemplate("acceptance_building")
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	target := model.Target{CategoryCode: "building", TableCodes: []string{"building_1_exterior"}, CoverageConfirmed: true}
	target.ID = "target-1"
	names := map[string]string{"building_1_exterior\x00wall_finish": "外墙饰面"}
	svc := &BatchService{}
	if err := svc.fillCoveredNormal(mother, []model.Target{target}, nil, names, "building"); err != nil {
		t.Fatal(err)
	}
	got, err := mother.CellText(4, 7, 1)
	if err != nil || got != "√" {
		t.Fatalf("已覆盖无问题项应为合格: %q %v", got, err)
	}
	problem := model.Record{TargetID: target.ID, TableCode: "building_1_exterior", ItemCode: "wall_finish", Result: "problem", Note: "外墙破损"}
	if err := svc.fillCoveredNormal(mother, []model.Target{target}, []model.Record{problem}, names, "building"); err != nil {
		t.Fatal(err)
	}
	if err := svc.fillAcceptanceRecord(mother, acceptanceTableSlots[problem.TableCode], problem, names); err != nil {
		t.Fatal(err)
	}
	got, err = mother.CellText(4, 7, 1)
	if err != nil || got != "×" {
		t.Fatalf("问题事实必须覆盖默认合格: %q %v", got, err)
	}
}

func TestHandoverProblemValidationRequiresEvidence(t *testing.T) {
	fileID := "acceptance-file"
	problems := []model.Problem{{Fact: "外墙重大风险", RiskLevel: "major"}}
	if be := validateHandoverProblemRows(problems); be == nil {
		t.Fatal("未填写措施必须阻断")
	}
	problems[0].SafetyMeasure = "设置围挡并暂停使用"
	if be := validateHandoverProblemRows(problems); be == nil {
		t.Fatal("重大风险措施未获批必须阻断")
	}
	problems[0].MeasureApproved = true
	if be := validateHandoverProblemRows(problems); be == nil {
		t.Fatal("无接收方签署文件必须阻断")
	}
	problems[0].Accepted = true
	problems[0].AcceptedBy = "接收方代表"
	problems[0].AcceptanceFileID = &fileID
	if be := validateHandoverProblemRows(problems); be != nil {
		t.Fatalf("措施获批且接收证据齐全应通过: %v", be)
	}
}
