package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/equipment/model"
)

func EquipmentNames(db *gorm.DB, tenantID string, ids []string) (map[string]string, error) {
	names := map[string]string{}
	if tenantID == "" {
		return nil, fmt.Errorf("设备查询缺少租户范围")
	}
	if len(ids) == 0 {
		return names, nil
	}
	var rows []model.Equipment
	if err := db.Select("id", "name").Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}
