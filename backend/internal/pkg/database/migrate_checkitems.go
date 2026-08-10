package database

import (
	"strings"

	"gorm.io/gorm"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/types"
)

// extinguisherRequirements 灭火器演示模板各检查项的检查标准文案（搬迁存量时随项行写入）。
var extinguisherRequirements = map[string]string{
	"压力表指针在绿区": "指针处于绿色区域为正常；红区欠压需充装，黄区超压需检修",
	"瓶体无锈蚀变形":   "瓶体无锈蚀、无变形、无明显机械损伤，铭牌清晰可辨",
	"保险销完好":     "保险销与铅封完好，无脱落、无拆动痕迹",
	"喷管无龟裂":     "喷管无龟裂、无老化、无堵塞，连接牢固",
}

// migrateCheckItemData v18 存量数据搬迁（幂等，可随启动重复执行）：
//  1. check_template.items JSONB → check_template_item 行（按数组顺序定 sort；灭火器演示模板补 requirement 文案）；
//  2. checkin_record.check_items JSONB → checkin_record_item 行（快照 name/requirement/photo_required；
//     requirement/photo_required/template_item_id 按点位当前模板项 name 匹配补全，匹配不到按 none/NULL）。
//
// 幂等判定：目标表已存在该 template_id / record_id 的任一行即跳过该条搬迁。
func migrateCheckItemData(db *gorm.DB) error {
	if err := migrateTemplateItems(db); err != nil {
		return err
	}
	return migrateRecordItems(db)
}

func migrateTemplateItems(db *gorm.DB) error {
	var tpls []insmodel.CheckTemplate
	if err := db.Select("id", "name", "items").Where("deleted_at IS NULL").Find(&tpls).Error; err != nil {
		return err
	}
	for i := range tpls {
		t := &tpls[i]
		if len(t.Items) == 0 {
			continue
		}
		var count int64
		if err := db.Model(&insmodel.CheckTemplateItem{}).Where("template_id = ?", t.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue // 已搬迁
		}
		rows := make([]insmodel.CheckTemplateItem, 0, len(t.Items))
		extinguisher := strings.HasPrefix(t.Name, "灭火器")
		for j, it := range t.Items {
			pr := it.PhotoRequired
			if pr == "" {
				pr = types.PhotoReqNone
			}
			row := insmodel.CheckTemplateItem{
				TemplateID: t.ID, Name: it.Name, Required: it.Required,
				PhotoRequired: pr, Sort: j,
			}
			if extinguisher {
				if req, ok := extinguisherRequirements[it.Name]; ok {
					row.Requirement = &req
				}
			}
			rows = append(rows, row)
		}
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateRecordItems(db *gorm.DB) error {
	var recs []insmodel.CheckinRecord
	if err := db.Select("id", "point_id", "check_items").
		Where("check_items <> '[]'::jsonb").Find(&recs).Error; err != nil {
		return err
	}
	// point_id → template_id 与 template_id → 当前模板项（name 匹配补全血缘/要求）
	pointTpl := map[string]*string{}
	tplItems := map[string][]insmodel.CheckTemplateItem{}
	loadTplItems := func(tplID string) []insmodel.CheckTemplateItem {
		if items, ok := tplItems[tplID]; ok {
			return items
		}
		var items []insmodel.CheckTemplateItem
		db.Where("template_id = ?", tplID).Order("sort ASC").Find(&items)
		tplItems[tplID] = items
		return items
	}
	for i := range recs {
		r := &recs[i]
		var count int64
		if err := db.Model(&insmodel.CheckinRecordItem{}).Where("record_id = ?", r.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue // 已搬迁
		}
		tplID, ok := pointTpl[r.PointID]
		if !ok {
			var pt insmodel.InspectionPoint
			if err := db.Select("template_id").First(&pt, "id = ?", r.PointID).Error; err == nil {
				tplID = pt.TemplateID
			}
			pointTpl[r.PointID] = tplID
		}
		var items []insmodel.CheckTemplateItem
		if tplID != nil && *tplID != "" {
			items = loadTplItems(*tplID)
		}
		byName := map[string]*insmodel.CheckTemplateItem{}
		for j := range items {
			byName[items[j].Name] = &items[j]
		}
		rows := make([]insmodel.CheckinRecordItem, 0, len(r.CheckItems))
		for j, ci := range r.CheckItems {
			row := insmodel.CheckinRecordItem{
				RecordID: r.ID, Name: ci.Name, Pass: ci.Pass, Note: ci.Note,
				PhotoRequired: types.PhotoReqNone, Photos: ci.Photos, Sort: j,
			}
			if ti, ok := byName[ci.Name]; ok {
				row.TemplateItemID = &ti.ID
				row.Requirement = ti.Requirement
				row.PhotoRequired = ti.PhotoRequired
			}
			rows = append(rows, row)
		}
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}
