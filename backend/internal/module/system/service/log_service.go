package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
)

// LogService 日志检索服务（只读）。
type LogService struct {
	db *gorm.DB
}

func NewLogService(db *gorm.DB) *LogService { return &LogService{db: db} }

// OperationList 操作日志分页查询。
func (s *LogService) OperationList(q *dto.OperationLogQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysOperationLog{})
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.Module != "" {
		db = db.Where("module = ?", q.Module)
	}
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", logStatusStr(status))
	}
	var be *errs.Error
	if db, be = timeRange(db, q.StartTime, q.EndTime); be != nil {
		return nil, be
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysOperationLog
	offset, limit := q.Normalize()
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		list = append(list, map[string]any{
			"id":         r.ID,
			"user_id":    r.UserID,
			"username":   r.Username,
			"module":     r.Module,
			"action":     r.Action,
			"action_name": middleware.ActionName(r.Action),
			"method":     r.Method,
			"path":       r.Path,
			"params":     r.Params,
			"ip":         r.IP,
			"status":     logStatusInt(r.Status),
			"cost_ms":    r.CostMs,
			"created_at": timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// MyLoginLogs 当前用户自己的登录日志（个人中心；limit 默认 5，上限 50）。
func (s *LogService) MyLoginLogs(userID string, limit int) ([]gin.H, *errs.Error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	var rows []model.SysLoginLog
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		list = append(list, gin.H{
			"ip": r.IP, "ua": r.UA, "status": logStatusInt(r.Status),
			"msg": r.Msg, "created_at": timefmt.T(r.CreatedAt),
		})
	}
	return list, nil
}

// LoginList 登录日志分页查询。
func (s *LogService) LoginList(q *dto.LoginLogQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysLoginLog{})
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.IP != "" {
		db = db.Where("ip LIKE ?", "%"+q.IP+"%")
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", logStatusStr(status))
	}
	var be *errs.Error
	if db, be = timeRange(db, q.StartTime, q.EndTime); be != nil {
		return nil, be
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysLoginLog
	offset, limit := q.Normalize()
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		list = append(list, map[string]any{
			"id":         r.ID,
			"user_id":    r.UserID,
			"username":   r.Username,
			"channel":    r.Channel,
			"ip":         r.IP,
			"ua":         r.UA,
			"status":     logStatusInt(r.Status),
			"msg":        r.Msg,
			"created_at": timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// logStatusStr 日志状态：1 success / 0 fail。
func logStatusStr(status string) string {
	if status == "enabled" {
		return "success"
	}
	return "fail"
}

func logStatusInt(status string) int {
	if status == "success" {
		return 1
	}
	return 0
}

// timeRange 追加 created_at 时间范围过滤；格式非法返回 40001。
func timeRange(db *gorm.DB, start, end string) (*gorm.DB, *errs.Error) {
	if start != "" {
		t, err := timefmt.Parse(start)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("start_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where("created_at >= ?", t)
	}
	if end != "" {
		t, err := timefmt.Parse(end)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("end_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where("created_at <= ?", t)
	}
	return db, nil
}
