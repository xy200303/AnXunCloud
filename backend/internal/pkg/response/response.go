// Package response 提供统一响应结构输出（code/message/data）。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/pkg/errs"
)

// body 统一响应体。
type body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// OK 输出成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, body{Code: 0, Message: "success", Data: data})
}

// OKMsg 输出带自定义提示的成功响应（如导入完成汇总）。
func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, body{Code: 0, Message: msg, Data: data})
}

// Fail 输出业务错误响应；非 *errs.Error 一律按 50000 处理。
func Fail(c *gin.Context, err error) {
	if be, ok := err.(*errs.Error); ok {
		c.AbortWithStatusJSON(be.HTTP, body{Code: be.Code, Message: be.Msg, Data: be.Data})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, body{
		Code:    errs.ErrInternal.Code,
		Message: errs.ErrInternal.Msg,
		Data:    nil,
	})
}

// Page 分页响应结构（data 内字段）。
type Page struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// PageQuery 分页请求参数解析（page 默认 1，page_size 默认 20、上限 100）。
type PageQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// Normalize 规整分页参数，返回 offset/limit。
func (q *PageQuery) Normalize() (offset, limit int) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	return (q.Page - 1) * q.PageSize, q.PageSize
}
