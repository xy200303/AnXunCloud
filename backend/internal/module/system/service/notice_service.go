package service

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"

	"github.com/gin-gonic/gin"
)

// NoticeService 通知公告服务。
type NoticeService struct {
	db *gorm.DB
}

func NewNoticeService(db *gorm.DB) *NoticeService { return &NoticeService{db: db} }

// NoticeListQuery 公告列表查询。
type NoticeListQuery struct {
	response.PageQuery
	Title  string `form:"title"`
	Status *int   `form:"status"`
}

// NoticeSaveReq 公告新增/修改请求（status：0 草稿 / 1 发布 / 2 下线）。
type NoticeSaveReq struct {
	Title   string `json:"title" binding:"required,max=64"`
	Content string `json:"content" binding:"required"`
	Status  *int   `json:"status" binding:"omitempty,min=0,max=2"`
}

// List 公告分页列表。
func (s *NoticeService) List(q *NoticeListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysNotice{})
	if q.Title != "" {
		db = db.Where("title LIKE ?", "%"+q.Title+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysNotice
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		list = append(list, noticeItem(&r))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func noticeItem(n *model.SysNotice) gin.H {
	return gin.H{
		"id": n.ID, "title": n.Title, "content": n.Content, "status": n.Status,
		"publish_at": timefmt.TP(n.PublishAt), "created_by": n.CreatedByName,
		"created_at": timefmt.T(n.CreatedAt),
	}
}

// Create 新增公告；status=1 立即发布。
func (s *NoticeService) Create(req *NoticeSaveReq, operatorID string, operatorName string) (string, *errs.Error) {
	status := 0
	if req.Status != nil {
		status = *req.Status
	}
	n := model.SysNotice{
		Title: req.Title, Content: req.Content, Status: status,
		CreatedBy: &operatorID, CreatedByName: operatorName,
	}
	if status == 1 {
		now := time.Now()
		n.PublishAt = &now
	}
	if err := s.db.Create(&n).Error; err != nil {
		return "", errs.ErrInternal
	}
	return n.ID, nil
}

// Update 修改公告（含发布/下线；发布时间仅在首次发布时写入）。
func (s *NoticeService) Update(id string, req *NoticeSaveReq) *errs.Error {
	var n model.SysNotice
	if err := s.db.First(&n, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	updates := map[string]any{"title": req.Title, "content": req.Content}
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == 1 && n.PublishAt == nil {
			updates["publish_at"] = time.Now()
		}
	}
	if err := s.db.Model(&n).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 删除公告。
func (s *NoticeService) Delete(id string) *errs.Error {
	res := s.db.Delete(&model.SysNotice{}, "id = ?", id)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// Published 小程序端：已发布公告分页。
func (s *NoticeService) Published(page, pageSize int) (*response.Page, *errs.Error) {
	q := &NoticeListQuery{}
	q.Page, q.PageSize = page, pageSize
	status := 1
	q.Status = &status
	db := s.db.Model(&model.SysNotice{}).Where("status = 1")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysNotice
	offset, limit := q.Normalize()
	if err := db.Order("publish_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		list = append(list, gin.H{
			"id": r.ID, "title": r.Title, "content": r.Content, "publish_at": timefmt.TP(r.PublishAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
