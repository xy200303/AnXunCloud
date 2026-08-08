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
	authSvc := authsvc.NewAuthService(db, rdb, sess, jwtm, configSvc.Get)
	userSvc := systemsvc.NewUserService(db, authSvc.KillUserSessions)
	roleSvc := systemsvc.NewRoleService(db)
	menuSvc := systemsvc.NewMenuService(db)
	dictSvc := systemsvc.NewDictService(db)
	logSvc := systemsvc.NewLogService(db)
	noticeSvc := systemsvc.NewNoticeService(db)
	communitySvc := communitysvc.NewCommunityService(db)
	pointSvc := inspectionsvc.NewPointService(db, store, configSvc.Get)
	planSvc := inspectionsvc.NewPlanService(db, rdb)
	taskSvc := inspectionsvc.NewTaskService(db)
	orderSvc := workordersvc.NewOrderService(db, rdb)
	statsSvc := statssvc.NewStatsService(db, store)
	mpSvc := mpsvc.NewMPService(db, rdb, sess, jwtm, cfg.Wechat, orderSvc)
	checkinSvc := mpsvc.NewCheckinService(db, rdb, store, orderSvc, configSvc.Get)
	uploadSvc := mpsvc.NewUploadService(db, store, cfg.Upload, cfg.OSS)
	scheduler := inspectionsvc.NewScheduler(db, planSvc, configSvc.Get)

	authCtl := authctl.NewAuthController(authSvc)
	userCtl := systemctl.NewUserController(userSvc)
	roleCtl := systemctl.NewRoleController(roleSvc)
	menuCtl := systemctl.NewMenuController(menuSvc)
	dictCtl := systemctl.NewDictController(dictSvc)
	configCtl := systemctl.NewConfigController(configSvc)
	logCtl := systemctl.NewLogController(logSvc)
	noticeCtl := systemctl.NewNoticeController(noticeSvc)
	communityCtl := communityctl.NewCommunityController(communitySvc)
	inspectionCtl := inspectionctl.NewInspectionController(pointSvc, planSvc, taskSvc)
	orderCtl := workorderctl.NewOrderController(orderSvc)
	statsCtl := statsctl.NewStatsController(statsSvc)
	mpCtl := mpctl.NewMPController(mpSvc, checkinSvc, uploadSvc, orderSvc, noticeSvc)

	// 健康检查 + dev 模式本地文件静态路由
	r.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "up"}) })
	r.Static("/uploads", cfg.Upload.LocalDir)

	admin := r.Group("/api/admin")
	{
		admin.POST("/auth/login", middleware.OperLog(db, "system", "login"), authCtl.Login)
		admin.POST("/auth/refresh", authCtl.Refresh)
		admin.GET("/auth/register-config", authCtl.RegisterConfig)
		admin.POST("/auth/register", middleware.OperLog(db, "system", "register"), authCtl.Register)
	}

	// 管理后台鉴权分组
	secured := admin.Group("", middleware.Auth(db, sess, jwtm, authsvc.ChannelAdmin))
	{
		secured.POST("/auth/logout", authCtl.Logout)
		secured.GET("/auth/info", authCtl.Info)
		secured.GET("/auth/routes", authCtl.Routes)
		secured.GET("/dashboard", statsCtl.Dashboard)

		sys := secured.Group("/system")
		registerSystemRoutes(sys, db, userCtl, roleCtl, menuCtl, dictCtl, configCtl, logCtl, noticeCtl)

		// 小区与楼栋
		secured.GET("/communities", middleware.RequirePerm("community:list"), communityCtl.List)
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
	noticeCtl *systemctl.NoticeController) {

	users := sys.Group("/users")
	{
		// 个人中心（登录即可，无需权限点；静态段优先于 :id 注册）
		users.PUT("/profile", middleware.OperLog(db, "system", "update_profile"), userCtl.UpdateProfile)
		users.PUT("/password", middleware.OperLog(db, "system", "change_password"), userCtl.ChangePassword)

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
