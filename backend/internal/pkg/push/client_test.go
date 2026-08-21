package push

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
// pushCodeHook 可按次数改写 push 响应码（模拟 10001 token 失效）；pushBodyHook 拿到 push 原始请求体（可为 nil）。
func newTestServer(t *testing.T, authCalls, pushCalls *int64, pushCodeHook func(n int64) int, pushBodyHook func(body []byte)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/" + testAppID + "/auth":
			atomic.AddInt64(authCalls, 1)
			var req struct {
				Sign      string `json:"sign"`
				Timestamp string `json:"timestamp"`
				AppKey    string `json:"appkey"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("auth 请求体解析失败: %v", err)
			}
			if req.AppKey != testAppKey {
				t.Errorf("auth 请求体缺 appkey: got %q", req.AppKey)
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
			raw, _ := io.ReadAll(r.Body)
			if pushBodyHook != nil {
				pushBodyHook(raw)
			}
			var req struct {
				RequestID string `json:"request_id"`
				Audience  struct {
					CID []string `json:"cid"`
				} `json:"audience"`
				PushMessage struct {
					Notification struct {
						Title     string `json:"title"`
						Body      string `json:"body"`
						ClickType string `json:"click_type"`
						Payload   string `json:"payload"`
					} `json:"notification"`
				} `json:"push_message"`
			}
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Errorf("push 请求体解析失败: %v", err)
			}
			if req.RequestID == "" || len(req.Audience.CID) == 0 || req.Audience.CID[0] == "" {
				t.Errorf("request_id/audience.cid 为空: %+v", req)
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
	srv := newTestServer(t, &authCalls, &pushCalls, nil, nil)
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if !cli.Enabled() {
		t.Fatal("三要素齐全应 Enabled")
	}
	ok, failed, err := cli.PushToCIDs(context.Background(), []string{"cid-1", "cid-2"}, "标题", "内容", map[string]string{
		"type":   "workorder",
		"biz_id": "2f6f0d6e-0000-7000-8000-000000000001",
	}, 0)
	if err != nil {
		t.Fatalf("PushToCIDs 失败: %v", err)
	}
	if ok != 2 || failed != 0 {
		t.Fatalf("ok/failed = %d/%d, 期望 2/0", ok, failed)
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
	}, nil)
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if _, _, err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil, 0); err != nil {
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
	srv := newTestServer(t, &authCalls, &pushCalls, func(int64) int { return 30001 }, nil)
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	ok, failed, err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil, 0)
	if err == nil {
		t.Fatal("应返回推送失败错误")
	}
	if ok != 0 || failed != 1 {
		t.Fatalf("ok/failed = %d/%d, 期望 0/1", ok, failed)
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
	if _, _, err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil, 0); err == nil {
		t.Fatal("未配置时应返回错误")
	}
}

// TestPushBadgeIOS badge>0 时请求体带 push_channel.ios：type=notify、aps.alert 复用标题/内容、
// auto_badge 为绝对角标数字符串、payload 透传（个推 V2 文档结构，无 ios.badge 字段）。
func TestPushBadgeIOS(t *testing.T) {
	var authCalls, pushCalls int64
	var bodies [][]byte
	srv := newTestServer(t, &authCalls, &pushCalls, nil, func(body []byte) {
		bodies = append(bodies, body)
	})
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	ok, failed, err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "标题", "内容", map[string]string{"type": "workorder"}, 5)
	if err != nil || ok != 1 || failed != 0 {
		t.Fatalf("PushToCIDs: ok/failed/err = %d/%d/%v, 期望 1/0/nil", ok, failed, err)
	}
	if len(bodies) != 1 {
		t.Fatalf("push 请求数 = %d, 期望 1", len(bodies))
	}
	var req struct {
		PushChannel struct {
			IOS struct {
				Type    string `json:"type"`
				Payload string `json:"payload"`
				APS     struct {
					Alert struct {
						Title string `json:"title"`
						Body  string `json:"body"`
					} `json:"alert"`
				} `json:"aps"`
				AutoBadge string `json:"auto_badge"`
			} `json:"ios"`
		} `json:"push_channel"`
	}
	if err := json.Unmarshal(bodies[0], &req); err != nil {
		t.Fatalf("push 请求体解析失败: %v", err)
	}
	ios := req.PushChannel.IOS
	if ios.Type != "notify" {
		t.Errorf("push_channel.ios.type = %q, 期望 notify", ios.Type)
	}
	if ios.AutoBadge != "5" {
		t.Errorf("push_channel.ios.auto_badge = %q, 期望 \"5\"", ios.AutoBadge)
	}
	if ios.APS.Alert.Title != "标题" || ios.APS.Alert.Body != "内容" {
		t.Errorf("push_channel.ios.aps.alert = %+v, 期望复用通知标题/内容", ios.APS.Alert)
	}
	if ios.Payload == "" {
		t.Error("push_channel.ios.payload 不应为空（个推文档：非组件推送时必填）")
	}
}

// TestPushNoBadgeNoChannel badge=0 时不带 push_channel（Android 角标靠端内 setBadgeNumber 同步）。
func TestPushNoBadgeNoChannel(t *testing.T) {
	var authCalls, pushCalls int64
	var bodies [][]byte
	srv := newTestServer(t, &authCalls, &pushCalls, nil, func(body []byte) {
		bodies = append(bodies, body)
	})
	defer srv.Close()

	cli := NewClient(testAppID, testAppKey, testMasterSecret)
	cli.SetBaseURL(srv.URL)
	if _, _, err := cli.PushToCIDs(context.Background(), []string{"cid-1"}, "t", "b", nil, 0); err != nil {
		t.Fatalf("PushToCIDs 失败: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("push 请求数 = %d, 期望 1", len(bodies))
	}
	var req map[string]any
	if err := json.Unmarshal(bodies[0], &req); err != nil {
		t.Fatalf("push 请求体解析失败: %v", err)
	}
	if _, ok := req["push_channel"]; ok {
		t.Errorf("badge=0 时不应带 push_channel: %s", string(bodies[0]))
	}
}
