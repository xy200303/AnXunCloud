package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	authsvc "anxuncloud/internal/module/auth/service"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/timefmt"
)

// 小程序端登录/令牌段（从 mp_service.go 切分：登录、微信换号、双令牌签发与刷新、登录日志）。

// ========== 登录 ==========

// Login 微信 code 换登录态。
// 【mock 模式】mock=true（仅 dev 可用）时，code 传 "mock:<手机号>"：
// 按手机号查找已开通账号，首次自动绑定伪 openid（mock-openid-<手机号>），仅用于开发联调。
func (s *MPService) Login(ctx context.Context, req *dto.MPLoginReq, ip, ua string) (gin.H, *errs.Error) {
	// IP 维度失败限流（与后台登录一致的安全基线，防爆破/手机号枚举）
	if be := s.checkLoginLimit(ctx, ip); be != nil {
		return nil, be
	}
	var user *sysmodel.SysUser
	if s.wechat.MockEnabled() {
		u, be := s.mockLogin(req.Code)
		if be != nil {
			s.incrLoginFail(ctx, ip)
			s.loginLog(nil, nil, "", ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	} else {
		u, be := s.wxLogin(ctx, req)
		if be != nil {
			s.incrLoginFail(ctx, ip)
			s.loginLog(nil, nil, "", ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	}
	if user.Status != sysmodel.StatusEnabled {
		s.incrLoginFail(ctx, ip)
		s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "fail", "账号已停用")
		return nil, errs.ErrAccountDisabled
	}
	// 租户停用拒绝登录（P3 多租户：与后台登录同一规则）
	enabled, err := middleware.TenantEnabled(s.db, user.TenantID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	if !enabled {
		s.incrLoginFail(ctx, ip)
		s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "fail", "租户已停用")
		return nil, errs.ErrTenantDisabled
	}
	resp, be := s.issueTokens(ctx, user)
	if be != nil {
		return nil, be
	}
	s.rdb.Del(ctx, "limit:login:mp:"+ip)
	now := time.Now()
	s.db.Model(&sysmodel.SysUser{}).Where("id = ?", user.ID).Update("last_login_at", now)
	s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "success", "小程序登录成功")
	return resp, nil
}

// checkLoginLimit 小程序登录 IP 锁定（连续 10 次失败锁 10 分钟）。
func (s *MPService) checkLoginLimit(ctx context.Context, ip string) *errs.Error {
	n, err := s.rdb.Get(ctx, "limit:login:mp:"+ip).Int()
	if err == nil && n >= 10 {
		return errs.ErrTooMany.WithMsg("登录失败次数过多，请 10 分钟后再试")
	}
	return nil
}

// incrLoginFail 累计失败次数（10 分钟窗口）。
func (s *MPService) incrLoginFail(ctx context.Context, ip string) {
	key := "limit:login:mp:" + ip
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 10*time.Minute)
	pipe.Exec(ctx)
}

// mockLogin mock 模式登录（开发联调专用）。
func (s *MPService) mockLogin(code string) (*sysmodel.SysUser, *errs.Error) {
	var phone string
	if _, err := fmt.Sscanf(code, "mock:%s", &phone); err != nil || phone == "" {
		return nil, errs.ErrWxCodeInvalid.WithMsg("mock 模式 code 格式应为 mock:<手机号>")
	}
	openid := "mock-openid-" + phone
	var user sysmodel.SysUser
	// 先按 openid 找（已绑定），再按手机号找（首次绑定）
	if err := s.db.Where("openid = ?", openid).First(&user).Error; err != nil {
		if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
			return nil, errs.ErrWxUnbound
		}
		if err := s.db.Model(&user).Update("openid", openid).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	return &user, nil
}

// wxLogin 真实模式：code2session 换 openid，未绑定时用 phone_code 绑定手机号账号。
func (s *MPService) wxLogin(ctx context.Context, req *dto.MPLoginReq) (*sysmodel.SysUser, *errs.Error) {
	openid, be := s.code2Session(req.Code)
	if be != nil {
		return nil, be
	}
	var user sysmodel.SysUser
	if err := s.db.Where("openid = ?", openid).First(&user).Error; err == nil {
		return &user, nil
	}
	// 未绑定：需要 phone_code 换手机号完成绑定
	if req.PhoneCode == "" {
		return nil, errs.ErrWxUnbound
	}
	phone, be := s.phoneByCode(ctx, req.PhoneCode)
	if be != nil {
		return nil, be
	}
	if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, errs.ErrWxUnbound
	}
	if err := s.db.Model(&user).Update("openid", openid).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &user, nil
}

// code2Session 调微信 jscode2session。
func (s *MPService) code2Session(code string) (string, *errs.Error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.wechat.AppID, s.wechat.Secret, code)
	resp, err := s.httpc.Get(url)
	if err != nil {
		return "", errs.ErrWxCodeInvalid
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.OpenID == "" || out.ErrCode != 0 {
		return "", errs.ErrWxCodeInvalid
	}
	return out.OpenID, nil
}

