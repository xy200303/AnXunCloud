package push

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testAppID        = "testappid"
	testAppKey       = "testappkey"
	testMasterSecret = "testmastersecret"
)

// newTestServer 模拟个推 V2：auth 校验 sign，push 校验 token 与通知体；authCalls/pushCalls 计数。
// pushCodeHook 可按次数改写 push 响应码（模拟 10001 token 失效）。
func newTestServer(t *testing.T, authCalls, pushCalls *int64, pushCodeHook func(n int64) int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/" + testAppID + "/auth":
			atomic.AddInt64(authCalls, 1)
			var req struct {
				Sign      string `json:"sign"`
				Timestamp string `json:"timestamp"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("auth 请求体解析失败: %v", err)
			}
			sum := sha256.Sum256([]byte(testAppKey + req.Timestamp + testMasterSecret))
			if req.Sign != hex.EncodeToString(sum[:]) {
				t.Errorf("sign 校验失败: got %s", req.Sign)
			}
			if _, err := strconv.ParseInt(req.Timestamp, 10, 64); err != nil {
				t.Errorf("timestamp 非毫秒数字: %s", req.Timestamp)
			}
			fmt.Fprintf(w, `{"code":0,"msg":"success","data":{"token":"token-%d","expire_time":"%d"}}`,
				atomic.LoadInt64(authCalls), time.Now().Add(time.Hour).UnixMilli())
		case "/v2/" + testAppID + "/push/single/cid":
			n := atomic.AddInt64(pushCalls, 1)
			if r.Header.Get("token") == "" {
				t.Error("push 请求缺少 token 头")
			}
			var req struct {
				RequestID   string `json:"request_id"`
				CID         string `json:"cid"`
				PushMessage struct {
					Notification struct {
						Title     string `json:"title"`
						Body      string `json:"body"`
						ClickType string `json:"click_type"`
						Payload   string `json:"payload"`
					} `json:"notification"`
				} `json:"push_message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("push 请求体解析失败: %v", err)
			}
			if req.RequestID == "" || req.CID == "" {
				t.Errorf("request_id/cid 为空: %+v", req)
			}
			if req.PushMessage.Notification.ClickType != "payload_custom" {
				t.Errorf("click_type = %s, 期望 payload_custom", req.PushMessage.Notification.ClickType)
			}
			code := 0
			if pushCodeHook != nil {
				code = pushCodeHook(n)
			}
			fmt.Fprintf(w, `{"code":%d,"msg":"ok","data":{}}`, code)
		default:
			t.Errorf("未知请求路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestAuthAndPush 验证 auth 路径/sign 计算、push 路径/通知体、token 缓存（两次推送只鉴权一次）。
func TestAuthAndPush(t *testing.T) {
	var authCalls, pushCalls int64
	srv := newTestServer(t, &authCalls, &pushCalls, nil)
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if !cli.Enabled() {
		t.Fatal("三要素齐全应 Enabled")
	}
	err := cli.PushToCIDs(context.Background(), []string{"cid-1", "cid-2"}, "标题", "内容", map[string]string{
		"type":   "workorder",
		"biz_id": "2f6f0d6e-0000-7000-8000-000000000001",
	})
	if err != nil {
		t.Fatalf("PushToCIDs 失败: %v", err)
	}
	if pushCalls != 2 {
		t.Fatalf("push 调用次数 = %d, 期望 2", pushCalls)
	}
	if authCalls != 1 {
		t.Fatalf("auth 调用次数 = %d, 期望 1（token 应缓存复用）", authCalls)
	}
}

// TestTokenRefreshOn10001 模拟首次推送返回 10001（token 失效）：应重新鉴权并重试成功。
func TestTokenRefreshOn10001(t *testing.T) {
	var authCalls, pushCalls int64
	srv := newTestServer(t, &authCalls, &pushCalls, func(n int64) int {
		if n == 1 {
			return 10001
		}
		return 0
	})
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil); err != nil {
		t.Fatalf("10001 后应重试成功: %v", err)
	}
	if authCalls != 2 {
		t.Fatalf("auth 调用次数 = %d, 期望 2（失效后重新鉴权）", authCalls)
	}
	if pushCalls != 2 {
		t.Fatalf("push 调用次数 = %d, 期望 2（首次失败 + 重试）", pushCalls)
	}
}

// TestPushError 业务错误码（非 10001）直接报错，不重试。
func TestPushError(t *testing.T) {
	var authCalls, pushCalls int64
	srv := newTestServer(t, &authCalls, &pushCalls, func(int64) int { return 30001 })
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil); err == nil {
		t.Fatal("应返回推送失败错误")
	}
	if pushCalls != 1 {
		t.Fatalf("push 调用次数 = %d, 期望 1（非 token 错误不重试）", pushCalls)
	}
}

// TestDisabled 三要素不齐时 Enabled=false，推送直接报错且不发起任何请求。
func TestDisabled(t *testing.T) {
	cli := NewClient("", "", "")
	if cli.Enabled() {
		t.Fatal("三要素为空不应 Enabled")
	}
	if err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil); err == nil {
		t.Fatal("未配置时应返回错误")
	}
}
