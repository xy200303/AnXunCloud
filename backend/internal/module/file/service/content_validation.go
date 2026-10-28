package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"unicode/utf8"

	_ "golang.org/x/image/webp"

	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/ooxml"
)

const maxImagePixels = 40_000_000

func validateUploadContent(scene, ext string, data []byte) *errs.Error {
	if len(data) == 0 {
		return errs.ErrUploadType.WithMsg("不能上传空文件")
	}
	if scene == "doc_template" {
		if _, err := ooxml.OpenTemplate(data); err != nil {
			return errs.ErrUploadType.WithMsg("DOCX 母版内容或安全校验失败")
		}
		return nil
	}
	if imageExts[ext] {
		if ext == "heic" {
			if len(data) < 16 || !bytes.Contains(data[:16], []byte("ftyp")) {
				return errs.ErrUploadType.WithMsg("HEIC 文件内容无效")
			}
			return nil
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
			return errs.ErrUploadType.WithMsg("图片无法解码或像素超过限制")
		}
		expected := ext
		if expected == "jpg" {
			expected = "jpeg"
		}
		if format != expected {
			return errs.ErrUploadType.WithMsg("图片内容与扩展名不一致")
		}
		return nil
	}
	switch ext {
	case "pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return errs.ErrUploadType.WithMsg("PDF 文件内容无效")
		}
	case "docx", "xlsx", "pptx", "zip":
		if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return errs.ErrUploadType.WithMsg("Office/ZIP 文件内容无效")
		}
	case "doc", "xls", "ppt":
		if !bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return errs.ErrUploadType.WithMsg("Office 文件内容无效")
		}
	case "txt":
		if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
			return errs.ErrUploadType.WithMsg("文本文件必须为 UTF-8")
		}
	}
	return nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
