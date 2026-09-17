// Package dto 统一文件层请求结构。
package dto

// STSReq 直传凭证签发请求（scene 限定打卡/头像，其余场景走 multipart 本地上传）。
type STSReq struct {
	Scene string `json:"scene" binding:"required,oneof=checkin avatar"`
	Files []struct {
		Name string `json:"name" binding:"required"`
		Size int64  `json:"size"`
		MD5  string `json:"md5"`
	} `json:"files" binding:"required,min=1,max=6"`
}
