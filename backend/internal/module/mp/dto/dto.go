// Package dto 小程序端请求结构。
package dto

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
	// AbnormalTags 异常 tag 列表（须 ⊆ 该项模板 tags；非空服务端强制该项 pass=false）
	AbnormalTags []string `json:"abnormal_tags" binding:"omitempty,max=20"`
	// Photos 该项照片 file_id（一项一图硬约束：最多 1 张；不合格项与模板 required 项强制恰好 1 张；
	// 台账有效期合成项例外：允许携带 ≤3 张新标签照片，提交时触发服务端维保核验，pass 字段忽略）
	Photos []string `json:"photos" binding:"omitempty,max=3"`
	// AIVerdict/AIReason/AIReading 逐项 AI 识别确认提交（ai_confirmed=true）时带回的结论（均可空）
	AIVerdict     string `json:"ai_verdict"`
	AIReason      string `json:"ai_reason"`
	AIReading     string `json:"ai_reading"`
	ExceptionType string `json:"exception_type"`
	// Disposition 异常项处置方式（仅 !pass 项可填）：on_site_resolved=现场已处理（须带处置照片 ≥1 张）/
	// maintenance_registered=登记维保 / report_pending=上报待处理（记录强制转人工审核并通知审核人）
	Disposition string `json:"disposition"`
	// ResolutionFileIDs 处置照片 file_id（归属校验同 photos 口径）
	ResolutionFileIDs []string `json:"resolution_file_ids" binding:"omitempty,max=9"`
	ResolutionNote    string   `json:"resolution_note"`
	// 标签抽查合成项（judge_type=equipment_date_spot）：只交照片（+逃生 exception_type=label_missing）；
	// 日期由服务端从该项 AI 读标签草稿的 ai_reading 解析（M{生产年月}|W{维修年月}），不与台账比对则以实物为准；
	// 客户端 pass 被忽略，服务端按四规则与台账比对
}

// CheckinReq 打卡提交（离线补传单条结构相同）。
// ID 为可选的客户端生成 UUIDv7：离线场景下小程序本地暂存时即生成，补传/重试携带同一 ID，
// 服务端发现该 ID 已存在则直接幂等返回已有记录，不产生重复数据（UUIDv7 的核心收益）。
// 照片全部归属逐项（check_items[].photos），无记录级照片；"现场全貌"类需求用通用模板检查项表达。
type CheckinReq struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id" binding:"required"`
	PointID     string `json:"point_id" binding:"required"`
	CheckinType string `json:"checkin_type" binding:"required,oneof=qrcode fence nfc"`
	QRCodeNo    string `json:"qrcode_no"`
	NFCID       string `json:"nfc_id"`
	// Longitude/Latitude 手机定位（0,0=定位失败/未授权）：坐标降级为可选机制，
	// 仅围栏点位（require_fence）强制要求有效定位，校验在 checkMode 内按点位配置判定
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	// Altitude/Accuracy 定位辅助信息（米，可空，仅参考展示不参与校验）；<=0 视为未提供
	Altitude   float64 `json:"altitude"`
	Accuracy   float64 `json:"accuracy"`
	ClientTime string  `json:"client_time" binding:"required"`
	Result     string  `json:"result" binding:"required,oneof=normal abnormal"`
	Remark     string  `json:"remark"`
	// Force 重拍次数用尽后的强制提交：跳过同步 AI 判定直接落库，转人工复核
	Force bool `json:"force"`
	// AIConfirmed 逐项 AI 识别确认提交：采纳 check_items 逐项带回的 AI 结论，跳过服务端同步 AI 判定
	AIConfirmed bool             `json:"ai_confirmed"`
	CheckItems  []CheckinItemReq `json:"check_items"`
}

// AIItemJobReq 逐项 AI 识别任务提交（单个检查项恰好 1 张照片，异步识别后轮询取结果）。
type AIItemJobReq struct {
	TaskID  string   `json:"task_id" binding:"required"`
	PointID string   `json:"point_id" binding:"required"`
	Name    string   `json:"name" binding:"required"` // 检查项名（须属于该点位模板项）
	FileIDs []string `json:"file_ids" binding:"required,len=1"`
}

// ManualItemDraftReq 手动结论落云端草稿：选择即保存，断点恢复以服务端为准。
// 感官项与手动档向导的拍照项通用；拍照项携 file_ids（照片证据）与 abnormal_tags（异常观察点）。
type ManualItemDraftReq struct {
	TaskID  string `json:"task_id" binding:"required"`
	PointID string `json:"point_id" binding:"required"`
	Name    string `json:"name" binding:"required"` // 检查项名（须为该点位模板项）
	Pass    bool   `json:"pass"`
	Note    string `json:"note"`
	// FileIDs 手动档拍照项的照片（≤3，归属校验同逐项照片）；感官项为空
	FileIDs []string `json:"file_ids" binding:"omitempty,max=3"`
	// AbnormalTags 异常观察点 tag（⊆ 模板项 tags）
	AbnormalTags []string `json:"abnormal_tags" binding:"omitempty,max=20"`
}

// PhotoItemAbnormalDraftReq 拍照项异常逃生入口：设备不存在/无法拍摄时，拍照佐证后直接落异常草稿。
type PhotoItemAbnormalDraftReq struct {
	TaskID        string   `json:"task_id" binding:"required"`
	PointID       string   `json:"point_id" binding:"required"`
	Name          string   `json:"name" binding:"required"` // 检查项名（须为该点位模板的拍照项）
	FileIDs       []string `json:"file_ids" binding:"required,len=1"`
	Note          string   `json:"note"`
	ExceptionType string   `json:"exception_type" binding:"required,oneof=device_missing unable_to_capture"`
}

type OfflineSyncReq struct {
	Items []CheckinReq `json:"items" binding:"required,min=1"`
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
