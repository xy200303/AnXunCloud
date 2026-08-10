// Package router 路由注册（/api/admin、/api/mp 分组）。
package router

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	authctl "anxuncloud/internal/module/auth/controller"
	authsvc "anxuncloud/internal/module/auth/service"
	communityctl "anxuncloud/internal/module/community/controller"
	communitysvc "anxuncloud/internal/module/community/service"
	inspectionctl "anxuncloud/internal/module/inspection/controller"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	mpctl "anxuncloud/internal/module/mp/controller"
	mpsvc "anxuncloud/internal/module/mp/service"
	reportctl "anxuncloud/internal/module/report/controller"
	reportsvc "anxuncloud/internal/module/report/service"
	statsctl "anxuncloud/internal/module/stats/controller"
	statssvc "anxuncloud/internal/module/stats/service"
	systemctl "anxuncloud/internal/module/system/controller"
	systemsvc "anxuncloud/internal/module/system/service"
	workorderctl "anxuncloud/internal/module/workorder/controller"
	workordersvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/watermark"
)

// New 构建 HTTP 引擎并注册全部路由；返回引擎与巡检调度器（由 main 启停）。
func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client) (*gin.Engine, *inspectionsvc.Scheduler) {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.CORS(cfg.CORS.AllowOrigins))

	// 依赖装配
	jwtm := jwtutil.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	sess := session.NewStore(rdb)
	store := storage.New(cfg.Upload, cfg.OSS, cfg.App.BaseURL)
	watermark.Init(cfg.Watermark.FontPath)

	configSvc := systemsvc.NewConfigService(db, rdb)
	authSvc := authsvc.NewAuthService(db, rdb, sess, jwtm, configSvc.Get, store)
	signAssetSvc := systemsvc.NewSignAssetService(db, store)
	userSvc := systemsvc.NewUserService(db, authSvc.KillUserSessions, store, signAssetSvc)
	roleSvc := systemsvc.NewRoleService(db)
	menuSvc := systemsvc.NewMenuService(db)
	dictSvc := systemsvc.NewDictService(db)
	logSvc := systemsvc.NewLogService(db)
	noticeSvc := systemsvc.NewNoticeService(db)
	messageSvc := systemsvc.NewMessageService(db)
	communitySvc := communitysvc.NewCommunityService(db)
	pointSvc := inspectionsvc.NewPointService(db, store, configSvc.Get)
	planSvc := inspectionsvc.NewPlanService(db, rdb)
	taskSvc := inspectionsvc.NewTaskService(db, store)
	templateSvc := inspectionsvc.NewTemplateService(db)
	reviewSvc := inspectionsvc.NewReviewService(db)
	orderSvc := workordersvc.NewOrderService(db, rdb, store)
	statsSvc := statssvc.NewStatsService(db, store)
	reportSvc := reportsvc.NewReportService(db, store, configSvc.Get)
	mpSvc := mpsvc.NewMPService(db, rdb, sess, jwtm, cfg.Wechat, orderSvc)
	checkinSvc := mpsvc.NewCheckinService(db, rdb, store, orderSvc, configSvc.Get)
	uploadSvc := mpsvc.NewUploadService(db, store, cfg.Upload, cfg.OSS)
	scheduler := inspectionsvc.NewScheduler(db, planSvc, reportSvc, configSvc.Get)

	authCtl := authctl.NewAuthController(authSvc)
	userCtl := systemctl.NewUserController(userSvc)
	roleCtl := systemctl.NewRoleController(roleSvc)
	menuCtl := systemctl.NewMenuController(menuSvc)
	dictCtl := systemctl.NewDictController(dictSvc)
	configCtl := systemctl.NewConfigController(configSvc)
	logCtl := systemctl.NewLogController(logSvc)
	noticeCtl := systemctl.NewNoticeController(noticeSvc)
	messageCtl := systemctl.NewMessageController(messageSvc)
	uploadCtl := systemctl.NewUploadController(uploadSvc)
	signAssetCtl := systemctl.NewSignAssetController(signAssetSvc)
	communityCtl := communityctl.NewCommunityController(communitySvc)
	inspectionCtl := inspectionctl.NewInspectionController(pointSvc, planSvc, taskSvc)
	templateCtl := inspectionctl.NewTemplateController(templateSvc)
	reviewCtl := inspectionctl.NewReviewController(reviewSvc)
	orderCtl := workorderctl.NewOrderController(orderSvc)
	statsCtl := statsctl.NewStatsController(statsSvc)
	reportCtl := reportctl.NewReportController(reportSvc)
	mpCtl := mpctl.NewMPController(mpSvc, checkinSvc, uploadSvc, orderSvc, noticeSvc)

	// 健康检查 + dev 模式本地文件静态路由
	r.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "up"}) })
	r.Static("/uploads", cfg.Upload.LocalDir)

	admin := r.Group("/api/admin")
	{
		admin.POST("/auth/login", authCtl.Login) // 登录不写操作日志，由 sys_login_log 覆盖
		admin.POST("/auth/refresh", authCtl.Refresh)
		admin.GET("/auth/register-config", authCtl.RegisterConfig)
		admin.POST("/auth/register", authCtl.Register) // 注册不写操作日志，避免匿名噪声行
	}

	// 管理后台鉴权分组
	secured := admin.Group("", middleware.Auth(db, sess, jwtm, authsvc.ChannelAdmin))
	{
		secured.POST("/auth/logout", authCtl.Logout)
		secured.GET("/auth/info", authCtl.Info)
		secured.GET("/auth/routes", authCtl.Routes)
		secured.GET("/dashboard", statsCtl.Dashboard)

		// 顶栏消息（登录即可，仅本人消息）
		secured.GET("/system/messages", messageCtl.List)
		secured.PUT("/system/messages/:id/read", messageCtl.MarkRead)

		sys := secured.Group("/system")
		registerSystemRoutes(sys, db, userCtl, roleCtl, menuCtl, dictCtl, configCtl, logCtl, noticeCtl, uploadCtl, signAssetCtl)

		// 小区与楼栋
		secured.GET("/communities", middleware.RequirePerm("community:list"), communityCtl.List)
		secured.GET("/communities/tree", middleware.RequirePerm("community:list"), communityCtl.Tree)
		secured.POST("/communities", middleware.RequirePerm("community:create"), middleware.OperLog(db, "community", "create"), communityCtl.Create)
		secured.GET("/communities/:id", middleware.RequirePerm("community:list"), communityCtl.Detail)
		secured.PUT("/communities/:id", middleware.RequirePerm("community:update"), middleware.OperLog(db, "community", "update"), communityCtl.Update)
		secured.DELETE("/communities/:id", middleware.RequirePerm("community:delete"), middleware.OperLog(db, "community", "delete"), communityCtl.Delete)
		secured.GET("/buildings", middleware.RequirePerm("community:building:list", "community:list"), communityCtl.ListBuildings)
		secured.POST("/buildings", middleware.RequirePerm("community:building:create", "community:create"), middleware.OperLog(db, "community", "create"), communityCtl.CreateBuilding)
		secured.GET("/buildings/:id", middleware.RequirePerm("community:building:list", "community:list"), communityCtl.BuildingDetail)
		secured.PUT("/buildings/:id", middleware.RequirePerm("community:building:update", "community:update"), middleware.OperLog(db, "community", "update"), communityCtl.UpdateBuilding)
		secured.DELETE("/buildings/:id", middleware.RequirePerm("community:building:delete", "community:delete"), middleware.OperLog(db, "community", "delete"), communityCtl.DeleteBuilding)

		// 巡检管理：点位
		points := secured.Group("/inspection/points")
		{
			points.GET("", middleware.RequirePerm("inspection:point:list"), inspectionCtl.ListPoints)
			points.POST("", middleware.RequirePerm("inspection:point:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePoint)
			points.GET("/map", middleware.RequirePerm("inspection:point:list"), inspectionCtl.MapPoints)
			points.POST("/qrcodes", middleware.RequirePerm("inspection:point:qrcode"), middleware.OperLog(db, "inspection", "qrcode"), inspectionCtl.QRCodes)
			points.GET("/:id", middleware.RequirePerm("inspection:point:list"), inspectionCtl.PointDetail)
			points.PUT("/:id", middleware.RequirePerm("inspection:point:update"), middleware.OperLog(db, "inspection", "update"), inspectionCtl.UpdatePoint)
			points.DELETE("/:id", middleware.RequirePerm("inspection:point:delete"), middleware.OperLog(db, "inspection", "delete"), inspectionCtl.DeletePoint)
		}
		// 巡检计划
		plans := secured.Group("/inspection/plans")
		{
			plans.GET("", middleware.RequirePerm("inspection:plan:list"), inspectionCtl.ListPlans)
			plans.POST("", middleware.RequirePerm("inspection:plan:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePlan)
			plans.GET("/:id", middleware.RequirePerm("inspection:plan:list"), inspectionCtl.PlanDetail)
			plans.PUT("/:id", middleware.RequirePerm("inspection:plan:update"), middleware.OperLog(db, "inspection", "update"), inspectionCtl.UpdatePlan)
			plans.DELETE("/:id", middleware.RequirePerm("inspection:plan:delete"), middleware.OperLog(db, "inspection", "delete"), inspectionCtl.DeletePlan)
			plans.PUT("/:id/status", middleware.RequirePerm("inspection:plan:update", "inspection:plan:disable"), middleware.OperLog(db, "inspection", "update_status"), inspectionCtl.SetPlanStatus)
		}
		// 任务监控 + 打卡记录
		secured.GET("/inspection/tasks", middleware.RequirePerm("inspection:task:list", "inspection:task:monitor"), inspectionCtl.ListTasks)
		secured.GET("/inspection/tasks/:id/detail", middleware.RequirePerm("inspection:task:list", "inspection:task:monitor"), inspectionCtl.TaskDetail)
		secured.POST("/inspection/tasks/generate", middleware.RequirePerm("inspection:task:generate", "inspection:plan:create"), middleware.OperLog(db, "inspection", "generate"), inspectionCtl.GenerateTasks)
		secured.GET("/inspection/checkins", middleware.RequirePerm("inspection:checkin:list", "inspection:record:list"), inspectionCtl.ListCheckins)
		secured.GET("/inspection/checkins/:id", middleware.RequirePerm("inspection:checkin:list", "inspection:record:list"), inspectionCtl.CheckinDetail)

		// 检查项模板
		templates := secured.Group("/inspection/templates")
		{
			templates.GET("", middleware.RequirePerm("inspection:template:list"), templateCtl.List)
			templates.POST("", middleware.RequirePerm("inspection:template:create"), middleware.OperLog(db, "inspection", "create"), templateCtl.Create)
			templates.GET("/:id", middleware.RequirePerm("inspection:template:list"), templateCtl.Detail)
			templates.PUT("/:id", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "update"), templateCtl.Update)
			templates.DELETE("/:id", middleware.RequirePerm("inspection:template:delete"), middleware.OperLog(db, "inspection", "delete"), templateCtl.Delete)
			// 项级粒度：检查项按行增删改（模板 PUT 的整表替换保留兼容）
			templates.GET("/:id/items", middleware.RequirePerm("inspection:template:list"), templateCtl.ListItems)
			templates.POST("/:id/items", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "update"), templateCtl.CreateItem)
			templates.PUT("/:id/items/:itemId", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "update"), templateCtl.UpdateItem)
			templates.DELETE("/:id/items/:itemId", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "update"), templateCtl.DeleteItem)
		}
		// 打卡记录审核与抽查
		review := secured.Group("/inspection/review")
		{
			review.GET("/records", middleware.RequirePerm("inspection:checkin:review"), reviewCtl.Records)
			review.POST("/spotcheck", middleware.RequirePerm("inspection:checkin:spotcheck"), middleware.OperLog(db, "inspection", "spotcheck"), reviewCtl.Spotcheck)
			review.POST("/:id/pass", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_pass"), reviewCtl.Pass)
			review.POST("/:id/reject", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_reject"), reviewCtl.Reject)
		}

		// 异常工单
		orders := secured.Group("/workorders")
		{
			orders.GET("", middleware.RequirePerm("workorder:list"), orderCtl.List)
			orders.POST("", middleware.RequirePerm("workorder:create"), middleware.OperLog(db, "workorder", "create"), orderCtl.Create)
			orders.GET("/:id", middleware.RequirePerm("workorder:list"), orderCtl.Detail)
			orders.PUT("/:id", middleware.RequirePerm("workorder:update"), middleware.OperLog(db, "workorder", "update"), orderCtl.Update)
			orders.DELETE("/:id", middleware.RequirePerm("workorder:delete"), middleware.OperLog(db, "workorder", "delete"), orderCtl.Delete)
			orders.POST("/:id/assign", middleware.RequirePerm("workorder:assign"), middleware.OperLog(db, "workorder", "assign"), orderCtl.Assign)
			orders.POST("/:id/finish", middleware.RequirePerm("workorder:finish"), middleware.OperLog(db, "workorder", "finish"), orderCtl.Finish)
			orders.POST("/:id/review", middleware.RequirePerm("workorder:review"), middleware.OperLog(db, "workorder", "review"), orderCtl.Review)
		}

		// 统计报表
		stats := secured.Group("/stats")
		{
			stats.GET("/coverage", middleware.RequirePerm("stats:report", "stats:inspection"), statsCtl.Coverage)
			stats.GET("/timeliness", middleware.RequirePerm("stats:report", "stats:inspection"), statsCtl.Timeliness)
			stats.GET("/performance", middleware.RequirePerm("stats:report", "stats:performance"), statsCtl.Performance)
			stats.POST("/export", middleware.RequirePerm("stats:export"), middleware.OperLog(db, "stats", "export"), statsCtl.Export)
		}

		// 月度报告（三级电子确认签字 + PDF 归档）
		reports := secured.Group("/reports")
		{
			reports.GET("", middleware.RequirePerm("report:list"), reportCtl.List)
			reports.POST("/generate", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "generate"), reportCtl.Generate)
			reports.GET("/:id", middleware.RequirePerm("report:list"), reportCtl.Detail)
			reports.POST("/:id/sign-inspector", middleware.RequirePerm("report:sign:inspector"), middleware.OperLog(db, "report", "sign_inspector"), reportCtl.SignInspector)
			reports.POST("/:id/sign-supervisor", middleware.RequirePerm("report:sign:supervisor"), middleware.OperLog(db, "report", "sign_supervisor"), reportCtl.SignSupervisor)
			reports.POST("/:id/sign-manager", middleware.RequirePerm("report:sign:manager"), middleware.OperLog(db, "report", "sign_manager"), reportCtl.SignManager)
			reports.GET("/:id/pdf", middleware.RequirePerm("report:download"), reportCtl.PDF)
		}
	}

	// 小程序端
	mp := r.Group("/api/mp")
	{
		mp.POST("/login", mpCtl.Login)
		mp.POST("/refresh", mpCtl.Refresh)
		mp.POST("/upload/callback", mpCtl.Callback) // OSS 服务端间回调（验签，无 JWT）

		mpAuth := mp.Group("", middleware.Auth(db, sess, jwtm, mpsvc.ChannelMP))
		{
			mpAuth.GET("/tasks/today", mpCtl.TodayTasks)
			mpAuth.GET("/tasks/:id", mpCtl.TaskDetail)
			mpAuth.POST("/checkin", mpCtl.Checkin)
			mpAuth.POST("/checkin/offline-sync", mpCtl.OfflineSync)
			mpAuth.POST("/upload/sts", mpCtl.STS)
			mpAuth.POST("/upload/local", mpCtl.Local) // dev 模式本地上传
			mpAuth.GET("/workorders/mine", mpCtl.MyOrders)
			mpAuth.GET("/workorders/:id", mpCtl.OrderDetail)
			mpAuth.POST("/workorders/:id/accept", mpCtl.OrderAccept)
			mpAuth.POST("/workorders/:id/finish", mpCtl.OrderFinish)
			mpAuth.GET("/messages", mpCtl.Messages)
			mpAuth.PUT("/messages/:id/read", mpCtl.MarkRead)
			mpAuth.GET("/announcements", mpCtl.Announcements)
		}
	}

	// 生产单端口 SPA：托管前端构建产物，非 /api、/uploads 路径 fallback 到 index.html
	registerSPA(r, cfg.SPA.DistPath)

	return r, scheduler
}

