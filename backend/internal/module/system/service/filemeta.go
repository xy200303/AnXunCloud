package service

import (
	"crypto/sha256"
	"encoding/hex"

	"gorm.io/gorm"

	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/types"

	"go.uber.org/zap"
)

// SystemFileOwner 服务端生成文件的上传者占位（零 UUID，upload_file.user_id 非空约束）。
const SystemFileOwner = "00000000-0000-0000-0000-000000000000"

func RegisterGeneratedFile(db *gorm.DB, store *storage.Storage, tenantID *string, communityID, objectType, objectID, filename, mime, md5, key, url string) string {
	return RegisterGeneratedFileScene(db, store, tenantID, communityID, objectType, objectID, "export", filename, mime, md5, key, url)
}

func RegisterGeneratedFileScene(db *gorm.DB, store *storage.Storage, tenantID *string, communityID, objectType, objectID, scene, filename, mime, md5, key, url string) string {
	if tenantID == nil || *tenantID == "" || communityID == "" || objectType == "" || objectID == "" {
		logger.L.Warn("生成文件缺少租户或业务关联", zap.String("storage_key", key))
		return ""
	}
	data, err := store.ReadFile(key)
	if err != nil {
		logger.L.Warn("生成文件读取摘要失败", zap.String("storage_key", key), zap.Error(err))
		return ""
	}
	digest := sha256.Sum256(data)
	rec := sysmodel.UploadFile{
		TenantID: tenantID, StorageKey: key, Scene: scene, UserID: SystemFileOwner,
		MimeType: mime, URL: url, EvidenceRole: "original", SHA256: hex.EncodeToString(digest[:]), DerivativeMeta: types.JSONMap{},
		Name: filename, MD5: md5, Storage: store.DriverName(),
	}
	if err := db.Create(&rec).Error; err != nil {
		logger.L.Warn("生成文件登记 upload_file 失败", zap.String("storage_key", key), zap.Error(err))
		return ""
	}
	ref := filemodel.ObjectRef{TenantID: *tenantID, CommunityID: communityID, FileID: rec.ID, ObjectType: objectType, ObjectID: objectID}
	if err := db.Create(&ref).Error; err != nil {
		logger.L.Warn("生成文件登记业务关联失败", zap.String("file_id", rec.ID), zap.Error(err))
		return ""
	}
	return rec.ID
}
