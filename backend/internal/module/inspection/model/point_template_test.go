package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPointTemplateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&CheckTemplate{}, &CheckTemplateItem{}, &PointTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestLoadPointTemplateSets 多模板并集组装：按关联 sort（组合顺序）+ 项 sort 展开，
// 每项携模板名快照；无关联点位不出现。
func TestLoadPointTemplateSets(t *testing.T) {
	db := newPointTemplateTestDB(t)
	t1 := CheckTemplate{Name: "模板甲"}
	t2 := CheckTemplate{Name: "模板乙"}
	db.Create(&t1)
	db.Create(&t2)
	// t1 两项（逆序插入验证按 sort 排序），t2 一项
	db.Create(&CheckTemplateItem{TemplateID: t1.ID, Name: "灭火器压力", Sort: 2})
	db.Create(&CheckTemplateItem{TemplateID: t1.ID, Name: "消火栓外观", Sort: 1})
	db.Create(&CheckTemplateItem{TemplateID: t2.ID, Name: "巡检记录表", Sort: 1})
	// p1 组合顺序 t2 在前；p2 仅 t1；p3 无关联
	db.Create(&PointTemplate{PointID: "p1", TemplateID: t2.ID, Sort: 0})
	db.Create(&PointTemplate{PointID: "p1", TemplateID: t1.ID, Sort: 1})
	db.Create(&PointTemplate{PointID: "p2", TemplateID: t1.ID, Sort: 0})

	sets := LoadPointTemplateSets(db, []string{"p1", "p2", "p3"})

	s1 := sets["p1"]
	if s1 == nil {
		t.Fatal("p1 应有模板组合")
	}
	if len(s1.TemplateIDs) != 2 || s1.TemplateIDs[0] != t2.ID || s1.TemplateIDs[1] != t1.ID {
		t.Fatalf("p1 TemplateIDs 顺序错误: %v", s1.TemplateIDs)
	}
	wantNames := []string{"巡检记录表", "消火栓外观", "灭火器压力"}
	if len(s1.Items) != len(wantNames) {
		t.Fatalf("p1 并集项数错误: %d", len(s1.Items))
	}
	for i, w := range wantNames {
		if s1.Items[i].Name != w {
			t.Fatalf("p1 项 %d 应为 %s，got %s", i, w, s1.Items[i].Name)
		}
	}
	if s1.Items[0].TemplateName != "模板乙" {
		t.Fatalf("p1 首项模板快照错误: %+v", s1.Items[0])
	}
	if s1.Items[1].TemplateID != t1.ID || s1.Items[1].TemplateName != "模板甲" {
		t.Fatalf("p1 次项模板快照错误: %+v", s1.Items[1])
	}

	s2 := sets["p2"]
	if s2 == nil {
		t.Fatal("p2 应有模板组合")
	}
	if len(s2.Items) != 2 {
		t.Fatalf("p2 并集项数错误: %d", len(s2.Items))
	}

	if _, ok := sets["p3"]; ok {
		t.Fatal("p3 无关联模板，不应出现在结果中")
	}
}
