// Package controller 设备台账模块接口 HTTP 层（台账 CRUD / 导入导出 / 维保登记确认链）。
package controller

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// EquipmentController 设备台账接口。
type EquipmentController struct {
	equipment *service.EquipmentService
}

func NewEquipmentController(equipment *service.EquipmentService) *EquipmentController {
	return &EquipmentController{equipment: equipment}
}

func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("id 须为 UUID")
	}
	return id, nil
}

func write(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// List GET /equipment/list（或 /equipment；建议路由见模块报告）
func (ctl *EquipmentController) List(c *gin.Context) {
	var q dto.ListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.equipment.List(c, &q)
	write(c, page, be)
}

// Detail GET /equipment/:id
func (ctl *EquipmentController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.equipment.Detail(c, id)
	write(c, data, be)
}

// Create POST /equipment（warning 为点位类型软校验提示，非错误）
func (ctl *EquipmentController) Create(c *gin.Context) {
	var req dto.SaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, warning, be := ctl.equipment.Create(c, &req)
	write(c, gin.H{"id": id, "warning": warning}, be)
}

// Update PUT /equipment/:id
func (ctl *EquipmentController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.SaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	warning, be := ctl.equipment.Update(c, id, &req)
	write(c, gin.H{"warning": warning}, be)
}

// Delete DELETE /equipment/:id
func (ctl *EquipmentController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.equipment.Delete(c, id))
}

// writeExcel 输出 Excel 文件流（中文文件名：ASCII 兜底 + RFC 5987 filename*）。
func writeExcel(c *gin.Context, asciiName, chineseName string, data []byte) {
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		asciiName, url.PathEscape(chineseName)))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ImportTemplate GET /equipment/import-template（列头即甲方台账列结构，按列头名识别）
func (ctl *EquipmentController) ImportTemplate(c *gin.Context) {
	f, be := ctl.equipment.ImportTemplate()
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
	writeExcel(c, "equipment_import_template.xlsx", "设备台账导入模板.xlsx", buf.Bytes())
}

// importMaxFileSize 导入文件大小上限 5MB。
const importMaxFileSize = 5 << 20

// Import POST /equipment/import?community_id=xxx（multipart 上传 .xlsx，整个文件导入到指定小区）
func (ctl *EquipmentController) Import(c *gin.Context) {
	communityID := c.PostForm("community_id")
	if communityID == "" {
		communityID = c.Query("community_id")
	}
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
	result, msg, be := ctl.equipment.Import(c, communityID, f)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, msg, result)
}

// Export GET /equipment/export（同列表筛选，不分页，xlsx 附件）
func (ctl *EquipmentController) Export(c *gin.Context) {
	var q dto.ListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	f, be := ctl.equipment.Export(c, &q)
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
	stamp := time.Now().Format("20060102_150405")
	writeExcel(c, fmt.Sprintf("equipment_%s.xlsx", stamp), fmt.Sprintf("设备台账_%s.xlsx", stamp), buf.Bytes())
}
