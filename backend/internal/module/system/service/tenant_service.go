package service

import (
	"context"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// TenantService 租户管理（P3 多租户；租户 CRUD 仅超管，租户配置租户管理员可管自己租户）。
type TenantService struct {
	db           *gorm.DB
	killSessions func(ctx context.Context, userID string) // 停用租户时踢下线该租户全部账号
	cfg          *ConfigService
}

func NewTenantService(db *gorm.DB, killSessions func(ctx context.Context, userID string), cfg *ConfigService) *TenantService {
	return &TenantService{db: db, killSessions: killSessions, cfg: cfg}
}

// tenantCodeRe 租户代码（公司代码）格式：小写字母/数字/中划线，2–32 位。
var tenantCodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)

// tenantConfigWhitelist 租户可覆盖的配置 key 白名单（设计方案 §9.2：品牌类先行）。
// 与现有站点品牌配置（sys_config group=site）的 key 对齐——现有实现无 site_name/site_logo 独立 key，
// 系统名称即 site.company_name、主题色即 site.theme_color；登录页 LOGO/背景图属前端静态资源，后续需要时再扩白名单。
// 密钥类配置（ai.* / 存储配置）永不在此名单内。
var tenantConfigWhitelist = []string{
	"site.company_name", "site.slogan", "site.theme_color",
	"site.footer_note", "site.contact_phone", "site.contact_email",
}

// IsTenantConfigKey 判断 key 是否在租户可覆盖白名单内（纯函数，供单测）。
func IsTenantConfigKey(key string) bool {
	for _, k := range tenantConfigWhitelist {
		if k == key {
			return true
		}
	}
	return false
}

// ========== 租户 CRUD（仅 super_admin，路由层挂 tenant:* 权限点） ==========

