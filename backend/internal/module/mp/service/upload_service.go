package service

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/exifutil"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
)

// UploadService 上传服务：STS 凭证、dev 本地上传、OSS 回调。
type UploadService struct {
	db    *gorm.DB
	store *storage.Storage
	cfg   config.UploadConfig
	oss   config.OSSConfig
}

func NewUploadService(db *gorm.DB, store *storage.Storage, up config.UploadConfig, oss config.OSSConfig) *UploadService {
	return &UploadService{db: db, store: store, cfg: up, oss: oss}
}

// STS 申请直传凭证。dev 模式返回本地上传入口；oss 模式走 AssumeRole。
func (s *UploadService) STS(userID string, req *dto.STSReq) (gin.H, *errs.Error) {
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
	if s.store.IsDev() {
		// dev 模式：小程序/联调脚本直传本地接口
		return gin.H{
			"mode":          "dev",
			"upload_url":    s.store.BaseURL() + "/api/mp/upload/local",
			"dir":           dir,
			"max_file_size": s.store.MaxFileSize(),
			"allowed_types": s.store.AllowedTypes(),
			"expire_seconds": 3600,
		}, nil
	}
	creds, err := s.store.AssumeRole()
	if err != nil {
		return nil, errs.ErrSTSFailed
	}
	return gin.H{
		"access_key_id":     creds.AccessKeyID,
		"access_key_secret": creds.AccessKeySecret,
		"security_token":    creds.SecurityToken,
		"expiration":        creds.Expiration.Format(timefmt.Layout),
		"expire_seconds":    s.oss.ExpireSeconds,
		"bucket":            s.oss.Bucket,
		"endpoint":          "https://" + s.oss.Endpoint,
		"dir":               dir,
		"callback_url":      s.store.CallbackURL(),
		"max_file_size":     s.store.MaxFileSize(),
		"allowed_types":     s.store.AllowedTypes(),
	}, nil
}

func storageDir(store *storage.Storage, scene string, uid string) string {
	return fmt.Sprintf("%s/%s/%s/", scene, time.Now().Format("200601"), uid)
}

// SaveLocal dev 模式本地直传（multipart），写文件记录。
func (s *UploadService) SaveLocal(userID string, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	switch scene {
	case "checkin", "workorder", "avatar":
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
	key, url, err := s.store.SaveLocal(scene, userID, ext, r)
	if err != nil {
		return nil, errs.ErrInternal
	}
	// JPEG 尝试解析 EXIF 拍摄时间（失败不阻塞）
	var exifTime *time.Time
	if ext == "jpg" || ext == "jpeg" {
		exifTime = exifutil.ReadShotTime(s.store.LocalPath(key))
	}
	rec := sysmodel.UploadFile{
		FileKey: key, Scene: scene, UserID: userID, Size: size,
		MimeType: "image/" + ext, URL: url, ExifTime: exifTime,
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_key": key, "url": url}, nil
}

// SaveAdminLocal 管理端本地上传（/api/admin/system/upload）：签名/公章/头像/工单整改图片。
func (s *UploadService) SaveAdminLocal(userID string, scene, filename string, size int64, r io.Reader) (gin.H, *errs.Error) {
	switch scene {
	case "signature", "seal", "avatar", "workorder":
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
	key, url, err := s.store.SaveLocal(scene, userID, ext, r)
	if err != nil {
		return nil, errs.ErrInternal
	}
	rec := sysmodel.UploadFile{
		FileKey: key, Scene: scene, UserID: userID, Size: size,
		MimeType: "image/" + ext, URL: url,
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"file_key": key, "url": url}, nil
}

// Callback OSS 上传回调：验签（oss 模式）→ 幂等写文件记录 → {"Status":"OK"}。
func (s *UploadService) Callback(c *gin.Context, body []byte) (int, any) {
	if s.store.IsDev() {
		return 400, gin.H{"code": 40001, "message": "dev 模式无 OSS 回调", "data": nil}
	}
	if !s.store.VerifyCallback(c.GetHeader("Authorization"), c.GetHeader("x-oss-pub-key-url"), c.Request.URL.RequestURI(), body) {
		return 401, gin.H{"code": 46001, "message": "OSS 回调验签失败", "data": nil}
	}
	var form struct {
		Bucket   string `json:"bucket"`
		Object   string `json:"object"`
		Size     int64  `json:"size,string"`
		MimeType string `json:"mimeType"`
		UID      string `json:"x:uid"`
		Scene    string `json:"x:scene"`
	}
	if err := json.Unmarshal(body, &form); err != nil || form.Object == "" {
		return 400, gin.H{"Status": "BadRequest"}
	}
	// 幂等：同一 object 重复回调直接 OK
	var count int64
	s.db.Model(&sysmodel.UploadFile{}).Where("file_key = ?", form.Object).Count(&count)
	if count == 0 {
		rec := sysmodel.UploadFile{
			FileKey: form.Object, Scene: form.Scene, UserID: form.UID,
			Size: form.Size, MimeType: form.MimeType, URL: s.store.URL(form.Object),
		}
		if rec.Scene == "" {
			rec.Scene = "checkin"
		}
		s.db.Create(&rec)
	}
	return 200, gin.H{"Status": "OK"}
}
