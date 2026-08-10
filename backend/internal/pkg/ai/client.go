// Package ai OpenAI 兼容大模型视觉审核客户端（打卡照片智能审核）。
// 配置来自 sys_config 的 ai 分组（enabled/base_url/api_key/model/timeout_seconds/prompt），
// 每次调用实时读取，不缓存，配置改动即时生效。
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anxuncloud/internal/pkg/storage"
)

// 大模型审核结论（与 checkin_record.ai_verdict 取值一致）。
const (
	VerdictPass   = "pass"
	VerdictReview = "review"
	VerdictError  = "error"
)

const (
	maxPhotos          = 4
	maxPhotoBytes      = 8 << 20 // 单张图片上限 8MB
	defaultTimeoutSecs = 60
)

// builtinRules 内置审核规则（ai.prompt 非空时被整体替换，输出格式要求不变）。
const builtinRules = `你是物业巡检打卡审核助手。请根据打卡上下文与照片，判断本次打卡是否合规：
1. 照片是否清晰可辨（排除黑屏/白屏/严重模糊/与现场无关的图片）；
2. 照片内容与打卡点位、点位类型、检查项是否匹配；
3. 照片中是否存在明显的安全异常（设备损坏、漏水、明火、杂物阻塞消防通道等）；
4. 若照片按检查项分组给出（带检查项名称标注），逐项核对该项照片内容与该项检查要求是否一致。
只输出 JSON：{"verdict":"pass"|"review","reason":"简要中文理由","items":[{"name":"检查项名","verdict":"pass"|"review","reason":"该项简要理由"}]}，不要输出任何其他内容。
items 为逐项结论（与给出的检查项一一对应）；无法逐项判断时 items 可省略或为空数组。
有任何一项拿不准或存疑时，整体 verdict 一律输出 "review"，理由需说明疑点。`

// outputFormatHint 自定义 prompt 时仍强制要求的输出格式说明。
const outputFormatHint = "\n\n无论以上规则如何，你只输出 JSON：{\"verdict\":\"pass\"|\"review\",\"reason\":\"简要中文理由\",\"items\":[{\"name\":\"检查项名\",\"verdict\":\"pass\"|\"review\",\"reason\":\"该项简要理由\"}]}，不要输出任何其他内容；items 逐项结论无法判断时可省略；拿不准一律 review。"

// PhotoRef 待审核照片引用。
type PhotoRef struct {
	URL string
}

// ItemPhoto 检查项逐项照片（项名 + 该项照片），供大模型逐项核对。
type ItemPhoto struct {
	Name   string
	Photos []PhotoRef
}

// ItemVerdict 逐项大模型结论（检查项名 + 结论 + 理由）。
type ItemVerdict struct {
	Name    string
	Verdict string // pass / review
	Reason  string
}

// ReviewResult 大模型审核结果：整体结论 + 逐项结论（Items 可空，模型未返回时为空）。
type ReviewResult struct {
	Verdict string
	Reason  string
	Items   []ItemVerdict
}

// ReviewInput 打卡审核上下文。
type ReviewInput struct {
	PointName  string      // 点位名称
	PointType  string      // 点位类型
	CheckItems []string    // 检查项名称列表
	Remark     string      // 异常描述（可空）
	Photos     []PhotoRef  // 打卡照片（记录级/全景）
	ItemPhotos []ItemPhoto // 逐项照片（可空；空则回退仅按整组照片审核）
}

// Client OpenAI 兼容视觉审核客户端。
type Client struct {
	getCfg func(key string) (string, bool)
	store  *storage.Storage // 可空；dev 模式用于本地读照片转 base64
}

// Option 可选装配项。
type Option func(*Client)

// WithStorage 注入存储抽象（dev 模式从 URL 反推 file_key 读本地文件转 base64）。
func WithStorage(s *storage.Storage) Option {
	return func(c *Client) { c.store = s }
}

// NewClient 构造客户端；getCfg 每次调用时实时读取，不缓存。
func NewClient(getCfg func(key string) (string, bool), opts ...Option) *Client {
	c := &Client{getCfg: getCfg}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) cfg(key string) (string, bool) {
	if c.getCfg == nil {
		return "", false
	}
	return c.getCfg(key)
}

