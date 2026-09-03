package database

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/types"
)

// Seed 写入系统预置数据（超管、内置角色、菜单树、字典、默认参数）。
// 仅当 sys_role 为空时执行全量初始化；已初始化的库每次启动仍做平台默认行自愈
// （post_dict/duty_binding/approval_flow 的 tenant_id 为空行是开通租户的复制源，
// 可能被清库脚本或历史版本误删，缺失会导致岗位绑定角色并集失效）。
// 超管账号来自配置（ADMIN_USERNAME/PASSWORD/NAME）。
func Seed(db *gorm.DB, adminUsername, adminPassword, adminName string) error {
	var count int64
	if err := db.Model(&model.SysRole{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ensurePlatformRows(db)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		tenantID, err := seedTenant(tx)
		if err != nil {
			return err
		}
		roleIDs, err := seedRoles(tx)
		if err != nil {
			return err
		}
		menuIDs, err := seedMenus(tx)
		if err != nil {
			return err
		}
		if err := seedRoleMenus(tx, roleIDs, menuIDs); err != nil {
			return err
		}
		if err := seedAdmin(tx, roleIDs["super_admin"], tenantID, adminUsername, adminPassword, adminName); err != nil {
			return err
		}
		if err := seedDicts(tx); err != nil {
			return err
		}
		if err := seedConfigs(tx); err != nil {
			return err
		}
		if err := seedPosts(tx, roleIDs); err != nil {
			return err
		}
		if err := seedDutyBindings(tx); err != nil {
			return err
		}
		if err := seedApprovalFlow(tx); err != nil {
			return err
		}
		// 默认租户同样需要一份岗位与槽位默认（模板只读于开通/初始化那一刻，与开通租户同一复制逻辑）
		return systemsvc.CopyPostTemplatesToTenant(tx, tenantID)
	})
}

// seedTenant 确保默认租户存在（P3 多租户：私有化部署 = 只有默认租户的同一套系统）。
// 默认租户完全由本函数查询/创建（无迁移插入），返回租户 ID 供超管账号归属。
func seedTenant(tx *gorm.DB) (string, error) {
	var t model.Tenant
	if err := tx.Where("code = ?", model.DefaultTenantCode).First(&t).Error; err == nil {
		return t.ID, nil
	}
	t = model.Tenant{
		Code:   model.DefaultTenantCode,
		Name:   "默认租户",
		Status: model.StatusEnabled,
		Remark: "系统预置默认租户（私有化部署 = 只有该租户的同一套系统）",
	}
	if err := tx.Create(&t).Error; err != nil {
		return "", err
	}
	return t.ID, nil
}

// seedRoles 预置四个内置角色（权限模板，见设计方案 §3.1），返回 code → id 映射。
func seedRoles(tx *gorm.DB) (map[string]string, error) {
	roles := []model.SysRole{
		{Code: "super_admin", Name: "超级管理员", DataScope: model.ScopeAll, Remark: "拥有全部权限", IsBuiltin: true},
		{Code: "tenant_admin", Name: "租户管理员", DataScope: model.ScopeAll, Remark: "物业公司负责人：租户内全部业务功能与租户级系统管理", IsBuiltin: true},
		{Code: "project_admin", Name: "项目管理员", DataScope: model.ScopeProject, Remark: "项目经理/主管等项目级后台使用人员：业务菜单全开", IsBuiltin: true},
		{Code: "field_staff", Name: "一线人员", DataScope: model.ScopeSelf, Remark: "巡检员/维修工/楼管员/前台等移动端使用人员：打卡、任务、月报确认", IsBuiltin: true},
	}
	ids := make(map[string]string, len(roles))
	for i := range roles {
		roles[i].Status = model.StatusEnabled
		if err := tx.Create(&roles[i]).Error; err != nil {
			return nil, err
		}
		ids[roles[i].Code] = roles[i].ID
	}
	return ids, nil
}

// menuSeed 菜单种子节点（结构对应《数据库设计文档》§6.2 菜单树）。
type menuSeed struct {
	title    string
	path     string
	icon     string
	typ      string
	perms    string
	sort     int
	hidden   bool // 侧边栏不显示（路由仍注册，如个人中心从顶栏头像进入）
	platform bool // 平台级菜单（平台管理目录）：整棵子树继承，仅超管可见可授权
	children []menuSeed
}