// registerSPA 注册 SPA 静态托管与 history 路由回退（SPA_DIST_PATH 指向有效目录时生效）。
func registerSPA(r *gin.Engine, distPath string) {
	if distPath == "" {
		return
	}
	st, err := os.Stat(distPath)
	if err != nil || !st.IsDir() {
		return
	}
	root := filepath.Clean(distPath)
	index := filepath.Join(root, "index.html")
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") {
			c.JSON(404, gin.H{"code": 40400, "message": "接口不存在", "data": nil})
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			c.JSON(404, gin.H{"code": 40400, "message": "接口不存在", "data": nil})
			return
		}
		// 命中静态文件则直接返回，否则回退 index.html（前端 history 路由）
		clean := filepath.Clean(strings.TrimPrefix(p, "/"))
		fp := filepath.Join(root, clean)
		if strings.HasPrefix(fp, root+string(os.PathSeparator)) {
			if st, err := os.Stat(fp); err == nil && !st.IsDir() {
				c.File(fp)
				return
			}
		}
		c.File(index)
	})
}

// registerSystemRoutes 系统管理路由（与第一阶段一致 + 通知公告）。
func registerSystemRoutes(sys *gin.RouterGroup, db *gorm.DB,
	userCtl *systemctl.UserController, roleCtl *systemctl.RoleController, menuCtl *systemctl.MenuController,
	dictCtl *systemctl.DictController, configCtl *systemctl.ConfigController, logCtl *systemctl.LogController,
	noticeCtl *systemctl.NoticeController, uploadCtl *systemctl.UploadController, signAssetCtl *systemctl.SignAssetController) {

	// 管理端图片上传（登录即可：签名/公章/头像）
	sys.POST("/upload", middleware.OperLog(db, "system", "upload"), uploadCtl.Local)

	// 签章资产（手写签名/公章版本链）
	signAssets := sys.Group("/sign-assets")
	{
		signAssets.GET("", middleware.RequirePerm("system:signasset:list"), signAssetCtl.List)
		signAssets.POST("", middleware.RequirePerm("system:signasset:create"), middleware.OperLog(db, "system", "sign_asset_create"), signAssetCtl.Create)
		signAssets.POST("/:id/revoke", middleware.RequirePerm("system:signasset:revoke"), middleware.OperLog(db, "system", "sign_asset_revoke"), signAssetCtl.Revoke)
	}

	users := sys.Group("/users")
	{
		// 个人中心（登录即可，无需权限点；静态段优先于 :id 注册）
		users.PUT("/profile", middleware.OperLog(db, "system", "update_profile"), userCtl.UpdateProfile)
		users.PUT("/password", middleware.OperLog(db, "system", "change_password"), userCtl.ChangePassword)
		users.GET("/my-login-logs", logCtl.MyLoginLogs)

		users.GET("", middleware.RequirePerm("system:user:list"), userCtl.List)
		users.POST("", middleware.RequirePerm("system:user:create"), middleware.OperLog(db, "system", "create"), userCtl.Create)
		users.GET("/import-template", middleware.RequirePerm("system:user:import"), userCtl.ImportTemplate)
		users.POST("/import", middleware.RequirePerm("system:user:import"), middleware.OperLog(db, "system", "import"), userCtl.Import)
		users.GET("/export", middleware.RequirePerm("system:user:export"), middleware.OperLog(db, "system", "export"), userCtl.Export)
		users.GET("/:id", middleware.RequirePerm("system:user:list"), userCtl.Detail)
		users.PUT("/:id", middleware.RequirePerm("system:user:update"), middleware.OperLog(db, "system", "update"), userCtl.Update)
		users.PUT("/:id/password/reset", middleware.RequirePerm("system:user:reset-password"), middleware.OperLog(db, "system", "reset_password"), userCtl.ResetPassword)
		users.PUT("/:id/status", middleware.RequirePerm("system:user:update"), middleware.OperLog(db, "system", "update_status"), userCtl.SetStatus)
		users.DELETE("/:id", middleware.RequirePerm("system:user:delete"), middleware.OperLog(db, "system", "delete"), userCtl.Delete)
	}
	roles := sys.Group("/roles")
	{
		roles.GET("", middleware.RequirePerm("system:role:list"), roleCtl.List)
		roles.POST("", middleware.RequirePerm("system:role:create"), middleware.OperLog(db, "system", "create"), roleCtl.Create)
		roles.GET("/:id", middleware.RequirePerm("system:role:list"), roleCtl.Detail)
		roles.PUT("/:id", middleware.RequirePerm("system:role:update"), middleware.OperLog(db, "system", "update"), roleCtl.Update)
		roles.DELETE("/:id", middleware.RequirePerm("system:role:delete"), middleware.OperLog(db, "system", "delete"), roleCtl.Delete)
		roles.PUT("/:id/menus", middleware.RequirePerm("system:role:update"), middleware.OperLog(db, "system", "assign_menus"), roleCtl.AssignMenus)
	}
	menus := sys.Group("/menus")
	{
		menus.GET("", middleware.RequirePerm("system:menu:list"), menuCtl.Tree)
		menus.POST("", middleware.RequirePerm("system:menu:create"), middleware.OperLog(db, "system", "create"), menuCtl.Create)
		menus.GET("/:id", middleware.RequirePerm("system:menu:list"), menuCtl.Detail)
		menus.PUT("/:id", middleware.RequirePerm("system:menu:update"), middleware.OperLog(db, "system", "update"), menuCtl.Update)
		menus.DELETE("/:id", middleware.RequirePerm("system:menu:delete"), middleware.OperLog(db, "system", "delete"), menuCtl.Delete)
	}
	dictTypes := sys.Group("/dict-types")
	{
		dictTypes.GET("", middleware.RequirePerm("system:dict:list"), dictCtl.ListTypes)
		dictTypes.POST("", middleware.RequirePerm("system:dict:create"), middleware.OperLog(db, "system", "create"), dictCtl.CreateType)
		dictTypes.PUT("/:id", middleware.RequirePerm("system:dict:update"), middleware.OperLog(db, "system", "update"), dictCtl.UpdateType)
		dictTypes.DELETE("/:id", middleware.RequirePerm("system:dict:delete"), middleware.OperLog(db, "system", "delete"), dictCtl.DeleteType)
	}
	dictData := sys.Group("/dict-data")
	{
		dictData.GET("", middleware.RequirePerm("system:dict:list"), dictCtl.ListData)
		dictData.POST("", middleware.RequirePerm("system:dict:create"), middleware.OperLog(db, "system", "create"), dictCtl.CreateData)
		dictData.PUT("/:id", middleware.RequirePerm("system:dict:update"), middleware.OperLog(db, "system", "update"), dictCtl.UpdateData)
		dictData.DELETE("/:id", middleware.RequirePerm("system:dict:delete"), middleware.OperLog(db, "system", "delete"), dictCtl.DeleteData)
	}
	configs := sys.Group("/configs")
	{
		configs.GET("", middleware.RequirePerm("system:config:list"), configCtl.List)
		configs.GET("/groups", middleware.RequirePerm("system:config:list"), configCtl.Groups)
		configs.POST("", middleware.RequirePerm("system:config:create"), middleware.OperLog(db, "system", "create"), configCtl.Create)
		configs.PUT("/:id", middleware.RequirePerm("system:config:update"), middleware.OperLog(db, "system", "update"), configCtl.Update)
		configs.DELETE("/:id", middleware.RequirePerm("system:config:delete"), middleware.OperLog(db, "system", "delete"), configCtl.Delete)
	}
	notices := sys.Group("/notices")
	{
		notices.GET("", middleware.RequirePerm("system:notice:list"), noticeCtl.List)
		notices.POST("", middleware.RequirePerm("system:notice:create"), middleware.OperLog(db, "system", "create"), noticeCtl.Create)
		notices.PUT("/:id", middleware.RequirePerm("system:notice:update"), middleware.OperLog(db, "system", "update"), noticeCtl.Update)
		notices.DELETE("/:id", middleware.RequirePerm("system:notice:delete"), middleware.OperLog(db, "system", "delete"), noticeCtl.Delete)
	}
	logs := sys.Group("/logs")
	{
		logs.GET("/operations", middleware.RequirePerm("system:log:operation"), logCtl.Operations)
		logs.GET("/logins", middleware.RequirePerm("system:log:login"), logCtl.Logins)
	}
}
