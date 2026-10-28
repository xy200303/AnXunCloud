package service

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/session"
)

// ChannelMP 小程序端标记（仅用于登录日志/审计的客户端类型标识）。
// 会话通道自 v21 起与 App 合并为 ChannelApp：app/mp 共用一套会话体系，token 两端正通用。
const ChannelMP = "mp"

// MPService 小程序端服务（登录/任务/消息/公告）。
type MPService struct {
	db     *gorm.DB
	rdb    *redis.Client
	sess   *session.Store
	jwtm   *jwtutil.Manager
	wechat config.WechatConfig
	httpc  *http.Client
	msg    *systemsvc.MessageService
}

func NewMPService(db *gorm.DB, rdb *redis.Client, sess *session.Store, jwtm *jwtutil.Manager, wechat config.WechatConfig, msg *systemsvc.MessageService) *MPService {
	return &MPService{db: db, rdb: rdb, sess: sess, jwtm: jwtm, wechat: wechat, httpc: &http.Client{Timeout: 8 * time.Second}, msg: msg}
}

// ========== 消息 ==========

// Messages 消息列表 + 未读数。查询/未读口径收敛至 system 侧 MessageService.List，
// 此处仅保留 App 端响应差异（多回传 page/page_size）。
// 行为对齐说明：is_read 非法值统一返回 400（以 system 版为准）——旧 mp 版静默忽略
// 并非有意容错（无注释/契约说明，且 type 过滤之外两版逐行一致属漂移）；非法值仅可能
// 来自异常客户端，正常 App 调用不受影响。
func (s *MPService) Messages(userID string, q *systemsvc.MessageListQuery) (gin.H, *errs.Error) {
	res, be := s.msg.List(userID, q)
	if be != nil {
		return nil, be
	}
	res["page"] = q.Page
	res["page_size"] = q.PageSize
	return res, nil
}

// MarkRead 标记已读；id=0 全部已读。实现收敛至 system 侧 MessageService.MarkRead。
// （旧 mp 版 id="" 也按全读处理，但控制器 pathID 已拒绝空 id，该分支为死代码，随收敛移除。）
func (s *MPService) MarkRead(userID, id string) *errs.Error {
	return s.msg.MarkRead(userID, id)
}
