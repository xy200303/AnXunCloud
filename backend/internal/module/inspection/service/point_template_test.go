package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
)

// TestReplacePointTemplates point_template 整表替换保存：先删后插、sort 按入参顺序、空串/重复 id 去重、空列表清空。
func TestReplacePointTemplates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PointTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	load := func() []model.PointTemplate {
		var rows []model.PointTemplate
		db.Where("point_id = ?", "pt1").Order("sort ASC").Find(&rows)
		return rows
	}

	// 初次写入：顺序即 sort
	if err := replacePointTemplates(db, "pt1", []string{"tA", "tB"}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	rows := load()
	if len(rows) != 2 || rows[0].TemplateID != "tA" || rows[1].TemplateID != "tB" || rows[0].Sort != 0 || rows[1].Sort != 1 {
		t.Fatalf("首次写入结果错误: %+v", rows)
	}

	// 整表替换：旧行清除，新顺序生效
	if err := replacePointTemplates(db, "pt1", []string{"tB", "tC"}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	rows = load()
	if len(rows) != 2 || rows[0].TemplateID != "tB" || rows[1].TemplateID != "tC" {
		t.Fatalf("替换结果错误: %+v", rows)
	}

	// 去重与空串剔除
	if err := replacePointTemplates(db, "pt1", []string{"tD", "", "tD"}); err != nil {
		t.Fatalf("去重写入失败: %v", err)
	}
	rows = load()
	if len(rows) != 1 || rows[0].TemplateID != "tD" {
		t.Fatalf("去重结果错误: %+v", rows)
	}

	// 空列表 = 清空关联
	if err := replacePointTemplates(db, "pt1", nil); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if rows := load(); len(rows) != 0 {
		t.Fatalf("清空后仍有 %d 行", len(rows))
	}

	// 不影响其他点位的关联行
	if err := replacePointTemplates(db, "pt2", []string{"tA"}); err != nil {
		t.Fatalf("pt2 写入失败: %v", err)
	}
	if err := replacePointTemplates(db, "pt1", []string{"tA"}); err != nil {
		t.Fatalf("pt1 重写失败: %v", err)
	}
	var n int64
	db.Model(&model.PointTemplate{}).Where("point_id = ?", "pt2").Count(&n)
	if n != 1 {
		t.Fatalf("pt2 关联行被误清: %d", n)
	}
}
