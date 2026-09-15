package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// TypeSchemas GET /equipment/type-schemas?type=（类型字段方案：带 type 单查生效方案，不带列全部；租户级优先、回落平台默认）
func (ctl *EquipmentController) TypeSchemas(c *gin.Context) {
	data, be := ctl.equipment.ListTypeSchemas(c, c.Query("type"))
	write(c, data, be)
}

// SaveTypeSchema PUT /equipment/type-schemas/:type（租户级保存，整体替换 config，upsert）
func (ctl *EquipmentController) SaveTypeSchema(c *gin.Context) {
	typeValue := c.Param("type")
	if typeValue == "" {
		response.Fail(c, errs.ErrParam.WithMsg("type 必填"))
		return
	}
	var req dto.TypeSchemaSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.equipment.SaveTypeSchema(c, typeValue, &req)
	write(c, data, be)
}
