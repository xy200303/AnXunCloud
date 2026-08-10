package ai

import (
	"strings"
	"testing"
)

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
