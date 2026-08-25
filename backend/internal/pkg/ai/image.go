// 图片供给与共享 HTTP 助手：各协议适配器共用。
// openai 系支持 URL 引用（local 仍读本地转 base64 data URL）；gemini/claude 仅支持 inline base64，
// 云存储 URL 需先 HTTP GET 下载字节再内联（带 8MB 上限与超时）。
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
	"strings"
)

// resolvedImage 已解析照片：data 非空 = 本地已读字节（mime 有效）；否则为远程 URL。
type resolvedImage struct {
	url  string
	data []byte
	mime string
}

// ref 图片内容引用：openai 系传 URL（或 data URL），inline 系传 mime+base64。
func (r *resolvedImage) ref() string {
	if len(r.data) > 0 {
		return "data:" + r.mime + ";base64," + base64.StdEncoding.EncodeToString(r.data)
	}
	return r.url
}

// inline 取内联内容（mime + base64）；本地字节直接编码，远程 URL 下载（8MB 上限，沿用 httpc 超时）。
// 下载失败返回 err，调用方跳过该图（与 resolveImages 读失败跳过同策略）。
func (r *resolvedImage) inline(ctx context.Context, httpc *http.Client) (mime, b64 string, err error) {
	if len(r.data) > 0 {
		return r.mime, base64.StdEncoding.EncodeToString(r.data), nil
	}
	data, mime, err := downloadImage(ctx, httpc, r.url)
	if err != nil {
		return "", "", err
	}
	return mime, base64.StdEncoding.EncodeToString(data), nil
}

// resolveImages 图片供给：local 模式读本地文件转字节（读失败/超限跳过），云存储模式保留 URL。
func (c *Client) resolveImages(photos []PhotoRef) []resolvedImage {
	out := make([]resolvedImage, 0, maxPhotos)
	for _, p := range photos {
		if len(out) >= maxPhotos {
			break
		}
		if p.URL == "" {
			continue
		}
		if c.store != nil && c.store.IsLocal() {
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
			out = append(out, resolvedImage{data: data, mime: mimeFromExt(path)})
			continue
		}
		out = append(out, resolvedImage{url: p.URL})
	}
	return out
}

// downloadImage 下载远程图片（gemini/claude 云存储 URL 内联用）；超 8MB 或失败返回 err。
func downloadImage(ctx context.Context, httpc *http.Client, url string) (data []byte, mime string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("图片下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("图片下载返回 %d", resp.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxPhotoBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("图片读取失败: %w", err)
	}
	if len(data) > maxPhotoBytes {
		return nil, "", fmt.Errorf("图片超过 8MB 上限")
	}
	mime = resp.Header.Get("Content-Type")
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	if !strings.HasPrefix(mime, "image/") {
		mime = mimeFromExt(url)
	}
	return data, mime, nil
}

// mimeFromExt 按扩展名推断图片 MIME（缺省 jpeg）。
func mimeFromExt(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}

// postJSON 各协议共用的 JSON POST：非 200 带截断响应体报错，响应体限读 1MB。
func postJSON(ctx context.Context, httpc *http.Client, url string, headers map[string]string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("大模型请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("大模型返回 %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return respBody, nil
}
