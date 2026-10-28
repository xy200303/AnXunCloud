package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/types"
)

type NotificationArgs struct {
	JobID   string   `json:"job_id" river:"unique"`
	UserIDs []string `json:"user_ids"`
	Type    string   `json:"type"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	BizID   *string  `json:"biz_id"`
}

func (NotificationArgs) Kind() string { return "notify.app.push" }

func (NotificationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "notify", MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true, ByQueue: true}}
}

func NewNotificationWorker(n *Notifier) river.Worker[NotificationArgs] {
	return river.WorkFunc(func(ctx context.Context, job *river.Job[NotificationArgs]) error {
		return n.pushAsync(job.Args.UserIDs, job.Args.Type, job.Args.Title, job.Args.Content, job.Args.BizID)
	})
}

func (n *Notifier) BindQueue(queue *queuepkg.Queue) { n.queue = queue }

func (n *Notifier) enqueue(ctx context.Context, userIDs []string, tenantID *string, msgType, title, content string, bizID *string) error {
	if n.queue == nil {
		return fmt.Errorf("通知任务队列未配置")
	}
	tx, err := n.queue.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO sys_message (id, tenant_id, user_id, type, title, content, biz_id, is_read, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,false,$8)`, types.NewID(), tenantID, userID, msgType, title, content, bizID, time.Now()); err != nil {
			return err
		}
	}
	args := NotificationArgs{JobID: types.NewID(), UserIDs: userIDs, Type: msgType, Title: title, Content: content, BizID: bizID}
	if _, err := n.queue.InsertTx(ctx, tx, args, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
