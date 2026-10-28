package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/file/model"
)

func LinkObject(tx *gorm.DB, tenantID, communityID, objectType, objectID string, fileIDs []string) error {
	if tenantID == "" || communityID == "" || objectType == "" || objectID == "" {
		return fmt.Errorf("文件业务关联缺少范围或对象身份")
	}
	link := func(fileID string) error {
		ref := model.ObjectRef{TenantID: tenantID, CommunityID: communityID, FileID: fileID, ObjectType: objectType, ObjectID: objectID}
		return tx.Where("file_id = ? AND object_type = ? AND object_id = ?", fileID, objectType, objectID).
			Assign(ref).FirstOrCreate(&ref).Error
	}
	for _, fileID := range fileIDs {
		if fileID == "" {
			continue
		}
		if err := link(fileID); err != nil {
			return err
		}
		var derivativeIDs []string
		if err := tx.Table("upload_file").Where("parent_file_id = ?", fileID).Pluck("id", &derivativeIDs).Error; err != nil {
			return err
		}
		for _, derivativeID := range derivativeIDs {
			if err := link(derivativeID); err != nil {
				return err
			}
		}
	}
	return nil
}
