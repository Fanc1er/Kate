// Package response 提供统一响应格式 {code, message, data} 与分页结构。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// Body 统一响应体。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Page 分页数据。
type Page struct {
	List  any `json:"list"`
	Total int64 `json:"total"`
}

// OK 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data})
}

// Fail 以业务错误响应。
func Fail(c *gin.Context, e *errs.APIError) {
	c.JSON(errs.StatusOf(e.Code), Body{Code: e.Code, Message: e.Message, Data: nil})
}

// FailCode 以错误码响应，message 为空时取错误码默认文案。
func FailCode(c *gin.Context, code int) {
	Fail(c, errs.New(code, ""))
}

// FailMsg 以错误码 + 自定义文案响应。
func FailMsg(c *gin.Context, code int, msg string) {
	Fail(c, errs.New(code, msg))
}
