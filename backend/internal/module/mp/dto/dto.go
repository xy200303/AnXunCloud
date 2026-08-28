// Package dto 小程序端请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type MPLoginReq struct {
	Code      string `json:"code" binding:"required"`
	PhoneCode string `json:"phone_code"`
}

type MPRefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// CheckinItemReq 打卡逐项检查结果提交项。
type CheckinItemReq struct {
	Name string `json:"name" binding:"required"`
	Pass bool   `json:"pass"`
	Note string `json:"note"`
	// Photos 该项照片 file_key（一项一图硬约束：最多 1 张；不合格项与模板 required 项强制恰好 1 张）
	Photos []string `json:"photos" binding:"omitempty,max=1"`
	// AIVerdict/AIReason/AIReading 逐项 AI 识别确认提交（ai_confirmed=true）时带回的结论（均可空）
	AIVerdict string `json:"ai_verdict"`
	AIReason  string `json:"ai_reason"`
	AIReading string `json:"ai_reading"`
}

// CheckinReq 打卡提交（离线补传单条结构相同）。
// ID 为可选的客户端生成 UUIDv7：离线场景下小程序本地暂存时即生成，补传/重试携带同一 ID，
// 服务端发现该 ID 已存在则直接幂等返回已有记录，不产生重复数据（UUIDv7 的核心收益）。
// 照片全部归属逐项（check_items[].photos），无记录级照片；"现场全貌"类需求用通用模板检查项表达。
type CheckinReq struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id" binding:"required"`
	PointID     string  `json:"point_id" binding:"required"`
	CheckinType string  `json:"checkin_type" binding:"required,oneof=qrcode fence nfc"`
	QRCodeNo    string  `json:"qrcode_no"`
	NFCID       string  `json:"nfc_id"`
	Longitude   float64 `json:"longitude" binding:"required"`
	Latitude    float64 `json:"latitude" binding:"required"`
	ClientTime  string  `json:"client_time" binding:"required"`
	Result      string  `json:"result" binding:"required,oneof=normal abnormal"`
	Remark      string  `json:"remark"`
	// Force 重拍次数用尽后的强制提交：跳过同步 AI 判定直接落库，转人工复核
	Force bool `json:"force"`
	// AIConfirmed 逐项 AI 识别确认提交：采纳 check_items 逐项带回的 AI 结论，跳过服务端同步 AI 判定
	AIConfirmed bool             `json:"ai_confirmed"`
	CheckItems  []CheckinItemReq `json:"check_items"`
}

// AIItemJobReq 逐项 AI 识别任务提交（单个检查项恰好 1 张照片，异步识别后轮询取结果）。
type AIItemJobReq struct {
	TaskID   string   `json:"task_id" binding:"required"`
	PointID  string   `json:"point_id" binding:"required"`
	Name     string   `json:"name" binding:"required"`                 // 检查项名（须属于该点位模板项）
	FileKeys []string `json:"file_keys" binding:"required,len=1"`      // 该项照片 file_key（一项一图硬约束）
}

// ManualItemDraftReq 手动确认项（感官项）选择落云端草稿：选择即保存，断点恢复以服务端为准。
type ManualItemDraftReq struct {
	TaskID  string `json:"task_id" binding:"required"`
	PointID string `json:"point_id" binding:"required"`
	Name    string `json:"name" binding:"required"` // 检查项名（须为该点位模板的手动项）
	Pass    bool   `json:"pass"`
	Note    string `json:"note"`
}

type OfflineSyncReq struct {
	Items []CheckinReq `json:"items" binding:"required,min=1"`
}

type STSReq struct {
	Scene string `json:"scene" binding:"required,oneof=checkin avatar"`
	Files []struct {
		Name string `json:"name" binding:"required"`
		Size int64  `json:"size"`
	} `json:"files" binding:"required,min=1,max=6"`
}

type MyOrdersQuery struct {
	response.PageQuery
	Type   string `form:"type"`
	Status string `form:"status"`
}

type MessageQuery struct {
	response.PageQuery
	Type   string `form:"type"`
	IsRead string `form:"is_read"`
}

// PushDeviceBindReq 推送设备绑定（cid 为个推 SDK 客户端标识；platform 可选 android/ios）。
type PushDeviceBindReq struct {
	CID      string `json:"cid" binding:"required,max=128"`
	Platform string `json:"platform" binding:"omitempty,oneof=android ios"`
}

// PushDeviceUnbindReq 推送设备解绑（DELETE 支持 body 或 query 带 cid）。
type PushDeviceUnbindReq struct {
	CID string `json:"cid" form:"cid" binding:"required,max=128"`
}
