// Package router 路由注册（/api/admin、/api/mp 分组）。
package router

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	acceptancectl "anxuncloud/internal/module/acceptance/controller"
	acceptancesvc "anxuncloud/internal/module/acceptance/service"
	aictl "anxuncloud/internal/module/ai/controller"
	aisvc "anxuncloud/internal/module/ai/service"
	authctl "anxuncloud/internal/module/auth/controller"
	authsvc "anxuncloud/internal/module/auth/service"
	communityctl "anxuncloud/internal/module/community/controller"
	communitysvc "anxuncloud/internal/module/community/service"
	equipmentctl "anxuncloud/internal/module/equipment/controller"
	equipmentsvc "anxuncloud/internal/module/equipment/service"
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
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/libreoffice"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/push"
	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/watermark"
	sitetpl "anxuncloud/internal/template"
)

// New 构建 HTTP 引擎并注册全部路由；返回引擎与巡检调度器（由 main 启停）。
func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client) (*gin.Engine, *inspectionsvc.Scheduler) {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.CORS(cfg.CORS.AllowOrigins), middleware.AccessLog())

	// 依赖装配
	jwtm := jwtutil.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	sess := session.NewStore(rdb)
	// 租户级挂点预留（P3 设计方案 §9.2）：文件存储后端按租户路由（tenant_config 预留 COS 覆盖 key）
	// 后续在此经 ConfigService.Resolve(tenantID, ...) 取值，本期统一用平台默认配置，行为不变。
	store := storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL)
	watermark.Init(cfg.Watermark.FontPath, cfg.Watermark.LogoPath)

	// 统一通知出口：站内消息 + App 推送（uniPush 2.0 / 个推 V2；三要素未配置则推送关闭，仅站内消息）
	pushCli := push.NewClient(cfg.UniPush.AppID, cfg.UniPush.AppKey, cfg.UniPush.MasterSecret)
	if pushCli.Enabled() {
		logger.L.Info("App 推送已启用（uniPush 2.0）", zap.String("appid", cfg.UniPush.AppID))
	} else {
		logger.L.Info("App 推送未启用：UNIPUSH_APPID/APPKEY/MASTERSECRET 未配置齐全，仅站内消息")
	}
	notifier := notify.New(db, pushCli)

	configSvc := systemsvc.NewConfigService(db, rdb)
	authSvc := authsvc.NewAuthService(db, rdb, sess, jwtm, configSvc.Get, store)
	signAssetSvc := systemsvc.NewSignAssetService(db, store)
	userSvc := systemsvc.NewUserService(db, authSvc.KillUserSessions, store, signAssetSvc)
	roleSvc := systemsvc.NewRoleService(db)
	tenantSvc := systemsvc.NewTenantService(db, authSvc.KillUserSessions, configSvc)
	menuSvc := systemsvc.NewMenuService(db)
	dictSvc := systemsvc.NewDictService(db)
	logSvc := systemsvc.NewLogService(db)
	noticeSvc := systemsvc.NewNoticeService(db, notifier)
	messageSvc := systemsvc.NewMessageService(db)
	communitySvc := communitysvc.NewCommunityService(db)
	staffSvc := communitysvc.NewStaffService(db)
	pointSvc := inspectionsvc.NewPointService(db, store, configSvc.Get)
	planSvc := inspectionsvc.NewPlanService(db, rdb, notifier, configSvc.Get)
	taskSvc := inspectionsvc.NewTaskService(db, store, notifier)
	templateSvc := inspectionsvc.NewTemplateService(db)
	reviewSvc := inspectionsvc.NewReviewService(db, notifier)
	workorderSvc := workordersvc.NewWorkOrderService(db)
	reviewSvc.BindWorkOrderSink(workorderSvc)
	equipmentSvc := equipmentsvc.NewEquipmentService(db)
	maintSvc := equipmentsvc.NewMaintenanceService(db, notifier, ai.NewClient(configSvc.Get, ai.WithStorage(store)))
	expireSvc := equipmentsvc.NewExpireService(db, notifier)
	projection := statssvc.NewProjectionService(db, inspectionsvc.LoadTaskProjection, inspectionsvc.LoadConfigAlertProjection, inspectionsvc.ProjectionTaskIDs, inspectionsvc.ProjectionAlertIDs)
	inspectionsvc.BindProjectionSink(projection)
	statsSvc := statssvc.NewStatsService(db, store, projection)
	go func() {
		if err := projection.Rebuild(); err != nil {
			logger.L.Error("stats 全量投影重建失败", zap.Error(err))
		}
	}()
	reportSvc := reportsvc.NewReportService(db, rdb, store, configSvc.Get, notifier)
	var pdfConverter libreoffice.Converter = libreoffice.UnavailableConverter(nil)
	switch cfg.LibreOffice.Mode {
	case "exec":
		converter, err := libreoffice.NewExecConverter(cfg.LibreOffice.Binary, time.Duration(cfg.LibreOffice.TimeoutSeconds)*time.Second, cfg.LibreOffice.MaxConcurrent, cfg.LibreOffice.MaxInputBytes, cfg.LibreOffice.MaxOutputBytes)
		if err != nil {
			logger.L.Error("LibreOffice 初始化失败", zap.Error(err))
			pdfConverter = libreoffice.UnavailableConverter(err)
		} else {
			pdfConverter = converter
		}
	case "worker":
		converter, err := libreoffice.NewWorkerClient(cfg.LibreOffice.WorkerDir, time.Duration(cfg.LibreOffice.TimeoutSeconds)*time.Second, cfg.LibreOffice.MaxInputBytes, cfg.LibreOffice.MaxOutputBytes)
		if err != nil {
			logger.L.Error("LibreOffice worker 初始化失败", zap.Error(err))
			pdfConverter = libreoffice.UnavailableConverter(err)
		} else {
			pdfConverter = converter
		}
	}
	reportSvc.BindPDFConverter(pdfConverter)
	docTemplateSvc := reportsvc.NewDocTemplateService(db)
	reportSvc.BindDocTemplateService(docTemplateSvc)
	mpSvc := mpsvc.NewMPService(db, rdb, sess, jwtm, cfg.Wechat, messageSvc)
	taskQueue, err := queuepkg.New(cfg.Postgres)
	if err != nil {
		panic(err)
	}
	notifier.BindQueue(taskQueue)
	queuepkg.RegisterWorker(taskQueue, notify.NewNotificationWorker(notifier))
	reportSvc.BindQueue(taskQueue)
	queuepkg.RegisterWorker(taskQueue, reportsvc.NewReportRenderWorker(reportSvc))
	checkinSvc := mpsvc.NewCheckinService(db, rdb, store, configSvc.Get, notifier)
	reviewSvc.BindCheckinAIGate(checkinSvc.RunAIGate)
	checkinSvc.BindQueue(taskQueue)
	queuepkg.RegisterWorker(taskQueue, mpsvc.NewAIItemWorker(checkinSvc))
	uploadSvc := filesvc.NewUploadService(db, store, cfg.Upload, cfg.OSS)
	scheduler := inspectionsvc.NewScheduler(db, planSvc, reportSvc, configSvc.Get)
	scheduler.RegisterQueue(taskQueue)
	// 设备台账：每日到期扫描提醒（临期/逾期/升级经理，时间取 equipment.expire_check_time）
	scheduler.Register(inspectionsvc.PlanJob{Name: "equipment_expire", Run: expireSvc.Run})
	scheduler.Register(inspectionsvc.PlanJob{Name: "workorder_maintenance_due", Run: func(now time.Time) error {
		_, err := workorderSvc.CreateDueMaintenance(now)
		return err
	}})
	scheduler.Register(inspectionsvc.PlanJob{Name: "report_render_recovery", Run: func(time.Time) error {
		reportSvc.ResumePendingFrozenFiles()
		return nil
	}})

	authCtl := authctl.NewAuthController(authSvc)
	userCtl := systemctl.NewUserController(userSvc, db)
	roleCtl := systemctl.NewRoleController(roleSvc, db)
	tenantCtl := systemctl.NewTenantController(tenantSvc, db)
	menuCtl := systemctl.NewMenuController(menuSvc)
	dictCtl := systemctl.NewDictController(dictSvc)
	configCtl := systemctl.NewConfigController(configSvc)
	logCtl := systemctl.NewLogController(logSvc, db)
	noticeCtl := systemctl.NewNoticeController(noticeSvc, db)
	messageCtl := systemctl.NewMessageController(messageSvc)
	uploadCtl := systemctl.NewUploadController(uploadSvc)
	signAssetCtl := systemctl.NewSignAssetController(signAssetSvc, db)
	postSvc := systemsvc.NewPostService(db)
	postCtl := systemctl.NewPostController(postSvc, db)
	postTmplCtl := systemctl.NewPostTemplateController(postSvc)
	siteSvc := systemsvc.NewSiteService(db, store, configSvc)
	siteCtl := systemctl.NewSiteController(siteSvc, store)
	communityCtl := communityctl.NewCommunityController(communitySvc)
	staffCtl := communityctl.NewStaffController(staffSvc)
	inspectionCtl := inspectionctl.NewInspectionController(pointSvc, planSvc, taskSvc)
	equipmentCtl := equipmentctl.NewEquipmentController(equipmentSvc)
	maintCtl := equipmentctl.NewMaintenanceController(maintSvc)
	templateCtl := inspectionctl.NewTemplateController(templateSvc)
	reviewCtl := inspectionctl.NewReviewController(reviewSvc)
	workorderCtl := workorderctl.NewWorkOrderController(workorderSvc)
	acceptanceSchemaSvc := acceptancesvc.NewSchemaService(db, templateSvc.VerifyTemplates)
	acceptanceBatchSvc := acceptancesvc.NewBatchService(db, store, acceptanceSchemaSvc, func(tenantID, communityID string) ([]inspectionsvc.BuildingRef, error) {
		return inspectionsvc.BuildingRefs(db, tenantID, communityID)
	}, templateSvc.VerifyTemplates)
	acceptanceBatchSvc.BindQueue(taskQueue)
	acceptanceBatchSvc.BindPDFConverter(pdfConverter)
	queuepkg.RegisterWorker(taskQueue, acceptancesvc.NewAcceptanceRenderWorker(acceptanceBatchSvc))
	acceptanceCtl := acceptancectl.NewController(acceptanceBatchSvc, acceptanceSchemaSvc, db)
	registrySvc := aisvc.NewRegistryService(db)
	aiCtl := aictl.NewRegistryController(registrySvc, db)
	assistantCtl := aictl.NewAssistantController(aisvc.NewAssistantService(db, registrySvc, aisvc.NewUsageService(db)), db)
	mcpSvc := aisvc.NewMCPService(db, taskSvc.SummaryForAI)
	mcpCtl := aictl.NewMCPController(mcpSvc, db)
	usageSvc := aisvc.NewUsageService(db)
	agentCtl := aictl.NewAgentController(aisvc.NewAgentService(db, registrySvc, usageSvc, mcpSvc), db)
	usageCtl := aictl.NewUsageController(usageSvc, db)
	statsCtl := statsctl.NewStatsController(statsSvc)
	reportCtl := reportctl.NewReportController(reportSvc)
	docTemplateCtl := reportctl.NewDocTemplateController(docTemplateSvc, db)
	fileCtl := filectl.NewFileController(filesvc.NewFileService(db, store, uploadSvc))
	mpCtl := mpctl.NewMPController(mpSvc, checkinSvc, uploadSvc, noticeSvc)
	pushCtl := mpctl.NewPushController(notifier)

	// 上传对象全部由 /api/files 鉴权提供；/uploads 仅用于拒绝旧公共地址，不再直接读盘。
	r.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "up"}) })
	// 就绪探针：db SELECT 1 + Redis Ping，任一失败 503（K8s readiness 摘流量用；/healthz 仅存活）
	r.GET("/readyz", func(c *gin.Context) {
		if err := db.Exec("SELECT 1").Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		response.OK(c, gin.H{"status": "ready"})
	})
	r.GET("/uploads/*key", func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		key := strings.TrimPrefix(c.Param("key"), "/")
		if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `\:`) || storage.IsProtectedKey(key) {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在或已删除", "data": nil})
			return
		}
		root := filepath.Clean(cfg.Upload.LocalDir)
		path := filepath.Join(root, filepath.FromSlash(key))
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在或已删除", "data": nil})
			return
		}
		c.File(path)
	})

	// 短链接公开入口（免登录）：H5 点位信息页 + 脱敏摘要 API
	r.GET("/p/:code", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(200, "text/html; charset=utf-8", []byte(sitetpl.PointPageHTML))
	})
	r.GET("/api/public/point/:code", mpCtl.PublicPoint)

	// pdf.js 内嵌查看器静态资源（App web-view 打开 /pdfjs/viewer.html?file=... 渲染报告）
	pdfjsSub, err := fs.Sub(sitetpl.PdfjsFS, "pdfjs")
	if err != nil {
		panic(err)
	}
	pdfjsGzip := gzipStaticFS(pdfjsSub)
	r.GET("/pdfjs/*filepath", pdfjsGzip)
	r.HEAD("/pdfjs/*filepath", pdfjsGzip)

	r.POST("/mcp", mcpCtl.RPC)

	// 统一文件层：上传/直传凭证/下载（AuthAny 跨通道认证，scene 级读权限在 service 内判定）
	files := r.Group("/api/files", middleware.AuthAny(db, sess, jwtm, authsvc.ChannelAdmin, authsvc.ChannelApp, mpsvc.ChannelMP))
	{
		files.POST("", fileCtl.Upload)
		files.POST("/preflight", fileCtl.Preflight)
		files.POST("/sts", fileCtl.STS)
		files.GET("/*key", fileCtl.Download)
	}

	admin := r.Group("/api/admin")
	{
		admin.POST("/auth/login", authCtl.Login) // 登录不写操作日志，由 sys_login_log 覆盖
		admin.POST("/auth/refresh", authCtl.Refresh)
		admin.GET("/auth/register-config", authCtl.RegisterConfig)
		admin.GET("/auth/register-tenants", authCtl.RegisterTenants)
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

		// 租户管理（P3 多租户，仅超管；tenant:* 权限点只授 super_admin）
		secured.GET("/tenants", middleware.RequirePerm("tenant:list"), tenantCtl.List)
		secured.POST("/tenants", middleware.RequirePerm("tenant:create"), middleware.OperLog(db, "tenant", "create"), tenantCtl.Create)
		secured.PUT("/tenants/:id", middleware.RequirePerm("tenant:update"), middleware.OperLog(db, "tenant", "update"), tenantCtl.Update)
		secured.PUT("/tenants/:id/status", middleware.RequirePerm("tenant:update"), middleware.OperLog(db, "tenant", "update_status"), tenantCtl.SetStatus)
		// 租户配置覆盖（品牌类白名单 key；租户管理员管自己租户，超管可带 ?tenant_id= 管任意租户）
		secured.GET("/tenant-config", middleware.RequirePerm("tenant:config"), tenantCtl.GetConfig)
		secured.PUT("/tenant-config", middleware.RequirePerm("tenant:config"), middleware.OperLog(db, "tenant", "config_save"), tenantCtl.SaveConfig)

		sys := secured.Group("/system")
		registerSystemRoutes(sys, db, userCtl, roleCtl, menuCtl, dictCtl, configCtl, logCtl, noticeCtl, uploadCtl, signAssetCtl, postCtl, postTmplCtl)

		// 品牌官网管理（页面配置 + 下载渠道发布物）
		site := sys.Group("/site")
		{
			site.GET("/config", middleware.RequirePerm("system:site:list"), siteCtl.Config)
			site.PUT("/config", middleware.RequirePerm("system:site:update"), middleware.OperLog(db, "system", "site_config"), siteCtl.SaveConfig)
			site.GET("/releases", middleware.RequirePerm("system:site:list"), siteCtl.Releases)
			site.POST("/releases", middleware.RequirePerm("system:site:upload"), middleware.OperLog(db, "system", "site_release_upload"), siteCtl.Upload)
			site.DELETE("/releases/:id", middleware.RequirePerm("system:site:delete"), middleware.OperLog(db, "system", "site_release_delete"), siteCtl.Delete)
		}

		// 平台预置包维护与租户项目上线读取
		secured.GET("/platform/preset-packs", middleware.RequirePerm("system:site:list"), communityCtl.PlatformPresetPacks)
		secured.GET("/platform/preset-packs/:code", middleware.RequirePerm("system:site:list"), communityCtl.PlatformPresetPack)
		secured.PUT("/platform/preset-packs/:code", middleware.RequirePerm("system:site:update"), middleware.OperLog(db, "community", "preset_pack_save"), communityCtl.SavePresetPack)

		// 小区与楼栋
		secured.GET("/community/preset-packs", middleware.RequirePerm("community:create"), communityCtl.PresetPacks)
		secured.GET("/community/preset-packs/:code", middleware.RequirePerm("community:create"), communityCtl.PresetPack)
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

		// 任职与审批配置：角色来自用户，审批节点直接绑定角色。
		secured.GET("/dict-options", dictCtl.Options)
		secured.GET("/role-options", middleware.RequirePerm("community:staff:list"), staffCtl.RoleOptions)
		secured.GET("/communities/:id/staff", middleware.RequirePerm("community:staff:list"), staffCtl.List)
		secured.POST("/communities/:id/staff", middleware.RequirePerm("community:staff:edit"), middleware.OperLog(db, "community", "staff_create"), staffCtl.Create)
		secured.PUT("/communities/:id/staff/:staffId", middleware.RequirePerm("community:staff:edit"), middleware.OperLog(db, "community", "staff_update"), staffCtl.Update)
		secured.DELETE("/communities/:id/staff/:staffId", middleware.RequirePerm("community:staff:edit"), middleware.OperLog(db, "community", "staff_delete"), staffCtl.Delete)
		secured.GET("/communities/:id/review-flows/:flowCode", middleware.RequirePerm("community:staff:list"), staffCtl.GetFlow)
		secured.PUT("/communities/:id/review-flows/:flowCode", middleware.RequirePerm("system:reviewflow:update"), middleware.OperLog(db, "community", "review_flow_save"), staffCtl.SaveFlow)
		secured.GET("/communities/:id/flow-preview", middleware.RequirePerm("community:staff:list"), staffCtl.FlowPreview)

		// 巡检管理：点位
		points := secured.Group("/inspection/points")
		{
			points.GET("", middleware.RequirePerm("inspection:point:list"), inspectionCtl.ListPoints)
			points.POST("", middleware.RequirePerm("inspection:point:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePoint)
			points.GET("/map", middleware.RequirePerm("inspection:point:list"), inspectionCtl.MapPoints)
			points.POST("/qrcodes", middleware.RequirePerm("inspection:point:qrcode"), middleware.OperLog(db, "inspection", "qrcode"), inspectionCtl.QRCodes)
			points.GET("/import-template", middleware.RequirePerm("inspection:point:import"), inspectionCtl.PointImportTemplate)
			points.POST("/import", middleware.RequirePerm("inspection:point:import"), middleware.OperLog(db, "inspection", "import"), inspectionCtl.ImportPoints)
			points.POST("/batch", middleware.RequirePerm("inspection:point:create"), middleware.OperLog(db, "inspection", "batch_create"), inspectionCtl.BatchCreatePoints)
			points.GET("/:id", middleware.RequirePerm("inspection:point:list"), inspectionCtl.PointDetail)
			points.PUT("/:id", middleware.RequirePerm("inspection:point:update"), middleware.OperLog(db, "inspection", "update"), inspectionCtl.UpdatePoint)
			points.DELETE("/:id", middleware.RequirePerm("inspection:point:delete"), middleware.OperLog(db, "inspection", "delete"), inspectionCtl.DeletePoint)
		}
		// 设备台账与维保（《设备台账与维保周期管理设计方案》）
		equipment := secured.Group("/equipment")
		{
			equipment.GET("/list", middleware.RequirePerm("equipment:list"), equipmentCtl.List)
			equipment.GET("/import-template", middleware.RequirePerm("equipment:import"), equipmentCtl.ImportTemplate)
			equipment.POST("/import", middleware.RequirePerm("equipment:import"), middleware.OperLog(db, "equipment", "import"), equipmentCtl.Import)
			equipment.GET("/export", middleware.RequirePerm("equipment:export"), equipmentCtl.Export)
			equipment.POST("/export", middleware.RequirePerm("equipment:export"), equipmentCtl.ExportSelected)
			equipment.GET("/maintenance-pending", middleware.RequirePerm("equipment:confirm"), maintCtl.PendingList)
			equipment.GET("/maintenances", middleware.RequirePerm("equipment:list"), maintCtl.RecordList)
			equipment.POST("/maintenance", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance"), maintCtl.Register)
			equipment.POST("/maintenance/confirm", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "confirm"), maintCtl.Confirm)
			equipment.POST("/maintenance/reject", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "reject"), maintCtl.Reject)
			equipment.POST("/maintenance-batches/preview", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance_batch_preview"), maintCtl.PreviewBatch)
			equipment.POST("/maintenance-batches/:id", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance_batch_create"), maintCtl.CreateBatch)
			equipment.POST("/maintenance-batches/:id/confirm", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "maintenance_batch_confirm"), maintCtl.ConfirmBatch)
			equipment.POST("/maintenance-batches/:id/reject", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "maintenance_batch_reject"), maintCtl.RejectBatch)
			// 类型化台账：类型字段方案（列表列/导出列/表单字段/导入列头；租户级覆盖平台默认）
			equipment.GET("/type-schemas", middleware.RequirePerm("equipment:list"), equipmentCtl.TypeSchemas)
			equipment.PUT("/type-schemas/:type", middleware.RequirePerm("equipment:update"), middleware.OperLog(db, "equipment", "type_schema"), equipmentCtl.SaveTypeSchema)
			equipment.GET("/:id", middleware.RequirePerm("equipment:list"), equipmentCtl.Detail)
			equipment.GET("/:id/maintenances", middleware.RequirePerm("equipment:list"), maintCtl.History)
			equipment.POST("", middleware.RequirePerm("equipment:create"), middleware.OperLog(db, "equipment", "create"), equipmentCtl.Create)
			equipment.POST("/batch-delete", middleware.RequirePerm("equipment:delete"), middleware.OperLog(db, "equipment", "batch_delete"), equipmentCtl.BatchDelete)
			equipment.PUT("/:id", middleware.RequirePerm("equipment:update"), middleware.OperLog(db, "equipment", "update"), equipmentCtl.Update)
			equipment.DELETE("/:id", middleware.RequirePerm("equipment:delete"), middleware.OperLog(db, "equipment", "delete"), equipmentCtl.Delete)
		}
		// 巡检计划
		plans := secured.Group("/inspection/plans")
		{
			plans.GET("", middleware.RequirePerm("inspection:plan:list"), inspectionCtl.ListPlans)
			plans.POST("", middleware.RequirePerm("inspection:plan:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePlan)
			plans.GET("/preview-points", middleware.RequirePerm("inspection:plan:list"), inspectionCtl.PreviewPlanPoints)
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
		secured.GET("/inspection/checkins/export", middleware.RequirePerm("inspection:checkin:list", "inspection:record:list"), middleware.OperLog(db, "inspection", "checkin_export"), inspectionCtl.ExportCheckins)
		secured.GET("/inspection/checkins/:id", middleware.RequirePerm("inspection:checkin:list", "inspection:record:list"), inspectionCtl.CheckinDetail)

		// 检查项模板
		templates := secured.Group("/inspection/templates")
		{
			templates.GET("/import/template", middleware.RequirePerm("inspection:template:update"), templateCtl.DownloadImportTemplate)
			templates.POST("/import/preview", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "template_import_preview"), templateCtl.PreviewImport)
			templates.POST("/import/apply", middleware.RequirePerm("inspection:template:update"), middleware.OperLog(db, "inspection", "template_import_apply"), templateCtl.ApplyImport)
			templates.POST("/import/failure-export", middleware.RequirePerm("inspection:template:update"), templateCtl.ExportImportFailures)
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

		// 工单池：抢单、分派、整改、主管审核和关闭。
		orders := secured.Group("/workorders")
		{
			orders.GET("", middleware.RequirePerm("workorder:list"), workorderCtl.List)
			orders.GET("/candidates", middleware.RequirePerm("workorder:assign"), workorderCtl.Candidates)
			orders.POST("/merge", middleware.RequirePerm("workorder:merge"), middleware.OperLog(db, "workorder", "merge"), workorderCtl.Merge)
			orders.GET("/:id", middleware.RequirePerm("workorder:list"), workorderCtl.Detail)
			orders.POST("/:id/claim", middleware.RequirePerm("workorder:claim"), middleware.OperLog(db, "workorder", "claim"), workorderCtl.Claim)
			orders.POST("/:id/assign", middleware.RequirePerm("workorder:assign"), middleware.OperLog(db, "workorder", "assign"), workorderCtl.Assign)
			orders.POST("/:id/finish", middleware.RequirePerm("workorder:finish"), middleware.OperLog(db, "workorder", "finish"), workorderCtl.Finish)
			orders.POST("/:id/review", middleware.OperLog(db, "workorder", "review"), workorderCtl.Review)
			orders.POST("/:id/close", middleware.RequirePerm("workorder:close"), middleware.OperLog(db, "workorder", "close"), workorderCtl.Close)
		}

		acceptance := secured.Group("/acceptance")
		{
			acceptance.GET("/batches", middleware.RequirePerm("acceptance:list"), acceptanceCtl.List)
			acceptance.POST("/batches", middleware.RequirePerm("acceptance:manage"), middleware.OperLog(db, "acceptance", "batch_create"), acceptanceCtl.Create)
			acceptance.GET("/batches/:id", middleware.RequirePerm("acceptance:list"), acceptanceCtl.Detail)
			acceptance.POST("/batches/:id/start", middleware.RequirePerm("acceptance:manage"), middleware.OperLog(db, "acceptance", "batch_start"), acceptanceCtl.Start)
			acceptance.POST("/batches/:id/records", middleware.RequirePerm("acceptance:submit"), middleware.OperLog(db, "acceptance", "record_save"), acceptanceCtl.SaveRecord)
			acceptance.POST("/batches/:id/targets/:targetId/coverage", middleware.RequirePerm("acceptance:submit"), middleware.OperLog(db, "acceptance", "coverage_confirm"), acceptanceCtl.ConfirmCoverage)
			acceptance.POST("/batches/:id/submit", middleware.RequirePerm("acceptance:submit"), middleware.OperLog(db, "acceptance", "batch_submit"), acceptanceCtl.Submit)
			acceptance.POST("/batches/:id/review", middleware.OperLog(db, "acceptance", "batch_review"), acceptanceCtl.Review)
			acceptance.POST("/batches/:id/sign", middleware.OperLog(db, "acceptance", "batch_sign"), acceptanceCtl.Sign)
			acceptance.PUT("/batches/:id/problems/:problemId/acceptance", middleware.RequirePerm("acceptance:manage"), middleware.OperLog(db, "acceptance", "problem_accept"), acceptanceCtl.AcceptProblem)
			acceptance.PUT("/batches/:id/sign-subjects", middleware.RequirePerm("acceptance:manage"), middleware.OperLog(db, "acceptance", "subjects_save"), acceptanceCtl.UpdateSubjects)
			acceptance.POST("/batches/:id/withdraw", middleware.RequirePerm("acceptance:manage"), middleware.OperLog(db, "acceptance", "batch_withdraw"), acceptanceCtl.Withdraw)
			acceptance.POST("/batches/:id/reopen-requests", middleware.RequirePerm("acceptance:reopen"), middleware.OperLog(db, "acceptance", "reopen_request"), acceptanceCtl.RequestReopen)
			acceptance.POST("/batches/:id/reopen-requests/:requestId/decision", middleware.RequirePerm("acceptance:reopen"), middleware.OperLog(db, "acceptance", "reopen_decision"), acceptanceCtl.DecideReopen)
			acceptance.GET("/batches/:id/files/:fileId/download", middleware.RequirePerm("acceptance:list"), acceptanceCtl.Download)
		}
		schemas := secured.Group("/acceptance-schemas")
		{
			schemas.GET("", middleware.RequirePerm("acceptance:schema:list"), acceptanceCtl.Schemas)
			schemas.PUT("/:code", middleware.RequirePerm("acceptance:schema:update"), middleware.OperLog(db, "acceptance", "schema_save"), acceptanceCtl.SaveSchema)
			schemas.PUT("/:code/rows/:rowCode", middleware.RequirePerm("acceptance:schema:update"), middleware.OperLog(db, "acceptance", "schema_row_save"), acceptanceCtl.SaveRow)
		}

		aiRegistry := secured.Group("/ai")
		{
			aiRegistry.GET("/task-catalog", middleware.RequirePerm("ai:config:list"), aiCtl.TaskCatalog)
			aiRegistry.GET("/providers", middleware.RequirePerm("ai:config:list"), aiCtl.Providers)
			aiRegistry.PUT("/providers/:code", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "provider_save"), aiCtl.SaveProvider)
			aiRegistry.GET("/models", middleware.RequirePerm("ai:config:list"), aiCtl.Models)
			aiRegistry.PUT("/models/:providerCode/:modelId", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "model_save"), aiCtl.SaveModel)
			aiRegistry.GET("/bindings", middleware.RequirePerm("ai:config:list"), aiCtl.Bindings)
			aiRegistry.PUT("/bindings/:taskType", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "binding_save"), aiCtl.SaveBinding)
			aiRegistry.GET("/usage", middleware.RequirePerm("ai:config:list"), usageCtl.Usage)
			aiRegistry.PUT("/quotas/:taskType", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "quota_save"), usageCtl.SaveQuota)
			aiRegistry.DELETE("/quotas/:taskType", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "quota_reset"), usageCtl.ResetQuota)
			aiRegistry.GET("/mcp/tools", middleware.RequirePerm("assistant:settings"), mcpCtl.Tools)
			aiRegistry.GET("/mcp/tokens", middleware.RequirePerm("assistant:settings"), mcpCtl.Tokens)
			aiRegistry.POST("/mcp/tokens", middleware.RequirePerm("assistant:settings"), middleware.OperLog(db, "ai", "mcp_token_issue"), mcpCtl.Issue)
			aiRegistry.DELETE("/mcp/tokens/:id", middleware.RequirePerm("assistant:settings"), middleware.OperLog(db, "ai", "mcp_token_revoke"), mcpCtl.Revoke)
			assistant := aiRegistry.Group("/assistant", middleware.RequirePerm("assistant:chat"))
			{
				assistant.GET("/status", assistantCtl.Status)
				assistant.GET("/conversations", assistantCtl.Conversations)
				assistant.GET("/conversations/:id/messages", assistantCtl.Messages)
				assistant.POST("/chat", assistantCtl.Chat)
			}
			agent := aiRegistry.Group("/agent", middleware.RequirePerm("assistant:chat"))
			{
				agent.GET("/status", agentCtl.Status)
				agent.GET("/tools", agentCtl.Tools)
				agent.PUT("/tools/:code", middleware.RequirePerm("ai:config:update"), middleware.OperLog(db, "ai", "agent_tool_save"), agentCtl.SetTool)
				agent.POST("/chat", agentCtl.Chat)
				agent.GET("/conversations/:id/calls", agentCtl.Calls)
			}
		}

		// 统计报表
		stats := secured.Group("/stats")
		{
			stats.GET("/coverage", middleware.RequirePerm("stats:report", "stats:inspection"), statsCtl.Coverage)
			stats.GET("/timeliness", middleware.RequirePerm("stats:report", "stats:inspection"), statsCtl.Timeliness)
			stats.GET("/patrol-rounds", middleware.RequirePerm("stats:report", "stats:inspection"), statsCtl.PatrolRounds)
			stats.GET("/performance", middleware.RequirePerm("stats:report", "stats:performance"), statsCtl.Performance)
			stats.POST("/export", middleware.RequirePerm("stats:export"), middleware.OperLog(db, "stats", "export"), statsCtl.Export)
		}

		// 月度报告（动态审核链签字 + PDF 归档）
		docTemplates := secured.Group("/doc-templates")
		{
			docTemplates.GET("/style-packs", middleware.RequirePerm("report:template:list"), docTemplateCtl.StylePacks)
			docTemplates.GET("/:id/bindings", middleware.RequirePerm("report:template:list"), docTemplateCtl.Bindings)
			docTemplates.PUT("/:id/bindings", middleware.RequirePerm("report:template:update"), middleware.OperLog(db, "report", "template_output_bindings"), docTemplateCtl.SaveBindings)
			docTemplates.GET("", middleware.RequirePerm("report:template:list"), docTemplateCtl.List)
			docTemplates.PUT("/:id", middleware.RequirePerm("report:template:update"), middleware.OperLog(db, "report", "doc_template_save"), docTemplateCtl.Save)
		}
		reports := secured.Group("/reports")
		{
			reports.GET("", middleware.RequirePerm("report:list"), reportCtl.List)
			reports.POST("/generate", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "generate"), reportCtl.Generate)
			// 报告生成计划（计划任务-报告计划 tab；周期驱动自动生成 + 手动触发）
			reports.GET("/plans", middleware.RequirePerm("report:list"), reportCtl.ListReportPlans)
			reports.POST("/plans", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "plan_create"), reportCtl.CreateReportPlan)
			reports.PUT("/plans/:id", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "plan_update"), reportCtl.UpdateReportPlan)
			reports.DELETE("/plans/:id", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "plan_delete"), reportCtl.DeleteReportPlan)
			reports.POST("/plans/:id/run", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "plan_run"), reportCtl.RunReportPlanNow)
			reports.POST("/:id/files/:fileId/retry", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "file_retry"), reportCtl.RetryFile)
			reports.POST("/:id/void", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "void"), reportCtl.Void)
			reports.GET("/sign-candidates", middleware.RequirePerm("report:generate"), reportCtl.SignCandidates)
			reports.GET("/:id/records", middleware.RequirePerm("report:list"), reportCtl.PagedRecords)
			reports.GET("/:id/files/:fileId/download", reportCtl.DownloadFile)
			reports.GET("/:id", middleware.RequirePerm("report:list"), reportCtl.Detail)
			reports.POST("/:id/sign-step/:step", middleware.OperLog(db, "report", "sign_step"), reportCtl.SignStep)
		}
	}

	// 小程序端
	mp := r.Group("/api/mp")
	{
		mp.POST("/login", mpCtl.Login)
		mp.POST("/refresh", mpCtl.Refresh)
		mp.GET("/auth/register-config", authCtl.RegisterConfig)
		mp.GET("/auth/register-tenants", authCtl.RegisterTenants)
		mp.POST("/auth/register", authCtl.Register)
		mp.POST("/upload/callback", mpCtl.Callback) // OSS 服务端间回调（验签，无 JWT）

		mpAuth := mp.Group("", middleware.Auth(db, sess, jwtm, authsvc.ChannelApp)) // v21 起与 App 共用 app 通道会话
		{
			mpAuth.GET("/tasks/today", mpCtl.TodayTasks)
			mpAuth.GET("/tasks/history", mpCtl.HistoryTasks)
			mpAuth.GET("/tasks/:id", mpCtl.TaskDetail)
			mpAuth.GET("/points/by-code/:code", mpCtl.PointByCode)
			mpAuth.GET("/points/nearby", mpCtl.NearbyPoints)
			mpAuth.GET("/equipment/due", maintCtl.MpDueDevices)        // 设备台账：我的待维保列表（临期+逾期）
			mpAuth.GET("/equipment/list", equipmentCtl.MpList)         // 维保选设备：租户内在用设备分页（巡检员本职，无需 equipment:list 权限）
			mpAuth.POST("/equipment/maintenance", maintCtl.MpRegister) // 维保登记（一键+一拍，pending 待经理确认）
			mpAuth.GET("/equipment/maintenance-mine", maintCtl.Mine)   // 我的提交（全部状态，最新在前）
			mpAuth.PUT("/equipment/maintenance/:id", maintCtl.Update)  // 待确认登记修改（限本人，重走 AI 核验）
			mpAuth.POST("/checkin", mpCtl.Checkin)
			mpAuth.POST("/checkin/offline-sync", mpCtl.OfflineSync)
			mpAuth.POST("/checkin/ai-item-jobs", mpCtl.SubmitAIItemJob)                          // 逐项 AI 识别：提交
			mpAuth.GET("/checkin/ai-item-jobs", mpCtl.AIItemJobs)                                // 逐项 AI 识别：批量轮询结果
			mpAuth.GET("/checkin/item-drafts", mpCtl.ItemDrafts)                                 // 逐项过程草稿（断点恢复）
			mpAuth.DELETE("/checkin/item-drafts", mpCtl.DeleteItemDraft)                         // 撤销某项草稿（逃生选错回到待拍）
			mpAuth.POST("/checkin/item-drafts/manual", mpCtl.SaveManualDraft)                    // 手动项选择落草稿
			mpAuth.POST("/checkin/item-drafts/photo-abnormal", mpCtl.SavePhotoItemAbnormalDraft) // 拍照项异常逃生入口
			mpAuth.POST("/checkin/point-cred", mpCtl.SavePointCredDraft)                         // 点位凭证核验落草稿（断点恢复）
			mpAuth.GET("/checkins/:id", mpCtl.CheckinBrief)                                      // 本人打卡摘要（消息深链定位记录卡）
			mpAuth.GET("/checkins/:id/items", mpCtl.CheckinItems)                                // 本人打卡逐项 AI 结论
			mpAuth.POST("/upload/sts", mpCtl.STS)
			mpAuth.POST("/upload/preflight", mpCtl.Preflight)
			mpAuth.POST("/upload/local", mpCtl.Local) // local 模式上传
			mpAuth.GET("/messages", mpCtl.Messages)
			mpAuth.PUT("/messages/:id/read", mpCtl.MarkRead)
			mpAuth.POST("/push/device", pushCtl.BindDevice) // uniPush 设备 cid 绑定/解绑
			mpAuth.DELETE("/push/device", pushCtl.UnbindDevice)
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
			appAuth.GET("/tasks/history", mpCtl.HistoryTasks)
			appAuth.GET("/tasks/:id", mpCtl.TaskDetail)
			appAuth.GET("/points/by-code/:code", mpCtl.PointByCode)
			appAuth.GET("/points/nearby", mpCtl.NearbyPoints)
			appAuth.POST("/checkin", mpCtl.Checkin)
			appAuth.POST("/checkin/offline-sync", mpCtl.OfflineSync)
			appAuth.POST("/checkin/ai-item-jobs", mpCtl.SubmitAIItemJob)                          // 逐项 AI 识别：提交
			appAuth.GET("/checkin/ai-item-jobs", mpCtl.AIItemJobs)                                // 逐项 AI 识别：批量轮询结果
			appAuth.GET("/checkin/item-drafts", mpCtl.ItemDrafts)                                 // 逐项过程草稿（断点恢复）
			appAuth.DELETE("/checkin/item-drafts", mpCtl.DeleteItemDraft)                         // 撤销某项草稿（逃生选错回到待拍）
			appAuth.POST("/checkin/item-drafts/manual", mpCtl.SaveManualDraft)                    // 手动项选择落草稿
			appAuth.POST("/checkin/item-drafts/photo-abnormal", mpCtl.SavePhotoItemAbnormalDraft) // 拍照项异常逃生入口
			appAuth.POST("/checkin/point-cred", mpCtl.SavePointCredDraft)                         // 点位凭证核验落草稿（断点恢复）
			appAuth.GET("/checkins/:id", mpCtl.CheckinBrief)                                      // 本人打卡摘要（消息深链定位记录卡）
			appAuth.GET("/checkins/:id/items", mpCtl.CheckinItems)                                // 本人打卡逐项 AI 结论
			appAuth.POST("/upload/sts", mpCtl.STS)
			appAuth.POST("/upload/preflight", mpCtl.Preflight)
			appAuth.POST("/upload/local", mpCtl.Local)
			// 消息 / 公告
			appAuth.GET("/messages", mpCtl.Messages)
			appAuth.PUT("/messages/:id/read", mpCtl.MarkRead)
			appAuth.POST("/push/device", pushCtl.BindDevice) // uniPush 设备 cid 绑定/解绑（与 mp 组同 handler）
			appAuth.DELETE("/push/device", pushCtl.UnbindDevice)
			appAuth.GET("/announcements", mpCtl.Announcements)
			appAuth.GET("/announcements/:id", mpCtl.AnnouncementDetail)

			// ===== 管理功能（App 端）：复用 PC 控制器 + 同一套权限点，入口由 App 按 perms 显隐 =====
			appAuth.GET("/tenants", middleware.RequirePerm("tenant:list"), tenantCtl.List) // 超管「当前公司」切换的租户列表
			// 设备台账（App 端）：复用 PC/mp 控制器 + 同一套权限点
			appEquipment := appAuth.Group("/equipment")
			{
				appEquipment.GET("/list", middleware.RequirePerm("equipment:list"), equipmentCtl.List)
				appEquipment.GET("/due", maintCtl.MpDueDevices)
				appEquipment.POST("/maintenance", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance"), maintCtl.Register)
				appEquipment.GET("/maintenance-mine", maintCtl.Mine)
				appEquipment.PUT("/maintenance/:id", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance_update"), maintCtl.Update)
				appEquipment.GET("/maintenance-pending", middleware.RequirePerm("equipment:confirm"), maintCtl.PendingList)
				appEquipment.POST("/maintenance/confirm", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "confirm"), maintCtl.Confirm)
				appEquipment.POST("/maintenance/reject", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "reject"), maintCtl.Reject)
				appEquipment.POST("/maintenance-batches/preview", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance_batch_preview"), maintCtl.PreviewBatch)
				appEquipment.POST("/maintenance-batches/:id", middleware.RequirePerm("equipment:maintenance"), middleware.OperLog(db, "equipment", "maintenance_batch_create"), maintCtl.CreateBatch)
				appEquipment.POST("/maintenance-batches/:id/confirm", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "maintenance_batch_confirm"), maintCtl.ConfirmBatch)
				appEquipment.POST("/maintenance-batches/:id/reject", middleware.RequirePerm("equipment:confirm"), middleware.OperLog(db, "equipment", "maintenance_batch_reject"), maintCtl.RejectBatch)
				appEquipment.GET("/:id", middleware.RequirePerm("equipment:list", "equipment:maintenance"), equipmentCtl.Detail)
				appEquipment.GET("/:id/maintenances", middleware.RequirePerm("equipment:list", "equipment:maintenance"), maintCtl.History)
			}
			appOrders := appAuth.Group("/workorders")
			{
				appOrders.GET("", middleware.RequirePerm("workorder:list"), workorderCtl.List)
				appOrders.POST("/merge", middleware.RequirePerm("workorder:merge"), middleware.OperLog(db, "workorder", "merge"), workorderCtl.Merge)
				appOrders.GET("/:id", middleware.RequirePerm("workorder:list"), workorderCtl.Detail)
				appOrders.POST("/:id/claim", middleware.RequirePerm("workorder:claim"), workorderCtl.Claim)
				appOrders.POST("/:id/finish", middleware.RequirePerm("workorder:finish"), workorderCtl.Finish)
				appOrders.POST("/:id/review", workorderCtl.Review)
			}
			appAuth.GET("/dashboard", statsCtl.Dashboard)
			appAuth.GET("/communities/tree", middleware.RequirePerm("community:list", "inspection:point:list"), communityCtl.Tree)
			appAuth.GET("/system/users", middleware.RequirePerm("system:user:list"), userCtl.List)
			// 点位管理（现场建点：GPS 录坐标 + NFC 写卡；删除仅 PC 端）
			appPoints := appAuth.Group("/inspection/points")
			{
				appPoints.GET("", middleware.RequirePerm("inspection:point:list"), inspectionCtl.ListPoints)
				appPoints.POST("", middleware.RequirePerm("inspection:point:create"), middleware.OperLog(db, "inspection", "create"), inspectionCtl.CreatePoint)
				appPoints.GET("/:id", middleware.RequirePerm("inspection:point:list"), inspectionCtl.PointDetail)
				appPoints.PUT("/:id", middleware.RequirePerm("inspection:point:update"), middleware.OperLog(db, "inspection", "update"), inspectionCtl.UpdatePoint)
			}
			appAuth.GET("/dict-options", dictCtl.Options) // 业务字典只读选项（报告类型下拉等，登录即可）
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
			// 管理端打卡明细（看板/任务监控点位详情用；复用 PC 记录详情，数据范围按小区校验）
			appAuth.GET("/inspection/checkins/:id",
				middleware.RequirePerm("inspection:task:list", "inspection:task:monitor", "inspection:record:list", "inspection:checkin:list", "inspection:checkin:review"),
				inspectionCtl.CheckinDetail)
			// 月报签字（权限点与 PC 完全一致；待我签用 ?pending_mine=1）
			appReports := appAuth.Group("/reports")
			{
				appReports.POST("/generate", middleware.RequirePerm("report:generate"), middleware.OperLog(db, "report", "generate"), reportCtl.Generate) // App 管理端手动生成报告
				appReports.GET("/sign-candidates", middleware.RequirePerm("report:generate"), reportCtl.SignCandidates)
				appReports.GET("", middleware.RequirePerm("report:list"), reportCtl.List)
				appReports.GET("/:id/records", middleware.RequirePerm("report:list"), reportCtl.PagedRecords)
				appReports.GET("/:id", middleware.RequirePerm("report:list"), reportCtl.Detail)
				appReports.POST("/:id/sign-step/:step", middleware.OperLog(db, "report", "sign_step"), reportCtl.SignStep)
				appReports.GET("/:id/files/:fileId/download", reportCtl.DownloadFile)
			}
		}
	}

	// 品牌官网（SSR：/、/download、/robots.txt、/sitemap.xml、/site/*）与 App 下载公开接口
	registerSiteRoutes(r, cfg, siteSvc, siteCtl)

	// 生产单端口 SPA：管理后台托管在 /admin 子路径，非 /api、/uploads 路径 fallback 到 index.html
	registerSPA(r, cfg.SPA.DistPath)

	return r, scheduler
}

