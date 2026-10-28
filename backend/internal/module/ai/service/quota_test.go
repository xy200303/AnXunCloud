package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
)

func TestUsageQuotaRejectsConcurrencyAndDailyExhaustion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:quota_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Quota{}, &model.UsageReservation{}, &model.Conversation{}, &model.Message{}); err != nil {
		t.Fatal(err)
	}
	svc := NewUsageService(db)
	actor := Actor{TenantID: "01a00000-0000-7000-8000-000000000001", UserID: "01a00000-0000-7000-8000-000000000002"}
	if be := svc.SaveQuota(actor, "assistant", QuotaReq{MaxRequestsDaily: 2, MaxTokensDaily: 100, MaxConcurrent: 1}); be != nil {
		t.Fatal(be.Msg)
	}
	first, be := svc.Reserve(actor, "assistant")
	if be != nil {
		t.Fatal(be.Msg)
	}
	if _, be := svc.Reserve(actor, "assistant"); be == nil {
		t.Fatal("concurrent quota must reject a second reservation")
	}
	svc.Finish(first, 5, 6, false)
	second, be := svc.Reserve(actor, "assistant")
	if be != nil {
		t.Fatal(be.Msg)
	}
	svc.Finish(second, 5, 6, false)
	if _, be := svc.Reserve(actor, "assistant"); be == nil || be.Msg != "该 AI 任务的日请求配额已耗尽" {
		t.Fatalf("daily request exhaustion must reject new calls: %v", be)
	}
	if be := svc.SaveQuota(actor, "assistant", QuotaReq{MaxRequestsDaily: 10, MaxTokensDaily: 11, MaxConcurrent: 1}); be != nil {
		t.Fatal(be.Msg)
	}
	if _, be := svc.Reserve(actor, "assistant"); be == nil || be.Msg != "该 AI 任务的日 token 配额已耗尽" {
		t.Fatalf("daily token exhaustion must reject new calls: %v", be)
	}
	usage, be := svc.Usage(actor)
	if be != nil || usage["requests"] != int64(2) || usage["tokens"] != int64(22) || usage["source"] != "tenant" {
		t.Fatalf("unexpected usage: %#v, %v", usage, be)
	}
}
