// Package push 个推（getui）REST API V2 客户端：uniPush 2.0 服务端推送出口。
// 三要素（appID/appKey/masterSecret）任一为空即视为未启用，Enabled()=false，调用方跳过推送。
package push

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// defaultBaseURL 个推 REST API V2 默认地址（测试可经 SetBaseURL 注入 httptest server）。
const defaultBaseURL = "https://restapi.getui.com"

// tokenExpireSkew token 过期提前量（服务端给的过期时间减去该值后本地即视为过期）。
const tokenExpireSkew = 5 * time.Minute

// Client 个推 V2 客户端（token 内存缓存，并发安全）。
type Client struct {
	appID        string
	appKey       string
	masterSecret string
	baseURL      string
	http         *http.Client

	mu          sync.Mutex
	token       string
	tokenExpire time.Time
}

// NewClient 创建客户端；三要素任一为空时 Enabled()=false（不发起任何请求）。
// 值做 TrimSpace：docker compose env_file 解析 CRLF 行尾的 .env 时会把 \r 带进值里（个推 400 的常见根因）。
func NewClient(appID, appKey, masterSecret string) *Client {
	return &Client{
		appID:        strings.TrimSpace(appID),
		appKey:       strings.TrimSpace(appKey),
		masterSecret: strings.TrimSpace(masterSecret),
		baseURL:      defaultBaseURL,
		http:         &http.Client{Timeout: 10 * time.Second},
	}
}

// SetBaseURL 覆盖 API 地址（仅测试注入用）。
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// Enabled 三要素是否齐全（未配置=推送关闭，站内通知不受影响）。
func (c *Client) Enabled() bool {
	return c.appID != "" && c.appKey != "" && c.masterSecret != ""
}

// authResp / pushResp 个推统一响应壳（code=0 成功；10001=token 失效）。
type apiResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type authData struct {
	Token      string `json:"token"`
	ExpireTime string `json:"expire_time"` // 毫秒时间戳字符串
}

// auth 鉴权取 token：sign=sha256(appkey+timestamp+masterSecret) 十六进制。
func (c *Client) auth(ctx context.Context) error {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sum := sha256.Sum256([]byte(c.appKey + timestamp + c.masterSecret))
	body, _ := json.Marshal(map[string]string{
		"sign":      hex.EncodeToString(sum[:]),
		"timestamp": timestamp,
	})
	var resp apiResp
	if err := c.doPost(ctx, "/v2/"+c.appID+"/auth", "", body, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("个推鉴权失败: code=%d msg=%s", resp.Code, resp.Msg)
	}
	var data authData
	if err := json.Unmarshal(resp.Data, &data); err != nil || data.Token == "" {
		return fmt.Errorf("个推鉴权响应异常: %s", string(resp.Data))
	}
	c.token = data.Token
	c.tokenExpire = time.Now().Add(time.Hour) // 兜底：响应缺 expire_time 时按 1 小时缓存
	if ms, err := strconv.ParseInt(data.ExpireTime, 10, 64); err == nil && ms > 0 {
		c.tokenExpire = time.UnixMilli(ms).Add(-tokenExpireSkew)
	}
	return nil
}

// getToken 取有效 token（过期/失效自动重取，mutex 防并发重复鉴权）。
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpire) {
		return c.token, nil
	}
	if err := c.auth(ctx); err != nil {
		return "", err
	}
	return c.token, nil
}

// invalidateToken 清除缓存 token（10001 token 失效时调用，下次推送前重新鉴权）。
func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.tokenExpire = time.Time{}
}

// PushToCIDs 推送通知到一批 cid（逐个走 /push/single/cid；任一失败返回首个错误，不 panic）。
// payload 为点击通知后传给 App 的自定义数据（至少含 type/biz_id），序列化为 JSON 字符串下发。
func (c *Client) PushToCIDs(ctx context.Context, cids []string, title, body string, payload map[string]string) error {
	if !c.Enabled() {
		return fmt.Errorf("个推未配置（UNIPUSH_APPID/APPKEY/MASTERSECRET）")
	}
	if len(cids) == 0 {
		return nil
	}
	payloadJSON, _ := json.Marshal(payload)
	var firstErr error
	for _, cid := range cids {
		if cid == "" {
			continue
		}
		if err := c.pushSingle(ctx, cid, title, body, string(payloadJSON)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// pushSingle 单 cid 推送；遇 10001（token 失效）重取 token 后重试一次。
func (c *Client) pushSingle(ctx context.Context, cid, title, body, payload string) error {
	reqBody, _ := json.Marshal(map[string]any{
		"request_id": uuid.Must(uuid.NewV7()).String(),
		"cid":        cid,
		"push_message": map[string]any{
			"notification": map[string]string{
				"title":      title,
				"body":       body,
				"click_type": "payload_custom",
				"payload":    payload,
			},
		},
	})
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.getToken(ctx)
		if err != nil {
			return err
		}
		var resp apiResp
		if err := c.doPost(ctx, "/v2/"+c.appID+"/push/single/cid", token, reqBody, &resp); err != nil {
			return err
		}
		if resp.Code == 0 {
			return nil
		}
		if resp.Code == 10001 && attempt == 0 { // token 失效：重取后重试一次
			c.invalidateToken()
			continue
		}
		return fmt.Errorf("个推推送失败: code=%d msg=%s cid=%s", resp.Code, resp.Msg, cid)
	}
	return fmt.Errorf("个推推送失败: token 重取后仍失效 cid=%s", cid)
}

// doPost 发起 JSON POST 并解析统一响应壳（token 为空则不带鉴权头，仅 auth 接口如此）。
func (c *Client) doPost(ctx context.Context, path, token string, body []byte, out *apiResp) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if token != "" {
		req.Header.Set("token", token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("个推接口 HTTP %d: %s", resp.StatusCode, string(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("个推响应解析失败: %w", err)
	}
	return nil
}
