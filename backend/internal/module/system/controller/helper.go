// Package controller 系统管理接口 HTTP 层。
package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"property-inspection/internal/pkg/errs"
)

// pathID 解析路径参数 {id}（UUID）。
func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("id 须为 UUID")
	}
	return id, nil
}
