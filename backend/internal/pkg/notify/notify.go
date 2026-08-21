// Package notify 统一通知出口：站内消息（sys_message）+ App 推送（uniPush 2.0 / 个推 V2）。
// 推送为可选增强：push.Client 未配置（Enabled()=false）时只写站内消息，行为与旧直写口径一致；
// 推送异步执行，失败仅记日志，绝不影响主流程。
package notify

import (
	"context"
	"encoding/json"
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

// pushAsync 异步推送：同步查接收人已绑 cid 与各用户未读数（事务句柄在提交前查询才有效），HTTP 推送放 goroutine。
// 推送按用户分组逐用户下发：badge=该用户最新未读数（站内消息已先于推送写入，事务内计数即最新），
// 供 iOS 服务端角标（push_channel.ios.auto_badge）使用；Android 角标由端内 setBadgeNumber 尽力同步。
func (n *Notifier) pushAsync(userIDs []string, msgType, title, content string, bizID *string) {
	if n.push == nil || !n.push.Enabled() {
		return
	}
	var devices []sysmodel.UserPushDevice
	if err := n.db.Model(&sysmodel.UserPushDevice{}).Where("user_id IN ?", userIDs).Find(&devices).Error; err != nil {
		logger.L.Warn("App 推送：查询设备 cid 失败", zap.Error(err))
		return
	}
	if len(devices) == 0 {
		// 常见原因：App 未重新打包/未登录绑定 cid——静默返回难以排查，记 Info 留痕
		logger.L.Info("App 推送跳过：接收人无已绑设备", zap.String("type", msgType), zap.Int("recipients", len(userIDs)))
		return
	}
	cidByUser := map[string][]string{}
	cidCount := 0
	for _, d := range devices {
		cidByUser[d.UserID] = append(cidByUser[d.UserID], d.CID)
		cidCount++
	}
	// 一次查完全部接收人未读数（sys_message user_id + is_read=false 口径，与消息中心一致）
	unreadByUser := map[string]int{}
	var unreadRows []struct {
		UserID string `gorm:"column:user_id"`
		Cnt    int    `gorm:"column:cnt"`
	}
	if err := n.db.Model(&sysmodel.SysMessage{}).
		Select("user_id, count(*) AS cnt").
		Where("user_id IN ? AND is_read = ?", userIDs, false).
		Group("user_id").Scan(&unreadRows).Error; err != nil {
		logger.L.Warn("App 推送：查询未读数失败，角标按 0 下发", zap.Error(err))
	} else {
		for _, r := range unreadRows {
			unreadByUser[r.UserID] = r.Cnt
		}
	}
	payload := map[string]string{"type": msgType}
	if bizID != nil {
		payload["biz_id"] = *bizID
	}
	go func() {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var ok, failed int
		var firstErr error
		for uid, cids := range cidByUser {
			uok, ufailed, err := n.push.PushToCIDs(ctx, cids, title, content, payload, unreadByUser[uid])
			ok += uok
			failed += ufailed
			if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			logger.L.Warn("App 推送失败", zap.Error(firstErr), zap.String("type", msgType), zap.Int("ok", ok), zap.Int("failed", failed))
		}
		n.logPush(userIDs, msgType, title, bizID, cidCount, ok, failed, int(time.Since(start).Milliseconds()), firstErr)
	}()
}

// logPush 推送结果落操作日志（module=push/action=push_send，系统操作员）：按接收人所属租户分组各写一行，
// 管理端「日志管理 → 操作日志」按租户上下文即可查看推送成败与原因；写日志失败仅记 Warn 不回传。
// status 口径：有任一设备送达即 success（部分失败在 params.ok/failed 体现），全部失败才 fail。
func (n *Notifier) logPush(userIDs []string, msgType, title string, bizID *string, cidCount, okCount, failCount, costMs int, pushErr error) {
	// 接收人 → 租户分组（租户上下文过滤口径与操作日志查询一致）
	var users []struct {
		ID       string  `gorm:"column:id"`
		TenantID *string `gorm:"column:tenant_id"`
	}
	if err := n.db.Table("sys_user").Select("id", "tenant_id").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		logger.L.Warn("App 推送日志：查询接收人租户失败", zap.Error(err))
		return
	}
	type group struct {
		tenantID *string
		count    int
	}
	groups := map[string]*group{}
	for _, u := range users {
		key := ""
		if u.TenantID != nil {
			key = *u.TenantID
		}
		if groups[key] == nil {
			groups[key] = &group{tenantID: u.TenantID}
		}
		groups[key].count++
	}
	status := "success"
	errText := ""
	if okCount == 0 && pushErr != nil {
		status = "fail"
	}
	if pushErr != nil {
		errText = pushErr.Error()
		if len(errText) > 500 {
			errText = errText[:500]
		}
	}
	for _, g := range groups {
		params, _ := json.Marshal(map[string]any{
			"type": msgType, "biz_id": bizID, "title": title,
			"recipients": g.count, "cids": cidCount, "ok": okCount, "failed": failCount, "error": errText,
		})
		row := sysmodel.SysOperationLog{
			TenantID: g.tenantID,
			Username: "system",
			Module:   "push",
			Action:   "push_send",
			Method:   "PUSH",
			Path:     "uniPush /push/single/cid",
			Params:   string(params),
			Status:   status,
			CostMs:   costMs,
		}
		if err := n.db.Create(&row).Error; err != nil {
			logger.L.Warn("App 推送日志写入失败", zap.Error(err))
		}
	}
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