func (c *Client) cfgInt(key string, def int) int {
	if v, ok := c.cfg(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// Enabled ai.enabled=true 且 api_key 非空。
func (c *Client) Enabled() bool {
	v, ok := c.cfg("ai.enabled")
	if !ok || v != "true" {
		return false
	}
	key, _ := c.cfg("ai.api_key")
	return strings.TrimSpace(key) != ""
}

// ReviewCheckin 调用大模型审核打卡记录，返回整体 verdict（pass/review）、理由与逐项结论。
// 任何 HTTP/解析失败都返回 err，由调用方兜底（ai_verdict=error，不阻断业务）。
// 模型未返回逐项结论时 Items 为空，不视为错误。
func (c *Client) ReviewCheckin(ctx context.Context, input ReviewInput) (*ReviewResult, error) {
	baseURL, _ := c.cfg("ai.base_url")
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("ai.base_url 未配置")
	}
	apiKey, _ := c.cfg("ai.api_key")
	model, ok := c.cfg("ai.model")
	if !ok || strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("ai.model 未配置")
	}

	messages := c.buildMessages(input)
	payload := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": 1024,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpc := &http.Client{Timeout: time.Duration(c.cfgInt("ai.timeout_seconds", defaultTimeoutSecs)) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("大模型请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("大模型返回 %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(out.Choices[0].Message.Content)
}

// buildMessages 组装 vision messages：system 审核规则 + user 文本上下文与图片。
func (c *Client) buildMessages(input ReviewInput) []map[string]any {
	rules := builtinRules
	if p, ok := c.cfg("ai.prompt"); ok && strings.TrimSpace(p) != "" {
		rules = strings.TrimSpace(p) + outputFormatHint
	}

	var sb strings.Builder
	sb.WriteString("打卡点位：" + input.PointName + "\n")
	if input.PointType != "" {
		sb.WriteString("点位类型：" + input.PointType + "\n")
	}
	if len(input.CheckItems) > 0 {
		sb.WriteString("检查项：" + strings.Join(input.CheckItems, "、") + "\n")
	}
	if strings.TrimSpace(input.Remark) != "" {
		sb.WriteString("异常描述：" + input.Remark + "\n")
	}
	sb.WriteString("请审核以上打卡上下文及以下照片。")

	content := []map[string]any{{"type": "text", "text": sb.String()}}
	budget := maxPhotos
	if len(input.ItemPhotos) > 0 {
		// 逐项照片：每项一段文字标注后跟该项照片，让模型逐项核对
		for _, ip := range input.ItemPhotos {
			imgs := c.resolveImages(ip.Photos)
			if len(imgs) == 0 || budget <= 0 {
				continue
			}
			content = append(content, map[string]any{"type": "text", "text": "以下是检查项「" + ip.Name + "」的对应照片，请核对该项现场状态："})
			for _, img := range imgs {
				if budget <= 0 {
					break
				}
				content = append(content, map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": img},
				})
				budget--
			}
		}
	}
	if len(input.ItemPhotos) == 0 || budget > 0 {
		imgs := c.resolveImages(input.Photos)
		if len(imgs) > 0 {
			if len(input.ItemPhotos) > 0 {
				content = append(content, map[string]any{"type": "text", "text": "以下是本次打卡的全景/记录级照片："})
			}
			for _, img := range imgs {
				if budget <= 0 {
					break
				}
				content = append(content, map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": img},
				})
				budget--
			}
		}
	}
	return []map[string]any{
		{"role": "system", "content": rules},
		{"role": "user", "content": content},
	}
}

// resolveImages 图片供给：dev 模式读本地文件转 base64 data URL（读失败/超限跳过），OSS 模式直接传 URL。
func (c *Client) resolveImages(photos []PhotoRef) []string {
	out := make([]string, 0, maxPhotos)
	for _, p := range photos {
		if len(out) >= maxPhotos {
			break
		}
		if p.URL == "" {
			continue
		}
		if c.store != nil && c.store.IsDev() {
			idx := strings.Index(p.URL, "/uploads/")
			if idx < 0 {
				continue
			}
			key := p.URL[idx+len("/uploads/"):]
			path := c.store.LocalPath(key)
			info, err := os.Stat(path)
			if err != nil || info.Size() > maxPhotoBytes {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			mime := "image/jpeg"
			if strings.EqualFold(filepath.Ext(path), ".png") {
				mime = "image/png"
			}
			out = append(out, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
			continue
		}
		out = append(out, p.URL)
	}
	return out
}

// jsonRe 从模型输出中提取首个 JSON 对象（容错 Markdown 代码块/前后杂文本）。
var jsonRe = regexp.MustCompile(`\{[\s\S]*\}`)

// parseReview 解析模型输出；整体 verdict 非 pass/review 视为 error；
// items 逐项结论容错解析（缺失/非法项跳过，不影响整体结论）。
func parseReview(content string) (*ReviewResult, error) {
	m := jsonRe.FindString(content)
	if m == "" {
		return nil, fmt.Errorf("输出中未找到 JSON: %s", truncate(content, 200))
	}
	var v struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
		Items   []struct {
			Name    string `json:"name"`
			Verdict string `json:"verdict"`
			Reason  string `json:"reason"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(m), &v); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if v.Verdict != VerdictPass && v.Verdict != VerdictReview {
		return nil, fmt.Errorf("非法 verdict: %q", v.Verdict)
	}
	res := &ReviewResult{Verdict: v.Verdict, Reason: v.Reason}
	for _, it := range v.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" || (it.Verdict != VerdictPass && it.Verdict != VerdictReview) {
			continue // 非法逐项结论跳过，不报错
		}
		res.Items = append(res.Items, ItemVerdict{Name: name, Verdict: it.Verdict, Reason: it.Reason})
	}
	return res, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
