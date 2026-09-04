package service

import (
	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/storage"

	"go.uber.org/zap"
)

// SystemFileOwner 服务端生成文件的上传者占位（零 UUID，upload_file.user_id 非空约束）。
const SystemFileOwner = "00000000-0000-0000-0000-000000000000"

// RegisterGeneratedFile 登记服务端生成的文件（月报 PDF、二维码包、统计导出）到 upload_file，
// 统一文件层（/api/files）据此检索原始文件名与摘要；登记失败仅记日志，不阻断主流程。
func RegisterGeneratedFile(db *gorm.DB, store *storage.Storage, filename, mime, md5, key, url string) string {
	rec := sysmodel.UploadFile{
		StorageKey: key, Scene: "export", UserID: SystemFileOwner,
		MimeType: mime, URL: url,
		Name: filename, MD5: md5, Storage: store.DriverName(),
	}
	if err := db.Create(&rec).Error; err != nil {
		logger.L.Warn("生成文件登记 upload_file 失败", zap.String("storage_key", key), zap.Error(err))
	}
	return rec.ID
}
