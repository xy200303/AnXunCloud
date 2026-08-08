// Package service 系统管理业务逻辑。
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// configCacheKey 参数全量缓存（《数据库设计文档》§7.2：config:all）。
const configCacheKey = "config:all"

// ConfigService 参数配置服务。
type ConfigService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewConfigService(db *gorm.DB, rdb *redis.Client) *ConfigService {
	return &ConfigService{db: db, rdb: rdb}
}

// List 参数分页列表。
func (s *ConfigService) List(q *dto.ConfigQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysConfig{})
	if q.Key != "" {
		db = db.Where("key LIKE ?", "%"+q.Key+"%")
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysConfig
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		list = append(list, map[string]any{
			"id":         r.ID,
			"key":        r.Key,
			"name":       r.Name,
			"value":      r.Value,
			"remark":     r.Remark,
			"updated_at": r.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Create 新增参数（key 唯一）。
func (s *ConfigService) Create(req *dto.ConfigSaveReq) (string, *errs.Error) {
	if req.Key == "" {
		return "", errs.ErrParam.WithMsg("key 为必填项")
	}
	var count int64
	s.db.Model(&model.SysConfig{}).Where("key = ?", req.Key).Count(&count)
	if count > 0 {
		return "", errs.ErrConfigKeyExists
	}
	row := model.SysConfig{Key: req.Key, Name: req.Name, Value: req.Value, Remark: req.Remark}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	s.invalidate()
	return row.ID, nil
}

// Update 修改参数（key 不可改）。
func (s *ConfigService) Update(id string, req *dto.ConfigSaveReq) *errs.Error {
	var row model.SysConfig
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	updates := map[string]any{"name": req.Name, "value": req.Value, "remark": req.Remark}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	s.invalidate()
	return nil
}

// Delete 删除参数。
func (s *ConfigService) Delete(id string) *errs.Error {
	res := s.db.Delete(&model.SysConfig{}, "id = ?", id)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	s.invalidate()
	return nil
}

// Get 按 key 读取参数值（走 config:all 缓存，miss 回源重建）。
func (s *ConfigService) Get(key string) (string, bool) {
	ctx := context.Background()
	all, err := s.loadAll(ctx)
	if err != nil {
		return "", false
	}
	v, ok := all[key]
	return v, ok
}

// loadAll 读取全量参数（优先缓存）。
func (s *ConfigService) loadAll(ctx context.Context) (map[string]string, error) {
	cached, err := s.rdb.Get(ctx, configCacheKey).Result()
	if err == nil {
		var m map[string]string
		if json.Unmarshal([]byte(cached), &m) == nil {
			return m, nil
		}
	}
	var rows []model.SysConfig
	if err := s.db.Select("key", "value").Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	if b, err := json.Marshal(m); err == nil {
		s.rdb.Set(ctx, configCacheKey, b, 30*time.Minute)
	}
	return m, nil
}

// invalidate 参数变更后使缓存失效。
func (s *ConfigService) invalidate() {
	s.rdb.Del(context.Background(), configCacheKey)
}
