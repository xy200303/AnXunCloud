package libreoffice

import (
	"bytes"
	"context"
	"fmt"
)

const (
	DefaultMaxInputBytes  int64 = 50 << 20
	DefaultMaxOutputBytes int64 = 100 << 20
)

type Converter interface {
	Convert(ctx context.Context, docx []byte) ([]byte, error)
}

type unavailableConverter struct{ err error }

func (c unavailableConverter) Convert(context.Context, []byte) ([]byte, error) {
	return nil, c.err
}

func UnavailableConverter(err error) Converter {
	if err == nil {
		err = fmt.Errorf("PDF 转换未启用")
	}
	return unavailableConverter{err: err}
}

func validateInput(docx []byte, maxBytes int64) error {
	if len(docx) < 4 || !bytes.Equal(docx[:2], []byte("PK")) {
		return fmt.Errorf("输入不是有效 DOCX 文件")
	}
	if maxBytes > 0 && int64(len(docx)) > maxBytes {
		return fmt.Errorf("DOCX 超过转换大小限制")
	}
	return nil
}

func ValidatePDF(data []byte, maxBytes int64) error {
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return fmt.Errorf("PDF 超过输出大小限制")
	}
	if len(data) < 8 || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return fmt.Errorf("转换结果不是有效 PDF")
	}
	tail := data
	if len(tail) > 1024 {
		tail = tail[len(tail)-1024:]
	}
	if !bytes.Contains(tail, []byte("%%EOF")) {
		return fmt.Errorf("PDF 文件不完整")
	}
	return nil
}
