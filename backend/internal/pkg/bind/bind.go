// Package bind 统一请求绑定与参数校验错误转换。
package bind

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"anxuncloud/internal/pkg/errs"
)

// JSON 绑定 JSON 请求体；语法错误返回 40002，校验失败返回 40001（含字段原因）。
func JSON(c *gin.Context, req any) *errs.Error {
	if err := c.ShouldBindJSON(req); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) || errors.Is(err, io.EOF) {
			return errs.ErrBodyFormat
		}
		return errs.ErrParam.WithMsg(validationMsg(err))
	}
	return nil
}

// Query 绑定 query 参数。
func Query(c *gin.Context, req any) *errs.Error {
	if err := c.ShouldBindQuery(req); err != nil {
		return errs.ErrParam.WithMsg(validationMsg(err))
	}
	return nil
}

// validationMsg 将 validator 错误转为可读中文提示（取首个字段）。
func validationMsg(err error) string {
	var ves validator.ValidationErrors
	if !errors.As(err, &ves) || len(ves) == 0 {
		return "请求参数错误: " + err.Error()
	}
	fe := ves[0]
	field := fe.Field()
	// 字段名首字母小写，接近 JSON 命名
	if field != "" {
		field = strings.ToLower(field[:1]) + field[1:]
	}
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s 为必填项", field)
	case "min":
		return fmt.Sprintf("%s 长度/数量不能小于 %s", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s 长度/数量不能超过 %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s 取值非法（允许: %s）", field, fe.Param())
	case "gt", "gte", "lt", "lte":
		return fmt.Sprintf("%s 数值范围非法", field)
	default:
		return fmt.Sprintf("%s 校验未通过（%s）", field, fe.Tag())
	}
}
