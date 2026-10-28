package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

func UserDisplayNames(db *gorm.DB, tenantID string, ids []string) (map[string]string, error) {
	names := make(map[string]string, len(ids))
	if tenantID == "" {
		return nil, fmt.Errorf("用户查询缺少租户范围")
	}
	if len(ids) == 0 {
		return names, nil
	}
	var users []model.SysUser
	if err := db.Select("id", "name").Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		names[user.ID] = user.Name
	}
	return names, nil
}

type DictRef struct {
	Label string
	Value string
	Sort  int
	Attrs map[string]any
}

func EnabledDictionary(db *gorm.DB, typeCode string) ([]DictRef, error) {
	if typeCode == "" {
		return nil, fmt.Errorf("字典查询缺少类型")
	}
	var rows []model.SysDictData
	if err := db.Where("type_code = ? AND status = ?", typeCode, model.StatusEnabled).Order("sort, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	refs := make([]DictRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, DictRef{Label: row.Label, Value: row.Value, Sort: row.Sort, Attrs: row.Attrs})
	}
	return refs, nil
}
