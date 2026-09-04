package service

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/exifutil"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
)

// UploadService 上传服务：云端直传凭证、local 本地上传、OSS 回调。
type UploadService struct {
	db    *gorm.DB
	store *storage.Storage
	cfg   config.UploadConfig
	oss   config.OSSConfig
}

func NewUploadService(db *gorm.DB, store *storage.Storage, up config.UploadConfig, oss config.OSSConfig) *UploadService {
	return &UploadService{db: db, store: store, cfg: up, oss: oss}
}

func (s *UploadService) UserTenantID(userID string) *string {
	var user struct {
		TenantID *string `gorm:"column:tenant_id"`
	}
	if s.db.Model(&sysmodel.SysUser{}).Select("tenant_id").Where("id = ?", userID).First(&user).Error != nil {
		return nil
	}
	return user.TenantID
}

// STS 申请直传凭证。local 模式返回本地上传入口；云存储模式走临时凭证。
func (s *UploadService) STS(userID string, req *dto.STSReq) (gin.H, *errs.Error) {
	if be := s.AuthorizeScene(userID, req.Scene); be != nil {
		return nil, be
	}
	// 预校验类型与大小
	for _, f := range req.Files {
		if _, be := s.store.CheckExt(f.Name); be != nil {
			return nil, be
		}
		if f.Size > s.store.MaxFileSize() {
			return nil, errs.ErrUploadTooLarge
		}
	}
	dir := storageDir(s.store, req.Scene, userID)
	if s.store.IsLocal() {
		// local 模式：小程序/联调脚本直传本地接口
		return gin.H{
			"mode":           "local",
			"upload_url":     s.store.BaseURL() + "/api/mp/upload/local",
			"dir":            dir,
			"max_file_size":  s.store.MaxFileSize(),
			"allowed_types":  s.store.AllowedTypes(),
			"expire_seconds": 3600,
		}, nil
	}
	creds, err := s.store.AssumeRole(dir)
	if err != nil {
		return nil, errs.ErrSTSFailed
	}
	return gin.H{
		"access_key_id":      creds.AccessKeyID,
		"access_key_secret":  creds.AccessKeySecret,
		"security_token":     creds.SecurityToken,
		"expiration":         creds.Expiration.Format(timefmt.Layout),
		"expire_seconds":     s.oss.ExpireSeconds,
		"bucket":             s.oss.Bucket,
		"endpoint":           "https://" + s.oss.Endpoint,
		"dir":                dir,
		"callback_url":       s.store.CallbackURL(),
		"callback_body_type": "application/json",
		"callback_body":      `{"bucket":"${bucket}","object":"${object}","size":"${size}","mimeType":"${mimeType}","etag":"${etag}","x:name":"${x:name}","x:uid":"${x:uid}","x:scene":"${x:scene}","x:md5":"${x:md5}"}`,
		"callback_vars":      gin.H{"x:uid": userID, "x:scene": req.Scene},
		"max_file_size":      s.store.MaxFileSize(),
		"allowed_types":      s.store.AllowedTypes(),
	}, nil
}

// Preflight 上传前预检：客户端提交摘要后，服务端返回同租户同场景下可直接复用的文件。
func (s *UploadService) Preflight(userID string, req *dto.STSReq) (gin.H, *errs.Error) {
	if be := s.AuthorizeScene(userID, req.Scene); be != nil {
		return nil, be
	}
	tenantID := s.UserTenantID(userID)
	files := make([]gin.H, 0, len(req.Files))
	for _, f := range req.Files {
		if _, be := s.store.CheckExt(f.Name); be != nil {
			return nil, be
		}
		if f.Size <= 0 || f.Size > s.store.MaxFileSize() {
			return nil, errs.ErrUploadTooLarge
		}
		md5hex := strings.ToLower(strings.TrimSpace(f.MD5))
		if decoded, err := hex.DecodeString(md5hex); err != nil || len(decoded) != 16 {
			return nil, errs.ErrParam.WithMsg("files.md5 必须是 32 位十六进制摘要")
		}
		item := gin.H{"name": filepath.Base(f.Name), "size": f.Size, "md5": md5hex, "reused": false}
		if existing, ok := s.FindReusable(req.Scene, md5hex, f.Size, tenantID); ok {
			item["reused"] = true
			item["file_id"] = existing.ID
			item["url"] = existing.URL
		}
		files = append(files, item)
	}
	return gin.H{"files": files}, nil
}

func storageDir(store *storage.Storage, scene string, uid string) string {
	return fmt.Sprintf("%s/%s/%s/", scene, time.Now().Format("200601"), uid)
}

