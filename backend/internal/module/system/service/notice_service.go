package service

import (
	"strconv"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"

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
	Status string `form:"status"`
}

// NoticeAttachment 公告附件元素（name/url 均必填，url 为上传接口返回地址，原样存储）。
type NoticeAttachment struct {
	Name string `json:"name" binding:"required,max=255"`
	URL  string `json:"url" binding:"required,max=512"`
}

// NoticeSaveReq 公告新增/修改请求（status：0 草稿 / 1 发布 / 2 下线）。
type NoticeSaveReq struct {
	Title       string             `json:"title" binding:"required,max=64"`
	Content     string             `json:"content" binding:"required"`
	Status      *int               `json:"status" binding:"omitempty,min=0,max=2"`
	Attachments []NoticeAttachment `json:"attachments" binding:"omitempty,dive"`
}

// toAttachmentArray 请求附件转存储类型（nil → 空数组，保证落库 [] 而非 NULL）。
func toAttachmentArray(list []NoticeAttachment) types.AttachmentArray {
	out := make(types.AttachmentArray, 0, len(list))
	for _, a := range list {
		out = append(out, types.AttachmentItem{Name: a.Name, URL: a.URL})
	}
	return out
}

// List 公告分页列表。
func (s *NoticeService) List(q *NoticeListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysNotice{})
	if q.Title != "" {
		db = db.Where("title LIKE ?", "%"+q.Title+"%")
	}
	if q.Status != "" {
		st, err := strconv.Atoi(q.Status)
		if err != nil || st < 0 || st > 2 {
			return nil, errs.ErrParam.WithMsg("status 取值应为 0/1/2")
		}
		db = db.Where("status = ?", st)
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
	atts := n.Attachments
	if atts == nil {
		atts = types.AttachmentArray{}
	}
	return gin.H{
		"id": n.ID, "title": n.Title, "content": n.Content, "status": n.Status,
		"attachments": atts,
		"publish_at":  timefmt.TP(n.PublishAt), "created_by": n.CreatedByName,
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
		Attachments: toAttachmentArray(req.Attachments),
		CreatedBy:   &operatorID, CreatedByName: operatorName,
	}
	if status == 1 {
		now := time.Now()
		n.PublishAt = &now
	}
	if err := s.db.Create(&n).Error; err != nil {
		return "", errs.ErrInternal
	}
	if status == 1 {
		s.broadcast(&n)
	}
	return n.ID, nil
}

// broadcast 公告发布时给全体启用用户写站内消息（type=announcement），
// 移动端消息列表可见并计入未读徽章；content 截断至 500 字符（sys_message.content 上限 512）。
func (s *NoticeService) broadcast(n *model.SysNotice) {
	var ids []string
	if err := s.db.Model(&model.SysUser{}).Where("status = ?", model.StatusEnabled).Pluck("id", &ids).Error; err != nil {
		return
	}
	if len(ids) == 0 {
		return
	}
	content := []rune(n.Content)
	if len(content) > 500 {
		content = append(content[:500], []rune("……（全文见公告）")...)
	}
	msgs := make([]model.SysMessage, 0, len(ids))
	for _, uid := range ids {
		msgs = append(msgs, model.SysMessage{
			UserID: uid, Type: "announcement",
			Title: "公告：" + n.Title, Content: string(content), BizID: &n.ID,
		})
	}
	s.db.Create(&msgs)
}

// Update 修改公告（含发布/下线；发布时间仅在首次发布时写入）。
func (s *NoticeService) Update(id string, req *NoticeSaveReq) *errs.Error {
	var n model.SysNotice
	if err := s.db.First(&n, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	updates := map[string]any{"title": req.Title, "content": req.Content}
	// attachments 仅在请求显式携带时更新（nil=未传，保留原值；[]=清空）
	if req.Attachments != nil {
		updates["attachments"] = toAttachmentArray(req.Attachments)
	}
	// 由非发布态转为发布态（首次发布/下线后重新发布）时再次广播站内消息
	republish := false
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == 1 && n.PublishAt == nil {
			updates["publish_at"] = time.Now()
		}
		republish = *req.Status == 1 && n.Status != 1
	}
	if err := s.db.Model(&n).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	if republish {
		n.Title, n.Content = req.Title, req.Content
		s.broadcast(&n)
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
		atts := r.Attachments
		if atts == nil {
			atts = types.AttachmentArray{}
		}
		list = append(list, gin.H{
			"id": r.ID, "title": r.Title, "content": r.Content, "publish_at": timefmt.TP(r.PublishAt),
			"attachments": atts,
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// PublishedDetail 移动端：已发布公告详情（不存在/未发布返回 404）。
func (s *NoticeService) PublishedDetail(id string) (gin.H, *errs.Error) {
	var n model.SysNotice
	if err := s.db.First(&n, "id = ? AND status = 1", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return noticeItem(&n), nil
}
