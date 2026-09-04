package storage

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anxuncloud/internal/config"
)

// Driver 存储驱动抽象：local 本地磁盘 / oss 阿里云 OSS / cos 腾讯云 COS（后续接入）。
// 统一文件层按驱动分发读写；新增 COS 只需实现本接口并在 New 里装配。
type Driver interface {
	Name() string                      // local|oss|cos
	Put(key string, r io.Reader) error // 服务端写入
	Read(key string) ([]byte, error)   // 服务端读取（PDF 归档、照片内嵌等）
	Delete(key string) error           // 删除对象（去重清理）
	URL(key string) string             // 对外访问地址
}

// ========== 本地磁盘驱动 ==========

type localDriver struct {
	dir     string
	baseURL string
}

func (d *localDriver) Name() string { return "local" }

func (d *localDriver) Put(key string, r io.Reader) error {
	path, err := safeLocalPath(d.dir, key)
	if err != nil {
		return err
	}
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
	path, err := safeLocalPath(d.dir, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (d *localDriver) Delete(key string) error {
	path, err := safeLocalPath(d.dir, key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func safeLocalPath(root, key string) (string, error) {
	if key == "" || strings.ContainsAny(key, `\:`) {
		return "", fmt.Errorf("invalid storage key")
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage path escapes root")
	}
	return path, nil
}

func (d *localDriver) URL(key string) string {
	// 敏感场景（签名/公章/导出文件）走统一文件层 /api/files（鉴权）；内容图继续 /uploads 静态
	if IsProtectedKey(key) {
		return d.baseURL + "/api/files/" + key
	}
	return d.baseURL + "/uploads/" + key
}

// IsProtectedKey 判断内部存储键是否属于敏感场景（signature/seal/export 前缀，读需鉴权）。
func IsProtectedKey(key string) bool {
	return strings.HasPrefix(key, "signature/") || strings.HasPrefix(key, "seal/") || strings.HasPrefix(key, "export/")
}

// ========== 阿里云 OSS 驱动 ==========

type ossDriver struct {
	bucket          string
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	httpc           *http.Client
}

func (d *ossDriver) Name() string { return "oss" }

// Put OSS 服务端直写需要主账号签名，当前业务无此路径（生成文件统一落本地驱动），预留不实现。
func (d *ossDriver) Put(_ string, _ io.Reader) error {
	return fmt.Errorf("oss 驱动暂不支持服务端写入（客户端走 STS 直传）")
}

func (d *ossDriver) Read(key string) ([]byte, error) {
	if key == "" || strings.ContainsAny(key, `\\:`) || strings.Contains(key, "..") {
		return nil, fmt.Errorf("invalid storage key")
	}
	u := &url.URL{Scheme: "https", Host: d.bucket + "." + d.endpoint, Path: "/" + strings.TrimPrefix(key, "/")}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if d.accessKeyID != "" && d.accessKeySecret != "" {
		now := time.Now().UTC().Format(http.TimeFormat)
		req.Header.Set("Date", now)
		req.Header.Set("Authorization", d.authorization(http.MethodGet, key, now))
	}
	resp, err := d.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSS 读取失败: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (d *ossDriver) authorization(method, key, date string) string {
	resource := "/" + d.bucket + "/" + strings.TrimPrefix(key, "/")
	stringToSign := method + "\n\n\n" + date + "\n" + resource
	h := hmac.New(sha1.New, []byte(d.accessKeySecret))
	_, _ = h.Write([]byte(stringToSign))
	return "OSS " + d.accessKeyID + ":" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (d *ossDriver) Delete(key string) error {
	if key == "" || strings.ContainsAny(key, `\\:`) || strings.Contains(key, "..") {
		return fmt.Errorf("invalid storage key")
	}
	now := time.Now().UTC().Format(http.TimeFormat)
	authorization := d.authorization(http.MethodDelete, key, now)
	u := &url.URL{Scheme: "https", Host: d.bucket + "." + d.endpoint, Path: "/" + strings.TrimPrefix(key, "/")}
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Date", now)
	req.Header.Set("Authorization", authorization)
	resp, err := d.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("OSS 删除失败: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (d *ossDriver) URL(key string) string {
	return fmt.Sprintf("https://%s.%s/%s", d.bucket, d.endpoint, key)
}

// ========== 腾讯云 COS 驱动 ==========

type cosDriver struct {
	secretID  string
	secretKey string
	bucket    string // bucketname-appid
	region    string
	expireSec int
	httpc     *http.Client
}

func (d *cosDriver) Name() string { return "cos" }

func (d *cosDriver) host() string {
	return fmt.Sprintf("%s.cos.%s.myqcloud.com", d.bucket, d.region)
}

// URL 对象访问地址（桶需公共读；私有读改签名 GET 为后续项）。
func (d *cosDriver) URL(key string) string {
	return "https://" + d.host() + "/" + key
}

func (d *cosDriver) Read(key string) ([]byte, error) {
	resp, err := d.httpc.Get(d.URL(key))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("COS 读取失败: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (d *cosDriver) Delete(_ string) error {
	return fmt.Errorf("cos 驱动暂不支持服务端删除")
}

// Put 服务端签名直写（COS XML API Put Object，q-sign-algorithm=sha1 鉴权）。
// 客户端直传（预签名 URL/STS）为后续优化项；当前上传经服务端中转，功能完整可用。
func (d *cosDriver) Put(key string, r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		return err
	}
	u := &url.URL{Scheme: "https", Host: d.host(), Path: "/" + key}
	req, err := http.NewRequest(http.MethodPut, u.String(), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(data))
	req.Header.Set("Authorization", d.authHeader(http.MethodPut, u.EscapedPath()))
	resp, err := d.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("COS 写入失败: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// authHeader COS XML API 鉴权头（HMAC-SHA1 方案；仅签 host 头、无 url 参数）。
// 公式：SignKey=HMAC-SHA1(SecretKey, KeyTime)；StringToSign=sha1\nKeyTime\nSHA1(HttpString)\n；
// HttpString=method\nuriPathname\nparams\nheaders\n。
func (d *cosDriver) authHeader(method, uriPath string) string {
	now := time.Now().Unix()
	expire := d.expireSec
	if expire <= 0 {
		expire = 3600
	}
	keyTime := fmt.Sprintf("%d;%d", now, now+int64(expire))
	signKey := hmacHexSHA1([]byte(d.secretKey), keyTime)
	httpString := strings.ToLower(method) + "\n" + uriPath + "\n\nhost=" + url.QueryEscape(d.host()) + "\n"
	stringToSign := "sha1\n" + keyTime + "\n" + sha1Hex(httpString) + "\n"
	signature := hmacHexSHA1([]byte(signKey), stringToSign)
	return fmt.Sprintf("q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s&q-key-time=%s&q-header-list=host&q-url-param-list=&q-signature=%s",
		d.secretID, keyTime, keyTime, signature)
}

func hmacHexSHA1(key []byte, data string) string {
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
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

// newDriver 按模式装配驱动。
func newDriver(up config.UploadConfig, oss config.OSSConfig, cos config.COSConfig, baseURL string) Driver {
	switch up.Mode {
	case "oss":
		return &ossDriver{
			bucket: oss.Bucket, endpoint: oss.Endpoint,
			accessKeyID: oss.AccessKeyID, accessKeySecret: oss.AccessKeySecret,
			httpc: &http.Client{Timeout: 15 * time.Second},
		}
	case "cos":
		return &cosDriver{
			secretID: cos.SecretID, secretKey: cos.SecretKey, bucket: cos.Bucket,
			region: cos.Region, expireSec: cos.ExpireSeconds, httpc: &http.Client{Timeout: 30 * time.Second},
		}
	}
	return &localDriver{dir: up.LocalDir, baseURL: strings.TrimRight(baseURL, "/")}
}
