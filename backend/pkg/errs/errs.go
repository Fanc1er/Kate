// Package errs 定义统一业务错误码与 APIError。
package errs

import "net/http"

// 业务错误码，遵循设计文档「错误码枚举」。
const (
	CodeOK                 = 0
	CodeValidationFailed   = 1000 // 参数校验失败
	CodeInvalidFormat      = 1002 // 规则文件格式错误
	CodeAuthFailed         = 2000 // JWT 缺失/无效
	CodeTokenExpired       = 2001 // JWT 过期
	CodeAccountLocked      = 2002 // 账户锁定
	CodeUserDisabled       = 2003 // 禁用用户
	CodeForbidden          = 2100 // 角色无权限
	CodeScopeDenied        = 2101 // API Token 无对应 scope
	CodeDuplicateURL       = 3000 // 资产 URL 重复
	CodeTaskStateConflict  = 3001 // 任务状态冲突
	CodeNotFound           = 4000 // 资源不存在
	CodeEvidenceTampered   = 4001 // 证据 Hash 校验失败
	CodeRuleVersionMismatch= 4002 // 规则版本冲突
	CodeAssetQuota         = 4290 // 资产超配额
	CodeWorkerQuota        = 4291 // Worker 超配额
	CodeEngineTimeout      = 5000 // 引擎超时
	CodeTargetBreakerOpen  = 5001 // 目标熔断
	CodeWorkerUnauthorized = 5002 // Worker 凭证非法
	CodeNotifyFailed       = 6000 // 通知推送失败
	CodeIntelSourceOffline = 6001 // 情报源离线
	CodeLicenseRequired        = 2400 // 缺少有效授权
	CodeLicenseInvalid         = 2401 // 授权文件无效（签名/格式）
	CodeLicenseExpired         = 2402 // 授权已过期
	CodeLicenseMachineMismatch = 2403 // 机器不匹配
	CodeLicenseNotYetActive    = 2404 // 授权未生效（延迟激活）
)

// CodeMessage 错误码 → 人类可读消息。
var CodeMessage = map[int]string{
	CodeOK:                  "ok",
	CodeValidationFailed:    "参数校验失败",
	CodeInvalidFormat:       "规则文件格式错误",
	CodeAuthFailed:          "认证失败",
	CodeTokenExpired:        "凭证已过期",
	CodeAccountLocked:       "账户已锁定，请稍后再试",
	CodeUserDisabled:        "用户已被禁用",
	CodeForbidden:           "没有操作权限",
	CodeScopeDenied:         "API Token 权限不足",
	CodeDuplicateURL:        "资产 URL 已存在",
	CodeTaskStateConflict:   "任务状态冲突",
	CodeNotFound:            "目标资源不存在",
	CodeEvidenceTampered:    "证据已被破坏",
	CodeRuleVersionMismatch: "规则版本冲突，请刷新后重试",
	CodeAssetQuota:          "资产数量已达套餐上限",
	CodeWorkerQuota:         "Worker 数量已达套餐上限",
	CodeEngineTimeout:       "引擎执行超时",
	CodeTargetBreakerOpen:   "目标已被熔断",
	CodeWorkerUnauthorized:  "Worker 凭证非法",
	CodeNotifyFailed:        "通知推送失败",
	CodeIntelSourceOffline:  "情报源离线",
	CodeLicenseRequired:        "缺少有效授权文件",
	CodeLicenseInvalid:         "授权文件无效",
	CodeLicenseExpired:         "授权已过期",
	CodeLicenseMachineMismatch: "授权与当前机器不匹配",
	CodeLicenseNotYetActive:    "授权尚未生效",
}

// HTTPStatus 错误码 → HTTP 状态码。
var HTTPStatus = map[int]int{
	CodeValidationFailed: http.StatusUnprocessableEntity,
	CodeInvalidFormat:    http.StatusUnprocessableEntity,
	CodeAuthFailed:       http.StatusUnauthorized,
	CodeTokenExpired:     http.StatusUnauthorized,
	CodeAccountLocked:    http.StatusLocked,
	CodeUserDisabled:     http.StatusForbidden,
	CodeForbidden:        http.StatusForbidden,
	CodeScopeDenied:      http.StatusForbidden,
	CodeDuplicateURL:     http.StatusConflict,
	CodeTaskStateConflict: http.StatusConflict,
	CodeNotFound:         http.StatusNotFound,
	CodeEvidenceTampered: http.StatusUnprocessableEntity,
	CodeRuleVersionMismatch: http.StatusConflict,
	CodeAssetQuota:       http.StatusTooManyRequests,
	CodeWorkerQuota:      http.StatusTooManyRequests,
	CodeEngineTimeout:    http.StatusRequestTimeout,
	CodeTargetBreakerOpen: http.StatusBadGateway,
	CodeWorkerUnauthorized: http.StatusUnauthorized,
	CodeNotifyFailed:     http.StatusInternalServerError,
	CodeIntelSourceOffline: http.StatusBadGateway,
	CodeLicenseRequired:        http.StatusForbidden,
	CodeLicenseInvalid:         http.StatusForbidden,
	CodeLicenseExpired:         http.StatusForbidden,
	CodeLicenseMachineMismatch: http.StatusForbidden,
	CodeLicenseNotYetActive:    http.StatusForbidden,
}

// APIError 业务错误。
type APIError struct {
	Code    int
	Message string
	Err     error
}

func (e *APIError) Error() string {
	return e.Message
}

// New 构造业务错误。
func New(code int, msg string) *APIError {
	if msg == "" {
		msg = CodeMessage[code]
	}
	return &APIError{Code: code, Message: msg}
}

// Wrap 构造带底层错误的业务错误。
func Wrap(code int, err error) *APIError {
	return &APIError{Code: code, Message: CodeMessage[code], Err: err}
}

// FromError 判断 err 是否为 APIError，不是则包装为通用 500。
func FromError(err error) *APIError {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*APIError); ok {
		return ae
	}
	return &APIError{Code: 500, Message: err.Error(), Err: err}
}

// CodeOf 返回错误对应的业务错误码，非 APIError 返回 -1。
func CodeOf(err error) int {
	if err == nil {
		return CodeOK
	}
	if ae, ok := err.(*APIError); ok {
		return ae.Code
	}
	return -1
}

// StatusOf 返回错误对应的 HTTP 状态码。
func StatusOf(code int) int {
	if s, ok := HTTPStatus[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}
