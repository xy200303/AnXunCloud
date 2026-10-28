package libreoffice

import "testing"

func TestValidatePDF(t *testing.T) {
	valid := []byte("%PDF-1.7\nbody\n%%EOF\n")
	if err := ValidatePDF(valid, 1024); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"docx":      []byte("PK\x03\x04not-pdf"),
		"truncated": []byte("%PDF-1.7\nmissing-end"),
	} {
		if err := ValidatePDF(data, 1024); err == nil {
			t.Fatalf("%s 不应通过 PDF 校验", name)
		}
	}
	if err := ValidatePDF(valid, 8); err == nil {
		t.Fatal("超过输出限制不应通过")
	}
}
