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
	"anxuncloud/internal/pkg/uploadfile"
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

// 官网可配置项（key 白名单：SSR 注入与后台表单渲染都只用这些）
var siteConfigKeys = []string{
	"site.slogan", "site.company_name", "site.contact_phone", "site.contact_email",
	"site.contact_wechat", "site.address", "site.icp", "site.theme_color", "site.footer_note",
}

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
		if len(v) > 200 {
			return errs.ErrParam.WithMsg("配置值过长（≤200 字符）：" + k)
		}
		if be := s.cfg.SetValue(k, v); be != nil {
			return be
		}
	}
	return nil
}

// BrandConfigMap 官网 SSR 注入用配置（键名去掉 site. 前缀，只含白名单项；空值不返回）。
// 租户解析（P3）：公开接口无登录态，统一按默认租户解析（私有化 = 默认租户）；
// SaaS 按域名/来源解析租户的方案后续迭代（届时在此替换 tenantID 来源）。
func (s *SiteService) BrandConfigMap() map[string]string {
	tenantID := s.cfg.DefaultTenantID()
	out := make(map[string]string, len(siteConfigKeys))
	for _, k := range siteConfigKeys {
		if v, ok := s.cfg.Resolve(tenantID, k); ok && v != "" {
			out[strings.TrimPrefix(k, "site.")] = v
		}
	}
	return out
}

// ReleaseFileKey 发布物 file_id → 存储路径（统一文件 ID 口径；未登记返回 false）。
func (s *SiteService) ReleaseFileKey(rel *model.AppRelease) (string, bool) {
	f, err := uploadfile.ByID(s.db, rel.FileID)
	if err != nil {
		return "", false
	}
	return f.StorageKey, true
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
// force=强制更新标记（App 启动检查更新时强制弹窗不可跳过）。
func (s *SiteService) UploadRelease(userID, platform, version, note, filename string, size int64, force bool, r io.Reader) (*model.AppRelease, *errs.Error) {
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
		Platform: platform, Version: version,
		Name: filename, Size: size, Note: strings.TrimSpace(note),
		ForceUpdate: force,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		meta := model.UploadFile{
			StorageKey: key, Scene: "app", UserID: userID,
			MimeType: mime, URL: s.store.URL(key), Name: filename, MD5: md5hex,
			Storage: s.store.DriverName(),
		}
		if err := tx.Create(&meta).Error; err != nil {
			return err
		}
		rel.FileID = meta.ID
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

// LatestReleases 官网下载页 SSR 用：每个平台最新一条发布物。
func (s *SiteService) LatestReleases() map[string]model.AppRelease {
	var rows []model.AppRelease
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return map[string]model.AppRelease{}
	}
	out := map[string]model.AppRelease{}
	for _, r := range rows {
		if _, ok := out[r.Platform]; !ok {
			out[r.Platform] = r
		}
	}
	return out
}

// LatestReleaseInfo App 检查更新：某平台最新一条发布物（版本/强制标记/备注/下载地址）。
func (s *SiteService) LatestReleaseInfo(platform string) (*model.AppRelease, *errs.Error) {
	if _, ok := releasePlatforms[platform]; !ok || platform == "wechat_mp" {
		return nil, errs.ErrParam.WithMsg("platform 取值非法（android/harmony/ios）")
	}
	var rel model.AppRelease
	if err := s.db.Where("platform = ?", platform).Order("created_at DESC").First(&rel).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("该平台暂未发布版本")
	}
	return &rel, nil
}

// ReleaseFile 按 ID 取发布物（公开下载用）。
func (s *SiteService) ReleaseFile(id string) (*model.AppRelease, *errs.Error) {
	var rel model.AppRelease
	if err := s.db.First(&rel, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("发布物不存在或已删除")
	}
	return &rel, nil
}
