// Package notify 统一通知出口：站内消息（sys_message）+ App 推送（uniPush 2.0 / 个推 V2）。
// 推送为可选增强：push.Client 未配置（Enabled()=false）时只写站内消息，行为与旧直写口径一致；
// 推送异步执行，失败仅记日志，绝不影响主流程。
package notify

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/push"
)

// Notifier 统一通知出口（db 写站内消息，push 发个推推送；push 为叶子依赖，无循环引用）。
type Notifier struct {
	db   *gorm.DB
	push *push.Client
}

func New(db *gorm.DB, pushCli *push.Client) *Notifier {
	return &Notifier{db: db, push: pushCli}
}

// WithDB 返回使用指定 DB 句柄（事务）的副本：工单创建等事务内通知随事务提交/回滚。
func (n *Notifier) WithDB(db *gorm.DB) *Notifier {
	return &Notifier{db: db, push: n.push}
}

// Send 单发通知：写 sys_message（字段口径与原直写点一致）；推送开启时异步推送到该用户全部已绑设备。
// 返回站内消息写入错误（推送错误不回传）。
func (n *Notifier) Send(userID, msgType, title, content string, bizID *string) error {
	msg := sysmodel.SysMessage{UserID: userID, Type: msgType, Title: title, Content: content, BizID: bizID}
	err := n.db.Create(&msg).Error
	n.pushAsync([]string{userID}, msgType, title, content, bizID)
	return err
}

// SendBatch 群发通知：一次批量写入 sys_message（tenantID 冗余列可空，公告群发传租户 ID，与旧口径一致）；
// 推送按全体接收人已绑设备异步下发。
func (n *Notifier) SendBatch(userIDs []string, tenantID *string, msgType, title, content string, bizID *string) error {
	if len(userIDs) == 0 {
		return nil
	}
	msgs := make([]sysmodel.SysMessage, 0, len(userIDs))
	for _, uid := range userIDs {
		msgs = append(msgs, sysmodel.SysMessage{
			TenantID: tenantID,
			UserID:   uid, Type: msgType, Title: title, Content: content, BizID: bizID,
		})
	}
	err := n.db.Create(&msgs).Error
	n.pushAsync(userIDs, msgType, title, content, bizID)
	return err
}

// pushAsync 异步推送：同步查接收人已绑 cid（事务句柄在提交前查询才有效），HTTP 推送放 goroutine。
func (n *Notifier) pushAsync(userIDs []string, msgType, title, content string, bizID *string) {
	if n.push == nil || !n.push.Enabled() {
		return
	}
	var cids []string
	if err := n.db.Model(&sysmodel.UserPushDevice{}).Where("user_id IN ?", userIDs).Pluck("cid", &cids).Error; err != nil {
		logger.L.Warn("App 推送：查询设备 cid 失败", zap.Error(err))
		return
	}
	if len(cids) == 0 {
		return
	}
	payload := map[string]string{"type": msgType}
	if bizID != nil {
		payload["biz_id"] = *bizID
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := n.push.PushToCIDs(ctx, cids, title, content, payload); err != nil {
			logger.L.Warn("App 推送失败", zap.Error(err), zap.String("type", msgType), zap.Int("cids", len(cids)))
		}
	}()
}

// BindDevice 绑定设备 cid 到用户（upsert：cid 冲突时改绑当前用户，一个 cid 只属于最后登录的人）。
func (n *Notifier) BindDevice(userID, cid, platform string) error {
	d := sysmodel.UserPushDevice{UserID: userID, CID: cid, Platform: platform}
	return n.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cid"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "platform", "updated_at"}),
	}).Create(&d).Error
}

// UnbindDevice 解绑当前用户的 cid（退出登录时调用；cid 属于他人时不误删）。
func (n *Notifier) UnbindDevice(userID, cid string) error {
	return n.db.Where("user_id = ? AND cid = ?", userID, cid).Delete(&sysmodel.UserPushDevice{}).Error
}