// SaveLocal local 模式直传（multipart），写文件记录。
// scene=signature 为 App 端月报签字的一次性手写签名（报告 resolveSignKey 按 scene+user_id 校验归属）。
func (s *UploadService) SaveLocal(userID string, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	switch scene {
	case "checkin", "avatar", "signature":
	default:
		return nil, errs.ErrParam.WithMsg("scene 取值非法")
	}
	ext, be := s.store.CheckExt(filename)
	if be != nil {
		return nil, be
	}
	if size > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, s.store.MaxFileSize()+1))
	if err != nil || int64(len(data)) > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	if existing, ok := s.FindReusable(scene, storage.MD5Hex(data), int64(len(data)), s.UserTenantID(userID)); ok {
		return gin.H{"file_id": existing.ID, "url": existing.URL}, nil
	}
	key, url, data, md5, err := s.store.Save(scene, userID, ext, bytes.NewReader(data))
	if err != nil {
		return nil, errs.ErrInternal
	}
	// JPEG 尝试解析 EXIF 拍摄时间（失败不阻塞）
	var exifTime *time.Time
	if ext == "jpg" || ext == "jpeg" {
		exifTime = exifutil.ReadShotTimeBytes(data)
	}
	rec := sysmodel.UploadFile{
		TenantID: s.UserTenantID(userID), StorageKey: key, Scene: scene, UserID: userID,
		MimeType: "image/" + ext, URL: url, ExifTime: exifTime,
		Name: filepath.Base(filename), MD5: md5, Size: int64(len(data)), Storage: s.store.DriverName(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_id": rec.ID, "url": url}, nil
}

// SaveAdminLocal 管理端本地上传（/api/admin/system/upload）：签名/公章/头像/公告附件。
func (s *UploadService) SaveAdminLocal(userID string, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	switch scene {
	case "signature", "seal", "avatar", "notice":
	default:
		return nil, errs.ErrParam.WithMsg("scene 取值非法")
	}
	if be := s.AuthorizeScene(userID, scene); be != nil {
		return nil, be
	}
	var ext string
	var be *errs.Error
	if scene == "notice" {
		ext, be = checkNoticeExt(filename)
	} else {
		ext, be = s.store.CheckExt(filename)
	}
	if be != nil {
		return nil, be
	}
	if size > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, s.store.MaxFileSize()+1))
	if err != nil || int64(len(data)) > s.store.MaxFileSize() {
		return nil, errs.ErrUploadTooLarge
	}
	if existing, ok := s.FindReusable(scene, storage.MD5Hex(data), int64(len(data)), s.UserTenantID(userID)); ok {
		return gin.H{"file_id": existing.ID, "url": existing.URL}, nil
	}
	key, url, _, md5, err := s.store.Save(scene, userID, ext, bytes.NewReader(data))
	if err != nil {
		return nil, errs.ErrInternal
	}
	mime := "image/" + ext
	if scene == "notice" && !noticeImageExts[ext] {
		mime = "application/octet-stream"
	}
	rec := sysmodel.UploadFile{
		TenantID: s.UserTenantID(userID), StorageKey: key, Scene: scene, UserID: userID,
		MimeType: mime, URL: url,
		Name: filepath.Base(filename), MD5: md5, Size: int64(len(data)), Storage: s.store.DriverName(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_id": rec.ID, "url": url}, nil
}

// FindReusable 查找同场景、同租户下内容相同的已登记文件。
// 租户条件用于避免复用记录后触发下载权限串租户；空租户只匹配空租户记录。
func (s *UploadService) FindReusable(scene, md5 string, size int64, tenantID *string) (*sysmodel.UploadFile, bool) {
	if md5 == "" || size < 0 {
		return nil, false
	}
	var existing sysmodel.UploadFile
	query := s.db.Where("scene = ? AND md5 = ? AND size = ?", scene, md5, size)
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	if query.Order("created_at ASC").First(&existing).Error != nil {
		return nil, false
	}
	return &existing, true
}

func (s *UploadService) AuthorizeScene(userID, scene string) *errs.Error {
	if scene != "seal" && scene != "notice" {
		return nil
	}
	perms := []string{"system:signasset:create"}
	if scene == "notice" {
		perms = []string{"system:notice:create", "system:notice:update"}
	}
	ok, err := authz.EnforceAny(userID, perms...)
	if err != nil {
		return errs.ErrInternal
	}
	if !ok {
		return errs.ErrNoPerm
	}
	return nil
}

// noticeImageExts 公告附件中按图片处理的扩展名（App 端缩略图预览）。
var noticeImageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "heic": true,
}

