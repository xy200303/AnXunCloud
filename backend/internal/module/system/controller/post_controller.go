package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/types"
)

// PostController 岗位管理接口（系统管理 /system/posts，租户上下文）。db 用于解析租户上下文。
type PostController struct {
	svc *service.PostService
	db  *gorm.DB
}

func NewPostController(svc *service.PostService, db *gorm.DB) *PostController {
	return &PostController{svc: svc, db: db}
}

// writePost 统一响应（data 为 nil 时返回空 data）。
func writePost(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// List GET /posts（按 line+sort 排序，含业务线与绑定角色名称）
func (ctl *PostController) List(c *gin.Context) {
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.List(&tid)
	writePost(c, items, be)
}

// Create POST /posts
func (ctl *PostController) Create(c *gin.Context) {
	var req dto.PostSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.Create(&tid, &req)
	writePost(c, gin.H{"id": id}, be)
}

// Update PUT /posts/:id（code 创建后不可改）
func (ctl *PostController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.PostSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.Update(&tid, id, &req))
}

// Delete DELETE /posts/:id（被编制/职责绑定引用时拒绝）
func (ctl *PostController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.Delete(&tid, id))
}

// DutyBindings GET /posts/duty-bindings（租户级默认槽位绑定：各槽位解析结果与来源 tenant/platform）
func (ctl *PostController) DutyBindings(c *gin.Context) {
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.ListDutyBindings(&tid)
	writePost(c, items, be)
}

// SaveDutyBindings PUT /posts/duty-bindings（写 project_id NULL + tenant_id=上下文租户 的行）
func (ctl *PostController) SaveDutyBindings(c *gin.Context) {
	var req dto.PostDutyBindingsSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.SaveDutyBindings(&tid, &req))
}

// GetReviewFlow GET /posts/review-flow（租户级打卡审核链视图：tenant/platform/default）
func (ctl *PostController) GetReviewFlow(c *gin.Context) {
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.GetReviewFlow(&tid)
	writePost(c, data, be)
}

// SaveReviewFlow PUT /posts/review-flow（写 tenant_id=上下文租户 的审核链覆盖行）
func (ctl *PostController) SaveReviewFlow(c *gin.Context) {
	var req struct {
		Steps types.FlowStepArray `json:"steps" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.SaveReviewFlow(&tid, req.Steps))
}

// PostTemplateController 岗位模板库接口（平台管理 /platform/post-templates，is_platform 仅超管）。
// 作用域为 post_dict 的 tenant_id IS NULL 行；仅作开通租户的初始拷贝源，不参与租户实际业务。
type PostTemplateController struct {
	svc *service.PostService
}

func NewPostTemplateController(svc *service.PostService) *PostTemplateController {
	return &PostTemplateController{svc: svc}
}

// List GET /post-templates
func (ctl *PostTemplateController) List(c *gin.Context) {
	items, be := ctl.svc.List(nil)
	writePost(c, items, be)
}

// Create POST /post-templates（role_id 只允许绑内置共享角色）
func (ctl *PostTemplateController) Create(c *gin.Context) {
	var req dto.PostSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.Create(nil, &req)
	writePost(c, gin.H{"id": id}, be)
}

// Update PUT /post-templates/:id
func (ctl *PostTemplateController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.PostSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.Update(nil, id, &req))
}

// Delete DELETE /post-templates/:id（不影响已开通租户的复制行）
func (ctl *PostTemplateController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.Delete(nil, id))
}

// DutyBindings GET /post-templates/duty-bindings（平台默认槽位绑定，来源恒为 platform）
func (ctl *PostTemplateController) DutyBindings(c *gin.Context) {
	items, be := ctl.svc.ListDutyBindings(nil)
	writePost(c, items, be)
}

// SaveDutyBindings PUT /post-templates/duty-bindings（写 project_id NULL + tenant_id NULL 平台默认行）
func (ctl *PostTemplateController) SaveDutyBindings(c *gin.Context) {
	var req dto.PostDutyBindingsSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.SaveDutyBindings(nil, &req))
}

// GetReviewFlow GET /post-templates/review-flow（平台默认打卡审核链视图，来源 platform/default）
func (ctl *PostTemplateController) GetReviewFlow(c *gin.Context) {
	data, be := ctl.svc.GetReviewFlow(nil)
	writePost(c, data, be)
}

// SaveReviewFlow PUT /post-templates/review-flow（写平台默认审核链行）
func (ctl *PostTemplateController) SaveReviewFlow(c *gin.Context) {
	var req struct {
		Steps types.FlowStepArray `json:"steps" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	writePost(c, nil, ctl.svc.SaveReviewFlow(nil, req.Steps))
}
