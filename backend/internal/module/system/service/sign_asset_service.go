package service

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"

	"go.uber.org/zap"
)

// SignAssetService 签章资产管理：手写签名/公章版本链（换签名/换章不删旧记录）。
type SignAssetService struct {
	db    *gorm.DB
	store *storage.Storage
}

func NewSignAssetService(db *gorm.DB, store *storage.Storage) *SignAssetService {
	return &SignAssetService{db: db, store: store}
}

// List 签章资产分页列表（含 URL、归属人姓名、sha256 短码）。
// 租户隔离：仅看操作者所属租户的资产（含超管，本阶段不做跨租户查看）。
func (s *SignAssetService) List(q *dto.SignAssetQuery, tenantID string) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SignAsset{}).Where("tenant_id = ?", tenantID)
	if q.AssetType != "" {
		db = db.Where("asset_type = ?", q.AssetType)
	}
	if q.OwnerID != "" {
		db = db.Where("owner_id = ?", q.OwnerID)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SignAsset
	offset, limit := q.Normalize()
	if err := db.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 归属人姓名反查
	ownerIDs := map[string]bool{}
	for i := range rows {
		if rows[i].OwnerID != nil {
			ownerIDs[*rows[i].OwnerID] = true
		}
	}
	names := map[string]string{}
	if len(ownerIDs) > 0 {
		ids := make([]string, 0, len(ownerIDs))
		for id := range ownerIDs {
			ids = append(ids, id)
		}
		var users []model.SysUser
		s.db.Select("id", "name").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Name
		}
	}
	items := make([]dto.SignAssetItem, 0, len(rows))
	for i := range rows {
		items = append(items, s.toItem(&rows[i], names))
	}
	return &response.Page{List: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// toItem 转换为列表视图。
func (s *SignAssetService) toItem(a *model.SignAsset, names map[string]string) dto.SignAssetItem {
	item := dto.SignAssetItem{
		ID:            a.ID,
		AssetType:     a.AssetType,
		OwnerID:       a.OwnerID,
		FileKey:       a.FileKey,
		SHA256:        a.SHA256,
		Version:       a.Version,
		Status:        a.Status,
		Remark:        a.Remark,
		CreatedBy:     a.CreatedBy,
		CreatedAt:     timefmt.T(a.CreatedAt),
		UpdatedAt:     timefmt.T(a.UpdatedAt),
		RevokedAt:     timefmt.TP(a.RevokedAt),
		RevokedReason: a.RevokedReason,
	}
	if a.OwnerID != nil {
		item.OwnerName = names[*a.OwnerID]
	}
	if s.store != nil && a.FileKey != "" {
		item.URL = s.store.URL(a.FileKey)
	}
	if len(a.SHA256) > 12 {
		item.SHA256Short = a.SHA256[:12]
	} else {
		item.SHA256Short = a.SHA256
	}
	return item
}

// Create 新增签章资产（创建即 active；同租户+type+owner 原 active 同事务置 replaced）。
// tenantID 取操作者所属租户（个人签名/公章同规则）。
func (s *SignAssetService) Create(operatorID, tenantID string, req *dto.SignAssetCreateReq) (*dto.SignAssetItem, *errs.Error) {
	asset, be := s.create(operatorID, tenantID, req.AssetType, req.OwnerID, strings.TrimSpace(req.FileKey), req.Remark)
	if be != nil {
		return nil, be
	}
	item := s.toItem(asset, map[string]string{})
	if asset.OwnerID != nil {
		var u model.SysUser
		if s.db.Select("name").First(&u, "id = ?", *asset.OwnerID).Error == nil {
			item.OwnerName = u.Name
		}
	}
	return &item, nil
}

// create 核心创建逻辑（个人签名替换与后台新增共用）；tenantID 为资产归属租户。
func (s *SignAssetService) create(operatorID, tenantID, assetType string, ownerID *string, fileKey, remark string) (*model.SignAsset, *errs.Error) {
	if fileKey == "" {
		return nil, errs.ErrParam.WithMsg("file_key 为必填项")
	}
	if tenantID == "" {
		return nil, errs.ErrParam.WithMsg("租户上下文缺失")
	}
	var owner *string
	switch assetType {
	case model.SignAssetTypeUserSignature:
		if ownerID == nil || strings.TrimSpace(*ownerID) == "" {
			return nil, errs.ErrParam.WithMsg("user_signature 类型必须指定 owner_id（签名归属用户）")
		}
		var count int64
		s.db.Model(&model.SysUser{}).Where("id = ?", *ownerID).Count(&count)
		if count == 0 {
			return nil, errs.ErrParam.WithMsg("owner_id 用户不存在")
		}
		owner = ownerID
	case model.SignAssetTypeCompanySeal:
		owner = nil // 公章按租户唯一 active，忽略传入的 owner_id
	default:
		return nil, errs.ErrParam.WithMsg("asset_type 取值非法（user_signature/company_seal）")
	}

	sha := s.sha256Of(fileKey)
	var asset model.SignAsset
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 同租户+type+owner 的查询基座（每次重建，避免 statement 条件残留）
		base := func() *gorm.DB {
			q := tx.Model(&model.SignAsset{}).Where("tenant_id = ? AND asset_type = ?", tenantID, assetType)
			if owner != nil {
				return q.Where("owner_id = ?", *owner)
			}
			return q.Where("owner_id IS NULL")
		}
		// 原 active 置 replaced（保留版本链，不删记录）
		if err := base().Where("status = ?", model.SignAssetStatusActive).
			Update("status", model.SignAssetStatusReplaced).Error; err != nil {
			return err
		}
		var maxV int
		if err := base().Select("COALESCE(MAX(version), 0)").Scan(&maxV).Error; err != nil {
			return err
		}
		asset = model.SignAsset{
			TenantID:  &tenantID,
			AssetType: assetType,
			OwnerID:   owner,
			FileKey:   fileKey,
			SHA256:    sha,
			Version:   maxV + 1,
			Status:    model.SignAssetStatusActive,
			Remark:    remark,
			CreatedBy: &operatorID,
		}
		return tx.Create(&asset).Error
	})
	if err != nil {
		return nil, errs.ErrInternal
	}
	return &asset, nil
}

