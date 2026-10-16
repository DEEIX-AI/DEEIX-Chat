package middleware

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// compressMinBytes 是值得压缩的最小响应体；更小的响应压缩后几乎不变，反而多付 gzip 头尾与 CPU。
const compressMinBytes = 1024

var gzipWriterPool = sync.Pool{
	New: func() any {
		writer, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return writer
	},
}

// Compress 对接受 gzip 的客户端压缩文本类响应（JSON、HTML、JS、CSS 等）。
// 不压缩：流式响应（NDJSON、SSE）、Range 请求、已编码或 no-transform 的响应、无响应体的状态码与小响应。
// 是否压缩在首次写出响应体时决定，此时处理器已设置好响应头。
func Compress() gin.HandlerFunc {
	return func(c *gin.Context) {
		request := c.Request
		if request.Method == http.MethodHead ||
			request.Header.Get("Range") != "" ||
			c.IsWebsocket() ||
			!acceptsGzip(request.Header.Get("Accept-Encoding")) {
			c.Next()
			return
		}

		writer := &compressWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		defer func() {
			writer.finish()
			// 外层中间件（访问日志等）在 c.Next() 之后读取状态码与写出字节数，恢复原始 writer。
			c.Writer = writer.ResponseWriter
		}()
		c.Next()
	}
}

type compressWriter struct {
	gin.ResponseWriter
	decided bool
	gz      *gzip.Writer
}

func (w *compressWriter) Write(data []byte) (int, error) {
	if !w.decided {
		w.decide(len(data))
	}
	if w.gz == nil {
		return w.ResponseWriter.Write(data)
	}
	return w.gz.Write(data)
}

func (w *compressWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *compressWriter) Flush() {
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	w.ResponseWriter.Flush()
}

// decide 在首次写出响应体前决定是否压缩；firstChunk 用于未声明 Content-Length 的一次性响应（如 c.JSON）。
func (w *compressWriter) decide(firstChunk int) {
	w.decided = true
	// 响应头已经发出（处理器提前 Flush 或 WriteHeaderNow）时无法再改为压缩。
	if w.ResponseWriter.Written() {
		return
	}
	header := w.Header()
	if !compressibleResponse(w.Status(), header, firstChunk) {
		return
	}
	header.Del("Content-Length")
	header.Del("Accept-Ranges")
	header.Set("Content-Encoding", "gzip")
	header.Add("Vary", "Accept-Encoding")
	// 压缩后的字节与原始内容不同，强 ETag 只能降为弱 ETag。
	if etag := header.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		header.Set("ETag", "W/"+etag)
	}
	gz, _ := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
}

func (w *compressWriter) finish() {
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	w.gz.Reset(io.Discard)
	gzipWriterPool.Put(w.gz)
	w.gz = nil
}

func compressibleResponse(status int, header http.Header, firstChunk int) bool {
	if status < http.StatusOK || status == http.StatusNoContent || status == http.StatusPartialContent || status == http.StatusNotModified {
		return false
	}
	if header.Get("Content-Encoding") != "" || strings.Contains(strings.ToLower(header.Get("Cache-Control")), "no-transform") {
		return false
	}
	size := firstChunk
	if declared := header.Get("Content-Length"); declared != "" {
		if parsed, err := strconv.Atoi(declared); err == nil {
			size = parsed
		}
	}
	if size < compressMinBytes {
		return false
	}
	return compressibleContentType(header.Get("Content-Type"))
}

// compressibleContentType 只放行文本格式；图片、视频、压缩包与字体保持原样。
// NDJSON 与 SSE 需要逐条立即送达，不在白名单内。
func compressibleContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch mediaType {
	case "text/event-stream":
		return false
	case "application/json", "application/javascript", "application/xml", "application/wasm", "image/svg+xml":
		return true
	}
	return strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json") ||
		strings.HasSuffix(mediaType, "+xml")
}

// acceptsGzip 解析 Accept-Encoding，gzip 显式标为 q=0 时视为拒绝。
func acceptsGzip(acceptEncoding string) bool {
	for _, part := range strings.Split(acceptEncoding, ",") {
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		for _, param := range strings.Split(params, ";") {
			key, value, found := strings.Cut(strings.TrimSpace(param), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			if quality, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && quality <= 0 {
				return false
			}
		}
		return true
	}
	return false
}
