// Package ai 多协议大模型视觉审核客户端（打卡照片智能审核）。
// 支持 4 种接口协议（ai.protocol 配置）：openai_chat（OpenAI 兼容 Chat Completions，默认）、
// openai_responses（OpenAI Responses API）、gemini（Google Gemini generateContent）、claude（Anthropic Messages）。
// 配置来自 sys_config 的 ai 分组（enabled/protocol/base_url/api_key/model/timeout_seconds/prompt），
// 每次调用实时读取，不缓存，配置改动即时生效。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anxuncloud/internal/pkg/storage"
)

// 大模型审核结论（与 checkin_record.ai_verdict 取值一致）。
const (
	VerdictPass     = "pass"
	VerdictReview   = "review"
	VerdictAbnormal = "abnormal" // 逐项结论专用：确认存在明确异常
	VerdictError    = "error"
)

// 接口协议标识（ai.protocol 配置值）。
const (
	ProtocolOpenAIChat      = "openai_chat"      // 默认：OpenAI 兼容 Chat Completions
	ProtocolOpenAIResponses = "openai_responses" // OpenAI Responses API
	ProtocolGemini          = "gemini"           // Google Gemini generateContent
	ProtocolClaude          = "claude"           // Anthropic Messages
)

// 检查项判定类型（check_template_item.judge_type，打卡时快照进 checkin_record_item.judge_type）。
// 一期实现 10 类；baseline 基线对比为二期预留，一期按 general 处理。
const (
	JudgeGeneral   = "general"   // 通用综合判定（现状行为）
	JudgePresence  = "presence"  // 有无：设施/物品是否存在
	JudgeDamage    = "damage"    // 是否损坏：破损/锈蚀/变形
	JudgeMetric    = "metric"    // 指标区间：读取表计数值并判定区间（judge_config: metric/unit/min/max）
	JudgeState     = "state"     // 状态位置：实际状态是否符合期望（judge_config: expected）
	JudgeLabel     = "label"     // 有效期标识：检验日期/合格证/铅封
	JudgePassage   = "passage"   // 通道遮挡：通道占用/设施遮挡堆物
	JudgeLeak      = "leak"      // 渗漏痕迹：漏水/渗油/积水/水渍
	JudgeIndicator = "indicator" // 指示灯状态（judge_config.expected 可描述期望灯态）
	JudgeTidiness  = "tidiness"  // 环境整洁：杂物堆放
	JudgeBaseline  = "baseline"  // 基线对比（二期预留，一期按 general 处理）
	// JudgeManual 手动确认项：不拍照不调 AI，由巡检员现场手选正常/异常（如噪音、气味等照片无法判定的项）
	JudgeManual = "manual"
)

// NormalizeJudgeType 判定类型归一化：非法/空值回 general（兜底，不报错）。
func NormalizeJudgeType(jt string) string {
	switch strings.TrimSpace(jt) {
	case JudgePresence, JudgeDamage, JudgeMetric, JudgeState, JudgeLabel,
		JudgePassage, JudgeLeak, JudgeIndicator, JudgeTidiness, JudgeBaseline, JudgeManual:
		return strings.TrimSpace(jt)
	}
	return JudgeGeneral
}

const (
	maxPhotos          = 4
	maxPhotoBytes      = 8 << 20 // 单张图片上限 8MB
	defaultTimeoutSecs = 60
)

// builtinRules 内置审核规则（ai.prompt 非空时被整体替换，输出格式要求不变）。
// 两层判定：先照片质量（模糊/过暗/未拍到目标设施），再逐项内容判定。
const builtinRules = `你是物业巡检打卡审核助手。请根据打卡上下文与照片完成两层判定：
第一层【照片质量】：判断照片是否清晰可辨、是否拍到了目标设施（排除黑屏/白屏/严重模糊/过暗/与现场无关的图片）；质量不达标时 quality.pass=false，并在 quality.issue 给出面向巡检员的简要中文提示（如"照片模糊，请重新拍摄"）。
第二层【内容判定】：
1. 照片内容与打卡点位、点位类型、检查项是否匹配；
2. 照片中是否存在明显的安全异常（设备损坏、漏水、明火、杂物阻塞消防通道等）；
3. 若照片按检查项分组给出（带检查项名称与判定要求标注），逐项按标注的判定要求核对该项照片；判定要求涉及表计读数的项，将读出的数值填入 reading。
只输出 JSON：{"quality":{"pass":true|false,"issue":""},"verdict":"pass"|"review","reason":"简要中文理由","items":[{"name":"检查项名","verdict":"pass"|"review"|"abnormal","reason":"该项简要理由","reading":""}]}，不要输出任何其他内容。
items 为逐项结论（与给出的检查项一一对应）；无法逐项判断时 items 可省略或为空数组；reading 仅表计读数类检查项填写，其余留空。
quality.pass=false 时 verdict 仍照常给出；逐项确认存在明确异常时该项 verdict 输出 "abnormal"；有任何一项拿不准或存疑时，整体 verdict 一律输出 "review"，理由需说明疑点。`

