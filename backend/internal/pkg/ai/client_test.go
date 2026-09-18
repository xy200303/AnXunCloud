package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseReviewWithItems 逐项结论：整体 + 逐项均解析；非法逐项跳过不报错。
func TestParseReviewWithItems(t *testing.T) {
	res, err := parseReview(`{"quality":{"pass":true},"verdict":"review","reason":"保险销疑似脱落","items":[
		{"name":"压力表指针在绿区","verdict":"pass","reason":"指针在绿区"},
		{"name":"保险销完好","verdict":"review","reason":"保险销缺失"}
	]}`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Verdict != VerdictReview || res.Reason != "保险销疑似脱落" {
		t.Errorf("整体结论异常: %+v", res)
	}
	if len(res.Items) != 2 {
		t.Fatalf("逐项结论数=%d，期望 2（非法项跳过）", len(res.Items))
	}
	if res.Items[0].Name != "压力表指针在绿区" || res.Items[0].Verdict != VerdictPass {
		t.Errorf("逐项[0]异常: %+v", res.Items[0])
	}
	if res.Items[1].Name != "保险销完好" || res.Items[1].Verdict != VerdictReview || res.Items[1].Reason != "保险销缺失" {
		t.Errorf("逐项[1]异常: %+v", res.Items[1])
	}
}

// TestParseReviewRequiresCompleteContract 缺失质量或逐项结论必须拒绝，避免错误放行。
func TestParseReviewRequiresCompleteContract(t *testing.T) {
	if _, err := parseReview(`{"verdict":"pass","reason":"照片清晰","items":[{"name":"检查项","verdict":"pass"}]}`); err == nil {
		t.Fatal("缺少质量结论应报错")
	}
	if _, err := parseReview(`{"quality":{"pass":true},"verdict":"pass","reason":"照片清晰"}`); err == nil {
		t.Fatal("缺少逐项结论应报错")
	}
}

// TestParseReviewInvalidVerdict 整体 verdict 非法仍报错。
func TestParseReviewInvalidVerdict(t *testing.T) {
	if _, err := parseReview(`{"verdict":"unknown","reason":"x"}`); err == nil {
		t.Fatal("非法 verdict 应报错")
	}
	if _, err := parseReview("没有 JSON 的输出"); err == nil {
		t.Fatal("无 JSON 应报错")
	}
}

// TestBuildMessagesWithItemPhotos 逐项照片：prompt 中按检查项分组标注，模型可逐项核对。
func TestBuildMessagesWithItemPhotos(t *testing.T) {
	c := NewClient(func(string) (string, bool) { return "", false })
	msgs := c.buildMessages(ReviewInput{
		PointName:  "配电房",
		PointType:  "fire_control",
		CheckItems: []string{"压力表指针在绿区", "保险销完好"},
		ItemPhotos: []ItemPhoto{
			{Name: "压力表指针在绿区", Photos: []PhotoRef{{URL: "http://example.com/gauge.jpg"}}},
			{Name: "保险销完好", Photos: []PhotoRef{{URL: "http://example.com/pin.jpg"}}},
		},
	})
	if len(msgs) != 2 {
		t.Fatalf("messages 条数=%d，期望 2", len(msgs))
	}
	content, ok := msgs[1]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("user content 类型异常: %T", msgs[1]["content"])
	}
	var texts []string
	imgs := 0
	for _, part := range content {
		switch part["type"] {
		case "text":
			texts = append(texts, part["text"].(string))
		case "image_url":
			imgs++
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "检查项「压力表指针在绿区」的对应照片") {
		t.Errorf("缺逐项照片标注（压力表）：%s", joined)
	}
	if !strings.Contains(joined, "检查项「保险销完好」的对应照片") {
		t.Errorf("缺逐项照片标注（保险销）：%s", joined)
	}
	if imgs != 2 {
		t.Errorf("图片数=%d，期望 2（逐项各一张）", imgs)
	}
}

// TestBuildMessagesItemHints 逐项标注带标准要求与 AI 识别要点（§3.3）；无要点的项不追加标注。
func TestBuildMessagesItemHints(t *testing.T) {
	c := NewClient(func(string) (string, bool) { return "", false })
	msgs := c.buildMessages(ReviewInput{
		PointName: "1栋1层电梯厅消防箱",
		ItemPhotos: []ItemPhoto{
			{Name: "灭火器压力正常", Requirement: "指针在绿区", AIHint: "指针位于绿色区域",
				Photos: []PhotoRef{{URL: "http://example.com/gauge.jpg"}}},
			{Name: "箱体与通道", Photos: []PhotoRef{{URL: "http://example.com/box.jpg"}}},
		},
	})
	content := msgs[1]["content"].([]map[string]any)
	var texts []string
	for _, part := range content {
		if part["type"] == "text" {
			texts = append(texts, part["text"].(string))
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "检查项「灭火器压力正常」（标准要求：指针在绿区）（AI 识别要点：指针位于绿色区域）的对应照片") {
		t.Errorf("逐项标注缺标准要求/AI 识别要点：%s", joined)
	}
	if strings.Contains(joined, "检查项「箱体与通道」（") {
		t.Errorf("无标准要求/识别要点的项不应追加标注：%s", joined)
	}
}

// TestPostJSONRateLimit 429 响应映射为 *RateLimitError（worker 侧指数退避重试依据），
// 错误文本保持"大模型返回 429: ..."口径；普通状态码不误判为限流。
func TestPostJSONRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/limited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	})
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := postJSON(context.Background(), srv.Client(), srv.URL+"/limited", nil, map[string]any{})
	if !IsRateLimited(err) {
		t.Fatalf("429 应识别为限流错误，got %v", err)
	}
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("RateLimitError 状态码不符: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "大模型返回 429") {
		t.Errorf("错误文本口径变化: %v", err)
	}

	_, err = postJSON(context.Background(), srv.Client(), srv.URL+"/broken", nil, map[string]any{})
	if err == nil || IsRateLimited(err) {
		t.Fatalf("500 不应识别为限流错误，got %v", err)
	}
	if IsRateLimited(nil) {
		t.Fatal("nil 不应识别为限流错误")
	}
}
