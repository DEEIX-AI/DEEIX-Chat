package filecontent

import (
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	"github.com/gin-gonic/gin"
)

// Write 将已授权的文件内容以统一的安全响应头写入 HTTP 响应。
func Write(c *gin.Context, result *appupload.FileContentResult, public bool) error {
	defer result.Reader.Close() //nolint:errcheck

	contentType := safeContentType(result.ContentType)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", buildContentDisposition(result.File.FileName, isPassiveInlineContentType(contentType)))
	if public {
		c.Header("Cache-Control", "public, max-age=60")
	} else {
		c.Header("Cache-Control", "private, max-age=60")
	}
	applySecurityHeaders(c, public)
	if result.SizeBytes > 0 {
		c.Header("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	if !result.ModTime.IsZero() {
		c.Header("Last-Modified", result.ModTime.UTC().Format(http.TimeFormat))
	}
	if _, err := io.Copy(c.Writer, result.Reader); err != nil {
		c.Abort()
		return err
	}
	return nil
}

func buildContentDisposition(fileName string, inline bool) string {
	normalizedName := strings.TrimSpace(fileName)
	if normalizedName == "" {
		normalizedName = "file"
	}
	escapedName := strings.NewReplacer("\\", "_", "\"", "_", "\n", "_", "\r", "_").Replace(normalizedName)
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	return disposition + `; filename="` + escapedName + `"; filename*=UTF-8''` + url.PathEscape(normalizedName)
}

func safeContentType(contentType string) string {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		mediaType = strings.TrimSpace(contentType)
		params = nil
	}
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	if normalized == "" {
		return "application/octet-stream"
	}
	if isActiveContentType(normalized) {
		return "text/plain; charset=utf-8"
	}
	if len(params) == 0 {
		return normalized
	}
	return mime.FormatMediaType(normalized, params)
}

func isActiveContentType(mediaType string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "text/html",
		"text/css",
		"text/javascript",
		"text/xml",
		"application/javascript",
		"application/ecmascript",
		"application/x-javascript",
		"application/typescript",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml":
		return true
	default:
		return false
	}
}

func isPassiveInlineContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		mediaType = contentType
	}
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	if normalized == "application/pdf" {
		return true
	}
	switch normalized {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/bmp":
		return true
	default:
		return false
	}
}

func applySecurityHeaders(c *gin.Context, public bool) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'; base-uri 'none'; form-action 'none'; script-src 'none'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data: blob:; media-src 'self' data: blob:")
	if public {
		c.Header("Cross-Origin-Resource-Policy", "cross-origin")
		return
	}
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
}
