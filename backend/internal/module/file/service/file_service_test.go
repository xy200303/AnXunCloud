package service

import (
	"bytes"
	"image"
	"image/png"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/storage"
)

const (
	fileTenantA    = "11111111-1111-1111-1111-111111111111"
	fileTenantB    = "22222222-2222-2222-2222-222222222222"
	fileCommunityA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	fileCommunityB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func newFileSecurityTest(t *testing.T) (*FileService, *sysmodel.UploadFile) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.UploadFile{}, &sysmodel.Community{}, &filemodel.ObjectRef{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []sysmodel.Community{
		{TenantID: fileTenantA, Name: "A"},
		{TenantID: fileTenantB, Name: "B"},
	} {
		row.ID = fileCommunityA
		if row.TenantID == fileTenantB {
			row.ID = fileCommunityB
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rec := &sysmodel.UploadFile{TenantID: ptr(fileTenantA), Scene: "checkin", UserID: "owner"}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	return &FileService{db: db}, rec
}

func fileContext(identity *middleware.Identity) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/files/test", nil)
	middleware.SetIdentity(c, identity)
	return c
}

func TestFileReadRejectsCrossTenantAndUnknownScene(t *testing.T) {
	svc, rec := newFileSecurityTest(t)
	if be := svc.checkRead(fileContext(&middleware.Identity{UserID: "other", TenantID: fileTenantB}), rec); be == nil {
		t.Fatal("跨租户文件必须拒绝")
	}
	rec.TenantID = ptr(fileTenantB)
	rec.Scene = "unknown"
	if be := svc.checkRead(fileContext(&middleware.Identity{UserID: "other", TenantID: fileTenantB}), rec); be == nil {
		t.Fatal("未知场景必须默认拒绝")
	}
}

func TestFileReadRequiresCommunityScope(t *testing.T) {
	svc, rec := newFileSecurityTest(t)
	ref := filemodel.ObjectRef{TenantID: fileTenantA, CommunityID: fileCommunityA, FileID: rec.ID, ObjectType: "checkin_record", ObjectID: rec.ID}
	if err := svc.db.Create(&ref).Error; err != nil {
		t.Fatal(err)
	}
	allowed := &middleware.Identity{UserID: "user", TenantID: fileTenantA, ProjectIDs: []string{fileCommunityA}}
	if be := svc.checkRead(fileContext(allowed), rec); be != nil {
		t.Fatalf("本小区文件应允许读取: %v", be)
	}
	denied := &middleware.Identity{UserID: "user", TenantID: fileTenantA, ProjectIDs: []string{}}
	if be := svc.checkRead(fileContext(denied), rec); be == nil {
		t.Fatal("无任职范围的用户不得读取业务文件")
	}
}

func TestFileReadSuperAdminRequiresExplicitTenant(t *testing.T) {
	svc, rec := newFileSecurityTest(t)
	if be := svc.checkRead(fileContext(&middleware.Identity{UserID: "root", SuperAdmin: true}), rec); be == nil {
		t.Fatal("超管未指定目标租户时必须拒绝")
	}
}

func TestUploadContentValidation(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if be := validateUploadContent("checkin", "png", imageData.Bytes()); be != nil {
		t.Fatalf("有效 PNG 应通过: %v", be)
	}
	if be := validateUploadContent("checkin", "jpg", imageData.Bytes()); be == nil {
		t.Fatal("扩展名与真实图片格式不符时必须拒绝")
	}
	raw, _, ok := ooxml.BuiltinTemplate("fire_equipment")
	if !ok {
		t.Fatal("缺少内置母版")
	}
	if be := validateUploadContent("doc_template", "docx", raw); be != nil {
		t.Fatalf("有效甲方母版应通过: %v", be)
	}
	if be := validateUploadContent("doc_template", "docx", []byte("PK\x03\x04not-a-docx")); be == nil {
		t.Fatal("伪造 DOCX 必须拒绝")
	}
}

func TestCleanedDerivativePreservesOriginalEvidence(t *testing.T) {
	svc, rec := newFileSecurityTest(t)
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	rec.StorageKey = "checkin/202610/owner/evidence.png"
	rec.SHA256 = "original-sha"
	if err := svc.db.Save(rec).Error; err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	uploader := &UploadService{db: svc.db, store: store}
	if err := uploader.createCleanedDerivative(rec, imageData.Bytes()); err != nil {
		t.Fatal(err)
	}
	var original sysmodel.UploadFile
	if err := svc.db.First(&original, "id = ?", rec.ID).Error; err != nil {
		t.Fatal(err)
	}
	if original.SHA256 != "original-sha" || original.EvidenceRole != "original" {
		t.Fatal("原件证据元数据不得被副本覆盖")
	}
	var child sysmodel.UploadFile
	if err := svc.db.First(&child, "parent_file_id = ? AND evidence_role = ?", rec.ID, "cleaned").Error; err != nil {
		t.Fatal(err)
	}
	if child.ParentFileID == nil || *child.ParentFileID != original.ID || child.SHA256 == "" || child.SHA256 == original.SHA256 {
		t.Fatal("清洗副本必须独立登记摘要和父子关系")
	}
	data, err := store.ReadFile(child.StorageKey)
	if err != nil || len(data) == 0 {
		t.Fatalf("清洗副本不可读取: %v", err)
	}
}

func ptr(value string) *string { return &value }
