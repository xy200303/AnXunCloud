package database

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

// migrateSignAssetData v16 存量数据迁移（幂等，可随启动重复执行）：
//  1. sys_user.signature_file_key 非空的用户 → 生成 active user_signature 资产（sha256 读文件计算，缺失容错空串）；
//  2. sys_config 的 report.seal_file_key 非空 → 生成 active company_seal 资产，随后删除该配置行（company_name 保留）。
func migrateSignAssetData(db *gorm.DB, uploadDir string) error {
	// 用户签名 → user_signature 资产
	var users []model.SysUser
	if err := db.Select("id", "signature_file_key").Where("signature_file_key <> ''").Find(&users).Error; err != nil {
		return err
	}
	for i := range users {
		u := &users[i]
		if err := ensureMigratedAsset(db, uploadDir, model.SignAssetTypeUserSignature, &u.ID, u.SignatureFileKey); err != nil {
			return err
		}
	}

	// 公章配置 → company_seal 资产（先读后删）
	var cfg model.SysConfig
	if err := db.Select("value").Where("key = ?", "report.seal_file_key").First(&cfg).Error; err == nil {
		if cfg.Value != "" {
			if err := ensureMigratedAsset(db, uploadDir, model.SignAssetTypeCompanySeal, nil, cfg.Value); err != nil {
				return err
			}
		}
		if err := db.Where("key = ?", "report.seal_file_key").Delete(&model.SysConfig{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ensureMigratedAsset 若同 type+owner 下尚无该 file_key 的资产，则插入一条 active 版本
// （同 type+owner 原 active 置 replaced；version 取组内最大+1）。
func ensureMigratedAsset(db *gorm.DB, uploadDir, assetType string, ownerID *string, fileKey string) error {
	ownerCond := "owner_id IS NULL"
	args := []any{assetType}
	if ownerID != nil {
		ownerCond = "owner_id = ?"
		args = append(args, *ownerID)
	}
	var count int64
	q := func() *gorm.DB { return db.Model(&model.SignAsset{}).Where("asset_type = ? AND "+ownerCond, args...) }
	if err := q().Where("file_key = ?", fileKey).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil // 已迁移过
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SignAsset{}).
			Where("asset_type = ? AND "+ownerCond+" AND status = ?", append(args, model.SignAssetStatusActive)...).
			Update("status", model.SignAssetStatusReplaced).Error; err != nil {
			return err
		}
		var maxV int
		if err := tx.Model(&model.SignAsset{}).
			Select("COALESCE(MAX(version), 0)").
			Where("asset_type = ? AND "+ownerCond, args...).Scan(&maxV).Error; err != nil {
			return err
		}
		asset := model.SignAsset{
			AssetType: assetType,
			OwnerID:   ownerID,
			FileKey:   fileKey,
			SHA256:    localFileSHA256(uploadDir, fileKey),
			Version:   maxV + 1,
			Status:    model.SignAssetStatusActive,
			Remark:    "v16 存量迁移",
		}
		return tx.Create(&asset).Error
	})
}

// localFileSHA256 计算本地上传目录内文件的 SHA-256（hex）；文件缺失/不可读返回空串（容错）。
func localFileSHA256(uploadDir, fileKey string) string {
	if uploadDir == "" || fileKey == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(uploadDir, filepath.FromSlash(fileKey)))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
