package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Driver 存储驱动抽象：local 本地磁盘 / oss 阿里云 OSS / cos 腾讯云 COS（后续接入）。
// 统一文件层按驱动分发读写；新增 COS 只需实现本接口并在 New 里装配。
type Driver interface {
	Name() string                      // local|oss|cos
	Put(key string, r io.Reader) error // 服务端写入
	Read(key string) ([]byte, error)   // 服务端读取（PDF 归档、照片内嵌等）
	URL(key string) string             // 对外访问地址
}

// ========== 本地磁盘驱动 ==========

type localDriver struct {
	dir     string
	baseURL string
}

func (d *localDriver) Name() string { return "local" }

func (d *localDriver) Put(key string, r io.Reader) error {
	path := filepath.Join(d.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (d *localDriver) Read(key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(d.dir, filepath.FromSlash(key)))
}

func (d *localDriver) URL(key string) string {
	// 敏感场景（签名/公章/导出文件）走统一文件层 /api/files（鉴权）；内容图继续 /uploads 静态
	if IsProtectedKey(key) {
		return d.baseURL + "/api/files/" + key
	}
	return d.baseURL + "/uploads/" + key
}

// IsProtectedKey 敏感场景的 file_key 判定（signature/seal/export 前缀，读需鉴权）。
func IsProtectedKey(key string) bool {
	return strings.HasPrefix(key, "signature/") || strings.HasPrefix(key, "seal/") || strings.HasPrefix(key, "export/")
}

// ========== 阿里云 OSS 驱动 ==========

type ossDriver struct {
	bucket   string
	endpoint string
	httpc    *http.Client
}

func (d *ossDriver) Name() string { return "oss" }

// Put OSS 服务端直写需要主账号签名，当前业务无此路径（生成文件统一落本地驱动），预留不实现。
func (d *ossDriver) Put(_ string, _ io.Reader) error {
	return fmt.Errorf("oss 驱动暂不支持服务端写入（客户端走 STS 直传）")
}

func (d *ossDriver) Read(key string) ([]byte, error) {
	resp, err := d.httpc.Get(d.URL(key))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSS 读取失败: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (d *ossDriver) URL(key string) string {
	return fmt.Sprintf("https://%s.%s/%s", d.bucket, d.endpoint, key)
}

// ========== 摘要工具 ==========

// MD5Hex 计算内容 MD5（十六进制小写），用于完整性校验与同内容去重查询。
func MD5Hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

// FileMD5 计算本地文件 MD5。
func FileMD5(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// newDriver 按模式装配驱动（cos 配置位预留）。
func newDriver(mode, localDir, baseURL string, ossBucket, ossEndpoint string) Driver {
	if mode == "oss" {
		return &ossDriver{bucket: ossBucket, endpoint: ossEndpoint, httpc: &http.Client{Timeout: 15 * time.Second}}
	}
	return &localDriver{dir: localDir, baseURL: strings.TrimRight(baseURL, "/")}
}
