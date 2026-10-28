package ooxml

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
)

func TestMotherTemplateFillPreservesStructureAndInsertsRealData(t *testing.T) {
	raw, _, ok := BuiltinTemplate("fire_equipment")
	if !ok {
		t.Fatal("内置母版不存在")
	}
	mother, err := OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := mother.SetCellText(0, 0, 1, "端到端验证小区", 11); err != nil {
		t.Fatal(err)
	}
	if err := mother.SetRecordRows(6, 1, true, []map[int]string{{1: "P000021", 6: "8.5"}, {1: "P000022", 6: "8.6"}}, 14); err != nil {
		t.Fatal(err)
	}
	var pixel bytes.Buffer
	if err := png.Encode(&pixel, image.NewRGBA(image.Rect(0, 0, 2, 1))); err != nil {
		t.Fatal(err)
	}
	if err := mother.SetCellImage(2, 1, 0, pixel.Bytes(), "png", 360000); err != nil {
		t.Fatal(err)
	}
	built, err := mother.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mother.parts["word/styles.xml"], rawPart(t, raw, "word/styles.xml")) {
		t.Fatal("母版样式部件被改变")
	}
	reader, err := zip.NewReader(bytes.NewReader(built), int64(len(built)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	media := 0
	for _, file := range reader.File {
		content, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(content)
		_ = content.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[file.Name] = string(data)
		if strings.HasPrefix(file.Name, "word/media/mother-image-") {
			media++
		}
	}
	document := parts["word/document.xml"]
	for _, expected := range []string{"P000021", "P000022", "8.5", "8.6", "noWrap", "rIdMotherImage1"} {
		if !strings.Contains(document, expected) {
			t.Fatalf("母版输出缺少 %q", expected)
		}
	}
	if !strings.Contains(parts["word/_rels/document.xml.rels"], `Target="media/mother-image-1.png"`) || media != 1 {
		t.Fatal("真实图片关系未写入")
	}
	if _, err := OpenTemplate(built); err != nil {
		t.Fatalf("生成结果不是有效母版包: %v", err)
	}
}

func rawPart(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		content, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(content)
		_ = content.Close()
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	t.Fatalf("部件不存在: %s", name)
	return nil
}