// noticeAttachmentExts 公告附件允许的扩展名：图片 + 常见文档/表格/压缩包
// （全局 upload.allowed_types 仅图片，公告附件单独放宽）。
var noticeAttachmentExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "heic": true,
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true,
	"ppt": true, "pptx": true, "txt": true, "zip": true,
}

// checkNoticeExt 校验公告附件扩展名。
func checkNoticeExt(name string) (string, *errs.Error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if noticeAttachmentExts[ext] {
		return ext, nil
	}
	return "", errs.ErrUploadType.WithMsg("公告附件仅支持图片（jpg/png/gif/webp 等）与 pdf/word/excel/ppt/txt/zip")
}

// Callback OSS 上传回调：验签（oss 模式）→ 幂等写文件记录 → {"Status":"OK"}。
func (s *UploadService) Callback(c *gin.Context, body []byte) (int, any) {
	if s.store.IsLocal() {
		return 400, gin.H{"code": 40001, "message": "local 模式无 OSS 回调", "data": nil}
	}
	if !s.store.VerifyCallback(c.GetHeader("Authorization"), c.GetHeader("x-oss-pub-key-url"), c.Request.URL.RequestURI(), body) {
		return 401, gin.H{"code": 46001, "message": "OSS 回调验签失败", "data": nil}
	}
	var form struct {
		Bucket   string `json:"bucket"`
		Object   string `json:"object"`
		Size     int64  `json:"size,string"`
		MimeType string `json:"mimeType"`
		ETag     string `json:"etag"`   // OSS 回调携带（非分片上传即内容 MD5）
		MD5      string `json:"x:md5"`  // 客户端可传真实内容 MD5（分片上传时使用）
		Name     string `json:"x:name"` // 客户端自定义参数带回的原始文件名
		UID      string `json:"x:uid"`
		Scene    string `json:"x:scene"`
	}
	if err := json.Unmarshal(body, &form); err != nil || form.Object == "" {
		return 400, gin.H{"Status": "BadRequest"}
	}
	if form.Scene == "" {
		form.Scene = "checkin"
	}
	if !isUploadScene(form.Scene) || form.UID == "" || form.Size <= 0 || form.Size > s.store.MaxFileSize() {
		return 400, gin.H{"Status": "BadRequest"}
	}
	if s.oss.Bucket != "" && form.Bucket != "" && form.Bucket != s.oss.Bucket {
		return 400, gin.H{"Status": "BadRequest"}
	}
	parts := strings.Split(strings.TrimPrefix(form.Object, "/"), "/")
	if len(parts) != 4 || parts[0] != form.Scene || parts[2] != form.UID || parts[3] == "" {
		return 400, gin.H{"Status": "BadRequest"}
	}
	if _, be := s.store.CheckExt(parts[3]); be != nil {
		return 400, gin.H{"Status": "BadRequest"}
	}
	var user sysmodel.SysUser
	if err := s.db.Select("id", "tenant_id").Where("id = ?", form.UID).First(&user).Error; err != nil {
		return 400, gin.H{"Status": "BadRequest"}
	}
	// 幂等：同一 object 重复回调直接 OK
	var count int64
	s.db.Model(&sysmodel.UploadFile{}).Where("storage_key = ?", form.Object).Count(&count)
	if count == 0 {
		tenantID := user.TenantID
		md5hex := strings.ToLower(strings.TrimSpace(form.MD5))
		if md5hex == "" {
			md5hex = strings.ToLower(strings.Trim(form.ETag, `"`))
		}
		if form.MD5 != "" {
			data, err := s.store.ReadFile(form.Object)
			if err != nil || int64(len(data)) != form.Size || storage.MD5Hex(data) != md5hex {
				return 400, gin.H{"Status": "MD5Mismatch"}
			}
		}
		if existing, ok := s.FindReusable(form.Scene, md5hex, form.Size, &tenantID); ok {
			if err := s.store.Delete(form.Object); err != nil {
				return 500, gin.H{"Status": "DuplicateCleanupFailed"}
			}
			return 200, gin.H{"Status": "OK", "file_id": existing.ID, "url": existing.URL}
		}
		rec := sysmodel.UploadFile{
			TenantID: &tenantID, StorageKey: form.Object, Scene: form.Scene, UserID: form.UID,
			MimeType: form.MimeType, URL: s.store.URL(form.Object),
			Name: filepath.Base(form.Name), MD5: md5hex, Size: form.Size, Storage: s.store.DriverName(),
		}
		if rec.Scene == "" {
			rec.Scene = "checkin"
		}
		s.db.Create(&rec)
	}
	return 200, gin.H{"Status": "OK"}
}

func isUploadScene(scene string) bool {
	switch scene {
	case "checkin", "avatar", "signature", "seal", "notice":
		return true
	default:
		return false
	}
}
