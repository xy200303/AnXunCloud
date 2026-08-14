// Package router 路由注册（/api/admin、/api/mp 分组）。
package router

import (
	"embed"
	"io/fs"
	"net/http"
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
	filectl "anxuncloud/internal/module/file/controller"
	filesvc "anxuncloud/internal/module/file/service"
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

// pointPageHTML NFC/二维码短链接 H5 点位信息页（免登录，内嵌资源随二进制分发）。
//
//go:embed point_page.html
var pointPageHTML string

// pdfjsAssets 内嵌 pdf.js 精简查看器（App 端 web-view 内渲染报告 PDF，随二进制分发）。
//
//go:embed pdfjs
var pdfjsAssets embed.FS

// New 构建 HTTP 引擎并注册全部路由；返回引擎与巡检调度器（由 main 启停）。
func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client) (*gin.Engine, *inspectionsvc.Scheduler) {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.CORS(cfg.CORS.AllowOrigins))

	// 依赖装配
	jwtm := jwtutil.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	sess := session.NewStore(rdb)
	store := storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL)
	watermark.Init(cfg.Watermark.FontPath, cfg.Watermark.LogoPath)

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
	reportSvc := reportsvc.NewReportService(db, rdb, store, configSvc.Get)
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
	fileCtl := filectl.NewFileController(filesvc.NewFileService(db, store, uploadSvc))
	mpCtl := mpctl.NewMPController(mpSvc, checkinSvc, uploadSvc, orderSvc, noticeSvc)

	// 健康检查 + 本地文件静态路由（仅非敏感场景：checkin/workorder/avatar/notice 等内容图；
	// signature/seal/export 由 /api/files 鉴权提供，store.URL 已按前缀分流）
	r.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "up"}) })
	r.GET("/uploads/*key", func(c *gin.Context) {
		key := strings.TrimPrefix(c.Param("key"), "/")
		if key == "" || strings.Contains(key, "..") || storage.IsProtectedKey(key) {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在或已删除", "data": nil})
			return
		}
		c.File(filepath.Join(cfg.Upload.LocalDir, filepath.FromSlash(key)))
	})

	// 短链接公开入口（免登录）：H5 点位信息页 + 脱敏摘要 API
	r.GET("/p/:code", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(200, "text/html; charset=utf-8", []byte(pointPageHTML))
	})
	r.GET("/api/public/point/:code", mpCtl.PublicPoint)

	// pdf.js 内嵌查看器静态资源（App web-view 打开 /pdfjs/viewer.html?file=... 渲染报告）
	pdfjsSub, err := fs.Sub(pdfjsAssets, "pdfjs")
	if err != nil {
		panic(err)
	}
	r.StaticFS("/pdfjs", http.FS(pdfjsSub))
	// 报告 PDF 公开下载（仅凭一次性 ticket；ticket 由登录接口签发，见 /api/app/reports/:id/pdf-ticket）
	r.GET("/api/public/report-pdf/:id", reportCtl.PDFByTicket)

	// 统一文件层：上传/直传凭证/下载（AuthAny 跨通道认证，scene 级读权限在 service 内判定）
	files := r.Group("/api/files", middleware.AuthAny(db, sess, jwtm, authsvc.ChannelAdmin, authsvc.ChannelApp, mpsvc.ChannelMP))
	{
		files.POST("", fileCtl.Upload)
		files.POST("/sts", fileCtl.STS)
		files.GET("/*key", fileCtl.Download)
	}

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

		// 地图服务配置与地点搜索（登录即可：物业主管也要能地图选点，不绑系统配置权限）
		secured.GET("/map/config", configCtl.MapConfig)
		secured.GET("/map/search", configCtl.MapSearch)

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
			points.GET("/import-template", middleware.RequirePerm("inspection:point:import"), inspectionCtl.PointImportTemplate)
			points.POST("/import", middleware.RequirePerm("inspection:point:import"), middleware.OperLog(db, "inspection", "import"), inspectionCtl.ImportPoints)
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
		secured.GET("/inspection/checkins/audit-counts", middleware.RequirePerm("inspection:checkin:list", "inspection:record:list"), inspectionCtl.CheckinAuditCounts)
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
			review.POST("/batch-pass", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_batch_pass"), reviewCtl.BatchPass)
			review.POST("/:id/pass", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_pass"), reviewCtl.Pass)
			review.POST("/:id/reject", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_reject"), reviewCtl.Reject)
			review.POST("/:id/reopen", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_reopen"), reviewCtl.Reopen)
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
			reports.GET("/sign-candidates", middleware.RequirePerm("report:generate"), reportCtl.SignCandidates)
			reports.GET("/:id", middleware.RequirePerm("report:list"), reportCtl.Detail)
			reports.POST("/:id/sign-inspector", middleware.RequirePerm("report:sign:inspector", "report:sign:proxy"), middleware.OperLog(db, "report", "sign_inspector"), reportCtl.SignInspector)
			reports.POST("/:id/sign-supervisor", middleware.RequirePerm("report:sign:supervisor"), middleware.OperLog(db, "report", "sign_supervisor"), reportCtl.SignSupervisor)
			reports.POST("/:id/sign-manager", middleware.RequirePerm("report:sign:manager"), middleware.OperLog(db, "report", "sign_manager"), reportCtl.SignManager)
			reports.GET("/:id/pdf", reportCtl.PDF) // 权限在 service 内判定：report:download 或报告相关人
		}
	}

	// 小程序端
	mp := r.Group("/api/mp")
	{
		mp.POST("/login", mpCtl.Login)
		mp.POST("/refresh", mpCtl.Refresh)
		mp.GET("/auth/register-config", authCtl.RegisterConfig)
		mp.POST("/auth/register", authCtl.Register)
		mp.POST("/upload/callback", mpCtl.Callback) // OSS 服务端间回调（验签，无 JWT）

		mpAuth := mp.Group("", middleware.Auth(db, sess, jwtm, authsvc.ChannelApp)) // v21 起与 App 共用 app 通道会话
		{
			mpAuth.GET("/tasks/today", mpCtl.TodayTasks)
			mpAuth.GET("/tasks/:id", mpCtl.TaskDetail)
			mpAuth.GET("/points/by-code/:code", mpCtl.PointByCode)
			mpAuth.POST("/checkin", mpCtl.Checkin)
			mpAuth.POST("/checkin/offline-sync", mpCtl.OfflineSync)
			mpAuth.POST("/upload/sts", mpCtl.STS)
			mpAuth.POST("/upload/local", mpCtl.Local) // dev 模式本地上传
			mpAuth.GET("/workorders/mine", mpCtl.MyOrders)
			mpAuth.GET("/workorders/mine/counts", mpCtl.OrderCounts)
			mpAuth.GET("/workorders/:id", mpCtl.OrderDetail)
			mpAuth.POST("/workorders/:id/accept", mpCtl.OrderAccept)
			mpAuth.POST("/workorders/:id/finish", mpCtl.OrderFinish)
			mpAuth.GET("/messages", mpCtl.Messages)
			mpAuth.PUT("/messages/:id/read", mpCtl.MarkRead)
			mpAuth.GET("/announcements", mpCtl.Announcements)
			mpAuth.GET("/announcements/:id", mpCtl.AnnouncementDetail)
		}
	}

	// APP 端（Android/iOS/鸿蒙）：业务接口与 mp 组同形同逻辑（同一批 Service），
	// 会话渠道 app；登录为账号密码，另挂月报签字与个人中心（小程序端不开放 PC 侧签字/配置入口，需要时再加）。
	app := r.Group("/api/app")
	{
		app.POST("/login", authCtl.LoginApp)
		app.POST("/wx-login", mpCtl.Login) // 微信 code2session 登录（与 /api/mp/login 同一入口，移动端认证已合并）
		app.POST("/refresh", authCtl.RefreshApp)
		app.GET("/auth/register-config", authCtl.RegisterConfig)
		app.POST("/auth/register", authCtl.Register)

		appAuth := app.Group("", middleware.Auth(db, sess, jwtm, authsvc.ChannelApp))
		{
			appAuth.POST("/auth/logout", authCtl.LogoutApp)
			// 个人中心（登录即可）
			appAuth.GET("/profile", authCtl.Info)
			appAuth.PUT("/profile", middleware.OperLog(db, "system", "update_profile"), userCtl.UpdateProfile)
			appAuth.PUT("/password", middleware.OperLog(db, "system", "change_password"), userCtl.ChangePassword)
			// 任务 / 打卡 / 上传（复用 mp 控制器）
			appAuth.GET("/tasks/today", mpCtl.TodayTasks)
			appAuth.GET("/tasks/:id", mpCtl.TaskDetail)
			appAuth.GET("/points/by-code/:code", mpCtl.PointByCode)
			appAuth.POST("/checkin", mpCtl.Checkin)
			appAuth.POST("/checkin/offline-sync", mpCtl.OfflineSync)
			appAuth.POST("/upload/sts", mpCtl.STS)
			appAuth.POST("/upload/local", mpCtl.Local)
			// 工单 / 消息 / 公告
			appAuth.GET("/workorders/mine", mpCtl.MyOrders)
			appAuth.GET("/workorders/mine/counts", mpCtl.OrderCounts)
			appAuth.GET("/workorders/:id", mpCtl.OrderDetail)
			appAuth.POST("/workorders/:id/accept", mpCtl.OrderAccept)
			appAuth.POST("/workorders/:id/finish", mpCtl.OrderFinish)
			appAuth.GET("/messages", mpCtl.Messages)
			appAuth.PUT("/messages/:id/read", mpCtl.MarkRead)
			appAuth.GET("/announcements", mpCtl.Announcements)
			appAuth.GET("/announcements/:id", mpCtl.AnnouncementDetail)

			// ===== 管理功能（App 端）：复用 PC 控制器 + 同一套权限点，入口由 App 按 perms 显隐 =====
			appAuth.GET("/dashboard", statsCtl.Dashboard)
			appAuth.GET("/communities/tree", middleware.RequirePerm("community:list", "inspection:point:list"), communityCtl.Tree)
			appAuth.GET("/system/users", middleware.RequirePerm("system:user:list", "workorder:assign"), userCtl.List)
			// 点位管理（现场建点：GPS 录坐标 + NFC 写卡；删除仅 PC 端）
			appPoints := appAuth.Group("/inspection/points")
			{
				appPoints.GET("", middleware.RequirePerm("inspection:point:list"), inspectionCtl.ListPoints)
				appPoints.POST("", middleware.RequirePerm("inspection:point:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePoint)
				appPoints.GET("/:id", middleware.RequirePerm("inspection:point:list"), inspectionCtl.PointDetail)
				appPoints.PUT("/:id", middleware.RequirePerm("inspection:point:update"), middleware.OperLog(db, "inspection", "update"), inspectionCtl.UpdatePoint)
			}
			// 检查项模板（建点位时下拉选项）
			appAuth.GET("/inspection/templates", middleware.RequirePerm("inspection:template:list", "inspection:point:create"), templateCtl.List)
			// 任务监控 + 一键催办
			appAuth.GET("/inspection/tasks", middleware.RequirePerm("inspection:task:list", "inspection:task:monitor"), inspectionCtl.ListTasks)
			appAuth.GET("/inspection/tasks/:id/detail", middleware.RequirePerm("inspection:task:list", "inspection:task:monitor"), inspectionCtl.TaskDetail)
			appAuth.POST("/inspection/tasks/:id/remind", middleware.RequirePerm("inspection:task:list", "inspection:task:monitor"), middleware.OperLog(db, "inspection", "remind"), inspectionCtl.RemindTask)
			// 打卡审核
			appReview := appAuth.Group("/inspection/review")
			{
				appReview.GET("/records", middleware.RequirePerm("inspection:checkin:review"), reviewCtl.Records)
				appReview.POST("/:id/pass", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_pass"), reviewCtl.Pass)
				appReview.POST("/:id/reject", middleware.RequirePerm("inspection:checkin:review"), middleware.OperLog(db, "inspection", "review_reject"), reviewCtl.Reject)
			}
			// 工单管理（派单/验收）：与巡检员「我的工单」路径隔离
			appOrders := appAuth.Group("/manage/workorders")
			{
				appOrders.GET("", middleware.RequirePerm("workorder:list"), orderCtl.List)
				appOrders.GET("/:id", middleware.RequirePerm("workorder:list"), orderCtl.Detail)
				appOrders.POST("/:id/assign", middleware.RequirePerm("workorder:assign"), middleware.OperLog(db, "workorder", "assign"), orderCtl.Assign)
				appOrders.POST("/:id/review", middleware.RequirePerm("workorder:review"), middleware.OperLog(db, "workorder", "review"), orderCtl.Review)
			}
			// 月报签字（权限点与 PC 完全一致；待我签用 ?pending_mine=1）
			appReports := appAuth.Group("/reports")
			{
				appReports.GET("", middleware.RequirePerm("report:list"), reportCtl.List)
				appReports.GET("/:id", middleware.RequirePerm("report:list"), reportCtl.Detail)
				appReports.POST("/:id/sign-inspector", middleware.RequirePerm("report:sign:inspector", "report:sign:proxy"), middleware.OperLog(db, "report", "sign_inspector"), reportCtl.SignInspector)
				appReports.POST("/:id/sign-supervisor", middleware.RequirePerm("report:sign:supervisor"), middleware.OperLog(db, "report", "sign_supervisor"), reportCtl.SignSupervisor)
				appReports.POST("/:id/sign-manager", middleware.RequirePerm("report:sign:manager"), middleware.OperLog(db, "report", "sign_manager"), reportCtl.SignManager)
				appReports.GET("/:id/pdf", reportCtl.PDF) // 同 PC：service 内判定（report:download 或报告相关人）
				appReports.POST("/:id/pdf-ticket", reportCtl.PDFTicket) // 签发 web-view 预览用一次性 ticket
			}
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
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") || strings.HasPrefix(p, "/pdfjs/") {
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
				// 带内容哈希的构建产物（/assets/*.js|css）可长缓存
				if strings.HasPrefix(clean, "assets"+string(os.PathSeparator)) || strings.HasPrefix(clean, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				c.File(fp)
				return
			}
		}
		// index.html 不长缓存：重新部署后浏览器总能拿到最新的 chunk 引用（防白屏）
		c.Header("Cache-Control", "no-cache")
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
		roles.PUT("/:id/menus", middleware.RequirePerm("system:role:update", "system:role:assign"), middleware.OperLog(db, "system", "assign_menus"), roleCtl.AssignMenus)
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
