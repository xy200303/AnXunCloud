// Package service 统一文件层（/api/files）：跨 admin/app/mp 通道的上传与下载。
// 鉴权：AuthAny（任一通道合法会话）；下载权限按 scene 规则判定（见 checkRead）。
package service

import (
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	mpdto "anxuncloud/internal/module/mp/dto"
	mpsvc "anxuncloud/internal/module/mp/service"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/exifutil"
	"anxuncloud/internal/pkg/storage"
)

// FileService 统一文件服务。
type FileService struct {
	db     *gorm.DB
	store  *storage.Storage
	upload *mpsvc.UploadService // 复用 STS 直传签发
}

func NewFileService(db *gorm.DB, store *storage.Storage, upload *mpsvc.UploadService) *FileService {
	return &FileService{db: db, store: store, upload: upload}
}

// 上传 scene 白名单（各端合集）：图片类走全局扩展名，notice 放宽到办公文档
var uploadScenes = map[string]bool{
	"checkin": true, "avatar": true, "signature": true, "seal": true, "notice": true,
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
func (s *FileService) STS(userID string, req *mpdto.STSReq) (gin.H, *errs.Error) {
	return s.upload.STS(userID, req)
}

// Upload 统一上传（multipart：file + scene）。登记 upload_file 元数据（name/md5/storage）。
func (s *FileService) Upload(c *gin.Context, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	if !uploadScenes[scene] {
		return nil, errs.ErrParam.WithMsg("scene 取值非法")
	}
	var ext string
	var be *errs.Error
	if scene == "notice" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
		if !noticeExts[ext] {
			return nil, errs.ErrUploadType.WithMsg("公告附件仅支持图片与 pdf/word/excel/ppt/txt/zip")
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
	key, url, data, md5, err := s.store.Save(scene, uid, ext, r)
	if err != nil {
		return nil, errs.ErrInternal
	}
	mime := "image/" + ext
	if scene == "notice" && !imageExts[ext] {
		mime = mimeByExt(ext)
	}
	var exifTime *time.Time
	if ext == "jpg" || ext == "jpeg" {
		exifTime = exifutil.ReadShotTimeBytes(data)
	}
	rec := sysmodel.UploadFile{
		FileKey: key, Scene: scene, UserID: uid, Size: size,
		MimeType: mime, URL: url, ExifTime: exifTime,
		Name: filepath.Base(filename), MD5: md5, Storage: s.store.DriverName(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_key": key, "url": url, "name": rec.Name, "md5": rec.MD5}, nil
}

// Download 统一下载/预览：查元数据 → scene 权限 → local 流式返回（原始文件名）/ 云存储 302。
// 返回 data（local 文件字节）或 redirect（云存储访问地址）；name 为原始文件名。
func (s *FileService) Download(c *gin.Context, key string) (data []byte, redirect, name, mime string, be *errs.Error) {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return nil, "", "", "", errs.ErrNotFound
	}
	var rec sysmodel.UploadFile
	if err := s.db.Where("file_key = ?", key).First(&rec).Error; err != nil {
		// 兼容未登记的历史文件：按扩展名推断，仍走 scene 前缀判定
		rec = sysmodel.UploadFile{FileKey: key, Scene: sceneOfKey(key), Name: filepath.Base(key)}
	}
	if be := s.checkRead(c, &rec); be != nil {
		return nil, "", "", "", be
	}
	mime = rec.MimeType
	if mime == "" {
		mime = mimeByExt(strings.TrimPrefix(strings.ToLower(filepath.Ext(key)), "."))
	}
	if s.store.IsLocal() {
		data, err := s.store.ReadFile(key)
		if err != nil {
			return nil, "", "", "", errs.ErrNotFound
		}
		return data, "", rec.Name, mime, nil
	}
	// 云存储：重定向到对象访问地址（后续接 COS 签名 URL 在此扩展）
	return nil, s.store.URL(key), rec.Name, mime, nil
}

// sceneOfKey 从 file_key 第一段推断 scene（兼容未登记记录）。
func sceneOfKey(key string) string {
	if i := strings.IndexByte(key, '/'); i > 0 {
		return key[:i]
	}
	return ""
}

// checkRead 按 scene 的读权限规则：
// - export/{reports,stats,qrcode}：对应权限点（报告下载/统计导出/点位二维码）；
// - signature/seal：本人或签章/报告相关权限；
// - 其余（checkin/avatar/notice）：登录即可（内部系统内容图）。
func (s *FileService) checkRead(c *gin.Context, rec *sysmodel.UploadFile) *errs.Error {
	id := middleware.CurrentIdentity(c)
	if id == nil {
		return errs.ErrUnauthorized
	}
	if id.SuperAdmin {
		return nil
	}
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
		sub := ""
		parts := strings.Split(rec.FileKey, "/")
		if len(parts) > 1 {
			sub = parts[1]
		}
		switch sub {
		case "reports":
			return allow("report:download")
		case "stats":
			return allow("stats:export")
		case "qrcode":
			return allow("inspection:point:qrcode")
		}
		return allow("report:download", "stats:export", "inspection:point:qrcode")
	case "signature", "seal":
		if rec.UserID != "" && rec.UserID == id.UserID {
			return nil
		}
		return allow("system:signasset:list", "report:list", "report:download")
	}
	return nil
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
