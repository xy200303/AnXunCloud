package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/config"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/storage"
)

// 测试用租户/用户 ID（语义化名即可，列类型为 uuid 但 sqlite 不做格式校验）
const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"
	userA1  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB1  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	fileA1 = "f1111111-1111-1111-1111-111111111111"
	fileB1 = "f2222222-2222-2222-2222-222222222222"
	fileA2 = "f3333333-3333-3333-3333-333333333333"
	fileA  = "f4444444-4444-4444-4444-444444444444"
	fileB  = "f5555555-5555-5555-5555-555555555555"
)

// newSignAssetTestSvc 内存 sqlite + 本地存储驱动的测试服务（文件不存在时 sha256 容错空串）。
func newSignAssetTestSvc(t *testing.T) *SignAssetService {
	t.Helper()
	logger.L = zap.NewNop() // sha256Of 读文件失败走 logger，测试环境需非 nil
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.SignAsset{}, &model.SysUser{}, &model.UploadFile{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 预置上传文件记录（Create 走 uploadfile.ByID 校验 file_id 存在且为合法 UUID）
	for _, fid := range []string{fileA1, fileB1, fileA2, fileA, fileB} {
		uf := model.UploadFile{StorageKey: "test/" + fid, Scene: "sign", UserID: userA1, Name: fid + ".png", Storage: "local"}
		uf.ID = fid
		if err := db.Create(&uf).Error; err != nil {
			t.Fatalf("造文件失败: %v", err)
		}
	}
	seedUser := func(id, tenantID string) {
		u := model.SysUser{TenantID: tenantID, Username: id, Name: id, Status: model.StatusEnabled}
		u.ID = id
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("造用户失败: %v", err)
		}
	}
	seedUser(userA1, tenantA)
	seedUser(userB1, tenantB)
	store := storage.New(config.UploadConfig{}, config.OSSConfig{}, config.COSConfig{}, "")
	return NewSignAssetService(db, store)
}

// TestCreateCompanySealPerTenant 公章按租户隔离：两租户各持一枚 active，互不替换。
func TestCreateCompanySealPerTenant(t *testing.T) {
	svc := newSignAssetTestSvc(t)

	a1, be := svc.Create(userA1, tenantA, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileA1})
	if be != nil {
		t.Fatalf("租户A建章失败: %v", be)
	}
	if a1.ID == "" {
		t.Fatal("租户A公章应创建成功")
	}
	b1, be := svc.Create(userB1, tenantB, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileB1})
	if be != nil {
		t.Fatalf("租户B建章失败: %v", be)
	}
	// 租户B建章不应影响租户A的 active
	var cnt int64
	svc.db.Model(&model.SignAsset{}).Where("asset_type = ? AND status = ?", model.SignAssetTypeCompanySeal, model.SignAssetStatusActive).Count(&cnt)
	if cnt != 2 {
		t.Fatalf("两租户应各有 1 枚 active 公章，实际 %d", cnt)
	}
	// 同租户换章：旧章置 replaced，新章 version 递增
	a2, be := svc.Create(userA1, tenantA, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileA2})
	if be != nil {
		t.Fatalf("租户A换章失败: %v", be)
	}
	var old model.SignAsset
	if err := svc.db.First(&old, "id = ?", a1.ID).Error; err != nil {
		t.Fatalf("读取旧章失败: %v", err)
	}
	if old.Status != model.SignAssetStatusReplaced {
		t.Fatalf("租户A旧章应为 replaced，实际 %s", old.Status)
	}
	if a2.Version != 2 {
		t.Fatalf("租户A新章版本应为 2，实际 %d", a2.Version)
	}
	// 租户间版本链互不影响
	var bAsset model.SignAsset
	if err := svc.db.First(&bAsset, "id = ?", b1.ID).Error; err != nil {
		t.Fatalf("读取租户B公章失败: %v", err)
	}
	if bAsset.Status != model.SignAssetStatusActive || bAsset.Version != 1 {
		t.Fatalf("租户B公章不应受租户A换章影响，实际 status=%s version=%d", bAsset.Status, bAsset.Version)
	}
}

// TestActiveSealIDByTenant ActiveSealID 按租户取章（返回 file_id）；无章租户/空租户返回空串。
func TestActiveSealIDByTenant(t *testing.T) {
	svc := newSignAssetTestSvc(t)
	if _, be := svc.Create(userA1, tenantA, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileA}); be != nil {
		t.Fatalf("租户A建章失败: %v", be)
	}
	ta, tb := tenantA, tenantB
	if got := svc.ActiveSealID(&ta); got != fileA {
		t.Fatalf("租户A应取到自己的章，实际 %q", got)
	}
	if got := svc.ActiveSealID(&tb); got != "" {
		t.Fatalf("租户B无章应返回空串，实际 %q", got)
	}
	if got := svc.ActiveSealID(nil); got != "" {
		t.Fatalf("空租户应返回空串，实际 %q", got)
	}
}

// TestListTenantIsolation List 仅返回操作者租户资产。
func TestListTenantIsolation(t *testing.T) {
	svc := newSignAssetTestSvc(t)
	svc.Create(userA1, tenantA, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileA})
	svc.Create(userB1, tenantB, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileB})

	page, be := svc.List(&dto.SignAssetQuery{}, tenantA)
	if be != nil {
		t.Fatalf("List 失败: %v", be)
	}
	if page.Total != 1 {
		t.Fatalf("租户A只能看到 1 条资产，实际 %d", page.Total)
	}
}

// TestRevokeCrossTenant 跨租户作废按 404 处理，且原资产状态不变。
func TestRevokeCrossTenant(t *testing.T) {
	svc := newSignAssetTestSvc(t)
	item, be := svc.Create(userA1, tenantA, &dto.SignAssetCreateReq{AssetType: model.SignAssetTypeCompanySeal, FileID: fileA})
	if be != nil {
		t.Fatalf("租户A建章失败: %v", be)
	}
	if be := svc.Revoke(item.ID, "越权作废", tenantB); be != errs.ErrNotFound {
		t.Fatalf("跨租户作废应返回 404，实际 %v", be)
	}
	var a model.SignAsset
	svc.db.First(&a, "id = ?", item.ID)
	if a.Status != model.SignAssetStatusActive {
		t.Fatalf("跨租户作废不应生效，实际状态 %s", a.Status)
	}
	// 本租户正常作废
	if be := svc.Revoke(item.ID, "停用", tenantA); be != nil {
		t.Fatalf("本租户作废失败: %v", be)
	}
}

// TestSetUserSignatureTenant 个人签名写入归属用户的 tenant_id。
func TestSetUserSignatureTenant(t *testing.T) {
	svc := newSignAssetTestSvc(t)
	if be := svc.SetUserSignature(userB1, fileB); be != nil {
		t.Fatalf("设置签名失败: %v", be)
	}
	var a model.SignAsset
	if err := svc.db.First(&a, "asset_type = ? AND owner_id = ? AND status = ?",
		model.SignAssetTypeUserSignature, userB1, model.SignAssetStatusActive).Error; err != nil {
		t.Fatalf("签名资产不存在: %v", err)
	}
	if a.TenantID == nil || *a.TenantID != tenantB {
		t.Fatalf("签名 tenant_id 应为归属用户租户 %s，实际 %v", tenantB, a.TenantID)
	}
}