// registerSPA 注册管理后台 SPA 静态托管与 history 路由回退（SPA_DIST_PATH 指向有效目录时生效）。
// 管理后台统一挂在 /admin 子路径下，根路径留给品牌官网。
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
		// 非 /admin 路径一律回到官网首页（官网自身路由已显式注册，不会走到这里）
		if p != "/admin" && !strings.HasPrefix(p, "/admin/") {
			c.Redirect(http.StatusFound, "/")
			return
		}
		// 命中静态文件则直接返回，否则回退 index.html（前端 history 路由，base 为 /admin/）
		clean := filepath.Clean(strings.TrimPrefix(p, "/admin"))
		fp := filepath.Join(root, clean)
		if strings.HasPrefix(fp, root+string(os.PathSeparator)) {
			if st, err := os.Stat(fp); err == nil && !st.IsDir() {
				// 带内容哈希的构建产物（/assets/*.js|css）可长缓存（clean 以 / 开头）
				if strings.HasPrefix(clean, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				serveFileSmart(c, fp, st)
				return
			}
		}
		// index.html 不长缓存：重新部署后浏览器总能拿到最新的 chunk 引用（防白屏）
		c.Header("Cache-Control", "no-cache")
		if st, err := os.Stat(index); err == nil {
			serveFileSmart(c, index, st)
			return
		}
		c.File(index)
	})
}

