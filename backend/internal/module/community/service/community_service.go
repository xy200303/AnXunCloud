// Package service 小区与楼栋业务逻辑（含数据权限过滤）。
package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"property-inspection/internal/middleware"
	"property-inspection/internal/module/community/dto"
	insmodel "property-inspection/internal/module/inspection/model"
	sysmodel "property-inspection/internal/module/system/model"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
	"property-inspection/internal/pkg/timefmt"
)

// CommunityService 小区与楼栋服务。
type CommunityService struct {
	db *gorm.DB
}

func NewCommunityService(db *gorm.DB) *CommunityService { return &CommunityService{db: db} }

// ListCommunities 小区分页列表（数据权限按 community_ids 过滤）。
func (s *CommunityService) ListCommunities(c *gin.Context, q *dto.CommunityListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&sysmodel.Community{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", sysmodel.StatusStr(*q.Status))
	}
	db = middleware.ApplyCommunityFilter(db, c, "id")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []sysmodel.Community
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		var buildingCount, pointCount int64
		s.db.Model(&insmodel.Building{}).Where("community_id = ?", r.ID).Count(&buildingCount)
		s.db.Model(&insmodel.InspectionPoint{}).Where("community_id = ?", r.ID).Count(&pointCount)
		managerName := ""
		if r.ManagerID != nil {
			var u sysmodel.SysUser
			if s.db.Select("name").First(&u, *r.ManagerID).Error == nil {
				managerName = u.Name
			}
		}
		list = append(list, gin.H{
			"id": r.ID, "name": r.Name, "address": r.Address,
			"manager_id": r.ManagerID, "manager_name": managerName,
			"building_count": buildingCount, "point_count": pointCount,
			"status": sysmodel.StatusInt(r.Status), "created_at": timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// CreateCommunity 新增小区（名称唯一）。
func (s *CommunityService) CreateCommunity(req *dto.CommunitySaveReq) (string, *errs.Error) {
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		return "", errs.ErrCommunityNameExists
	}
	status := sysmodel.StatusEnabled
	if req.Status != nil {
		status = sysmodel.StatusStr(*req.Status)
	}
	row := sysmodel.Community{Name: req.Name, Address: req.Address, ManagerID: req.ManagerID, Status: status, Remark: req.Remark}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

// CommunityDetail 小区详情（含楼栋列表），越权返回 40302。
func (s *CommunityService) CommunityDetail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var row sysmodel.Community
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, row.ID); be != nil {
		return nil, be
	}
	var buildings []insmodel.Building
	s.db.Where("community_id = ?", id).Order("sort ASC, id ASC").Find(&buildings)
	items := make([]gin.H, 0, len(buildings))
	for _, b := range buildings {
		var pointCount int64
		s.db.Model(&insmodel.InspectionPoint{}).Where("building_id = ?", b.ID).Count(&pointCount)
		items = append(items, gin.H{"id": b.ID, "name": b.Name, "type": b.Type, "point_count": pointCount})
	}
	managerName := ""
	if row.ManagerID != nil {
		var u sysmodel.SysUser
		if s.db.Select("name").First(&u, *row.ManagerID).Error == nil {
			managerName = u.Name
		}
	}
	return gin.H{
		"id": row.ID, "name": row.Name, "address": row.Address,
		"manager_id": row.ManagerID, "manager_name": managerName,
		"status": sysmodel.StatusInt(row.Status), "remark": row.Remark,
		"buildings": items,
		"created_at": timefmt.T(row.CreatedAt), "updated_at": timefmt.T(row.UpdatedAt),
	}, nil
}

// UpdateCommunity 修改小区。
func (s *CommunityService) UpdateCommunity(c *gin.Context, id string, req *dto.CommunitySaveReq) *errs.Error {
	var row sysmodel.Community
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, row.ID); be != nil {
		return be
	}
	if req.Name != row.Name {
		var count int64
		s.db.Model(&sysmodel.Community{}).Where("name = ? AND id <> ?", req.Name, id).Count(&count)
		if count > 0 {
			return errs.ErrCommunityNameExists
		}
	}
	updates := map[string]any{"name": req.Name, "address": req.Address, "manager_id": req.ManagerID, "remark": req.Remark}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// DeleteCommunity 软删除小区；存在楼栋或点位时拒绝（42002）。
func (s *CommunityService) DeleteCommunity(c *gin.Context, id string) *errs.Error {
	var row sysmodel.Community
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, row.ID); be != nil {
		return be
	}
	var bCount, pCount int64
	s.db.Model(&insmodel.Building{}).Where("community_id = ?", id).Count(&bCount)
	s.db.Model(&insmodel.InspectionPoint{}).Where("community_id = ?", id).Count(&pCount)
	if bCount > 0 || pCount > 0 {
		return errs.ErrCommunityHasChildren
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// ListBuildings 楼栋/区域分页列表。
func (s *CommunityService) ListBuildings(c *gin.Context, q *dto.BuildingListQuery) (*response.Page, *errs.Error) {
	if be := middleware.CheckCommunity(c, q.CommunityID); be != nil {
		return nil, be
	}
	db := s.db.Model(&insmodel.Building{}).Where("community_id = ?", q.CommunityID)
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Type != "" {
		db = db.Where("type = ?", q.Type)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []insmodel.Building
	offset, limit := q.Normalize()
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	commName := ""
	var comm sysmodel.Community
	if s.db.Select("name").First(&comm, "id = ?", q.CommunityID).Error == nil {
		commName = comm.Name
	}
	list := make([]gin.H, 0, len(rows))
	for _, b := range rows {
		var pointCount int64
		s.db.Model(&insmodel.InspectionPoint{}).Where("building_id = ?", b.ID).Count(&pointCount)
		list = append(list, gin.H{
			"id": b.ID, "community_id": b.CommunityID, "community_name": commName,
			"name": b.Name, "type": b.Type, "sort": b.Sort, "point_count": pointCount,
			"status": sysmodel.StatusInt(b.Status), "created_at": timefmt.T(b.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// CreateBuilding 新增楼栋/区域（同小区下名称唯一）。
func (s *CommunityService) CreateBuilding(c *gin.Context, req *dto.BuildingSaveReq) (string, *errs.Error) {
	if be := middleware.CheckCommunity(c, req.CommunityID); be != nil {
		return "", be
	}
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("id = ?", req.CommunityID).Count(&count)
	if count == 0 {
		return "", errs.ErrCommunityNotExist
	}
	s.db.Model(&insmodel.Building{}).Where("community_id = ? AND name = ?", req.CommunityID, req.Name).Count(&count)
	if count > 0 {
		return "", errs.ErrParam.WithMsg("同小区下楼栋/区域名称已存在")
	}
	row := insmodel.Building{CommunityID: req.CommunityID, Name: req.Name, Type: req.Type, Sort: req.Sort, Status: sysmodel.StatusEnabled}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

// BuildingDetail 楼栋详情。
func (s *CommunityService) BuildingDetail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var b insmodel.Building
	if err := s.db.First(&b, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, b.CommunityID); be != nil {
		return nil, be
	}
	var pointCount int64
	s.db.Model(&insmodel.InspectionPoint{}).Where("building_id = ?", b.ID).Count(&pointCount)
	return gin.H{
		"id": b.ID, "community_id": b.CommunityID, "name": b.Name, "type": b.Type,
		"sort": b.Sort, "point_count": pointCount, "status": sysmodel.StatusInt(b.Status),
		"created_at": timefmt.T(b.CreatedAt),
	}, nil
}

// UpdateBuilding 修改楼栋/区域。
func (s *CommunityService) UpdateBuilding(c *gin.Context, id string, req *dto.BuildingSaveReq) *errs.Error {
	var b insmodel.Building
	if err := s.db.First(&b, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, b.CommunityID); be != nil {
		return be
	}
	if req.Name != b.Name {
		var count int64
		s.db.Model(&insmodel.Building{}).Where("community_id = ? AND name = ? AND id <> ?", b.CommunityID, req.Name, id).Count(&count)
		if count > 0 {
			return errs.ErrParam.WithMsg("同小区下楼栋/区域名称已存在")
		}
	}
	if err := s.db.Model(&b).Updates(map[string]any{"name": req.Name, "type": req.Type, "sort": req.Sort}).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// DeleteBuilding 删除楼栋；存在点位时拒绝（42003）。
func (s *CommunityService) DeleteBuilding(c *gin.Context, id string) *errs.Error {
	var b insmodel.Building
	if err := s.db.First(&b, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, b.CommunityID); be != nil {
		return be
	}
	var count int64
	s.db.Model(&insmodel.InspectionPoint{}).Where("building_id = ?", id).Count(&count)
	if count > 0 {
		return errs.ErrBuildingHasPoints
	}
	if err := s.db.Delete(&b).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
