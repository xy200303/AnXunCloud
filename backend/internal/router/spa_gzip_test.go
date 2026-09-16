package router

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServeFileSmartGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	// >1KB 的 JS 内容
	content := strings.Repeat("console.log('hello');\n", 100)
	fp := filepath.Join(dir, "app.js")
	if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fp)
	if err != nil {
		t.Fatal(err)
	}

	newReq := func(method, acceptEncoding string) *httptest.ResponseRecorder {
		r := gin.New()
		r.NoRoute(func(c *gin.Context) { serveFileSmart(c, fp, info) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/assets/app.js", nil)
		if acceptEncoding != "" {
			req.Header.Set("Accept-Encoding", acceptEncoding)
		}
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("gzip 压缩文本资源", func(t *testing.T) {
		w := newReq(http.MethodGet, "gzip, deflate")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if got := w.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding = %q", got)
		}
		if got := w.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
			t.Fatalf("Vary = %q", got)
		}
		if w.Header().Get("Content-Length") != "" {
			t.Fatal("压缩响应不应带 Content-Length")
		}
		gz, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		body, err := io.ReadAll(gz)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != content {
			t.Fatal("解压后内容不一致")
		}
		if w.Body.Len() >= len(content) {
			t.Fatal("压缩后体积未减小")
		}
	})

	t.Run("HEAD 只设头不写体", func(t *testing.T) {
		w := newReq(http.MethodHead, "gzip")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Fatal("HEAD 未设 Content-Encoding")
		}
		if w.Body.Len() != 0 {
			t.Fatal("HEAD 不应有响应体")
		}
	})

	t.Run("不支持 gzip 时原样返回", func(t *testing.T) {
		w := newReq(http.MethodGet, "")
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("不应压缩")
		}
		if w.Body.String() != content {
			t.Fatal("内容不一致")
		}
	})

	t.Run("不支持压缩的扩展名原样返回", func(t *testing.T) {
		png := filepath.Join(dir, "logo.png")
		if err := os.WriteFile(png, bytes.Repeat([]byte{0x89}, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
		pinfo, _ := os.Stat(png)
		r := gin.New()
		r.NoRoute(func(c *gin.Context) { serveFileSmart(c, png, pinfo) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/assets/logo.png", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(w, req)
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("png 不应压缩")
		}
	})
}
