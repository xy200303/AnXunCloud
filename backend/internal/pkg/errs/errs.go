// Package errs 定义统一业务错误码（与接口文档 §1.5 保持一致）。
package errs

import "net/http"

// Error 业务错误，code 为文档约定的业务码，HTTP 为配套状态码。
type Error struct {
	Code int    `json:"code"`
	HTTP int    `json:"-"`
	Msg  string `json:"message"`
	// Data 附加数据（如 40109 需公司选择时下发候选租户列表），可空
	Data any `json:"-"`
}

func (e *Error) Error() string { return e.Msg }

// New 创建业务错误。
func New(code, httpStatus int, msg string) *Error {
	return &Error{Code: code, HTTP: httpStatus, Msg: msg}
}

// WithMsg 基于已有错误码替换提示文案（用于携带具体字段原因）。
func (e *Error) WithMsg(msg string) *Error {
	return &Error{Code: e.Code, HTTP: e.HTTP, Msg: msg, Data: e.Data}
}

// WithData 基于已有错误码附加数据（如 40109 的候选公司列表）。
func (e *Error) WithData(data any) *Error {
	return &Error{Code: e.Code, HTTP: e.HTTP, Msg: e.Msg, Data: data}
}

// 通用错误码
var (
	ErrParam      = New(40001, http.StatusBadRequest, "请求参数错误")
	ErrBodyFormat = New(40002, http.StatusBadRequest, "请求体格式错误")
	ErrNotFound   = New(40400, http.StatusNotFound, "资源不存在或已删除")
	ErrConflict   = New(40900, http.StatusConflict, "数据状态冲突")
	ErrTooMany    = New(42900, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
	ErrInternal   = New(50000, http.StatusInternalServerError, "服务器内部错误")
)

// 401xx 认证与会话
var (
	ErrUnauthorized    = New(40101, http.StatusUnauthorized, "未登录或 token 无效")
	ErrTokenExpired    = New(40102, http.StatusUnauthorized, "access token 已过期")
	ErrRefreshInvalid  = New(40103, http.StatusUnauthorized, "refresh token 已过期或无效，请重新登录")
	ErrAccountDisabled = New(40104, http.StatusUnauthorized, "账号已停用")
	ErrBadCredentials  = New(40105, http.StatusUnauthorized, "用户名或密码错误")
)

// 403xx 权限
var (
	ErrNoPerm           = New(40301, http.StatusForbidden, "无该接口/按钮权限")
	ErrDataScope        = New(40302, http.StatusForbidden, "数据权限不足（超出所辖小区范围）")
	ErrRegisterDisabled = New(40303, http.StatusForbidden, "注册功能未开放")
	ErrNotInSlot        = New(40304, http.StatusForbidden, "当前用户不在该环节授权名单内")
)

// 410xx 系统管理
var (
	ErrUsernameExists    = New(41001, http.StatusConflict, "用户名已存在")
	ErrRoleCodeExists    = New(41002, http.StatusConflict, "角色编码已存在")
	ErrRoleHasUsers      = New(41003, http.StatusConflict, "角色下存在用户，不可删除")
	ErrDictCodeExists    = New(41004, http.StatusConflict, "字典编码已存在")
	ErrConfigKeyExists   = New(41005, http.StatusConflict, "参数 key 已存在")
	ErrSelfOperation     = New(41006, http.StatusConflict, "不能停用/删除当前登录账号")
	ErrBuiltin           = New(41007, http.StatusConflict, "内置角色/菜单不可删除")
	ErrPhoneExists       = New(41008, http.StatusConflict, "手机号已存在")
	ErrRoleNotExist      = New(41009, http.StatusBadRequest, "角色不存在")
	ErrCommunityNotExist = New(41010, http.StatusBadRequest, "小区不存在")
	ErrImportFileType    = New(41011, http.StatusBadRequest, "导入文件格式错误（仅支持 .xlsx）")
	ErrImportEmpty       = New(41012, http.StatusBadRequest, "导入文件为空或无有效数据行")
	ErrImportTooMany     = New(41013, http.StatusBadRequest, "超过单次导入上限（500 行）")
	ErrBuiltinAccount    = New(41014, http.StatusConflict, "内置账号禁止该操作")
	ErrTenantCodeExists  = New(41015, http.StatusConflict, "租户代码已存在")
)

// 42xxx 小区与楼栋
var (
	ErrCommunityNameExists  = New(42001, http.StatusConflict, "小区名称已存在")
	ErrCommunityHasChildren = New(42002, http.StatusConflict, "小区下存在楼栋或点位，不可删除")
	ErrBuildingHasPoints    = New(42003, http.StatusConflict, "楼栋下存在点位，不可删除")
)

// 43xxx 点位/计划/任务
var (
	ErrQRCodeExists     = New(43001, http.StatusConflict, "二维码编号已存在")
	ErrPointReferenced  = New(43002, http.StatusConflict, "点位已被巡检计划引用，不可删除")
	ErrPlanCycleInvalid = New(43003, http.StatusBadRequest, "计划周期配置非法")
	ErrPlanDateInvalid  = New(43004, http.StatusBadRequest, "计划日期范围非法")
)

// 431xx 打卡（小程序）
var (
	ErrQRCodeMismatch   = New(43101, http.StatusBadRequest, "二维码码值与点位不匹配")
	ErrOutOfFence       = New(43102, http.StatusBadRequest, "超出围栏范围，禁止提交")
	ErrDuplicateCheckin = New(43103, http.StatusConflict, "该点位已打卡，请勿重复提交")
	ErrPhotoMissing     = New(43104, http.StatusBadRequest, "必拍项照片缺失")
	ErrTaskNotOwned     = New(43105, http.StatusForbidden, "任务不存在或不属于当前巡检员")
	ErrPhotoNotUploaded = New(43106, http.StatusBadRequest, "照片尚未上传完成，请稍后重试")
	ErrPhotoQuality     = New(43107, http.StatusBadRequest, "照片质量不达标，请重新拍摄")
	ErrAIDisabled       = New(43108, http.StatusBadRequest, "AI 识别未启用，请切换手动打卡模式")
	ErrCheckinLocked    = New(43109, http.StatusConflict, "该点位已归档报告，不可修改")
)

// 44xxx 工单
var (
	ErrOrderStatusNotAllowed = New(44001, http.StatusConflict, "工单当前状态不允许该操作")
	ErrReviewRemarkRequired  = New(44002, http.StatusBadRequest, "复核驳回必须填写驳回原因")
	ErrAssigneeInvalid       = New(44003, http.StatusBadRequest, "被指派人不存在或已停用")
	ErrOrderNotInSlot        = New(44004, http.StatusForbidden, "当前用户不在该环节授权名单内")
	ErrOrderGrabDisabled     = New(44005, http.StatusConflict, "该项目未开启抢单模式")
	ErrTriageNoteRequired    = New(44006, http.StatusBadRequest, "受理驳回必须填写驳回原因")
	ErrConfirmNoteRequired   = New(44007, http.StatusBadRequest, "验收不通过必须填写退回原因")
)

// 45xxx 统计与导出
var (
	ErrExportNotFound = New(45001, http.StatusNotFound, "导出任务不存在")
	ErrExportExpired  = New(45002, http.StatusGone, "导出文件已过期")
)

// 47xxx 月度报告
var (
	ErrReportStatusNotAllowed     = New(47001, http.StatusConflict, "报告当前状态不允许该操作")
	ErrReportApproved             = New(47002, http.StatusConflict, "已终审归档的报告不可重算")
	ErrReportNotInspector         = New(47003, http.StatusForbidden, "当前用户不在应确认巡检员名单内")
	ErrReportAlreadySigned        = New(47004, http.StatusConflict, "当前用户已确认过该报告")
	ErrReportRejectReasonRequired = New(47005, http.StatusBadRequest, "驳回必须填写驳回原因")
	ErrReportNotSigner            = New(47007, http.StatusForbidden, "当前用户不在该级指定签字人名单内")
	ErrSignatureMissing           = New(47008, http.StatusConflict, "未配置手写签名，请先在个人中心设置手写签名后再签字")
)

// 46xxx 上传与 OSS
var (
	ErrOSSCallbackAuth = New(46001, http.StatusUnauthorized, "OSS 回调验签失败")
	ErrSTSFailed       = New(46002, http.StatusInternalServerError, "STS 凭证签发失败")
	ErrUploadType      = New(46003, http.StatusBadRequest, "上传文件类型不支持（仅 jpg/jpeg/png/heic）")
	ErrUploadTooLarge  = New(46004, http.StatusBadRequest, "文件大小超限（单张 ≤ 20MB）")
)

// 401xx 小程序补充
var (
	ErrWxCodeInvalid = New(40106, http.StatusUnauthorized, "微信 code 无效或已过期")
	ErrWxUnbound     = New(40107, http.StatusUnauthorized, "微信账号未绑定系统账号，请联系管理员开通")
)

// 401xx 多租户（P3）
var (
	ErrTenantDisabled     = New(40108, http.StatusUnauthorized, "租户已停用，请联系平台方")
	ErrTenantCodeRequired = New(40109, http.StatusUnauthorized, "该用户名存在于多家公司，请填写公司代码")
)
