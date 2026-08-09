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

// CheckinPhotoRef 打卡照片引用。
type CheckinPhotoRef struct {
	Item    string `json:"item"`
	FileKey string `json:"file_key" binding:"required"`
}

// CheckinItemReq 打卡逐项检查结果提交项。
type CheckinItemReq struct {
	Name string `json:"name" binding:"required"`
	Pass bool   `json:"pass"`
	Note string `json:"note"`
}

// CheckinReq 打卡提交（离线补传单条结构相同）。
// ID 为可选的客户端生成 UUIDv7：离线场景下小程序本地暂存时即生成，补传/重试携带同一 ID，
// 服务端发现该 ID 已存在则直接幂等返回已有记录，不产生重复数据（UUIDv7 的核心收益）。
type CheckinReq struct {
	ID          string            `json:"id"`
	TaskID      string            `json:"task_id" binding:"required"`
	PointID     string            `json:"point_id" binding:"required"`
	CheckinType string            `json:"checkin_type" binding:"required,oneof=qrcode fence nfc"`
	QRCodeNo    string            `json:"qrcode_no"`
	NFCID       string            `json:"nfc_id"`
	Longitude   float64           `json:"longitude" binding:"required"`
	Latitude    float64           `json:"latitude" binding:"required"`
	ClientTime  string            `json:"client_time" binding:"required"`
	Result      string            `json:"result" binding:"required,oneof=normal abnormal"`
	Remark      string            `json:"remark"`
	CheckItems  []CheckinItemReq  `json:"check_items"`
	Photos      []CheckinPhotoRef `json:"photos" binding:"required,min=1"`
}

type OfflineSyncReq struct {
	Items []CheckinReq `json:"items" binding:"required,min=1"`
}

type STSReq struct {
	Scene string `json:"scene" binding:"required,oneof=checkin workorder avatar"`
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