// phoneByCode getPhoneNumber 换手机号（access_token 缓存于 Redis）。
func (s *MPService) phoneByCode(ctx context.Context, phoneCode string) (string, *errs.Error) {
	token, be := s.wxAccessToken(ctx)
	if be != nil {
		return "", be
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=%s", token)
	resp, err := s.httpc.Post(url, "application/json", strings.NewReader(fmt.Sprintf(`{"code":"%s"}`, phoneCode)))
	if err != nil {
		return "", errs.ErrWxCodeInvalid
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		ErrCode   int `json:"errcode"`
		PhoneInfo struct {
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ErrCode != 0 || out.PhoneInfo.PurePhoneNumber == "" {
		return "", errs.ErrWxCodeInvalid.WithMsg("手机号获取失败")
	}
	return out.PhoneInfo.PurePhoneNumber, nil
}

// wxAccessToken 小程序全局 access_token（Redis 缓存 7000s）。
func (s *MPService) wxAccessToken(ctx context.Context) (string, *errs.Error) {
	const key = "cache:wx:access_token"
	if v, err := s.rdb.Get(ctx, key).Result(); err == nil && v != "" {
		return v, nil
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		s.wechat.AppID, s.wechat.Secret)
	resp, err := s.httpc.Get(url)
	if err != nil {
		return "", errs.ErrInternal
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", errs.ErrInternal
	}
	ttl := time.Duration(out.ExpiresIn-200) * time.Second
	if ttl <= 0 {
		ttl = 7000 * time.Second
	}
	s.rdb.Set(ctx, key, out.AccessToken, ttl)
	return out.AccessToken, nil
}

// issueTokens 签发双令牌并写 mp 会话。
func (s *MPService) issueTokens(ctx context.Context, user *sysmodel.SysUser) (gin.H, *errs.Error) {
	access, accessJTI, err := s.jwtm.Generate(user.ID, user.Username, jwtutil.TypeAccess)
	if err != nil {
		return nil, errs.ErrInternal
	}
	refresh, refreshJTI, err := s.jwtm.Generate(user.ID, user.Username, jwtutil.TypeRefresh)
	if err != nil {
		return nil, errs.ErrInternal
	}
	var roles []string
	if len(user.RoleIDs) > 0 {
		s.db.Model(&sysmodel.SysRole{}).Where("id IN ?", []string(user.RoleIDs)).Pluck("code", &roles)
	}
	err = s.sess.Save(ctx, authsvc.ChannelApp, user.ID, session.Info{
		TokenID: accessJTI, RefreshID: refreshJTI, Name: user.Name,
		Roles: strings.Join(roles, ","), LoginAt: time.Now().Format(timefmt.Layout),
	}, s.jwtm.RefreshTTL())
	if err != nil {
		return nil, errs.ErrInternal
	}
	// 所属项目由 project_staff 在职编制推导
	projectIDs, _ := middleware.StaffProjectIDs(s.db, user.ID)
	projects := make([]gin.H, 0, len(projectIDs))
	for _, pid := range projectIDs {
		projects = append(projects, gin.H{"id": pid})
	}
	return gin.H{
		"token_type": "Bearer", "access_token": access, "refresh_token": refresh,
		"expires_in": int64(s.jwtm.AccessTTL().Seconds()),
		"user": gin.H{
			"id": user.ID, "name": user.Name, "avatar": user.Avatar,
			"roles": roles, "projects": projects,
		},
	}, nil
}

// Refresh 小程序端刷新双令牌（逻辑同后台）。
func (s *MPService) Refresh(ctx context.Context, refreshToken string) (gin.H, *errs.Error) {
	claims, err := s.jwtm.Parse(refreshToken)
	if err != nil || claims.Type != jwtutil.TypeRefresh {
		return nil, errs.ErrRefreshInvalid
	}
	black, _ := s.sess.IsBlacklisted(ctx, claims.ID)
	if black {
		return nil, errs.ErrRefreshInvalid
	}
	sessInfo, err := s.sess.GetByRefresh(ctx, authsvc.ChannelApp, claims.UserID, claims.ID)
	if err != nil || sessInfo == nil {
		return nil, errs.ErrRefreshInvalid
	}
	var user sysmodel.SysUser
	if err := s.db.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, errs.ErrRefreshInvalid
	}
	if user.Status != sysmodel.StatusEnabled {
		return nil, errs.ErrAccountDisabled
	}
	s.sess.Blacklist(ctx, claims.ID, time.Until(claims.ExpiresAt.Time))
	resp, be := s.issueTokens(ctx, &user)
	if be != nil {
		return nil, be
	}
	delete(resp, "user") // 刷新接口不下发 user
	return resp, nil
}

// loginLog 登录日志（channel=mp）。
// tenantID 取登录用户所属租户（日志管理按租户上下文过滤）；
// 无法识别用户时归默认租户，与后台 writeLoginLog 及迁移 00023 存量回填口径一致。
func (s *MPService) loginLog(userID *string, tenantID *string, username, ip, ua, status, msg string) {
	if tenantID == nil {
		var id string
		if err := s.db.Model(&sysmodel.Tenant{}).Select("id").Where("code = ?", sysmodel.DefaultTenantCode).
			Limit(1).Pluck("id", &id).Error; err == nil && id != "" {
			tenantID = &id
		}
	}
	rec := sysmodel.SysLoginLog{TenantID: tenantID, UserID: userID, Username: username, Channel: ChannelMP, IP: ip, Status: status, Msg: msg}
	if len(ua) > 500 {
		ua = ua[:500]
	}
	rec.UA = ua
	s.db.Create(&rec)
}
