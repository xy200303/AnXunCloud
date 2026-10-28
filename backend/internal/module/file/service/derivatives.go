package service

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"gorm.io/gorm"

	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/types"
)

func RegisterDerivative(db *gorm.DB, store *storage.Storage, original *sysmodel.UploadFile, role, suffix string, data []byte, mime string, meta types.JSONMap) (*sysmodel.UploadFile, error) {
	if original == nil || original.ID == "" || store == nil {
		return nil, fmt.Errorf("派生文件缺少原件或存储")
	}
	if role != "cleaned" && role != "watermarked" && role != "thumbnail" {
		return nil, fmt.Errorf("派生文件类型无效")
	}
	key := strings.TrimSuffix(original.StorageKey, filepath.Ext(original.StorageKey)) + suffix
	if err := store.PutBytes(key, data); err != nil {
		return nil, err
	}
	child := &sysmodel.UploadFile{
		TenantID: original.TenantID, StorageKey: key, Scene: original.Scene, UserID: original.UserID,
		MimeType: mime, URL: store.URL(key), Name: strings.TrimSuffix(original.Name, filepath.Ext(original.Name)) + suffix,
		MD5: storage.MD5Hex(data), Size: int64(len(data)), Storage: store.DriverName(),
		ParentFileID: &original.ID, EvidenceRole: role, SHA256: sha256Hex(data), DerivativeMeta: meta,
	}
	if err := db.Create(child).Error; err != nil {
		return nil, err
	}
	var refs []filemodel.ObjectRef
	if err := db.Where("file_id = ?", original.ID).Find(&refs).Error; err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if err := LinkObject(db, ref.TenantID, ref.CommunityID, ref.ObjectType, ref.ObjectID, []string{child.ID}); err != nil {
			return nil, err
		}
	}
	return child, nil
}

func (s *UploadService) createCleanedDerivative(original *sysmodel.UploadFile, data []byte) error {
	if original == nil {
		return fmt.Errorf("原始证据不存在")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(original.StorageKey)), ".")
	if !imageExts[ext] || ext == "heic" {
		return nil
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	if img.Bounds().Dx() > 4096 || img.Bounds().Dy() > 4096 {
		img = imaging.Fit(img, 4096, 4096, imaging.Lanczos)
	}
	var output bytes.Buffer
	if err := imaging.Encode(&output, img, imaging.JPEG, imaging.JPEGQuality(90)); err != nil {
		return err
	}
	child, err := RegisterDerivative(s.db, s.store, original, "cleaned", "_clean.jpg", output.Bytes(), "image/jpeg", types.JSONMap{
		"exif": "removed", "orientation": "normalized", "max_side": 4096,
	})
	if err != nil {
		return err
	}
	return s.db.Model(original).Update("watermarked_url", child.URL).Error
}
