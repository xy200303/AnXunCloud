package ai

import (
	"strings"
	"testing"
)

// TestParseReviewWithItems 逐项结论：整体 + 逐项均解析；非法逐项跳过不报错。
func TestParseReviewWithItems(t *testing.T) {
	res, err := parseReview(`{"verdict":"review","reason":"保险销疑似脱落","items":[
		{"name":"压力表指针在绿区","verdict":"pass","reason":"指针在绿区"},
		{"name":"保险销完好","verdict":"review","reason":"保险销缺失"},
		{"name":"","verdict":"pass","reason":"无名项跳过"},
		{"name":"喷管无龟裂","verdict":"maybe","reason":"非法结论跳过"}
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

// TestParseReviewWithoutItems 模型未返回逐项结论：Items 为空，不报错（旧输出格式兼容）。
func TestParseReviewWithoutItems(t *testing.T) {
	res, err := parseReview("前置杂文本```json\n{\"verdict\":\"pass\",\"reason\":\"照片清晰，无异常\"}\n```")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict=%q，期望 pass", res.Verdict)
	}
	if len(res.Items) != 0 {
		t.Errorf("逐项结论应为空: %+v", res.Items)
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
		Photos:     []PhotoRef{{URL: "http://example.com/pano.jpg"}},
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
	if !strings.Contains(joined, "全景/记录级照片") {
		t.Errorf("缺全景照片标注：%s", joined)
	}
	if imgs != 3 {
		t.Errorf("图片数=%d，期望 3（2 张逐项 + 1 张全景）", imgs)
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

// TestBuildMessagesLegacyFallback 无逐项照片的旧记录：回退整组照片逻辑，无逐项标注。
func TestBuildMessagesLegacyFallback(t *testing.T) {
	c := NewClient(func(string) (string, bool) { return "", false })
	msgs := c.buildMessages(ReviewInput{
		PointName: "水泵房",
		Photos:    []PhotoRef{{URL: "http://example.com/a.jpg"}, {URL: "http://example.com/b.jpg"}},
	})
	content := msgs[1]["content"].([]map[string]any)
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
	if strings.Contains(joined, "的对应照片") || strings.Contains(joined, "全景/记录级照片") {
		t.Errorf("旧记录不应出现逐项/全景标注：%s", joined)
	}
	if imgs != 2 {
		t.Errorf("图片数=%d，期望 2", imgs)
	}
}
