// Package service 统一文件层（/api/files）：跨 admin/app/mp 通道的上传与下载。
// 鉴权：AuthAny（任一通道合法会话）；下载权限按 scene 规则判定（见 checkRead）。
package service

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/file/dto"
	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/exifutil"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

// FileService 统一文件服务。
type FileService struct {
	db     *gorm.DB
	store  *storage.Storage
	upload *UploadService // 复用 STS 直传签发
}

func NewFileService(db *gorm.DB, store *storage.Storage, upload *UploadService) *FileService {
	return &FileService{db: db, store: store, upload: upload}
}

// 上传 scene 白名单（各端合集）：图片类走全局扩展名，notice 放宽到办公文档
var uploadScenes = map[string]bool{
	"checkin": true, "avatar": true, "signature": true, "seal": true, "notice": true, "doc_template": true,
}

var noticeExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "heic": true,
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true,
	"ppt": true, "pptx": true, "txt": true, "zip": true,
}

var imageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "heic": true,
}

// STS 直传凭证签发（复用上传服务的 local/云存储分支逻辑）。
func (s *FileService) STS(userID string, req *dto.STSReq) (gin.H, *errs.Error) {
	return s.upload.STS(userID, req)
}

func (s *FileService) Preflight(userID string, req *dto.STSReq) (gin.H, *errs.Error) {
	return s.upload.Preflight(userID, req)
}

// Upload 统一上传（multipart：file + scene）。登记 upload_file 元数据（name/md5/storage）。
func (s *FileService) Upload(c *gin.Context, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	if !uploadScenes[scene] {
		return nil, errs.ErrParam.WithMsg("scene 取值非法")
	}
	var ext string
	var be *errs.Error
	if scene == "notice" || scene == "doc_template" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
		if !noticeExts[ext] || (scene == "doc_template" && ext != "docx") {
			return nil, errs.ErrUploadType.WithMsg("母版仅支持 DOCX 文件")
		}
	} else {
		ext, be = s.store.CheckExt(filename)
		if be != nil {
			return nil, be
		}
	}
	if size > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	uid := middleware.CurrentUserID(c)
	if be := s.upload.AuthorizeScene(uid, scene); be != nil {
		return nil, be
	}
	data, err := io.ReadAll(io.LimitReader(r, s.store.MaxFileSize()+1))
	if err != nil || int64(len(data)) > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	if be := validateUploadContent(scene, ext, data); be != nil {
		return nil, be
	}
	tenantID := s.upload.UserTenantID(uid)
	if existing, ok := s.upload.FindReusable(scene, storage.MD5Hex(data), int64(len(data)), tenantID, uid); ok {
		return gin.H{"file_id": existing.ID, "url": existing.URL, "name": existing.Name, "md5": existing.MD5}, nil
	}
	key, url, storedData, md5, err := s.store.Save(scene, uid, ext, bytes.NewReader(data))
	if err != nil {
		return nil, errs.ErrInternal
	}
	mime := "image/" + ext
	if (scene == "notice" || scene == "doc_template") && !imageExts[ext] {
		mime = mimeByExt(ext)
	}
	var exifTime *time.Time
	if ext == "jpg" || ext == "jpeg" {
		exifTime = exifutil.ReadShotTimeBytes(storedData)
	}
	rec := sysmodel.UploadFile{
		TenantID: tenantID, StorageKey: key, Scene: scene, UserID: uid,
		MimeType: mime, URL: url, ExifTime: exifTime,
		Name: filepath.Base(filename), MD5: md5, Size: int64(len(data)), Storage: s.store.DriverName(),
		EvidenceRole: "original", SHA256: sha256Hex(data), DerivativeMeta: types.JSONMap{},
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if err := s.upload.createCleanedDerivative(&rec, data); err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_id": rec.ID, "url": url, "name": rec.Name, "md5": rec.MD5}, nil
}

