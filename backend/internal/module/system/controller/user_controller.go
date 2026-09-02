package controller

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
	"anxuncloud/internal/pkg/response"
)

// 导入文件大小上限 5MB。
const importMaxFileSize = 5 << 20

// UserController 用户管理接口。db 用于解析租户上下文（middleware.EffectiveTenantID）。
type UserController struct {
	svc *service.UserService
	db  *gorm.DB
}

func NewUserController(svc *service.UserService, db *gorm.DB) *UserController {
	return &UserController{svc: svc, db: db}
}

// List GET /system/users（租户上下文：?tenant_id= 或 X-Tenant-Id，仅超管可切换）
func (ctl *UserController) List(c *gin.Context) {
	var q dto.UserListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(&q, middleware.CurrentIdentity(c), tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// Create POST /system/users
func (ctl *UserController) Create(c *gin.Context) {
	var req dto.UserCreateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.Create(&req, middleware.CurrentIdentity(c), tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// Detail GET /system/users/:id
func (ctl *UserController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	detail, be := ctl.svc.Detail(id, middleware.CurrentIdentity(c))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, detail)
}

// Update PUT /system/users/:id
func (ctl *UserController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.UserUpdateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Update(id, &req, middleware.CurrentIdentity(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// ResetPassword PUT /system/users/:id/password/reset
func (ctl *UserController) ResetPassword(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ResetPasswordReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.ResetPassword(c.Request.Context(), id, req.NewPassword, middleware.CurrentIdentity(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// SetStatus PUT /system/users/:id/status
func (ctl *UserController) SetStatus(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.StatusReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.SetStatus(c.Request.Context(), id, req.Status, middleware.CurrentIdentity(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// Delete DELETE /system/users/:id
func (ctl *UserController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Delete(c.Request.Context(), id, middleware.CurrentIdentity(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// ImportTemplate GET /system/users/import-template（角色/小区软下拉，数据源按租户动态生成）
func (ctl *UserController) ImportTemplate(c *gin.Context) {
	f, be := ctl.svc.ImportTemplate(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	writeExcel(c, "user_import_template.xlsx", buf.Bytes())
}

// Import POST /system/users/import（multipart 上传 .xlsx）
func (ctl *UserController) Import(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("缺少上传文件 file"))
		return
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".xlsx") {
		response.Fail(c, errs.ErrImportFileType)
		return
	}
	if fileHeader.Size > importMaxFileSize {
		response.Fail(c, errs.ErrParam.WithMsg("导入文件不能超过 5MB"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	result, msg, be := ctl.svc.Import(f, middleware.CurrentIdentity(c), tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, msg, result)
}

// Export GET /system/users/export（按筛选导出 Excel 文件流）
func (ctl *UserController) Export(c *gin.Context) {
	var q dto.UserListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	rows, be := ctl.svc.Export(&q, middleware.CurrentIdentity(c), tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	f, err := excel.ExportUsers(rows)
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	writeExcel(c, fmt.Sprintf("users_%s.xlsx", time.Now().Format("20060102")), buf.Bytes())
}

// writeExcel 输出 Excel 文件流（非统一 JSON 结构）。
func writeExcel(c *gin.Context, filename string, data []byte) {
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// UpdateProfile PUT /system/users/profile（登录即可）：修改本人基本资料（可选签名图 file_id、头像 URL），返回最新用户信息。
func (ctl *UserController) UpdateProfile(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required"`
		Phone string `json:"phone" binding:"required"`
		// 手写签名图 file_id：缺省不改动；空串删除签名
		SignatureFileID *string `json:"signature_file_id"`
		// 头像 URL：缺省不改动；空串清除头像
		Avatar *string `json:"avatar"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	uid := middleware.CurrentUserID(c)
	if be := ctl.svc.UpdateProfile(uid, req.Name, req.Phone, req.SignatureFileID, req.Avatar); be != nil {
		response.Fail(c, be)
		return
	}
	// 返回最新用户信息（与 /auth/info 同构的用户字段子集）
	detail, be := ctl.svc.Detail(uid, middleware.CurrentIdentity(c))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, detail)
}

// ChangePassword PUT /system/users/password（登录即可）：修改本人密码。
// 取舍说明：改密成功后该用户全部会话（含当前）失效，需用新密码重新登录——
// 安全性优先，避免改密后旧 token 仍可用的窗口期。
func (ctl *UserController) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.ChangePassword(c.Request.Context(), middleware.CurrentUserID(c), req.OldPassword, req.NewPassword); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