// Revoke 作废签章（仅 active 可作废，reason 必填）；跨租户资产按 404 处理（不暴露存在性）。
func (s *SignAssetService) Revoke(id, reason, tenantID string) *errs.Error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errs.ErrParam.WithMsg("reason 为必填项")
	}
	var asset model.SignAsset
	if err := s.db.First(&asset, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if asset.TenantID == nil || *asset.TenantID != tenantID {
		return errs.ErrNotFound
	}
	if asset.Status != model.SignAssetStatusActive {
		return errs.ErrConflict.WithMsg("仅 active 状态的签章可作废")
	}
	now := time.Now()
	if err := s.db.Model(&asset).Updates(map[string]any{
		"status":         model.SignAssetStatusRevoked,
		"revoked_at":     now,
		"revoked_reason": reason,
	}).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// SetUserSignature 个人中心换签名：fileKey 非空创建/替换当前用户 active 签名资产（tenant_id 取该用户所属租户）；
// 空串表示移除签名（当前 active 置 revoked 留痕）。
func (s *SignAssetService) SetUserSignature(uid, fileKey string) *errs.Error {
	if fileKey == "" {
		var cur model.SignAsset
		if err := s.db.Where("asset_type = ? AND owner_id = ? AND status = ?",
			model.SignAssetTypeUserSignature, uid, model.SignAssetStatusActive).First(&cur).Error; err != nil {
			return nil // 本来就没有签名
		}
		now := time.Now()
		if err := s.db.Model(&cur).Updates(map[string]any{
			"status":         model.SignAssetStatusRevoked,
			"revoked_at":     now,
			"revoked_reason": "用户移除签名",
		}).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	// 租户取签名归属用户的所属租户（与操作者租户一致，个人中心场景二者同人）
	var u model.SysUser
	if err := s.db.Select("tenant_id").First(&u, "id = ?", uid).Error; err != nil {
		return errs.ErrNotFound
	}
	_, be := s.create(uid, u.TenantID, model.SignAssetTypeUserSignature, &uid, fileKey, "")
	return be
}

// ActiveSignature 用户当前 active 签名资产（file_key, asset_id）；无则空串。
func (s *SignAssetService) ActiveSignature(userID string) (fileKey, assetID string) {
	var a model.SignAsset
	if err := s.db.Select("id", "file_key").
		Where("asset_type = ? AND owner_id = ? AND status = ?",
			model.SignAssetTypeUserSignature, userID, model.SignAssetStatusActive).
		First(&a).Error; err != nil {
		return "", ""
	}
	return a.FileKey, a.ID
}

// ActiveSealKey 指定租户当前 active 公章 file_key（tenantID 为空或无 active 公章返回空串）。
func (s *SignAssetService) ActiveSealKey(tenantID *string) string {
	if tenantID == nil || *tenantID == "" {
		return ""
	}
	var a model.SignAsset
	if err := s.db.Select("file_key").
		Where("tenant_id = ? AND asset_type = ? AND status = ?", *tenantID, model.SignAssetTypeCompanySeal, model.SignAssetStatusActive).
		First(&a).Error; err != nil {
		return ""
	}
	return a.FileKey
}

// sha256Of 计算文件内容 SHA-256（hex）；读取失败容错空串并告警。
func (s *SignAssetService) sha256Of(fileKey string) string {
	data, err := s.readFile(fileKey)
	if err != nil {
		logger.L.Warn("签章文件读取失败，sha256 置空", zap.String("file_key", fileKey), zap.Error(err))
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readFile 按 file_key 读文件字节（local 读本地目录；云存储 HTTP 下载）。
func (s *SignAssetService) readFile(fileKey string) ([]byte, error) {
	if s.store.IsLocal() {
		return os.ReadFile(s.store.LocalPath(fileKey))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(s.store.URL(fileKey))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}