// outputFormatHint 自定义 prompt 时仍强制要求的输出格式说明。
const outputFormatHint = "\n\n无论以上规则如何，你只输出 JSON：{\"quality\":{\"pass\":true|false,\"issue\":\"\"},\"verdict\":\"pass\"|\"review\",\"reason\":\"简要中文理由\",\"items\":[{\"name\":\"检查项名\",\"verdict\":\"pass\"|\"review\"|\"abnormal\",\"reason\":\"该项简要理由\",\"reading\":\"\"}]}，不要输出任何其他内容；items 逐项结论无法判断时可省略；拿不准一律 review。"

// PhotoRef 待审核照片引用。
type PhotoRef struct {
	URL string
}

// ItemPhoto 检查项逐项照片（项名 + 标准要求 + AI 识别要点 + 判定类型/参数 + 该项照片），供大模型逐项核对。
type ItemPhoto struct {
	Name        string
	Requirement string         // 检查标准要求（可空）
	AIHint      string         // AI 识别要点（可空；空=该项不带识别要点）
	JudgeType   string         // 判定类型（可空；空=general 通用判定）
	JudgeConfig map[string]any // 判定参数（可空；metric 需 metric/unit/min/max，state/indicator 用 expected）
	Photos      []PhotoRef
}

// ItemVerdict 逐项大模型结论（检查项名 + 结论 + 理由 + 表计读数）。
type ItemVerdict struct {
	Name    string
	Verdict string // pass / review / abnormal
	Reason  string
	Reading string // 表计读数文本（metric 类检查项；可空）
}

// QualityResult 照片质量判定（第一层）；Pass=false 时 Issue 为面向巡检员的重拍提示。
type QualityResult struct {
	Pass  bool
	Issue string
}

// ReviewResult 大模型审核结果：照片质量 + 整体结论 + 逐项结论（Items 可空，模型未返回时为空）。
type ReviewResult struct {
	Verdict string
	Reason  string
	Quality QualityResult
	Items   []ItemVerdict
}

// ReviewInput 打卡审核上下文。
type ReviewInput struct {
	PointName  string      // 点位名称
	PointType  string      // 点位类型
	CheckItems []string    // 检查项名称列表
	Remark     string      // 异常描述（可空）
	ItemPhotos []ItemPhoto // 逐项照片（一项一图；无记录级照片）
}

// Client 多协议视觉审核客户端。
type Client struct {
	getCfg func(key string) (string, bool)
	store  *storage.Storage // 可空；local 模式用于本地读照片转 base64
}

// Option 可选装配项。
type Option func(*Client)

// WithStorage 注入存储抽象（local 模式从 URL 反推 file_key 读本地文件转 base64）。
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

// ReviewCheckin 调用大模型审核打卡记录，按 ai.protocol 分发到对应协议适配器，
// 返回照片质量 + 整体 verdict（pass/review）+ 逐项结论。
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

	httpc := &http.Client{Timeout: time.Duration(c.cfgInt("ai.timeout_seconds", defaultTimeoutSecs)) * time.Second}
	protocol, _ := c.cfg("ai.protocol")
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", ProtocolOpenAIChat:
		return c.callOpenAIChat(ctx, httpc, baseURL, apiKey, model, input)
	case ProtocolOpenAIResponses:
		return c.callOpenAIResponses(ctx, httpc, baseURL, apiKey, model, input)
	case ProtocolGemini:
		return c.callGemini(ctx, httpc, baseURL, apiKey, model, input)
	case ProtocolClaude:
		return c.callClaude(ctx, httpc, baseURL, apiKey, model, input)
	default:
		return nil, fmt.Errorf("未知 ai.protocol: %q", protocol)
	}
}

// rules 审核规则文本：ai.prompt 非空时整体替换内置规则（输出格式要求强制追加）。
func (c *Client) rules() string {
	if p, ok := c.cfg("ai.prompt"); ok && strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p) + outputFormatHint
	}
	return builtinRules
}

// promptPart 待发送内容段：文本或图片（各协议适配器自行映射为各自的内容结构）。
type promptPart struct {
	text string
	img  *resolvedImage
}

// buildParts 组装审核内容段：上下文文本 → 逐项标注+该项照片 → 全景/记录级照片。
// 各协议适配器共用；maxPhotos 为图片总预算。
func (c *Client) buildParts(input ReviewInput) []promptPart {
	parts := []promptPart{{text: contextText(input)}}
	budget := maxPhotos
	if len(input.ItemPhotos) > 0 {
		// 逐项照片：每项一段文字标注后跟该项照片，让模型逐项核对
		for _, ip := range input.ItemPhotos {
			imgs := c.resolveImages(ip.Photos)
			// 按项标注：项名 + 标准要求 + AI 识别要点（§3.3）+ 判定类型专项指令，让模型逐项对照识别
			label := "以下是检查项「" + ip.Name + "」"
			if strings.TrimSpace(ip.Requirement) != "" {
				label += "（标准要求：" + strings.TrimSpace(ip.Requirement) + "）"
			}
			if strings.TrimSpace(ip.AIHint) != "" {
				label += "（AI 识别要点：" + strings.TrimSpace(ip.AIHint) + "）"
			}
			label += "的对应照片"
			if inst := judgeInstruction(ip.JudgeType, ip.JudgeConfig); inst != "" {
				label += "（判定要求：" + inst + "）"
			}
			if len(imgs) == 0 {
				// 该项无照片：仅发文字标注；无图可核对时要求模型判存疑，不凭空结论
				parts = append(parts, promptPart{text: label + "：该项未提供照片，无法进行图像核对时请将该项判为存疑。"})
				continue
			}
			if budget <= 0 {
				continue
			}
			parts = append(parts, promptPart{text: label + "，请核对该项现场状态："})
			for _, img := range imgs {
				if budget <= 0 {
					break
				}
				parts = append(parts, promptPart{img: &img})
				budget--
			}
		}
	}
	return parts
}