// registerSystemRoutes 系统管理路由（与第一阶段一致 + 通知公告）。
func registerSystemRoutes(sys *gin.RouterGroup, db *gorm.DB,
	userCtl *systemctl.UserController, roleCtl *systemctl.RoleController, menuCtl *systemctl.MenuController,
	dictCtl *systemctl.DictController, configCtl *systemctl.ConfigController, logCtl *systemctl.LogController,
	noticeCtl *systemctl.NoticeController, uploadCtl *systemctl.UploadController, signAssetCtl *systemctl.SignAssetController,
	postCtl *systemctl.PostController, postTmplCtl *systemctl.PostTemplateController) {

	// 管理端图片上传（登录即可：签名/公章/头像）
	sys.POST("/upload", middleware.OperLog(db, "system", "upload"), uploadCtl.Local)

	// 签章资产（手写签名/公章版本链）
	signAssets := sys.Group("/sign-assets")
	{
		signAssets.GET("", middleware.RequirePerm("system:signasset:list"), signAssetCtl.List)
		signAssets.POST("", middleware.RequirePerm("system:signasset:create"), middleware.OperLog(db, "system", "sign_asset_create"), signAssetCtl.Create)
		signAssets.POST("/:id/revoke", middleware.RequirePerm("system:signasset:revoke"), middleware.OperLog(db, "system", "sign_asset_revoke"), signAssetCtl.Revoke)
	}

	// 同一审批模型覆盖平台默认、租户默认与项目覆盖，无独立业务配置接口。
	sys.GET("/review-flows/:flowCode", middleware.RequirePerm("system:reviewflow:list"), postCtl.GetFlow)
	sys.PUT("/review-flows/:flowCode", middleware.RequirePerm("system:reviewflow:update"), middleware.OperLog(db, "system", "review_flow_save"), postCtl.SaveFlow)
	sys.GET("/review-flow-templates/:flowCode", middleware.RequirePerm("platform:reviewflow:list"), postTmplCtl.GetFlow)
	sys.PUT("/review-flow-templates/:flowCode", middleware.RequirePerm("platform:reviewflow:update"), middleware.OperLog(db, "system", "review_flow_save"), postTmplCtl.SaveFlow)

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
