// Package authz 基于 Casbin（RBAC with domains）的权限校验与策略同步。
//
// 设计要点：
//   - domain 预留给多租户（未来按"物业公司"隔离），当前所有策略挂在默认域 default；
//   - 权限点映射：system:user:list → obj=/system/user, act=list（按最后一个冒号拆分，
//     obj 走 keyMatch2 路径匹配，支持 /* 通配；act 走 regexMatch）；
//   - 策略数据源仍是 sys_role / sys_menu / sys_role_menu（后台界面与 API 不变），
//     角色/用户变更后调用 SyncAll 全量重建 casbin 策略，保证不出脏策略；
//   - g 规则：user:<id> → role:<code>（默认域）；p 规则：role:<code> → 各权限点；
//     super_admin 角色额外下发通配策略 (role:super_admin, default, /*, .*)，新权限点天然覆盖。
package authz

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"

	sysmodel "property-inspection/internal/module/system/model"
	"property-inspection/internal/pkg/logger"
	"go.uber.org/zap"
)

// DefaultDomain 默认域（多租户预留）。
const DefaultDomain = "default"

//go:embed rbac_model.conf
var modelText string

var (
	enforcer *casbin.Enforcer
	mu       sync.Mutex
)

// Init 初始化 enforcer 单例（复用现有 PG 连接，策略表 casbin_rule）。
func Init(db *gorm.DB) error {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return fmt.Errorf("casbin adapter 初始化失败: %w", err)
	}
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return fmt.Errorf("casbin 模型解析失败: %w", err)
	}
	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return fmt.Errorf("casbin enforcer 初始化失败: %w", err)
	}
	mu.Lock()
	enforcer = e
	mu.Unlock()
	return nil
}

// SplitPerm 权限点拆分：system:user:list → ("/system/user", "list")。
func SplitPerm(perm string) (obj, act string) {
	idx := strings.LastIndex(perm, ":")
	if idx < 0 {
		return "/" + perm, ".*"
	}
	return "/" + strings.ReplaceAll(perm[:idx], ":", "/"), perm[idx+1:]
}

// EnforceAny 校验用户是否拥有任一权限点。
func EnforceAny(userID string, perms ...string) (bool, error) {
	mu.Lock()
	e := enforcer
	mu.Unlock()
	if e == nil {
		return false, fmt.Errorf("authz 未初始化")
	}
	sub := "user:" + userID
	for _, perm := range perms {
		obj, act := SplitPerm(perm)
		ok, err := e.Enforce(sub, DefaultDomain, obj, act)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// SyncAll 从 sys_role/sys_menu/sys_role_menu/sys_user 全量重建 casbin 策略。
// 在角色分配菜单、角色增删改、用户分配角色、seed 初始化后调用。
func SyncAll(db *gorm.DB) error {
	mu.Lock()
	defer mu.Unlock()
	if enforcer == nil {
		return fmt.Errorf("authz 未初始化")
	}
	rules := make([][]string, 0, 256)

	// p 规则：启用角色 → 其菜单权限点
	var roles []sysmodel.SysRole
	if err := db.Where("status = ?", sysmodel.StatusEnabled).Find(&roles).Error; err != nil {
		return err
	}
	for _, role := range roles {
		sub := "role:" + role.Code
		if role.Code == sysmodel.SuperAdminCode {
			// 超管通配策略：覆盖全部权限点（含后续新增）
			rules = append(rules, []string{sub, DefaultDomain, "/*", ".*"})
			continue
		}
		var perms []string
		if err := db.Model(&sysmodel.SysMenu{}).
			Distinct("perms").
			Where("perms <> '' AND status = ?", sysmodel.StatusEnabled).
			Where("id IN (?)", db.Model(&sysmodel.SysRoleMenu{}).Select("menu_id").Where("role_id = ?", role.ID)).
			Pluck("perms", &perms).Error; err != nil {
			return err
		}
		for _, perm := range perms {
			obj, act := SplitPerm(perm)
			rules = append(rules, []string{sub, DefaultDomain, obj, act})
		}
	}

	// g 规则：启用用户 → 其角色（user:<id> → role:<code>）
	var users []sysmodel.SysUser
	if err := db.Select("id", "role_ids").Where("status = ?", sysmodel.StatusEnabled).Find(&users).Error; err != nil {
		return err
	}
	roleCodeByID := map[string]string{}
	{
		var all []sysmodel.SysRole
		if err := db.Select("id", "code").Find(&all).Error; err != nil {
			return err
		}
		for _, r := range all {
			roleCodeByID[r.ID] = r.Code
		}
	}
	for _, u := range users {
		for _, rid := range u.RoleIDs {
			code, ok := roleCodeByID[rid]
			if !ok {
				continue
			}
			rules = append(rules, []string{"g", "user:" + u.ID, "role:" + code, DefaultDomain})
		}
	}

	// 全量替换：ClearPolicy + SavePolicy（gorm-adapter SavePolicy 为全量重写，无脏策略）
	enforcer.ClearPolicy()
	if len(rules) > 0 {
		var pRules, gRules [][]string
		for _, r := range rules {
			if r[0] == "g" {
				gRules = append(gRules, r[1:])
			} else {
				pRules = append(pRules, r)
			}
		}
		if len(pRules) > 0 {
			if _, err := enforcer.AddPolicies(pRules); err != nil {
				return err
			}
		}
		if len(gRules) > 0 {
			if _, err := enforcer.AddGroupingPolicies(gRules); err != nil {
				return err
			}
		}
	}
	if err := enforcer.SavePolicy(); err != nil {
		return err
	}
	return enforcer.LoadPolicy()
}

// SyncAllQuiet 同步策略（失败仅记日志，不回滚业务）。
func SyncAllQuiet(db *gorm.DB) {
	if err := SyncAll(db); err != nil {
		logger.L.Warn("casbin 策略同步失败", zap.Error(err))
	}
}
