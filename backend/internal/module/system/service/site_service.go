// 品牌官网：页面配置（sys_config group=site）+ 下载渠道发布物（app_release）管理。
package service

import (
	"io"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/storage"
)

// SiteService 品牌官网管理。
type SiteService struct {
	db    *gorm.DB
	store *storage.Storage
	cfg   *ConfigService
}

func NewSiteService(db *gorm.DB, store *storage.Storage, cfg *ConfigService) *SiteService {
	return &SiteService{db: db, store: store, cfg: cfg}
}

// 官网可配置项（key 白名单：公共接口只暴露这些，后台表单也只渲染这些）
var siteConfigKeys = []string{"site.slogan", "site.contact_phone", "site.contact_email", "site.theme_color", "site.footer_note", "site.show_admin_entry"}

var themeColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// 发布物平台规则：允许的扩展名 + 是否必须填版本号
var releasePlatforms = map[string]struct {
	exts        map[string]bool
	needVersion bool
}{
	"android":   {exts: map[string]bool{"apk": true}, needVersion: true},
	"harmony":   {exts: map[string]bool{"hap": true}, needVersion: true},
	"ios":       {exts: map[string]bool{"ipa": true}, needVersion: true},
	"wechat_mp": {exts: map[string]bool{"png": true, "jpg": true, "jpeg": true}, needVersion: false},
}

// ReleaseSizeLimit 安装包上传上限（512MB，独立于普通图片的 UPLOAD_MAX_FILE_SIZE）。
const ReleaseSizeLimit = 512 << 20

// BrandConfig 后台表单用：返回全部官网配置项（含 name/remark）。
func (s *SiteService) BrandConfig() ([]model.SysConfig, *errs.Error) {
	return s.cfg.GetByKeys(siteConfigKeys)
}

// SaveBrandConfig 保存官网配置（仅接受白名单 key；主题色校验格式）。
func (s *SiteService) SaveBrandConfig(values map[string]string) *errs.Error {
	allowed := make(map[string]bool, len(siteConfigKeys))
	for _, k := range siteConfigKeys {
		allowed[k] = true
	}
	for k, v := range values {
		if !allowed[k] {
			return errs.ErrParam.WithMsg("不支持的配置项：" + k)
		}
		v = strings.TrimSpace(v)
		if k == "site.theme_color" && v != "" && !themeColorRe.MatchString(v) {
			return errs.ErrParam.WithMsg("主题色须为 #RRGGBB 格式")
		}
		if k == "site.show_admin_entry" && v != "true" && v != "false" {
			return errs.ErrParam.WithMsg("管理后台入口开关取值仅支持 true/false")
		}
		if len(v) > 200 {
			return errs.ErrParam.WithMsg("配置值过长（≤200 字符）：" + k)
		}
		if be := s.cfg.SetValue(k, v); be != nil {
			return be
		}
	}
	return nil
}

// PublicBrandConfig 官网公开配置（键名去掉 site. 前缀，只含白名单项）。
func (s *SiteService) PublicBrandConfig() map[string]string {
	out := make(map[string]string, len(siteConfigKeys))
	for _, k := range siteConfigKeys {
		if v, ok := s.cfg.Get(k); ok && v != "" {
			out[strings.TrimPrefix(k, "site.")] = v
		}
	}
	return out
}

// ListReleases 发布物列表（新的在前）。
func (s *SiteService) ListReleases() ([]model.AppRelease, *errs.Error) {
	var rows []model.AppRelease
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}

// UploadRelease 上传发布物：流式落盘（scene=app）+ 登记 upload_file 元数据 + 建发布记录。
func (s *SiteService) UploadRelease(userID, platform, version, note, filename string, size int64, r io.Reader) (*model.AppRelease, *errs.Error) {
	rule, ok := releasePlatforms[platform]
	if !ok {
		return nil, errs.ErrParam.WithMsg("platform 取值非法（android/harmony/ios/wechat_mp）")
	}
	ext := ""
	if i := strings.LastIndex(filename, "."); i >= 0 {
		ext = strings.ToLower(filename[i+1:])
	}
	if !rule.exts[ext] {
		return nil, errs.ErrParam.WithMsg("文件格式与平台不匹配")
	}
	version = strings.TrimSpace(version)
	if rule.needVersion && version == "" {
		return nil, errs.ErrParam.WithMsg("请填写版本号")
	}
	if size > ReleaseSizeLimit {
		return nil, errs.ErrParam.WithMsg("文件超过 512MB 上限")
	}

	key, _, md5hex, written, err := s.store.SaveStream("app", userID, ext, r)
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("文件保存失败")
	}
	if written > 0 {
		size = written
	}
	mime := "application/octet-stream"
	if platform == "wechat_mp" {
		mime = "image/" + ext
	}

	rel := &model.AppRelease{
		Platform: platform, Version: version, FileKey: key,
		Name: filename, Size: size, Note: strings.TrimSpace(note),
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		meta := model.UploadFile{
			FileKey: key, Scene: "app", UserID: userID, Size: size,
			MimeType: mime, URL: s.store.URL(key), Name: filename, MD5: md5hex,
			Storage: s.store.DriverName(),
		}
		if err := tx.Create(&meta).Error; err != nil {
			return err
		}
		return tx.Create(rel).Error
	})
	if err != nil {
		return nil, errs.ErrInternal
	}
	return rel, nil
}

// DeleteRelease 删除发布物记录（文件保留在存储中，与打卡照片等历史文件策略一致）。
func (s *SiteService) DeleteRelease(id string) *errs.Error {
	res := s.db.Delete(&model.AppRelease{}, "id = ?", id)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// PublicReleases 官网下载页用：每个平台取最新一条，附下载地址。
func (s *SiteService) PublicReleases() map[string]any {
	var rows []model.AppRelease
	// 取每平台最新一条：按 created_at 倒序扫全量去重（数据量小，无需窗口函数）
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return map[string]any{}
	}
	seen := map[string]bool{}
	out := map[string]any{}
	for _, r := range rows {
		if seen[r.Platform] {
			continue
		}
		seen[r.Platform] = true
		out[r.Platform] = map[string]any{
			"id":         r.ID,
			"version":    r.Version,
			"size":       r.Size,
			"name":       r.Name,
			"updated_at": r.CreatedAt.Format("2006-01-02"),
			"url":        "/api/public/download/app/" + r.ID,
		}
	}
	return out
}

// ReleaseFile 按 ID 取发布物（公开下载用）。
func (s *SiteService) ReleaseFile(id string) (*model.AppRelease, *errs.Error) {
	var rel model.AppRelease
	if err := s.db.First(&rel, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("发布物不存在或已删除")
	}
	return &rel, nil
}
