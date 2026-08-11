package database

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/types"
)

// Seed 写入系统预置数据（超管、内置角色、菜单树、字典、默认参数）。
// 仅当 sys_role 为空时执行，保证幂等。超管账号来自配置（ADMIN_USERNAME/PASSWORD/NAME）。
func Seed(db *gorm.DB, adminUsername, adminPassword, adminName string) error {
	var count int64
	if err := db.Model(&model.SysRole{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
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
		if err := seedAdmin(tx, roleIDs["super_admin"], adminUsername, adminPassword, adminName); err != nil {
			return err
		}
		if err := seedDicts(tx); err != nil {
			return err
		}
		return seedConfigs(tx)
	})
}

// seedRoles 预置四个内置角色，返回 code → id 映射。
func seedRoles(tx *gorm.DB) (map[string]string, error) {
	roles := []model.SysRole{
		{Code: "super_admin", Name: "超级管理员", DataScope: model.ScopeAll, Remark: "拥有全部权限", IsBuiltin: true},
		{Code: "manager", Name: "物业主管", DataScope: model.ScopeCustom, Remark: "按所辖小区查看数据", IsBuiltin: true},
		{Code: "inspector", Name: "巡检员", DataScope: model.ScopeCustom, Remark: "小程序端打卡巡检", IsBuiltin: true},
		{Code: "repair", Name: "维修人员", DataScope: model.ScopeCustom, Remark: "小程序端处理工单", IsBuiltin: true},
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
			{title: "巡检计划", path: "/inspection/plans", icon: "Calendar", typ: model.MenuTypeMenu, perms: "inspection:plan:list", sort: 2, children: []menuSeed{
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
			{title: "检查项模板", path: "/inspection/templates", icon: "Finished", typ: model.MenuTypeMenu, perms: "inspection:template:list", sort: 5, children: []menuSeed{
				{title: "新增模板", typ: model.MenuTypeButton, perms: "inspection:template:create", sort: 1},
				{title: "编辑模板", typ: model.MenuTypeButton, perms: "inspection:template:update", sort: 2},
				{title: "删除模板", typ: model.MenuTypeButton, perms: "inspection:template:delete", sort: 3},
			}},
		}},
		{title: "工单管理", path: "/workorders", icon: "Tools", typ: model.MenuTypeDir, sort: 30, children: []menuSeed{
			{title: "工单列表", path: "/workorders/list", icon: "List", typ: model.MenuTypeMenu, perms: "workorder:list", sort: 1, children: []menuSeed{
				{title: "新建工单", typ: model.MenuTypeButton, perms: "workorder:create", sort: 1},
				{title: "编辑工单", typ: model.MenuTypeButton, perms: "workorder:update", sort: 2},
				{title: "删除工单", typ: model.MenuTypeButton, perms: "workorder:delete", sort: 3},
				{title: "派单", typ: model.MenuTypeButton, perms: "workorder:assign", sort: 4},
				{title: "处理反馈", typ: model.MenuTypeButton, perms: "workorder:finish", sort: 5},
				{title: "复核", typ: model.MenuTypeButton, perms: "workorder:review", sort: 6},
				{title: "导出", typ: model.MenuTypeButton, perms: "workorder:export", sort: 7},
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
				{title: "主管签字", typ: model.MenuTypeButton, perms: "report:sign:supervisor", sort: 3},
				{title: "经理终审", typ: model.MenuTypeButton, perms: "report:sign:manager", sort: 4},
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
		}},
		{title: "系统管理", path: "/system", icon: "Setting", typ: model.MenuTypeDir, sort: 60, children: []menuSeed{
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
			{title: "菜单管理", path: "/system/menus", icon: "Menu", typ: model.MenuTypeMenu, perms: "system:menu:list", sort: 3, children: []menuSeed{
				{title: "新增菜单", typ: model.MenuTypeButton, perms: "system:menu:create", sort: 1},
				{title: "编辑菜单", typ: model.MenuTypeButton, perms: "system:menu:update", sort: 2},
				{title: "删除菜单", typ: model.MenuTypeButton, perms: "system:menu:delete", sort: 3},
			}},
			{title: "字典管理", path: "/system/dicts", icon: "Collection", typ: model.MenuTypeMenu, perms: "system:dict:list", sort: 4, children: []menuSeed{
				{title: "新增字典", typ: model.MenuTypeButton, perms: "system:dict:create", sort: 1},
				{title: "编辑字典", typ: model.MenuTypeButton, perms: "system:dict:update", sort: 2},
				{title: "删除字典", typ: model.MenuTypeButton, perms: "system:dict:delete", sort: 3},
			}},
			{title: "系统配置", path: "/system/configs", icon: "Operation", typ: model.MenuTypeMenu, perms: "system:config:list", sort: 5, children: []menuSeed{
				{title: "新增参数", typ: model.MenuTypeButton, perms: "system:config:create", sort: 1},
				{title: "编辑参数", typ: model.MenuTypeButton, perms: "system:config:update", sort: 2},
				{title: "删除参数", typ: model.MenuTypeButton, perms: "system:config:delete", sort: 3},
			}},
			{title: "通知公告", path: "/system/notices", icon: "Bell", typ: model.MenuTypeMenu, perms: "system:notice:list", sort: 6, children: []menuSeed{
				{title: "新增公告", typ: model.MenuTypeButton, perms: "system:notice:create", sort: 1},
				{title: "编辑公告", typ: model.MenuTypeButton, perms: "system:notice:update", sort: 2},
				{title: "删除公告", typ: model.MenuTypeButton, perms: "system:notice:delete", sort: 3},
			}},
			{title: "日志管理", path: "/system/logs", icon: "Tickets", typ: model.MenuTypeMenu, perms: "system:log:list", sort: 7, children: []menuSeed{
				{title: "操作日志", typ: model.MenuTypeButton, perms: "system:log:operation", sort: 1},
				{title: "登录日志", typ: model.MenuTypeButton, perms: "system:log:login", sort: 2},
				{title: "日志导出", typ: model.MenuTypeButton, perms: "system:log:export", sort: 3},
			}},
			{title: "签章管理", path: "/system/sign-assets", icon: "Stamp", typ: model.MenuTypeMenu, perms: "system:signasset:list", sort: 8, children: []menuSeed{
				{title: "新增签章", typ: model.MenuTypeButton, perms: "system:signasset:create", sort: 1},
				{title: "作废签章", typ: model.MenuTypeButton, perms: "system:signasset:revoke", sort: 2},
			}},
		}},
		{title: "个人中心", path: "/profile", icon: "UserFilled", typ: model.MenuTypeMenu, perms: "profile:view", sort: 70, hidden: true},
	}

	ids := make(map[string]string)
	var walk func(nodes []menuSeed, parentID *string) error
	walk = func(nodes []menuSeed, parentID *string) error {
		for _, n := range nodes {
			m := model.SysMenu{
				ParentID:  parentID,
				Title:     n.title,
				Path:      n.path,
				Icon:      n.icon,
				Type:      n.typ,
				Perms:     n.perms,
				Sort:      n.sort,
				Visible:   n.typ != model.MenuTypeButton && !n.hidden,
				Status:    model.StatusEnabled,
				IsBuiltin: true,
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
			if err := walk(n.children, &m.ID); err != nil {
				return err
			}
		}
		return nil
	}
	return ids, walk(tree, nil)
}

// seedRoleMenus 分配角色菜单：超管全量；主管含工作台/巡检/工单/统计/个人中心。
func seedRoleMenus(tx *gorm.DB, roleIDs, menuIDs map[string]string) error {
	var allMenuIDs []string
	if err := tx.Model(&model.SysMenu{}).Pluck("id", &allMenuIDs).Error; err != nil {
		return err
	}
	// 主管可见的权限点（目录按 path 收录，保证侧边栏结构完整）
	managerPerms := []string{
		"/dashboard",
		"/inspection", "inspection:point:list", "inspection:point:create", "inspection:point:update", "inspection:point:delete", "inspection:point:qrcode", "inspection:point:import",
		"inspection:plan:list", "inspection:plan:create", "inspection:plan:update", "inspection:plan:disable",
		"inspection:task:monitor", "inspection:task:list", "inspection:record:list", "inspection:checkin:list",
		"inspection:template:list", "inspection:checkin:review", "inspection:checkin:spotcheck",
		"/workorders", "workorder:list", "workorder:create", "workorder:update", "workorder:delete",
		"workorder:assign", "workorder:finish", "workorder:review", "workorder:export",
		"/stats", "stats:inspection", "stats:performance", "stats:report", "stats:export",
		"report:list", "report:generate", "report:sign:supervisor", "report:sign:manager", "report:download",
		"/profile",
	}
	assign := func(roleID string, ids []string) error {
		for _, mid := range ids {
			if err := tx.Create(&model.SysRoleMenu{RoleID: roleID, MenuID: mid}).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if err := assign(roleIDs["super_admin"], allMenuIDs); err != nil {
		return err
	}
	var managerMenuIDs []string
	for _, key := range managerPerms {
		if id, ok := menuIDs[key]; ok {
			managerMenuIDs = append(managerMenuIDs, id)
		}
	}
	// 巡检员授予「月度报告」菜单（report:list）+ 巡检员确认按钮（report:sign:inspector）：
	// 月报三级签字的第一级"巡检员确认"要求应签巡检员本人调用（代签走 report:sign:proxy 留痕代签），
	// 否则报告永远停在 pending_inspector。PC 后台报告页即为其确认入口（生成/下载/上级签字按钮无权限点自动隐藏）；
	// 小程序端调同一组签字接口，两端规则一致。
	inspectorMenuKeys := []string{"report:list", "report:sign:inspector"}
	var inspectorMenuIDs []string
	for _, key := range inspectorMenuKeys {
		if id, ok := menuIDs[key]; ok {
			inspectorMenuIDs = append(inspectorMenuIDs, id)
		}
	}
	if err := assign(roleIDs["inspector"], inspectorMenuIDs); err != nil {
		return err
	}
	// 维修工仅有小程序接口权限点，无后台菜单，不分配
	return assign(roleIDs["manager"], managerMenuIDs)
}

// seedAdmin 写入超级管理员账号（凭据来自配置，bcrypt 入库）。
func seedAdmin(tx *gorm.DB, superRoleID string, username, pwd, name string) error {
	hash, err := password.Hash(pwd)
	if err != nil {
		return err
	}
	admin := model.SysUser{
		Username: username,
		Password: hash,
		Name:     name,
		IsBuiltin: true, // 唯一超管账号：禁止删除/停用/移除超管角色
		RoleIDs:  types.IDArray{superRoleID},
		UserType: "admin",
		Status:   model.StatusEnabled,
		Remark:   "系统预置超管，首次登录请修改密码",
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
		{"user_type", "用户类型", [][2]string{{"后台管理员", "admin"}, {"巡检员", "inspector"}, {"维修工", "repair"}}},
		{"data_scope", "数据范围", [][2]string{{"全部数据", "all"}, {"按小区", "custom"}}},
		{"menu_type", "菜单类型", [][2]string{{"目录", "dir"}, {"菜单", "menu"}, {"按钮", "button"}}},
		{"building_type", "楼栋类型", [][2]string{{"楼栋", "building"}, {"区域", "area"}}},
		{"point_type", "点位类型", [][2]string{{"普通点位", "common"}, {"配电房", "power_room"}, {"消防控制室", "fire_control"}, {"水泵房", "pump_room"}, {"电梯机房", "elevator"}, {"地下车库", "garage"}}},
		{"cycle_type", "计划周期", [][2]string{{"每天", "daily"}, {"每周", "weekly"}, {"每月", "monthly"}}},
		{"task_status", "任务状态", [][2]string{{"待开始", "pending"}, {"进行中", "doing"}, {"已完成", "done"}, {"已逾期", "overdue"}}},
		{"checkin_type", "打卡类型", [][2]string{{"扫码", "qrcode"}, {"围栏", "fence"}, {"离线补传", "offline"}, {"NFC", "nfc"}}},
		{"checkin_result", "打卡结果", [][2]string{{"正常", "normal"}, {"异常", "abnormal"}}},
		{"order_priority", "工单优先级", [][2]string{{"低", "low"}, {"普通", "normal"}, {"高", "high"}, {"紧急", "urgent"}}},
		{"work_order_status", "工单状态", [][2]string{{"待派单", "pending"}, {"已派单", "assigned"}, {"处理中", "processing"}, {"待复核", "review"}, {"已关闭", "closed"}, {"已驳回", "rejected"}}},
	}
	for _, d := range dicts {
		t := model.SysDictType{Code: d.code, Name: d.name, Remark: "系统预置"}
		if err := tx.Create(&t).Error; err != nil {
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
			if err := tx.Create(&data).Error; err != nil {
				return err
			}
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
		{Key: "mp.offline_sync_limit", Name: "离线补传单批上限", Value: "50", ConfigGroup: "mp", Remark: "单次补传最大条数"},
		{Key: "msg.subscribe_enabled", Name: "微信订阅消息开关", Value: "true", ConfigGroup: "msg", Remark: "任务提醒/工单指派/整改驳回"},
		{Key: "msg.wecom_webhook_enabled", Name: "企业微信工单推送开关", Value: "false", ConfigGroup: "msg", Remark: "开启需配置 webhook 地址（应用配置，不入库）"},
		{Key: "security.login_fail_limit", Name: "登录失败锁定次数", Value: "5", ConfigGroup: "security", Remark: "连续失败锁定 10 分钟（配合 Redis 计数）"},
		{Key: "ai.enabled", Name: "启用大模型审核", Value: "false", ConfigGroup: "ai", Remark: "开启后打卡照片由大模型辅助审查"},
		{Key: "ai.base_url", Name: "API 地址", Value: "https://api.openai.com/v1", ConfigGroup: "ai", Remark: "OpenAI 兼容接口地址"},
		{Key: "ai.api_key", Name: "API Key", Value: "", ConfigGroup: "ai", Remark: "sk-...（服务端保管，不回传前端）"},
		{Key: "ai.model", Name: "模型名称", Value: "gpt-4o-mini", ConfigGroup: "ai", Remark: "须支持图像理解（vision）"},
		{Key: "ai.timeout_seconds", Name: "超时秒数", Value: "60", ConfigGroup: "ai", Remark: "超时/失败默认通过并记录 ai_verdict=error"},
		{Key: "ai.prompt", Name: "审查规则", Value: "", ConfigGroup: "ai", Remark: "留空用内置规则：判断照片清晰度、与点位/检查项匹配度、有无明显异常，输出 pass/review"},
		{Key: "report.company_name", Name: "管理单位落款", Value: "", ConfigGroup: "report", Remark: "月报封面\"管理单位\"与页尾落款单位名称；空则留白"},
		// 公章自 v16 起由 sign_asset 签章资产表管理，不再使用 report.seal_file_key 配置项
		// auth.register_enabled 由迁移 v7 统一插入（覆盖新库与存量库），seed 不再重复
	}
	for i := range configs {
		if err := tx.Create(&configs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
