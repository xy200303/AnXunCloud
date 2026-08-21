package service

import (
	"strconv"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

// NoticeService 通知公告服务。
type NoticeService struct {
	db       *gorm.DB
	notifier *notify.Notifier
}

func NewNoticeService(db *gorm.DB, notifier *notify.Notifier) *NoticeService {
	return &NoticeService{db: db, notifier: notifier}
}

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
// 租户上下文（菜单归位方案 §2）：公告是发给本租户员工的，按 tenantID（EffectiveTenantID 解析）过滤。
func (s *NoticeService) List(q *NoticeListQuery, tenantID string) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysNotice{}).Where("tenant_id = ?", tenantID)
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

// Create 新增公告（归属上下文租户）；status=1 立即发布。
func (s *NoticeService) Create(req *NoticeSaveReq, operatorID string, operatorName string, tenantID string) (string, *errs.Error) {
	status := 0
	if req.Status != nil {
		status = *req.Status
	}
	n := model.SysNotice{
		TenantID: &tenantID,
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
		s.broadcast(&n, tenantID)
	}
	return n.ID, nil
}

// broadcast 公告发布时给本租户全体启用用户写站内消息（type=announcement）并 App 推送，
// 移动端消息列表可见并计入未读徽章；content 截断至 500 字符（sys_message.content 上限 512）。
func (s *NoticeService) broadcast(n *model.SysNotice, tenantID string) {
	var ids []string
	if err := s.db.Model(&model.SysUser{}).Where("status = ? AND tenant_id = ?", model.StatusEnabled, tenantID).Pluck("id", &ids).Error; err != nil {
		return
	}
	content := []rune(n.Content)
	if len(content) > 500 {
		content = append(content[:500], []rune("……（全文见公告）")...)
	}
	_ = s.notifier.SendBatch(ids, &tenantID, "announcement", "公告："+n.Title, string(content), &n.ID)
}

// Update 修改公告（含发布/下线；发布时间仅在首次发布时写入）。跨租户公告按 404 处理。
func (s *NoticeService) Update(id string, req *NoticeSaveReq, tenantID string) *errs.Error {
	var n model.SysNotice
	if err := s.db.First(&n, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
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
		s.broadcast(&n, tenantID)
	}
	return nil
}

// Delete 删除公告（跨租户按 404 处理）；同事务清除该公告广播产生的站内消息（type=announcement, biz_id=公告 id）。
// 注意：已到达手机通知栏的推送无法撤回，仅清除 App 消息中心记录。
func (s *NoticeService) Delete(id string, tenantID string) *errs.Error {
	var rows int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.SysNotice{}, "id = ? AND tenant_id = ?", id, tenantID)
		if res.Error != nil {
			return res.Error
		}
		rows = res.RowsAffected
		if rows == 0 {
			return nil
		}
		return tx.Where("type = ? AND biz_id = ?", "announcement", id).Delete(&model.SysMessage{}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	if rows == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// Published 小程序端：已发布公告分页（仅本租户公告）。
func (s *NoticeService) Published(page, pageSize int, tenantID string) (*response.Page, *errs.Error) {
	q := &NoticeListQuery{}
	q.Page, q.PageSize = page, pageSize
	db := s.db.Model(&model.SysNotice{}).Where("status = 1 AND tenant_id = ?", tenantID)
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

// PublishedDetail 移动端：已发布公告详情（不存在/未发布/跨租户返回 404）。
func (s *NoticeService) PublishedDetail(id string, tenantID string) (gin.H, *errs.Error) {
	var n model.SysNotice
	if err := s.db.First(&n, "id = ? AND status = 1 AND tenant_id = ?", id, tenantID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return noticeItem(&n), nil
}