// contextText 打卡上下文文本（点位/类型/检查项/异常描述）。
func contextText(input ReviewInput) string {
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
	return sb.String()
}

// judgeInstruction 按判定类型生成逐项判定指令文本；general/baseline（二期预留按通用处理）、
// manual（手动确认项，不调 AI 不入 prompt）与参数不全的项返回空串（不追加标注，按通用判定）。
func judgeInstruction(jt string, cfg map[string]any) string {
	cfgStr := func(key string) string {
		v, _ := cfg[key].(string)
		return strings.TrimSpace(v)
	}
	switch NormalizeJudgeType(jt) {
	case JudgePresence:
		return "判断照片中是否存在该检查项所述设施/物品，不存在则该项 verdict=abnormal"
	case JudgeDamage:
		return "判断设施外观是否破损、锈蚀、变形，存在则该项 verdict=abnormal"
	case JudgeMetric:
		metric := cfgStr("metric")
		if metric == "" {
			return "" // judge_config 缺失时按 general
		}
		unit := cfgStr("unit")
		lo, lok := cfg["min"]
		hi, hik := cfg["max"]
		if !lok || !hik {
			return ""
		}
		return fmt.Sprintf("读取照片中%s表计的数值填入 reading（单位：%s），并判断是否在允许区间 [%v, %v] 内，超出区间该项 verdict=abnormal，无法读出数值则该项 verdict=review", metric, unit, lo, hi)
	case JudgeState:
		expected := cfgStr("expected")
		if expected == "" {
			return ""
		}
		return "判断实际状态是否符合期望（" + expected + "），不符合则该项 verdict=abnormal"
	case JudgeLabel:
		return "识别检验日期/合格证/铅封信息，已过期或缺失则该项 verdict=abnormal"
	case JudgePassage:
		return "判断通道是否被占用、设施是否被遮挡或堆物，存在则该项 verdict=abnormal"
	case JudgeLeak:
		return "判断是否有漏水、渗油、积水、水渍痕迹，存在则该项 verdict=abnormal"
	case JudgeIndicator:
		inst := "判断指示灯/运行灯状态是否正常"
		if expected := cfgStr("expected"); expected != "" {
			inst += "（期望灯态：" + expected + "）"
		}
		return inst + "，异常则该项 verdict=abnormal"
	case JudgeTidiness:
		return "判断环境是否整洁、有无杂物堆放，不达标则该项 verdict=abnormal"
	case JudgeManual:
		return "" // 手动确认项不调 AI，不生成判定指令
	}
	return ""
}

// jsonRe 从模型输出中提取首个 JSON 对象（容错 Markdown 代码块/前后杂文本）。
var jsonRe = regexp.MustCompile(`\{[\s\S]*\}`)

// parseReview 解析模型输出；整体 verdict 非 pass/review 视为 error；
// quality 缺失（旧格式）视为 pass；items 逐项结论容错解析（缺失/非法项跳过，不影响整体结论）。
func parseReview(content string) (*ReviewResult, error) {
	m := jsonRe.FindString(content)
	if m == "" {
		return nil, fmt.Errorf("输出中未找到 JSON: %s", truncate(content, 200))
	}
	var v struct {
		Quality *struct {
			Pass  *bool  `json:"pass"`
			Issue string `json:"issue"`
		} `json:"quality"`
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
		Items   []struct {
			Name    string `json:"name"`
			Verdict string `json:"verdict"`
			Reason  string `json:"reason"`
			Reading string `json:"reading"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(m), &v); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if v.Verdict != VerdictPass && v.Verdict != VerdictReview {
		return nil, fmt.Errorf("非法 verdict: %q", v.Verdict)
	}
	res := &ReviewResult{Verdict: v.Verdict, Reason: v.Reason, Quality: QualityResult{Pass: true}}
	if v.Quality != nil && v.Quality.Pass != nil {
		res.Quality.Pass = *v.Quality.Pass
		res.Quality.Issue = v.Quality.Issue
	}
	for _, it := range v.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" || (it.Verdict != VerdictPass && it.Verdict != VerdictReview && it.Verdict != VerdictAbnormal) {
			continue // 非法逐项结论跳过，不报错
		}
		res.Items = append(res.Items, ItemVerdict{Name: name, Verdict: it.Verdict, Reason: it.Reason, Reading: strings.TrimSpace(it.Reading)})
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
