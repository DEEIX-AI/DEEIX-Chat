package billing

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

var (
	// ErrEPayGatewayInvalid 表示地址不满足易支付 submit.php 页面跳转协议的安全约束。
	ErrEPayGatewayInvalid = errors.New("epay gateway url is invalid")
)

// ResolveEPaySubmitURL 将易支付站点地址规范化为明确的 submit.php 页面跳转地址。
// 为兼容既有配置，除完整 submit.php 外的安全 HTTP(S) 地址均按站点地址处理。
func ResolveEPaySubmitURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrEPayGatewayInvalid
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", ErrEPayGatewayInvalid
	}

	cleanPath := parsed.EscapedPath()
	if cleanPath == "" {
		cleanPath = "/"
	}
	switch {
	case strings.EqualFold(path.Base(cleanPath), "submit.php"):
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
		parsed.RawPath = strings.TrimSuffix(parsed.RawPath, "/")
	default:
		parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/submit.php"
		parsed.RawPath = strings.TrimSuffix(parsed.RawPath, "/") + "/submit.php"
	}
	return parsed.String(), nil
}

// FormatEPayAmount 将人民币分精确格式化为易支付要求的两位小数金额。
func FormatEPayAmount(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}
