// SPA/pdfjs 静态资源 gzip 压缩：标准库实现，替代未引入的 gin-contrib/gzip。
// 只压可压缩文本类型（js/css/html/svg/json/map）且 >1KB 的 GET/HEAD 响应，
// 已压缩的二进制格式（woff2/png/jpg/ico/pdf 等）与 API 响应不受影响。
package router

import (
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// gzipMinSize 小于该体积压缩收益低于开销
const gzipMinSize = 1024

// gzipExts 值得压缩的文本类扩展名（woff2/png/jpg/ico/pdf 等已压缩格式不在列）
var gzipExts = map[string]bool{
	".js": true, ".css": true, ".html": true,
	".svg": true, ".json": true, ".map": true,
}

// shouldGzip 判断本次静态响应是否走 gzip：GET/HEAD + 客户端支持 + 文本扩展名 + 体积足够
func shouldGzip(c *gin.Context, name string, size int64) bool {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	if size <= gzipMinSize || !gzipExts[strings.ToLower(filepath.Ext(name))] {
		return false
	}
	return strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip")
}

// serveGzip 以 gzip 压缩写出文件内容；HEAD 请求只设头不写体
func serveGzip(c *gin.Context, body io.Reader, name string) {
	h := c.Writer.Header()
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); ct != "" {
		h.Set("Content-Type", ct)
	}
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Del("Content-Length") // 压缩后长度未知，交给 chunked
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		c.Writer.WriteHeaderNow()
		return
	}
	gz := gzip.NewWriter(c.Writer)
	defer gz.Close()
	_, _ = io.Copy(gz, body)
}

// serveFileSmart 托管磁盘静态文件：可压缩文本走 gzip，其余回退 c.File（保留 Range 等行为）
func serveFileSmart(c *gin.Context, fp string, info os.FileInfo) {
	if !shouldGzip(c, fp, info.Size()) {
		c.File(fp)
		return
	}
	f, err := os.Open(fp)
	if err != nil {
		c.File(fp)
		return
	}
	defer f.Close()
	serveGzip(c, f, fp)
}

// gzipStaticFS 返回替代 r.StaticFS 的 gzip 感知处理器（用于内嵌的 pdfjs 资源）
func gzipStaticFS(fsys fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		if rel == "" {
			c.Status(http.StatusNotFound)
			return
		}
		f, err := fsys.Open(rel)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}
		if shouldGzip(c, rel, info.Size()) {
			serveGzip(c, f, rel)
			return
		}
		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), rs)
			return
		}
		data, err := io.ReadAll(f)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, mime.TypeByExtension(strings.ToLower(filepath.Ext(rel))), data)
	}
}