// List 租户分页列表。
func (s *TenantService) List(q *dto.TenantListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.Tenant{})
	if q.Name != "" {
		db = db.Where("name LIKE ? OR code LIKE ?", "%"+q.Name+"%", "%"+q.Name+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.Tenant
	offset, limit := q.Normalize()
	if err := db.Order("created_at ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]dto.TenantItem, 0, len(rows))
	for _, r := range rows {
		var userCount int64
		s.db.Model(&model.SysUser{}).Where("tenant_id = ?", r.ID).Count(&userCount)
		list = append(list, dto.TenantItem{
			ID: r.ID, Code: r.Code, Name: r.Name,
			ContactName: r.ContactName, ContactPhone: r.ContactPhone,
			Status: model.StatusInt(r.Status), UserCount: userCount,
			Remark: r.Remark, CreatedAt: timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Create 开通租户：建租户 + 初始管理员账号（挂内置 tenant_admin 角色，首次登录强制改密）
//   + 平台模板岗位/平台默认槽位绑定整份复制为新租户行（方案 §3：模板库只读于开通那一刻，
//     此后租户自管，模板后续修改不影响老租户；存量租户由迁移 00025 同样回填）。
func (s *TenantService) Create(req *dto.TenantCreateReq) (string, *errs.Error) {
	if !tenantCodeRe.MatchString(req.Code) {
		return "", errs.ErrParam.WithMsg("code 须为 2–32 位小写字母/数字/中划线")
	}
	if req.Code == model.DefaultTenantCode {
		return "", errs.ErrTenantCodeExists
	}
	if !password.ValidUsername(req.AdminUsername) {
		return "", errs.ErrParam.WithMsg("admin_username 须为 4–32 位字母数字下划线")
	}
	if !password.ValidPassword(req.AdminPassword) {
		return "", errs.ErrParam.WithMsg("admin_password 须为 8–32 位且含字母与数字")
	}
	var count int64
	s.db.Model(&model.Tenant{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return "", errs.ErrTenantCodeExists
	}
	var adminRole model.SysRole
	if err := s.db.Where("code = ? AND tenant_id IS NULL", model.TenantAdminCode).First(&adminRole).Error; err != nil {
		return "", errs.ErrInternal
	}
	hash, err := password.Hash(req.AdminPassword)
	if err != nil {
		return "", errs.ErrInternal
	}
	tenant := model.Tenant{
		Code: req.Code, Name: req.Name,
		ContactName: req.ContactName, ContactPhone: req.ContactPhone,
		Status: model.StatusEnabled, Remark: req.Remark,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		adminName := strings.TrimSpace(req.AdminName)
		if adminName == "" {
			adminName = req.Name + "管理员"
		}
		admin := model.SysUser{
			TenantID:           tenant.ID,
			Username:           req.AdminUsername,
			Password:           hash,
			Name:               adminName,
			RoleIDs:            types.IDArray{adminRole.ID},
			Status:             model.StatusEnabled,
			MustChangePassword: true, // 初始密码仅用于首次登录
			Remark:             "开通租户时创建的初始管理员",
		}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		// 岗位模板库整份复制（岗位 + 平台默认槽位绑定 → 新租户行）
		return CopyPostTemplatesToTenant(tx, tenant.ID)
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	// 重建 casbin g 规则，否则新管理员登录后无任何权限（与 user_service 的既有约定一致）
	authz.SyncAllQuiet(s.db)
	return tenant.ID, nil
}

// Update 修改租户基础信息（code 不可改）。
func (s *TenantService) Update(id string, req *dto.TenantUpdateReq) *errs.Error {
	var t model.Tenant
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	updates := map[string]any{
		"name": req.Name, "contact_name": req.ContactName,
		"contact_phone": req.ContactPhone, "remark": req.Remark,
	}
	if err := s.db.Model(&t).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// SetStatus 停用/启用租户。停用即该租户全部账号无法登录（登录拒绝 + 每请求校验），
// 同时主动踢下线全部会话；默认租户不可停用（私有化部署的唯一租户，停用即系统不可用）。
func (s *TenantService) SetStatus(ctx context.Context, id string, status int) *errs.Error {
	var t model.Tenant
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if t.Code == model.DefaultTenantCode && status == 0 {
		return errs.ErrParam.WithMsg("默认租户不可停用")
	}
	if err := s.db.Model(&t).Update("status", model.StatusStr(status)).Error; err != nil {
		return errs.ErrInternal
	}
	if status == 0 && s.killSessions != nil {
		var userIDs []string
		s.db.Model(&model.SysUser{}).Where("tenant_id = ?", id).Pluck("id", &userIDs)
		for _, uid := range userIDs {
			s.killSessions(ctx, uid)
		}
	}
	return nil
}

// ========== 租户配置覆盖（tenant:config 企业品牌；tid 由 EffectiveTenantID 解析并校验存在性） ==========

// GetConfig 租户配置视图：白名单 key 的租户覆盖值 + 平台默认值 + 生效值。
func (s *TenantService) GetConfig(tid string) ([]dto.TenantConfigItem, *errs.Error) {
	var rows []model.TenantConfig
	s.db.Where("tenant_id = ?", tid).Find(&rows)
	overrides := make(map[string]string, len(rows))
	for _, r := range rows {
		overrides[r.Key] = r.Value
	}
	items := make([]dto.TenantConfigItem, 0, len(tenantConfigWhitelist))
	for _, key := range tenantConfigWhitelist {
		platform, _ := s.cfg.Get(key)
		value := overrides[key]
		effective := platform
		if value != "" {
			effective = value
		}
		items = append(items, dto.TenantConfigItem{
			Key: key, Value: value, Platform: platform, Effective: effective,
		})
	}
	return items, nil
}

// SaveConfig 保存租户配置（仅白名单 key，空值=清除覆盖回落平台默认；主题色校验格式）。
func (s *TenantService) SaveConfig(tid string, values map[string]string) *errs.Error {
	for k, v := range values {
		if !IsTenantConfigKey(k) {
			return errs.ErrParam.WithMsg("不支持的租户配置项：" + k)
		}
		v = strings.TrimSpace(v)
		if k == "site.theme_color" && v != "" && !themeColorRe.MatchString(v) {
			return errs.ErrParam.WithMsg("主题色须为 #RRGGBB 格式")
		}
		if len(v) > 200 {
			return errs.ErrParam.WithMsg("配置值过长（≤200 字符）：" + k)
		}
		if v == "" {
			// 空值 = 清除覆盖（读取时回落平台默认）
			if err := s.db.Where("tenant_id = ? AND key = ?", tid, k).Delete(&model.TenantConfig{}).Error; err != nil {
				return errs.ErrInternal
			}
			continue
		}
		var row model.TenantConfig
		err := s.db.Where("tenant_id = ? AND key = ?", tid, k).First(&row).Error
		if err != nil {
			row = model.TenantConfig{TenantID: tid, Key: k, Value: v}
			if err := s.db.Create(&row).Error; err != nil {
				return errs.ErrInternal
			}
			continue
		}
		if err := s.db.Model(&row).Update("value", v).Error; err != nil {
			return errs.ErrInternal
		}
	}
	return nil
}