// seedMenus 递归写入菜单树，返回 perms/路径 → id 映射（含全部节点）。
func seedMenus(tx *gorm.DB) (map[string]string, error) {
	tree := []menuSeed{
		{title: "工作台", path: "/dashboard", icon: "Odometer", typ: model.MenuTypeMenu, perms: "dashboard:view", sort: 10},
		{title: "巡检管理", path: "/inspection", icon: "Location", typ: model.MenuTypeDir, sort: 20, children: []menuSeed{
			{title: "点位管理", path: "/inspection/points", icon: "MapLocation", typ: model.MenuTypeMenu, perms: "inspection:point:list", sort: 1, children: []menuSeed{
				{title: "新增点位", typ: model.MenuTypeButton, perms: "inspection:point:create", sort: 1},
				{title: "编辑点位", typ: model.MenuTypeButton, perms: "inspection:point:update", sort: 2},
				{title: "删除点位", typ: model.MenuTypeButton, perms: "inspection:point:delete", sort: 3},
				{title: "批量生成二维码", typ: model.MenuTypeButton, perms: "inspection:point:qrcode", sort: 4},
				{title: "批量导入", typ: model.MenuTypeButton, perms: "inspection:point:import", sort: 5},
			}},
			{title: "计划任务", path: "/inspection/plans", icon: "Calendar", typ: model.MenuTypeMenu, perms: "inspection:plan:list", sort: 2, children: []menuSeed{
				{title: "新增计划", typ: model.MenuTypeButton, perms: "inspection:plan:create", sort: 1},
				{title: "编辑计划", typ: model.MenuTypeButton, perms: "inspection:plan:update", sort: 2},
				{title: "停用计划", typ: model.MenuTypeButton, perms: "inspection:plan:disable", sort: 3},
				{title: "删除计划", typ: model.MenuTypeButton, perms: "inspection:plan:delete", sort: 4},
			}},
			{title: "任务监控", path: "/inspection/tasks", icon: "Monitor", typ: model.MenuTypeMenu, perms: "inspection:task:monitor", sort: 3, children: []menuSeed{
				{title: "任务列表", typ: model.MenuTypeButton, perms: "inspection:task:list", sort: 1},
				{title: "手动生成任务", typ: model.MenuTypeButton, perms: "inspection:task:generate", sort: 2},
			}},
			{title: "巡检记录", path: "/inspection/records", icon: "Document", typ: model.MenuTypeMenu, perms: "inspection:record:list", sort: 4, children: []menuSeed{
				{title: "记录检索", typ: model.MenuTypeButton, perms: "inspection:checkin:list", sort: 1},
				{title: "记录审核", typ: model.MenuTypeButton, perms: "inspection:checkin:review", sort: 2},
				{title: "发起抽查", typ: model.MenuTypeButton, perms: "inspection:checkin:spotcheck", sort: 3},
			}},
			{title: "检查项模板", path: "/inspection/templates", icon: "Finished", typ: model.MenuTypeMenu, perms: "inspection:template:list", sort: 6, children: []menuSeed{
				{title: "新增模板", typ: model.MenuTypeButton, perms: "inspection:template:create", sort: 1},
				{title: "编辑模板", typ: model.MenuTypeButton, perms: "inspection:template:update", sort: 2},
				{title: "删除模板", typ: model.MenuTypeButton, perms: "inspection:template:delete", sort: 3},
			}},
		}},
		{title: "统计分析", path: "/stats", icon: "DataAnalysis", typ: model.MenuTypeDir, sort: 40, children: []menuSeed{
			{title: "巡检报表", path: "/stats/inspection", icon: "TrendCharts", typ: model.MenuTypeMenu, perms: "stats:inspection", sort: 1},
			{title: "绩效报表", path: "/stats/performance", icon: "Medal", typ: model.MenuTypeMenu, perms: "stats:performance", sort: 2},
			{title: "报表查看", typ: model.MenuTypeButton, perms: "stats:report", sort: 3},
			{title: "数据导出", typ: model.MenuTypeButton, perms: "stats:export", sort: 4},
			{title: "月度报告", path: "/stats/reports", icon: "Notebook", typ: model.MenuTypeMenu, perms: "report:list", sort: 5, children: []menuSeed{
				{title: "生成报告", typ: model.MenuTypeButton, perms: "report:generate", sort: 1},
				{title: "巡检员确认", typ: model.MenuTypeButton, perms: "report:sign:inspector", sort: 2},
				{title: "下载PDF", typ: model.MenuTypeButton, perms: "report:download", sort: 5},
				{title: "代签", typ: model.MenuTypeButton, perms: "report:sign:proxy", sort: 6},
			}},
		}},
		{title: "小区管理", path: "/community", icon: "OfficeBuilding", typ: model.MenuTypeMenu, perms: "community:list", sort: 50, children: []menuSeed{
			{title: "新增小区", typ: model.MenuTypeButton, perms: "community:create", sort: 1},
			{title: "编辑小区", typ: model.MenuTypeButton, perms: "community:update", sort: 2},
			{title: "删除小区", typ: model.MenuTypeButton, perms: "community:delete", sort: 3},
			{title: "楼栋列表", typ: model.MenuTypeButton, perms: "community:building:list", sort: 4},
			{title: "新增楼栋", typ: model.MenuTypeButton, perms: "community:building:create", sort: 5},
			{title: "编辑楼栋", typ: model.MenuTypeButton, perms: "community:building:update", sort: 6},
			{title: "删除楼栋", typ: model.MenuTypeButton, perms: "community:building:delete", sort: 7},
			{title: "编制名单", typ: model.MenuTypeButton, perms: "community:staff:list", sort: 8},
			{title: "编制维护", typ: model.MenuTypeButton, perms: "community:staff:edit", sort: 9},
			{title: "职责绑定", typ: model.MenuTypeButton, perms: "community:duty:edit", sort: 10},
		}},
		// 系统管理（租户级，《管理后台信息架构与菜单归位方案》第一章）：租户管理员管本公司内部事务，
		// 全部数据操作处在「租户上下文」（middleware.EffectiveTenantID）中
		{title: "系统管理", path: "/system", icon: "Suitcase", typ: model.MenuTypeDir, sort: 60, children: []menuSeed{
			{title: "用户管理", path: "/system/users", icon: "User", typ: model.MenuTypeMenu, perms: "system:user:list", sort: 1, children: []menuSeed{
				{title: "新增用户", typ: model.MenuTypeButton, perms: "system:user:create", sort: 1},
				{title: "编辑用户", typ: model.MenuTypeButton, perms: "system:user:update", sort: 2},
				{title: "删除用户", typ: model.MenuTypeButton, perms: "system:user:delete", sort: 3},
				{title: "重置密码", typ: model.MenuTypeButton, perms: "system:user:reset-password", sort: 4},
				{title: "导入用户", typ: model.MenuTypeButton, perms: "system:user:import", sort: 5},
				{title: "导出用户", typ: model.MenuTypeButton, perms: "system:user:export", sort: 6},
			}},
			{title: "角色管理", path: "/system/roles", icon: "Avatar", typ: model.MenuTypeMenu, perms: "system:role:list", sort: 2, children: []menuSeed{
				{title: "新增角色", typ: model.MenuTypeButton, perms: "system:role:create", sort: 1},
				{title: "编辑角色", typ: model.MenuTypeButton, perms: "system:role:update", sort: 2},
				{title: "删除角色", typ: model.MenuTypeButton, perms: "system:role:delete", sort: 3},
				{title: "分配权限", typ: model.MenuTypeButton, perms: "system:role:assign", sort: 4},
			}},
			// 岗位管理（方案第三章）：本租户岗位 + 岗位绑角色 + 租户级职责槽位默认绑定
			{title: "岗位管理", path: "/system/posts", icon: "Postcard", typ: model.MenuTypeMenu, perms: "system:post:list", sort: 3, children: []menuSeed{
				{title: "新增岗位", typ: model.MenuTypeButton, perms: "system:post:create", sort: 1},
				{title: "编辑岗位", typ: model.MenuTypeButton, perms: "system:post:update", sort: 2},
				{title: "删除岗位", typ: model.MenuTypeButton, perms: "system:post:delete", sort: 3},
				{title: "职责绑定", typ: model.MenuTypeButton, perms: "system:post:duty", sort: 4},
			}},
			{title: "签章管理", path: "/system/sign-assets", icon: "Stamp", typ: model.MenuTypeMenu, perms: "system:signasset:list", sort: 4, children: []menuSeed{
				{title: "新增签章", typ: model.MenuTypeButton, perms: "system:signasset:create", sort: 1},
				{title: "作废签章", typ: model.MenuTypeButton, perms: "system:signasset:revoke", sort: 2},
			}},
			{title: "日志管理", path: "/system/logs", icon: "Tickets", typ: model.MenuTypeMenu, perms: "system:log:list", sort: 5, children: []menuSeed{
				{title: "操作日志", typ: model.MenuTypeButton, perms: "system:log:operation", sort: 1},
				{title: "登录日志", typ: model.MenuTypeButton, perms: "system:log:login", sort: 2},
				{title: "日志导出", typ: model.MenuTypeButton, perms: "system:log:export", sort: 3},
			}},
			{title: "通知公告", path: "/system/notices", icon: "Bell", typ: model.MenuTypeMenu, perms: "system:notice:list", sort: 6, children: []menuSeed{
				{title: "新增公告", typ: model.MenuTypeButton, perms: "system:notice:create", sort: 1},
				{title: "编辑公告", typ: model.MenuTypeButton, perms: "system:notice:update", sort: 2},
				{title: "删除公告", typ: model.MenuTypeButton, perms: "system:notice:delete", sort: 3},
			}},
			// 企业品牌（tenant:config）：租户管理员管理本租户的品牌覆盖配置，授 super_admin + tenant_admin
			{title: "企业品牌", path: "/system/brand", icon: "Brush", typ: model.MenuTypeMenu, perms: "tenant:config", sort: 7},
			// 审批链管理（扩展方案 §3）：打卡审核链租户级配置（自 00003 起独立菜单，不再寄生于岗位管理页签）
			{title: "审批流程", path: "/system/review-flow", icon: "Connection", typ: model.MenuTypeMenu, perms: "system:reviewflow:list", sort: 8, children: []menuSeed{
				{title: "保存审批流程", typ: model.MenuTypeButton, perms: "system:reviewflow:update", sort: 1},
			}},
		}},
		// 平台管理（平台级，仅超管）：整棵子树 is_platform=true，租户角色不可见不可授权
		{title: "平台管理", path: "/platform", icon: "Setting", typ: model.MenuTypeDir, sort: 70, platform: true, children: []menuSeed{
			{title: "租户管理", path: "/platform/tenants", icon: "OfficeBuilding", typ: model.MenuTypeMenu, perms: "tenant:list", sort: 1, children: []menuSeed{
				{title: "新增租户", typ: model.MenuTypeButton, perms: "tenant:create", sort: 1},
				{title: "编辑租户", typ: model.MenuTypeButton, perms: "tenant:update", sort: 2},
			}},
			{title: "菜单管理", path: "/platform/menus", icon: "Menu", typ: model.MenuTypeMenu, perms: "system:menu:list", sort: 2, children: []menuSeed{
				{title: "新增菜单", typ: model.MenuTypeButton, perms: "system:menu:create", sort: 1},
				{title: "编辑菜单", typ: model.MenuTypeButton, perms: "system:menu:update", sort: 2},
				{title: "删除菜单", typ: model.MenuTypeButton, perms: "system:menu:delete", sort: 3},
			}},
			{title: "字典管理", path: "/platform/dicts", icon: "Collection", typ: model.MenuTypeMenu, perms: "system:dict:list", sort: 3, children: []menuSeed{
				{title: "新增字典", typ: model.MenuTypeButton, perms: "system:dict:create", sort: 1},
				{title: "编辑字典", typ: model.MenuTypeButton, perms: "system:dict:update", sort: 2},
				{title: "删除字典", typ: model.MenuTypeButton, perms: "system:dict:delete", sort: 3},
			}},
			{title: "系统配置", path: "/platform/configs", icon: "Operation", typ: model.MenuTypeMenu, perms: "system:config:list", sort: 4, children: []menuSeed{
				{title: "新增参数", typ: model.MenuTypeButton, perms: "system:config:create", sort: 1},
				{title: "编辑参数", typ: model.MenuTypeButton, perms: "system:config:update", sort: 2},
				{title: "删除参数", typ: model.MenuTypeButton, perms: "system:config:delete", sort: 3},
			}},
			{title: "品牌官网", path: "/platform/site", icon: "Platform", typ: model.MenuTypeMenu, perms: "system:site:list", sort: 5, children: []menuSeed{
				{title: "保存页面配置", typ: model.MenuTypeButton, perms: "system:site:update", sort: 1},
			}},
			// 应用发布（App 安装包/小程序码 + 强制更新标记）：自 00036 起从「品牌官网」拆出独立菜单
			{title: "应用发布", path: "/platform/releases", icon: "Iphone", typ: model.MenuTypeMenu, perms: "system:site:list", sort: 6, children: []menuSeed{
				{title: "上传发布物", typ: model.MenuTypeButton, perms: "system:site:upload", sort: 1},
				{title: "删除发布物", typ: model.MenuTypeButton, perms: "system:site:delete", sort: 2},
			}},
			// 岗位模板库（方案第三章）：平台模板岗位 + 平台默认槽位绑定，仅作开通租户时的初始拷贝源
			{title: "岗位模板库", path: "/platform/post-templates", icon: "CopyDocument", typ: model.MenuTypeMenu, perms: "platform:post:list", sort: 6, children: []menuSeed{
				{title: "新增模板岗位", typ: model.MenuTypeButton, perms: "platform:post:create", sort: 1},
				{title: "编辑模板岗位", typ: model.MenuTypeButton, perms: "platform:post:update", sort: 2},
				{title: "删除模板岗位", typ: model.MenuTypeButton, perms: "platform:post:delete", sort: 3},
				{title: "默认职责绑定", typ: model.MenuTypeButton, perms: "platform:post:duty", sort: 4},
			}},
			// 审批链模板（平台默认打卡审核链；开通租户时作为回落默认）
			{title: "审批流程模板", path: "/platform/review-flow-template", icon: "Share", typ: model.MenuTypeMenu, perms: "platform:reviewflow:list", sort: 7, children: []menuSeed{
				{title: "保存审批流程模板", typ: model.MenuTypeButton, perms: "platform:reviewflow:update", sort: 1},
			}},
		}},
		{title: "个人中心", path: "/profile", icon: "UserFilled", typ: model.MenuTypeMenu, perms: "profile:view", sort: 80, hidden: true},
	}

	ids := make(map[string]string)
	var walk func(nodes []menuSeed, parentID *string, platform bool) error
	walk = func(nodes []menuSeed, parentID *string, platform bool) error {
		for _, n := range nodes {
			np := platform || n.platform // 平台级目录的整棵子树继承标记（兄弟节点互不影响）
			m := model.SysMenu{
				ParentID:   parentID,
				Title:      n.title,
				Path:       n.path,
				Icon:       n.icon,
				Type:       n.typ,
				Perms:      n.perms,
				Sort:       n.sort,
				Visible:    n.typ != model.MenuTypeButton && !n.hidden,
				Status:     model.StatusEnabled,
				IsBuiltin:  true,
				IsPlatform: np,
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			if n.perms != "" {
				ids[n.perms] = m.ID
			}
			if n.path != "" {
				ids[n.path] = m.ID
			}
			if err := walk(n.children, &m.ID, np); err != nil {
				return err
			}
		}
		return nil
	}
	return ids, walk(tree, nil, false)
}

// seedRoleMenus 分配角色菜单：超管全量；租户管理员为全部非平台级菜单（is_platform=false，即系统管理 + 业务模块）；
// 项目管理员含工作台/巡检/统计/小区编制/个人中心。
func seedRoleMenus(tx *gorm.DB, roleIDs, menuIDs map[string]string) error {
	var allMenus []model.SysMenu
	if err := tx.Select("id", "perms", "path", "is_platform").Find(&allMenus).Error; err != nil {
		return err
	}
	var allMenuIDs, tenantAdminMenuIDs []string
	for _, m := range allMenus {
		allMenuIDs = append(allMenuIDs, m.ID)
		// 平台级菜单（is_platform：平台管理目录整棵子树）不下放租户管理员
		if m.IsPlatform {
			continue
		}
		tenantAdminMenuIDs = append(tenantAdminMenuIDs, m.ID)
	}
	// 项目管理员可见的权限点（目录按 path 收录，保证侧边栏结构完整）
	projectAdminPerms := []string{
		"/dashboard",
		"/inspection", "inspection:point:list", "inspection:point:create", "inspection:point:update", "inspection:point:delete", "inspection:point:qrcode", "inspection:point:import",
		"inspection:plan:list", "inspection:plan:create", "inspection:plan:update", "inspection:plan:disable",
		"inspection:task:monitor", "inspection:task:list", "inspection:record:list", "inspection:checkin:list",
		// 巡检记录按 path 补一条（perms 键只映射后写的菜单；问题清单已并入巡检记录）
		"/inspection/records",
		"inspection:template:list", "inspection:checkin:review", "inspection:checkin:spotcheck",
		"/stats", "stats:inspection", "stats:performance", "stats:report", "stats:export",
		"report:list", "report:generate", "report:download",
		"/community", "community:list", "community:staff:list", "community:staff:edit", "community:duty:edit",
		"/profile",
	}
	assign := func(roleID string, ids []string) error {
		// 去重：path 与 perms 两个键可能映射到同一菜单行（如小区管理 /community + community:list），
		// 同一角色重复绑定同一菜单会撞 sys_role_menu 主键
		seen := make(map[string]bool, len(ids))
		for _, mid := range ids {
			if seen[mid] {
				continue
			}
			seen[mid] = true
			if err := tx.Create(&model.SysRoleMenu{RoleID: roleID, MenuID: mid}).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if err := assign(roleIDs["super_admin"], allMenuIDs); err != nil {
		return err
	}
	// 租户管理员：租户内全部菜单（剔除 is_platform 平台级菜单；tenant:config 企业品牌保留）
	if err := assign(roleIDs["tenant_admin"], tenantAdminMenuIDs); err != nil {
		return err
	}
	var projectAdminMenuIDs []string
	for _, key := range projectAdminPerms {
		if id, ok := menuIDs[key]; ok {
			projectAdminMenuIDs = append(projectAdminMenuIDs, id)
		}
	}
	if err := assign(roleIDs["project_admin"], projectAdminMenuIDs); err != nil {
		return err
	}
	// 一线人员授予「月度报告」菜单（report:list）+ 巡检员确认按钮（report:sign:inspector）：
	// 月报三级签字的第一级"巡检员确认"要求应签巡检员本人调用（代签走 report:sign:proxy 留痕代签），
	// 否则报告永远停在 pending_inspector。PC 后台报告页即为其确认入口（生成/下载/上级签字按钮无权限点自动隐藏）；
	// 移动端调同一组签字接口，两端规则一致。
	fieldStaffMenuKeys := []string{"report:list", "report:sign:inspector"}
	var fieldStaffMenuIDs []string
	for _, key := range fieldStaffMenuKeys {
		if id, ok := menuIDs[key]; ok {
			fieldStaffMenuIDs = append(fieldStaffMenuIDs, id)
		}
	}
	return assign(roleIDs["field_staff"], fieldStaffMenuIDs)
}

// seedAdmin 写入超级管理员账号（凭据来自配置，bcrypt 入库；P3 起归属默认租户）。
func seedAdmin(tx *gorm.DB, superRoleID, tenantID, username, pwd, name string) error {
	hash, err := password.Hash(pwd)
	if err != nil {
		return err
	}
	admin := model.SysUser{
		TenantID:  tenantID,
		Username:  username,
		Password:  hash,
		Name:      name,
		IsBuiltin: true, // 唯一超管账号：禁止删除/停用/移除超管角色
		RoleIDs:   types.IDArray{superRoleID},
		Status:    model.StatusEnabled,
		Remark:    "系统预置超管，首次登录请修改密码",
	}
	return tx.Create(&admin).Error
}

// seedDicts 按《数据库设计文档》§4.1 全量预置字典。
func seedDicts(tx *gorm.DB) error {
	dicts := []struct {
		code  string
		name  string
		items [][2]string // label, value
	}{
		{"common_status", "通用状态", [][2]string{{"启用", "enabled"}, {"停用", "disabled"}}},
		{"data_scope", "数据范围", [][2]string{{"全部数据", "all"}, {"所在项目", "project"}, {"仅本人", "self"}}},
		{"menu_type", "菜单类型", [][2]string{{"目录", "dir"}, {"菜单", "menu"}, {"按钮", "button"}}},
		{"building_type", "楼栋类型", [][2]string{{"楼栋", "building"}, {"区域", "area"}}},
		{"point_type", "点位类型", [][2]string{{"普通点位", "common"}, {"配电房", "power_room"}, {"消防控制室", "fire_control"}, {"水泵房", "pump_room"}, {"电梯机房", "elevator"}, {"地下车库", "garage"}, {"消防箱", "fire_cabinet"}, {"灭火器", "fire_extinguisher"}}},
		{"cycle_type", "计划周期", [][2]string{{"每天", "daily"}, {"每周", "weekly"}, {"每月", "monthly"}}},
		{"task_status", "任务状态", [][2]string{{"待开始", "pending"}, {"进行中", "doing"}, {"已完成", "done"}, {"已逾期", "overdue"}}},
		{"checkin_type", "打卡类型", [][2]string{{"扫码", "qrcode"}, {"围栏", "fence"}, {"离线补传", "offline"}, {"NFC", "nfc"}}},
		{"checkin_result", "打卡结果", [][2]string{{"正常", "normal"}, {"异常", "abnormal"}}},
		{"patrol_type", "巡查类型", [][2]string{{"安全巡查", "safety"}, {"设备设施专项巡查", "equipment"}, {"环境巡查", "environment"}, {"楼栋巡查", "building"}, {"消防设施专项", "fire"}}},
	}
	for _, d := range dicts {
		t := model.SysDictType{Code: d.code, Name: d.name, Remark: "系统预置"}
		// OnConflict 跳过：同名字典已存在（历史库/迁移补丁）时跳过，保证可重复执行
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&t).Error; err != nil {
			return err
		}
		for i, item := range d.items {
			data := model.SysDictData{
				TypeCode: d.code,
				Label:    item[0],
				Value:    item[1],
				Sort:     i + 1,
				Status:   model.StatusEnabled,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&data).Error; err != nil {
				return err
			}
		}
	}
	// patrol_type 大类标记 attrs.category（《专项巡检与专项检查报告设计方案》§3.1：
	// daily_patrol 日常巡逻 / special 专项检查；前端按大类分组展示）
	patrolCategories := []struct {
		value    string
		category string
	}{
		{"safety", "daily_patrol"},
		{"equipment", "special"},
		{"environment", "special"},
		{"building", "special"},
		{"fire", "special"},
	}
	for _, pc := range patrolCategories {
		if err := tx.Model(&model.SysDictData{}).
			Where("type_code = ? AND value = ?", "patrol_type", pc.value).
			Update("attrs", types.JSONMap{"category": pc.category}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedConfigs 预置系统参数（《数据库设计文档》§6.4）。
func seedConfigs(tx *gorm.DB) error {
	configs := []model.SysConfig{
		{Key: "inspection.fence_default_radius", Name: "围栏默认半径(米)", Value: "100", ConfigGroup: "inspection", Remark: "新建点位时的默认值"},
		{Key: "inspection.watermark_enabled", Name: "照片水印开关", Value: "true", ConfigGroup: "inspection", Remark: "关闭后仅存原图（不推荐）"},
		{Key: "inspection.suspect_distance_ratio", Name: "疑似作弊距离倍数", Value: "1.0", ConfigGroup: "inspection", Remark: "距离 > fence_radius × 该值 时标记疑似作弊"},
		{Key: "inspection.exif_deviation_seconds", Name: "EXIF 时间偏差阈值(秒)", Value: "300", ConfigGroup: "inspection", Remark: "照片拍摄时间与打卡时间允许偏差"},
		{Key: "inspection.task_generate_days", Name: "任务提前生成天数", Value: "1", ConfigGroup: "inspection", Remark: "每日 00:05 生成未来 N 天任务"},
		{Key: "inspection.overdue_check_time", Name: "逾期翻转时间", Value: "00:10", ConfigGroup: "inspection", Remark: "每日该时刻将昨日未完成任务置 overdue"},
		{Key: "inspection.route_optimize", Name: "路线自动优化开关", Value: "true", ConfigGroup: "inspection", Remark: "任务生成时按楼栋聚类+最近邻重排点位，减少折返"},
		{Key: "mp.offline_sync_limit", Name: "离线补传单批上限", Value: "50", ConfigGroup: "mp", Remark: "单次补传最大条数"},
		{Key: "msg.subscribe_enabled", Name: "微信订阅消息开关", Value: "true", ConfigGroup: "msg", Remark: "任务提醒/审核通知"},
		{Key: "msg.wecom_webhook_enabled", Name: "企业微信消息推送开关", Value: "false", ConfigGroup: "msg", Remark: "开启需配置 webhook 地址（应用配置，不入库）"},
		{Key: "security.login_fail_limit", Name: "登录失败锁定次数", Value: "5", ConfigGroup: "security", Remark: "连续失败锁定 10 分钟（配合 Redis 计数）"},
		{Key: "map.tencent_key", Name: "腾讯地图前端 Key", Value: "", ConfigGroup: "map", Remark: "用于管理后台点位地图选点，需在腾讯位置服务配置 JSAPI 域名白名单"},
		{Key: "map.tencent_ws_key", Name: "腾讯地图 WebService Key", Value: "", ConfigGroup: "map", Remark: "用于后端代理地点搜索；留空时回退 map.tencent_key"},
		{Key: "ai.enabled", Name: "启用大模型审核", Value: "false", ConfigGroup: "ai", Remark: "开启后打卡照片由大模型辅助审查"},
		{Key: "ai.base_url", Name: "API 地址", Value: "https://api.openai.com/v1", ConfigGroup: "ai", Remark: "OpenAI 兼容接口地址"},
		{Key: "ai.api_key", Name: "API Key", Value: "", ConfigGroup: "ai", Remark: "sk-...（服务端保管，不回传前端）"},
		{Key: "ai.model", Name: "模型名称", Value: "gpt-4o-mini", ConfigGroup: "ai", Remark: "须支持图像理解（vision）"},
		{Key: "ai.timeout_seconds", Name: "超时秒数", Value: "60", ConfigGroup: "ai", Remark: "超时/失败默认通过并记录 ai_verdict=error"},
		{Key: "ai.prompt", Name: "审查规则", Value: "", ConfigGroup: "ai", Remark: "留空用内置规则：判断照片清晰度、与点位/检查项匹配度、有无明显异常，输出 pass/review"},
		{Key: "ai.protocol", Name: "AI 接口协议", Value: "openai_chat", ConfigGroup: "ai", Remark: "openai_chat（OpenAI 兼容 Chat Completions）/openai_responses/gemini/claude"},
		{Key: "ai.platform", Name: "AI 平台预设", Value: "", ConfigGroup: "ai", Remark: "平台预设标识（仅展示用，如 openai/qwen/doubao/gemini/claude）；实际生效以 protocol/base_url/model 为准"},
		{Key: "ai.sync_enabled", Name: "打卡同步 AI 判定", Value: "true", ConfigGroup: "ai", Remark: "开启后打卡提交时同步做质量+内容两层判定：质量不达标拒绝打卡，失败/超时放行转人工复核"},
		{Key: "ai.sync_timeout_seconds", Name: "同步判定超时秒数", Value: "15", ConfigGroup: "ai", Remark: "打卡同步 AI 判定的超时时间；超时按失败放行（ai_verdict=error 转人工复核）"},
		{Key: "ai.max_photo_attempts", Name: "照片质量重拍放行次数", Value: "3", ConfigGroup: "ai", Remark: "照片质量不达标允许重拍的次数，达到上限后 App 端可强制提交（App 端读取）"},
		{Key: "ai.result_editable", Name: "打卡结果允许覆盖修改", Value: "true", ConfigGroup: "ai", Remark: "关闭后已提交点位不可重拍覆盖（App 端读取）"},
		{Key: "ai.worker_concurrency", Name: "逐项识别并发数", Value: "4", ConfigGroup: "ai", Remark: "逐项 AI 识别队列的消费 worker 数（服务启动时读取）"},
		{Key: "report.company_name", Name: "管理单位落款", Value: "", ConfigGroup: "report", Remark: "月报封面\"管理单位\"与页尾落款单位名称；空则留白"},
		{Key: "site.slogan", Name: "官网标语", Value: "二维码 / NFC / GPS 围栏三重到点校验，拍照留证、异常复核、月度报告电子签，巡检情况后台一目了然。", ConfigGroup: "site", Remark: "官网首页主标题下的一句话介绍"},
		{Key: "site.contact_phone", Name: "联系电话", Value: "", ConfigGroup: "site", Remark: "官网页脚展示，留空不显示"},
		{Key: "site.contact_email", Name: "联系邮箱", Value: "", ConfigGroup: "site", Remark: "官网页脚展示，留空不显示"},
		{Key: "site.theme_color", Name: "官网主题色", Value: "#2b5aed", ConfigGroup: "site", Remark: "官网按钮/强调色，十六进制色值"},
		{Key: "site.footer_note", Name: "页脚文案", Value: "安巡云 AnXunCloud · 让每一次巡检都有据可查", ConfigGroup: "site", Remark: "官网页脚底部一行文案"},
		{Key: "site.company_name", Name: "公司名称", Value: "", ConfigGroup: "site", Remark: "官网联系区块与结构化数据展示"},
		{Key: "site.contact_wechat", Name: "微信号", Value: "", ConfigGroup: "site", Remark: "官网联系区块展示，留空不显示"},
		{Key: "site.address", Name: "公司地址", Value: "", ConfigGroup: "site", Remark: "官网联系区块与结构化数据展示，留空不显示"},
		{Key: "site.icp", Name: "ICP 备案号", Value: "", ConfigGroup: "site", Remark: "官网页脚展示（如 鄂ICP备2024xxxxxx号-1），留空不显示"},
		{Key: "auth.register_enabled", Name: "开放注册开关", Value: "false", ConfigGroup: "security", Remark: "开启后登录页显示注册入口，注册需选择所属公司"},
		// 公章自 v16 起由 sign_asset 签章资产表管理，不再使用 report.seal_file_key 配置项
	}
	for i := range configs {
		// OnConflict 跳过：同名配置项已存在（历史库/迁移补丁）时跳过，保证可重复执行
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&configs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedPosts 预置平台模板岗位（post_dict.tenant_id 空，菜单归位方案 §3）。
// 仅作开通租户时的复制源；role_id 绑定内置共享角色（有效角色实时并集的来源之一）。
func seedPosts(tx *gorm.DB, roleIDs map[string]string) error {
	type postSeed struct {
		code         string
		name         string
		line         string
		isSupervisor bool
		roleCode     string // 空 = 不绑角色
		sort         int
		status       string
		remark       string
	}
	posts := []postSeed{
		{"project_manager", "项目经理", "general", false, "project_admin", 1, model.StatusEnabled, "项目第一负责人，月报终审，全员管理"},
		{"safety_supervisor", "安全主管", "safety", true, "project_admin", 2, model.StatusEnabled, "管理巡检员，安全/秩序巡查，月报主管审批"},
		{"inspector", "巡检员", "safety", false, "field_staff", 3, model.StatusEnabled, "按计划执行巡查打卡"},
		{"engineering_supervisor", "工程主管", "engineering", true, "project_admin", 4, model.StatusEnabled, "管理维修工，设备设施专项巡查"},
		{"repairman", "维修工", "engineering", false, "field_staff", 5, model.StatusEnabled, "设备设施专项巡查与维修"},
		{"environment_supervisor", "环境主管", "environment", true, "project_admin", 6, model.StatusEnabled, "环境卫生/绿化巡查管理"},
		{"cleaner", "保洁员", "environment", false, "field_staff", 7, model.StatusDisabled, "预留岗位，本期不进系统"},
		{"service_supervisor", "客服主管", "service", true, "project_admin", 8, model.StatusEnabled, "管理前台接待和楼管员，报单受理"},
		{"building_manager", "楼管员", "service", false, "field_staff", 9, model.StatusEnabled, "负责若干楼栋，日常巡查、主动报单"},
		{"receptionist", "前台接待", "service", false, "field_staff", 10, model.StatusEnabled, "前台接报、录入报单"},
	}
	for _, p := range posts {
		// 幂等：平台模板行（tenant_id 为空）按 code 判重，缺失才补
		var n int64
		if err := tx.Model(&model.PostDict{}).Where("tenant_id IS NULL AND code = ?", p.code).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := model.PostDict{
			Code: p.code, Name: p.name, Line: p.line, IsSupervisor: p.isSupervisor,
			Sort: p.sort, Status: p.status, Remark: p.remark,
		}
		if p.roleCode != "" {
			if rid, ok := roleIDs[p.roleCode]; ok && rid != "" {
				row.RoleID = &rid
			}
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ensurePlatformRows 平台默认行自愈（每次启动执行，幂等）：
// post_dict 平台模板岗位 / duty_binding 平台默认槽位绑定 / approval_flow 平台默认审批链
// 均为 tenant_id 为空的行，是开通租户复制源（CopyPostTemplatesToTenant）与配置回落的最末一级，
// 被误清后这里按 code/slot/flow_code 补齐缺失行。
func ensurePlatformRows(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var roles []model.SysRole
		if err := tx.Select("id", "code").Find(&roles).Error; err != nil {
			return err
		}
		roleIDs := make(map[string]string, len(roles))
		for _, r := range roles {
			roleIDs[r.Code] = r.ID
		}
		if err := seedPosts(tx, roleIDs); err != nil { // 内部按 code 判重
			return err
		}
		if err := seedDutyBindings(tx); err != nil { // OnConflict 幂等
			return err
		}
		if err := seedConfigs(tx); err != nil { // OnConflict 幂等（新增配置项补入存量库）
			return err
		}
		var n int64
		if err := tx.Model(&model.ApprovalFlow{}).
			Where("tenant_id IS NULL AND project_id IS NULL AND flow_code = ?", model.FlowCheckinReview).
			Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return seedApprovalFlow(tx)
		}
		return nil
	})
}

// seedDutyBindings 预置平台默认职责槽位绑定（duty_binding：project_id/tenant_id 均空 = 平台默认）。
// 三级回落的最末一级；租户级/项目级绑定开通或配置时复制/覆盖。
func seedDutyBindings(tx *gorm.DB) error {
	bindings := []struct {
		slot  string
		codes types.StringArray
	}{
		{model.SlotReportSignSupervisor, types.StringArray{"safety_supervisor"}},
		{model.SlotReportSignManager, types.StringArray{"project_manager"}},
		{model.SlotPatrolExecute, types.StringArray{"inspector"}},
		{model.SlotPatrolReportLine, types.StringArray{"safety_supervisor"}},
		// 汇报线业务线维度槽位（扩展方案 §2.4）：安全线不设默认（回落通用槽位），
		// 设备/环境/楼栋线分别归工程/环境/客服主管；
		// fire（消防设施专项，专项巡检方案 §3.1）维度槽位按约定 patrol_report_line.<type> 衍生，默认归工程主管
		{model.SlotPatrolReportLineEquipment, types.StringArray{"engineering_supervisor"}},
		{model.SlotPatrolReportLineEnvironment, types.StringArray{"environment_supervisor"}},
		{model.SlotPatrolReportLineBuilding, types.StringArray{"service_supervisor"}},
		{model.SlotPatrolReportLine + ".fire", types.StringArray{"engineering_supervisor"}},
		// 项目经理复核槽位（审批链第二环节引用，扩展方案 §3.2）
		{model.SlotProjectReview, types.StringArray{"project_manager"}},
	}
	for _, b := range bindings {
		// OnConflict 跳过：同名槽位绑定已存在（历史库/迁移补丁）时跳过，保证可重复执行
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.DutyBinding{Slot: b.slot, PostCodes: b.codes}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedApprovalFlow 预置平台默认审批链（approval_flow：tenant_id/project_id 均空 = 平台默认）。
// 打卡审批流程默认单步「主管审核」（与链化前行为一致；租户可自行追加"项目经理复核"等环节）。
func seedApprovalFlow(tx *gorm.DB) error {
	flow := model.ApprovalFlow{
		FlowCode: model.FlowCheckinReview,
		Steps: types.FlowStepArray{
			{Slot: model.SlotPatrolReportLine, Name: "主管审核"},
		},
	}
	return tx.Create(&flow).Error
}
