package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const redactedHeaderValue = "********"
const defaultOutboundConnectTimeout = 10 * time.Second

// ErrUnsafeOutboundURL 表示外联地址不满足生产安全边界。
var ErrUnsafeOutboundURL = errors.New("unsafe outbound url")

// RedactHeadersJSON 对自定义请求头 JSON 中的敏感头做脱敏，避免 API 响应扩大密钥泄漏面。
func RedactHeadersJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil || payload == nil {
		return "{}"
	}
	result := make(map[string]interface{}, len(payload))
	for key, item := range payload {
		if IsSensitiveHeaderName(key) {
			result[key] = redactedHeaderValue
			continue
		}
		result[key] = item
	}
	normalized, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}
	return string(normalized)
}

// IsSensitiveHeaderName 判断 header 名称是否可能承载密钥、Token 或 Cookie。
func IsSensitiveHeaderName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return false
	}
	if normalized == "cookie" || normalized == "set-cookie" {
		return true
	}
	for _, marker := range []string{"authorization", "api-key", "apikey", "token", "secret"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// ValidateOutboundHTTPURL 校验管理员配置的外联 HTTP 地址；生产环境额外阻断本机、内网、链路本地和元数据地址。
func ValidateOutboundHTTPURL(raw string, env string) error {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: invalid url", ErrUnsafeOutboundURL)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: user info is not allowed", ErrUnsafeOutboundURL)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme", ErrUnsafeOutboundURL)
	}
	if !isProductionEnv(env) {
		return nil
	}
	host := normalizeURLHostname(parsed.Hostname())
	if host == "" || isUnsafeHostname(host) {
		return fmt.Errorf("%w: unsafe host", ErrUnsafeOutboundURL)
	}
	if ip := net.ParseIP(host); ip != nil && isUnsafeIP(ip) {
		return fmt.Errorf("%w: unsafe ip", ErrUnsafeOutboundURL)
	}
	return nil
}

// NewOutboundHTTPClient 创建带生产环境 SSRF 防护的 HTTP client。
func NewOutboundHTTPClient(env string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NewOutboundHTTPTransport(env, defaultOutboundConnectTimeout),
	}
}

// NewOutboundHTTPTransport 创建带生产环境 SSRF 防护的 HTTP transport。
func NewOutboundHTTPTransport(env string, connectTimeout time.Duration) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	if isProductionEnv(env) {
		transport.Proxy = nil
	}
	transport.DialContext = NewOutboundDialContext(env, connectTimeout, 30*time.Second)
	return transport
}

// NewOutboundDialContext 创建生产环境安全 dialer。
// 生产环境会先解析目标域名，拒绝 loopback/private/link-local/metadata IP，再直接拨打已校验 IP。
func NewOutboundDialContext(env string, timeout time.Duration, keepAlive time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = defaultOutboundConnectTimeout
	}
	if keepAlive == 0 {
		keepAlive = 30 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: keepAlive,
	}
	return newOutboundDialContext(env, net.DefaultResolver.LookupIPAddr, dialer.DialContext)
}

type lookupIPAddrFunc func(context.Context, string) ([]net.IPAddr, error)
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

func newOutboundDialContext(env string, lookupIPAddr lookupIPAddrFunc, dial dialContextFunc) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		if !isProductionEnv(env) {
			return dial(ctx, network, address)
		}
		addresses, err := resolveSafeDialAddresses(ctx, network, address, lookupIPAddr)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, dialAddress := range addresses {
			conn, err := dial(ctx, network, dialAddress)
			if err == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("%w: no dial address", ErrUnsafeOutboundURL)
	}
}

func resolveSafeDialAddresses(ctx context.Context, network string, address string, lookupIPAddr lookupIPAddrFunc) ([]string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid dial address", ErrUnsafeOutboundURL)
	}
	host = normalizeURLHostname(host)
	if host == "" || isUnsafeHostname(host) {
		return nil, fmt.Errorf("%w: unsafe host", ErrUnsafeOutboundURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return nil, fmt.Errorf("%w: unsafe ip", ErrUnsafeOutboundURL)
		}
		if !ipMatchesNetwork(ip, network) {
			return nil, fmt.Errorf("%w: no address for network", ErrUnsafeOutboundURL)
		}
		return []string{net.JoinHostPort(ip.String(), port)}, nil
	}
	records, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve host: %v", ErrUnsafeOutboundURL, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%w: no resolved ip", ErrUnsafeOutboundURL)
	}
	addresses := make([]string, 0, len(records))
	for _, record := range records {
		ip := record.IP
		if ip == nil {
			continue
		}
		if isUnsafeIP(ip) {
			return nil, fmt.Errorf("%w: unsafe resolved ip", ErrUnsafeOutboundURL)
		}
		if ipMatchesNetwork(ip, network) {
			addresses = append(addresses, net.JoinHostPort(ip.String(), port))
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%w: no address for network", ErrUnsafeOutboundURL)
	}
	return addresses, nil
}

func ipMatchesNetwork(ip net.IP, network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func normalizeURLHostname(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func isUnsafeHostname(host string) bool {
	switch host {
	case "localhost", "localhost.localdomain", "ip6-localhost", "metadata.google.internal":
		return true
	default:
		return strings.HasSuffix(host, ".localhost")
	}
}

func isUnsafeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	normalized := ip
	if v4 := ip.To4(); v4 != nil {
		normalized = v4
	}
	if normalized.IsLoopback() ||
		normalized.IsPrivate() ||
		normalized.IsLinkLocalUnicast() ||
		normalized.IsLinkLocalMulticast() ||
		normalized.IsUnspecified() ||
		normalized.IsMulticast() {
		return true
	}
	return normalized.Equal(net.ParseIP("100.100.100.200"))
}
