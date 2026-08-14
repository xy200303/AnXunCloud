// Package storage 文件存储抽象：dev 模式本地目录，oss 模式阿里云 STS 直传 + 回调验签。
package storage

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"anxuncloud/internal/config"
	"anxuncloud/internal/pkg/errs"
)

// Storage 存储服务（门面对象：驱动 + 上传策略；驱动实现见 driver.go，COS 接入时实现 Driver 即可）。
type Storage struct {
	cfg     config.UploadConfig
	oss     config.OSSConfig
	baseURL string
	httpc   *http.Client
	driver  Driver
}

func New(up config.UploadConfig, oss config.OSSConfig, baseURL string) *Storage {
	s := &Storage{cfg: up, oss: oss, baseURL: strings.TrimRight(baseURL, "/"), httpc: &http.Client{Timeout: 10 * time.Second}}
	s.driver = newDriver(up.Mode, up.LocalDir, s.baseURL, oss.Bucket, oss.Endpoint)
	return s
}

// DriverName 当前存储驱动名（local/oss/cos）。
func (s *Storage) DriverName() string { return s.driver.Name() }

// ReadFile 按 file_key 读取文件字节（统一文件层用；本地读盘，云存储走 HTTP）。
func (s *Storage) ReadFile(fileKey string) ([]byte, error) { return s.driver.Read(fileKey) }

// IsDev 是否本地开发模式。
func (s *Storage) IsDev() bool { return s.cfg.Mode != "oss" }

// MaxFileSize 单文件上限。
func (s *Storage) MaxFileSize() int64 { return s.cfg.MaxFileSize }

// AllowedTypes 允许的扩展名。
func (s *Storage) AllowedTypes() []string { return s.cfg.AllowedTypes }

// NewFileKey 按约定生成对象键：{scene}/{yyyyMM}/{uid}/{uuid}.{ext}
func (s *Storage) NewFileKey(scene string, uid string, ext string) string {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	return fmt.Sprintf("%s/%s/%s/%s.%s", scene, time.Now().Format("200601"), uid, uuid.NewString(), ext)
}

// CheckExt 校验扩展名，不允许返回 46003。
func (s *Storage) CheckExt(name string) (string, *errs.Error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	for _, t := range s.cfg.AllowedTypes {
		if ext == t {
			return ext, nil
		}
	}
	return "", errs.ErrUploadType
}

// URL 返回文件访问地址（dev：本地静态路由；oss：拼接桶域名，私有读需另签名）。
func (s *Storage) URL(fileKey string) string {
	return s.driver.URL(fileKey)
}

// LocalPath file_key 对应的本地路径（仅 dev 模式有效）。
func (s *Storage) LocalPath(fileKey string) string {
	return filepath.Join(s.cfg.LocalDir, filepath.FromSlash(fileKey))
}

// SaveLocal dev 模式保存上传文件，返回 file_key 与访问 URL。
func (s *Storage) SaveLocal(scene string, uid string, ext string, r io.Reader) (string, string, error) {
	key := s.NewFileKey(scene, uid, ext)
	path := s.LocalPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", "", err
	}
	return key, s.URL(key), nil
}

// SaveGenerated 保存服务端生成的文件（二维码包、报表），scene 固定 export。
// 文件名前缀随机段：/uploads 为免鉴权静态目录，随机段使导出文件 URL 不可预测（防穷举下载）。
func (s *Storage) SaveGenerated(subdir, filename string, data []byte) (string, string, error) {
	key := fmt.Sprintf("export/%s/%s/%s_%s", subdir, time.Now().Format("20060102"), uuid.NewString()[:8], filename)
	path := s.LocalPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", "", err
	}
	return key, s.URL(key), nil
}

// STSCredentials STS 签发结果。
type STSCredentials struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      time.Time
}

// AssumeRole 调阿里云 STS AssumeRole（RPC 签名 HMAC-SHA1，免 SDK）。
func (s *Storage) AssumeRole() (*STSCredentials, error) {
	expire := s.oss.ExpireSeconds
	if expire <= 0 {
		expire = 3600
	}
	params := map[string]string{
		"Action":           "AssumeRole",
		"Format":           "JSON",
		"Version":          "2015-04-01",
		"AccessKeyId":      s.oss.AccessKeyID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   uuid.NewString(),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"RoleArn":          s.oss.RoleArn,
		"RoleSessionName":  "pi-upload",
		"DurationSeconds":  fmt.Sprintf("%d", expire),
	}
	// 构造待签名字符串
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(url.QueryEscape(k))
		canonical.WriteByte('=')
		canonical.WriteString(url.QueryEscape(params[k]))
	}
	stringToSign := "GET&" + url.QueryEscape("/") + "&" + url.QueryEscape(canonical.String())
	mac := hmac.New(sha1.New, []byte(s.oss.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	reqURL := "https://sts.aliyuncs.com/?" + canonical.String() + "&Signature=" + url.QueryEscape(signature)
	resp, err := s.httpc.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var out struct {
		Credentials struct {
			AccessKeyID     string `json:"AccessKeyId"`
			AccessKeySecret string `json:"AccessKeySecret"`
			SecurityToken   string `json:"SecurityToken"`
			Expiration      string `json:"Expiration"`
		} `json:"Credentials"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("STS 响应解析失败: %w", err)
	}
	if out.Credentials.AccessKeyID == "" {
		return nil, fmt.Errorf("AssumeRole 失败: %s %s", out.Code, out.Message)
	}
	exp, _ := time.Parse(time.RFC3339, out.Credentials.Expiration)
	return &STSCredentials{
		AccessKeyID:     out.Credentials.AccessKeyID,
		AccessKeySecret: out.Credentials.AccessKeySecret,
		SecurityToken:   out.Credentials.SecurityToken,
		Expiration:      exp,
	}, nil
}

// VerifyCallback 校验 OSS 回调签名（RSA 公钥取自 x-oss-pub-key-url）。
// 签名内容：请求路径 + 查询串 + "\n" + body，算法 MD5withRSA。
func (s *Storage) VerifyCallback(authorization, pubKeyURL, requestURI string, body []byte) bool {
	if authorization == "" || pubKeyURL == "" {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(authorization)
	if err != nil {
		return false
	}
	keyBytes, err := base64.StdEncoding.DecodeString(pubKeyURL)
	if err != nil {
		return false
	}
	// 公钥 URL 必须来自阿里云官方域名，防伪造
	u, err := url.Parse(string(keyBytes))
	if err != nil || !strings.HasSuffix(u.Host, "aliyuncs.com") {
		return false
	}
	resp, err := s.httpc.Get(u.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	pubPEM, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return false
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return false
	}
	content := requestURI + "\n" + string(body)
	h := crypto.MD5.New()
	h.Write([]byte(content))
	return rsa.VerifyPKCS1v15(pub, crypto.MD5, h.Sum(nil), sig) == nil
}

// CallbackURL OSS 回调地址。
func (s *Storage) CallbackURL() string { return s.baseURL + "/api/mp/upload/callback" }

// BaseURL 对外访问基础地址。
func (s *Storage) BaseURL() string { return s.baseURL }

// OSSWatermarkProcess 生成 OSS 图片水印处理参数（oss 模式打卡照片用）。
func (s *Storage) OSSWatermarkProcess(text string) string {
	encoded := base64.URLEncoding.EncodeToString([]byte(text))
	return fmt.Sprintf("image/watermark,text_%s,size_20,color_ffffff,shadow_50,t_70,g_se,x_16,y_16", encoded)
}
