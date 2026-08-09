package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
)

// MessageService 管理端站内消息服务（顶栏铃铛；仅本人消息）。
type MessageService struct {
	db *gorm.DB
}

func NewMessageService(db *gorm.DB) *MessageService { return &MessageService{db: db} }

// MessageListQuery 消息列表查询（is_read 空串=全部）。
type MessageListQuery struct {
	response.PageQuery
	IsRead string `form:"is_read"`
}

// List 当前用户消息列表 + 未读数。
func (s *MessageService) List(userID string, q *MessageListQuery) (gin.H, *errs.Error) {
	db := s.db.Model(&model.SysMessage{}).Where("user_id = ?", userID)
	if v, ok, be := bind.BoolFilter(q.IsRead); be != nil {
		return nil, be
	} else if ok {
		db = db.Where("is_read = ?", v)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var unread int64
	s.db.Model(&model.SysMessage{}).Where("user_id = ? AND is_read = false", userID).Count(&unread)
	var rows []model.SysMessage
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, m := range rows {
		list = append(list, gin.H{
			"id": m.ID, "title": m.Title, "content": m.Content, "type": m.Type,
			"biz_id": m.BizID, "is_read": m.IsRead, "created_at": timefmt.T(m.CreatedAt),
		})
	}
	return gin.H{"list": list, "total": total, "unread_count": unread}, nil
}

// MarkRead 标记已读（仅本人消息）；id 传 "0" 表示全部已读。
func (s *MessageService) MarkRead(userID, id string) *errs.Error {
	if id == "0" {
		if err := s.db.Model(&model.SysMessage{}).
			Where("user_id = ? AND is_read = false", userID).Update("is_read", true).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	// WHERE user_id 限定：他人消息不可操作（RowsAffected=0 视为不存在）
	res := s.db.Model(&model.SysMessage{}).Where("id = ? AND user_id = ?", id, userID).Update("is_read", true)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}
