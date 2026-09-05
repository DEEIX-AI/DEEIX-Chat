// Package extract 提供提取引擎客户端共用的传输与文本处理行为。
package extract

import (
	"net/http"
	"strings"
)

// ApplyAuthHeaders 写入提取引擎及兼容代理支持的认证头。
func ApplyAuthHeaders(req *http.Request, authToken string) {
	if req == nil {
		return
	}
	token := strings.TrimSpace(authToken)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-API-Key", token)
	req.Header.Set("token", token)
}

// AwaitMultipartWriteError 等待异步 multipart 写入结束并返回首个错误。
func AwaitMultipartWriteError(errCh <-chan error) error {
	if errCh == nil {
		return nil
	}
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// NormalizeText 清理每行首尾空白并移除空行。
func NormalizeText(raw string) string {
	lines := strings.Split(raw, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value != "" {
			result = append(result, value)
		}
	}
	return strings.Join(result, "\n")
}
