package middleware

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

func newCompressRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "app.js"), []byte(strings.Repeat("console.log('deeix');\n", 200)), 0o600); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(Compress())
	large := strings.Repeat("会话内容 conversation content ", 200)
	router.GET("/json", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"content": large}) })
	router.GET("/small", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	router.GET("/stream", func(c *gin.Context) {
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString(large + "\n")
		c.Writer.Flush()
	})
	router.GET("/image", func(c *gin.Context) { c.Data(http.StatusOK, "image/png", []byte(large)) })
	router.GET("/encoded", func(c *gin.Context) {
		c.Header("Content-Encoding", "br")
		c.Data(http.StatusOK, "application/json", []byte(large))
	})
	router.GET("/flushed", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Writer.WriteHeaderNow()
		_, _ = c.Writer.WriteString(large)
	})
	router.GET("/app.js", func(c *gin.Context) {
		c.Header("ETag", `"v1"`)
		c.File(filepath.Join(staticDir, "app.js"))
	})
	return router
}

func serveCompress(router *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func gunzip(t *testing.T, body []byte) string {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("response is not gzip: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(decoded)
}

func TestCompressGzipsTextResponses(t *testing.T) {
	router := newCompressRouter(t)

	recorder := serveCompress(router, "/json", map[string]string{"Accept-Encoding": "br, gzip;q=0.8"})
	if recorder.Header().Get("Content-Encoding") != "gzip" || recorder.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("expected gzip with Vary, got headers %v", recorder.Header())
	}
	if !strings.Contains(gunzip(t, recorder.Body.Bytes()), "会话内容") {
		t.Fatal("decoded body lost the payload")
	}

	// 静态文件声明了 Content-Length 与强 ETag：长度随压缩失效，ETag 降为弱 ETag。
	static := serveCompress(router, "/app.js", map[string]string{"Accept-Encoding": "gzip"})
	if static.Header().Get("Content-Encoding") != "gzip" || static.Header().Get("Content-Length") != "" || static.Header().Get("ETag") != `W/"v1"` {
		t.Fatalf("unexpected static headers %v", static.Header())
	}
	if !strings.HasPrefix(gunzip(t, static.Body.Bytes()), "console.log") {
		t.Fatal("decoded static file does not match")
	}
}

func TestCompressLeavesOtherResponsesUntouched(t *testing.T) {
	router := newCompressRouter(t)
	cases := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{name: "client without gzip", path: "/json"},
		{name: "gzip refused", path: "/json", headers: map[string]string{"Accept-Encoding": "gzip;q=0, br"}},
		{name: "range request", path: "/app.js", headers: map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-99"}},
		{name: "small response", path: "/small", headers: map[string]string{"Accept-Encoding": "gzip"}},
		{name: "ndjson stream", path: "/stream", headers: map[string]string{"Accept-Encoding": "gzip"}},
		{name: "binary content", path: "/image", headers: map[string]string{"Accept-Encoding": "gzip"}},
		{name: "already encoded", path: "/encoded", headers: map[string]string{"Accept-Encoding": "gzip"}},
		{name: "headers already sent", path: "/flushed", headers: map[string]string{"Accept-Encoding": "gzip"}},
	}
	for _, tc := range cases {
		recorder := serveCompress(router, tc.path, tc.headers)
		if encoding := recorder.Header().Get("Content-Encoding"); encoding == "gzip" {
			t.Fatalf("%s: expected no gzip, got headers %v", tc.name, recorder.Header())
		}
		if recorder.Body.Len() == 0 {
			t.Fatalf("%s: expected a body", tc.name)
		}
	}
}

func TestCompressSkipsNotModified(t *testing.T) {
	router := newCompressRouter(t)
	first := serveCompress(router, "/app.js", map[string]string{"Accept-Encoding": "gzip"})
	lastModified := first.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("expected Last-Modified on the static file")
	}
	recorder := serveCompress(router, "/app.js", map[string]string{"Accept-Encoding": "gzip", "If-Modified-Since": lastModified})
	if recorder.Code != http.StatusNotModified || recorder.Header().Get("Content-Encoding") != "" || recorder.Body.Len() != 0 {
		t.Fatalf("expected a bare 304, got %d %v", recorder.Code, recorder.Header())
	}
}
