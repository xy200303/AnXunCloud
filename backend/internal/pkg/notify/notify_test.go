// notify 包集成测试：内存 sqlite + httptest 模拟个推 V2，验证 pushAsync 的按用户分组角标链路
// （未读数口径 sys_message user_id + is_read=false → push_channel.ios.auto_badge 绝对数），
// 以及无已绑设备时不发起任何推送请求。均不打真实个推网关。
package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/push"
)

const (
	notifyUser1 = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	notifyUser2 = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

// pushBody 记录一次 /push/single/cid 请求的关键字段（cid 与 iOS 角标）。
type pushBody struct {
	cid       string
	autoBadge string
	hasChannl bool
}

// newGetuiMock 模拟个推 V2：auth 直接发 token，push 记录请求体。返回 server 与并发安全的记录集。
func newGetuiMock(t *testing.T) (*httptest.Server, func() []pushBody) {
	t.Helper()
	var mu sync.Mutex
	var bodies []pushBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/notifyappid/auth":
			fmt.Fprintf(w, `{"code":0,"msg":"success","data":{"token":"t1","expire_time":"%d"}}`, time.Now().Add(time.Hour).UnixMilli())
		case "/v2/notifyappid/push/single/cid":
			var req struct {
				Audience struct {
					CID []string `json:"cid"`
				} `json:"audience"`
				PushChannel map[string]struct {
					AutoBadge string `json:"auto_badge"`
				} `json:"push_channel"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("push 请求体解析失败: %v", err)
			}
			b := pushBody{}
			if len(req.Audience.CID) > 0 {
				b.cid = req.Audience.CID[0]
			}
			if ios, ok := req.PushChannel["ios"]; ok {
				b.hasChannl = true
				b.autoBadge = ios.AutoBadge
			}
			mu.Lock()
			bodies = append(bodies, b)
			mu.Unlock()
			fmt.Fprint(w, `{"code":0,"msg":"ok","data":{}}`)
		default:
			t.Errorf("未知请求路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, func() []pushBody {
		mu.Lock()
		defer mu.Unlock()
		return append([]pushBody(nil), bodies...)
	}
}

// newTestNotifier 内存 sqlite 建表 + 个推 mock 客户端；logger 置 Nop（notify 路径会打日志）。
func newTestNotifier(t *testing.T, baseURL string) *Notifier {
	t.Helper()
	logger.L = zap.NewNop()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&sysmodel.SysMessage{}, &sysmodel.UserPushDevice{}, &sysmodel.SysUser{}, &sysmodel.SysOperationLog{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	cli := push.NewClient("notifyappid", "notifyappkey", "notifysecret")
	cli.SetBaseURL(baseURL)
	return New(db, cli)
}

// waitBodies 轮询直到收到 n 条 push 请求或超时（pushAsync 走 goroutine）。
func waitBodies(t *testing.T, get func() []pushBody, n int) []pushBody {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b := get(); len(b) >= n {
			return b
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待 push 请求超时：已收到 %d 条，期望 >= %d", len(get()), n)
	return nil
}

// TestPushAsyncBadgePerUser 验证按用户分组的角标下发：
// user1 已有 2 条未读 + 本次群发 1 条 → badge=3（两台设备同一角标）；
// user2 已有 0 条未读 + 本次 1 条 → badge=1。
func TestPushAsyncBadgePerUser(t *testing.T) {
	srv, getBodies := newGetuiMock(t)
	defer srv.Close()
	n := newTestNotifier(t, srv.URL)

	seed := func(uid string, cids []string, unread int) {
		for _, cid := range cids {
			if err := n.db.Create(&sysmodel.UserPushDevice{UserID: uid, CID: cid, Platform: "ios"}).Error; err != nil {
				t.Fatalf("种设备失败: %v", err)
			}
		}
		for i := 0; i < unread; i++ {
			if err := n.db.Create(&sysmodel.SysMessage{UserID: uid, Type: "notice", Title: "历史", Content: "历史"}).Error; err != nil {
				t.Fatalf("种未读消息失败: %v", err)
			}
		}
	}
	seed(notifyUser1, []string{"cid-u1-a", "cid-u1-b"}, 2)
	seed(notifyUser2, []string{"cid-u2-a"}, 0)

	if err := n.SendBatch([]string{notifyUser1, notifyUser2}, nil, "notice", "标题", "内容", nil); err != nil {
		t.Fatalf("SendBatch 失败: %v", err)
	}
	bodies := waitBodies(t, getBodies, 3)

	badgeOf := map[string]string{}
	for _, b := range bodies {
		if !b.hasChannl {
			t.Errorf("cid=%s 的请求缺少 push_channel（badge>0 应带 iOS 角标）", b.cid)
		}
		badgeOf[b.cid] = b.autoBadge
	}
	if badgeOf["cid-u1-a"] != "3" || badgeOf["cid-u1-b"] != "3" {
		t.Errorf("user1 角标 = %q/%q, 期望 3/3（历史 2 未读 + 本次 1 条）", badgeOf["cid-u1-a"], badgeOf["cid-u1-b"])
	}
	if badgeOf["cid-u2-a"] != "1" {
		t.Errorf("user2 角标 = %q, 期望 1（本次 1 条）", badgeOf["cid-u2-a"])
	}
}

// TestPushAsyncNoDeviceNoRequest 接收人无已绑设备时不发起任何个推请求（含 auth）。
func TestPushAsyncNoDeviceNoRequest(t *testing.T) {
	srv, getBodies := newGetuiMock(t)
	defer srv.Close()
	n := newTestNotifier(t, srv.URL)

	if err := n.Send(notifyUser1, "notice", "标题", "内容", nil); err != nil {
		t.Fatalf("Send 失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // 留足 goroutine 执行窗口
	if got := len(getBodies()); got != 0 {
		t.Fatalf("无设备时不应发起 push 请求，实际 %d 条", got)
	}
}