// Download 统一下载/预览：支持文件 UUID 或存储键，完成租户和业务授权后读取受控字节。
func (s *FileService) Download(c *gin.Context, key string) (data []byte, redirect, name, mime string, be *errs.Error) {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return nil, "", "", "", errs.ErrNotFound
	}
	rec, err := uploadfile.ByID(s.db, key)
	if err != nil {
		if err := s.db.Where("storage_key = ?", key).First(&rec).Error; err != nil {
			return nil, "", "", "", errs.ErrNotFound
		}
	}
	if be := s.checkRead(c, &rec); be != nil {
		return nil, "", "", "", be
	}
	if rec.EvidenceRole == "original" {
		if c.Query("original") == "1" {
			identity := middleware.CurrentIdentity(c)
			allowed, err := authz.EnforceAny(identity.UserID, "file:evidence:download")
			if err != nil {
				return nil, "", "", "", errs.ErrInternal
			}
			if !allowed {
				return nil, "", "", "", errs.ErrNoPerm
			}
		} else {
			var derivative sysmodel.UploadFile
			if err := s.db.Where("parent_file_id = ? AND evidence_role IN ?", rec.ID, []string{"watermarked", "cleaned"}).
				Order("CASE evidence_role WHEN 'watermarked' THEN 0 ELSE 1 END").Order("created_at DESC").First(&derivative).Error; err == nil {
				if be := s.checkRead(c, &derivative); be != nil {
					return nil, "", "", "", be
				}
				rec = derivative
			}
		}
	}
	mime = rec.MimeType
	if mime == "" {
		mime = mimeByExt(strings.TrimPrefix(strings.ToLower(filepath.Ext(rec.StorageKey)), "."))
	}
	data, err = s.store.ReadFile(rec.StorageKey)
	if err != nil {
		return nil, "", "", "", errs.ErrNotFound
	}
	return data, "", rec.Name, mime, nil
}

func (s *FileService) checkRead(c *gin.Context, rec *sysmodel.UploadFile) *errs.Error {
	id := middleware.CurrentIdentity(c)
	if id == nil {
		return errs.ErrUnauthorized
	}
	tenantID := id.TenantID
	if id.SuperAdmin {
		var be *errs.Error
		tenantID, be = middleware.ExplicitTenantID(c, s.db)
		if be != nil {
			return be
		}
	}
	if tenantID == "" || rec.TenantID == nil || *rec.TenantID != tenantID {
		return errs.ErrDataScope
	}
	allowedObject, be := s.checkObjectScope(c, rec)
	if be != nil {
		return be
	}
	owned := rec.UserID != "" && rec.UserID == id.UserID
	allow := func(perms ...string) *errs.Error {
		ok, err := authz.EnforceAny(id.UserID, perms...)
		if err != nil {
			return errs.ErrInternal
		}
		if !ok {
			return errs.ErrNoPerm
		}
		return nil
	}
	switch rec.Scene {
	case "export":
		if !allowedObject {
			return errs.ErrDataScope
		}
		parts := strings.Split(rec.StorageKey, "/")
		if len(parts) > 1 {
			switch parts[1] {
			case "reports":
				return allow("report:download")
			case "stats":
				return allow("stats:export")
			case "qrcode":
				return allow("inspection:point:qrcode")
			}
		}
		return errs.ErrDataScope
	case "signature", "seal":
		if owned {
			return nil
		}
		return allow("system:signasset:list")
	case "workorder":
		if !allowedObject && !owned {
			return errs.ErrDataScope
		}
		return allow("workorder:list")
	case "acceptance":
		if !allowedObject && !owned {
			return errs.ErrDataScope
		}
		return allow("acceptance:list")
	case "checkin":
		if !allowedObject && !owned {
			return errs.ErrDataScope
		}
		return nil
	case "equipment":
		if !allowedObject && !owned {
			return errs.ErrDataScope
		}
		return allow("equipment:list", "equipment:maintenance")
	case "doc_template":
		return allow("report:template:list")
	case "avatar", "notice":
		return nil
	default:
		return errs.ErrDataScope
	}
}

func (s *FileService) checkObjectScope(c *gin.Context, rec *sysmodel.UploadFile) (bool, *errs.Error) {
	var refs []filemodel.ObjectRef
	if err := s.db.Where("file_id = ? AND tenant_id = ?", rec.ID, *rec.TenantID).Find(&refs).Error; err != nil {
		return false, errs.ErrInternal
	}
	for _, ref := range refs {
		if middleware.CheckCommunity(s.db, c, ref.CommunityID) == nil {
			return true, nil
		}
	}
	return false, nil
}

// mimeByExt 常见扩展名 → MIME（登记缺失时兜底）。
func mimeByExt(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "heic":
		return "image/heic"
	case "pdf":
		return "application/pdf"
	case "zip":
		return "application/zip"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xls":
		return "application/vnd.ms-excel"
	case "doc":
		return "application/msword"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "txt":
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}
