package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const defaultOutboundConnectTimeout = 10 * time.Second

// ErrUnsafeOutboundURL 表示外联地址不满足安全边界。
var ErrUnsafeOutboundURL = errors.New("unsafe outbound url")

// ErrInvalidOutboundPolicy 表示 SSRF 出站白名单配置不合法。
var ErrInvalidOutboundPolicy = errors.New("invalid outbound policy")

// OutboundPolicy 是不可变的出站 HTTP 安全策略。
// 受信策略可显式放行私网/回环目标；链路本地、组播、未指定地址和元数据目标始终拒绝。
type OutboundPolicy struct {
	enforce         bool
	allowedHosts    map[string]struct{}
	allowedPrefixes []netip.Prefix
}

// NewOutboundPolicy 创建并校验出站策略。allowedHosts 仅支持精确主机名，allowedCIDRs 使用标准 CIDR。
func NewOutboundPolicy(enforce bool, allowedHosts []string, allowedCIDRs []string) (OutboundPolicy, error) {
	policy := OutboundPolicy{
		enforce:         enforce,
		allowedHosts:    make(map[string]struct{}, len(allowedHosts)),
		allowedPrefixes: make([]netip.Prefix, 0, len(allowedCIDRs)),
	}
	for _, raw := range allowedHosts {
		host := normalizeURLHostname(raw)
		if !isValidAllowlistedHostname(host) {
			return OutboundPolicy{}, fmt.Errorf("%w: invalid allowed host %q", ErrInvalidOutboundPolicy, strings.TrimSpace(raw))
		}
		if isNeverAllowedHostname(host) {
			return OutboundPolicy{}, fmt.Errorf("%w: metadata host %q cannot be allowed", ErrInvalidOutboundPolicy, host)
		}
		policy.allowedHosts[host] = struct{}{}
	}
	seenPrefixes := make(map[netip.Prefix]struct{}, len(allowedCIDRs))
	for _, raw := range allowedCIDRs {
		value := strings.TrimSpace(raw)
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Addr().Is4In6() {
			return OutboundPolicy{}, fmt.Errorf("%w: invalid allowed CIDR %q", ErrInvalidOutboundPolicy, value)
		}
		prefix = prefix.Masked()
		if _, exists := seenPrefixes[prefix]; exists {
			continue
		}
		seenPrefixes[prefix] = struct{}{}
		policy.allowedPrefixes = append(policy.allowedPrefixes, prefix)
	}
	return policy, nil
}

// NewStrictOutboundPolicy 创建不含私网白名单的策略。
func NewStrictOutboundPolicy(enforce bool) OutboundPolicy {
	return OutboundPolicy{enforce: enforce}
}

// ValidateTrustedOutboundHTTPURL 校验管理员可显式授权的 HTTP(S) 端点格式。
// 私网和回环端点允许被授权，但链路本地、元数据和无效主机始终拒绝。
func ValidateTrustedOutboundHTTPURL(raw string) error {
	_, err := parseTrustedHTTPURL(raw)
	return err
}

// HTTPOrigin 返回经过规范化的 HTTP(S) origin，用于把管理员配置的端点信任限制在 scheme、host 和 port。
// 默认端口会被折叠，路径、查询参数和片段不会进入 origin。
func HTTPOrigin(raw string) (string, error) {
	parsed, err := parseTrustedHTTPURL(raw)
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	hostname := normalizeURLHostname(parsed.Hostname())
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	port := parsed.Port()
	if port != "" && !isDefaultHTTPPort(scheme, port) {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

// WithTrustedHTTPURLs 返回一份仅额外信任指定 HTTP(S) 端点主机的策略副本。
// 该能力用于管理员显式配置的集成端点；不会修改原策略，也不能放行链路本地或元数据目标。
func (p OutboundPolicy) WithTrustedHTTPURLs(rawURLs ...string) (OutboundPolicy, error) {
	trusted := OutboundPolicy{
		enforce:         p.enforce,
		allowedHosts:    make(map[string]struct{}, len(p.allowedHosts)+len(rawURLs)),
		allowedPrefixes: append([]netip.Prefix(nil), p.allowedPrefixes...),
	}
	for host := range p.allowedHosts {
		trusted.allowedHosts[host] = struct{}{}
	}

	seenPrefixes := make(map[netip.Prefix]struct{}, len(trusted.allowedPrefixes)+len(rawURLs))
	for _, prefix := range trusted.allowedPrefixes {
		seenPrefixes[prefix] = struct{}{}
	}
	for _, raw := range rawURLs {
		parsed, err := parseTrustedHTTPURL(raw)
		if err != nil {
			return OutboundPolicy{}, err
		}
		host := normalizeURLHostname(parsed.Hostname())
		if ip, err := netip.ParseAddr(host); err == nil {
			ip = ip.Unmap()
			prefix := netip.PrefixFrom(ip, ip.BitLen())
			if _, exists := seenPrefixes[prefix]; !exists {
				seenPrefixes[prefix] = struct{}{}
				trusted.allowedPrefixes = append(trusted.allowedPrefixes, prefix)
			}
			continue
		}
		trusted.allowedHosts[host] = struct{}{}
	}
	return trusted, nil
}

func parseTrustedHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("%w: invalid trusted HTTP URL", ErrInvalidOutboundPolicy)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%w: trusted URL must use HTTP or HTTPS", ErrInvalidOutboundPolicy)
	}
	host := normalizeURLHostname(parsed.Hostname())
	if host == "" || strings.Contains(host, "%") {
		return nil, fmt.Errorf("%w: invalid trusted URL host", ErrInvalidOutboundPolicy)
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		if isNeverAllowedIP(net.IP(ip.AsSlice())) {
			return nil, fmt.Errorf("%w: target %q cannot be trusted", ErrInvalidOutboundPolicy, host)
		}
		return parsed, nil
	}
	if !isValidAllowlistedHostname(host) || isNeverAllowedHostname(host) {
		return nil, fmt.Errorf("%w: target host %q cannot be trusted", ErrInvalidOutboundPolicy, host)
	}
	return parsed, nil
}

func isDefaultHTTPPort(scheme string, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}

// ValidateOutboundHTTPURL 校验外联 HTTP 地址；启用 SSRF 防护时仅允许策略授权的本机/内网目标，并始终阻断链路本地和元数据地址。
func ValidateOutboundHTTPURL(raw string, policy OutboundPolicy) error {
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
	if !policy.enforce {
		return nil
	}
	host := normalizeURLHostname(parsed.Hostname())
	if host == "" || strings.Contains(host, "%") || isNeverAllowedHostname(host) || (isUnsafeHostname(host) && !policy.allowsHost(host)) {
		return fmt.Errorf("%w: unsafe host", ErrUnsafeOutboundURL)
	}
	if ip := net.ParseIP(host); ip != nil && (isNeverAllowedIP(ip) || (isPrivateOrLoopbackIP(ip) && !policy.allowsIP(ip))) {
		return fmt.Errorf("%w: unsafe ip", ErrUnsafeOutboundURL)
	}
	return nil
}

// NewOutboundHTTPClient 创建受策略保护的 HTTP client。
func NewOutboundHTTPClient(policy OutboundPolicy, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NewOutboundHTTPTransport(policy, defaultOutboundConnectTimeout),
	}
}

// NewOutboundHTTPTransport 创建受策略保护的 HTTP transport。
func NewOutboundHTTPTransport(policy OutboundPolicy, connectTimeout time.Duration) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	if policy.enforce {
		transport.Proxy = nil
	}
	transport.DialContext = NewOutboundDialContext(policy, connectTimeout, 30*time.Second)
	return transport
}

// NewOutboundDialContext 创建安全 dialer。
// 启用策略时先解析目标域名，仅放行策略授权的 loopback/private IP，并直接拨打已校验 IP。
func NewOutboundDialContext(policy OutboundPolicy, timeout time.Duration, keepAlive time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = defaultOutboundConnectTimeout
	}
	if keepAlive == 0 {
		keepAlive = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: keepAlive}
	return newOutboundDialContext(policy, net.DefaultResolver.LookupIPAddr, dialer.DialContext)
}

type lookupIPAddrFunc func(context.Context, string) ([]net.IPAddr, error)
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

func newOutboundDialContext(policy OutboundPolicy, lookupIPAddr lookupIPAddrFunc, dial dialContextFunc) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		if !policy.enforce {
			return dial(ctx, network, address)
		}
		addresses, err := resolveSafeDialAddresses(ctx, network, address, policy, lookupIPAddr)
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

func resolveSafeDialAddresses(ctx context.Context, network string, address string, policy OutboundPolicy, lookupIPAddr lookupIPAddrFunc) ([]string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid dial address", ErrUnsafeOutboundURL)
	}
	host = normalizeURLHostname(host)
	if host == "" || strings.Contains(host, "%") || isNeverAllowedHostname(host) {
		return nil, fmt.Errorf("%w: unsafe host", ErrUnsafeOutboundURL)
	}
	hostAllowed := policy.allowsHost(host)
	if isUnsafeHostname(host) && !hostAllowed {
		return nil, fmt.Errorf("%w: unsafe host", ErrUnsafeOutboundURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isNeverAllowedIP(ip) || (isPrivateOrLoopbackIP(ip) && !policy.allowsIP(ip)) {
			return nil, fmt.Errorf("%w: unsafe ip", ErrUnsafeOutboundURL)
		}
		if !ipMatchesNetwork(ip, network) {
			return nil, fmt.Errorf("%w: no address for network", ErrUnsafeOutboundURL)
		}
		return []string{net.JoinHostPort(ip.String(), port)}, nil
	}
	records, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve host: %w", ErrUnsafeOutboundURL, err)
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
		if isNeverAllowedIP(ip) || (isPrivateOrLoopbackIP(ip) && !hostAllowed && !policy.allowsIP(ip)) {
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

func normalizeURLHostname(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func (p OutboundPolicy) allowsHost(host string) bool {
	_, ok := p.allowedHosts[normalizeURLHostname(host)]
	return ok
}

func (p OutboundPolicy) allowsIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	for _, prefix := range p.allowedPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func isValidAllowlistedHostname(host string) bool {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil || strings.ContainsAny(host, "/:@?#[]%") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, char := range label {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
				continue
			}
			return false
		}
	}
	return true
}

func isNeverAllowedHostname(host string) bool {
	return host == "metadata.google.internal"
}

func isUnsafeHostname(host string) bool {
	switch host {
	case "localhost", "localhost.localdomain", "ip6-localhost":
		return true
	default:
		return strings.HasSuffix(host, ".localhost")
	}
}

func isPrivateOrLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	normalized := ip
	if v4 := ip.To4(); v4 != nil {
		normalized = v4
	}
	return normalized.IsLoopback() || normalized.IsPrivate()
}

func isNeverAllowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	normalized := ip
	if v4 := ip.To4(); v4 != nil {
		normalized = v4
	}
	return normalized.IsLinkLocalUnicast() ||
		normalized.IsLinkLocalMulticast() ||
		normalized.IsUnspecified() ||
		normalized.IsMulticast() ||
		normalized.Equal(net.ParseIP("100.100.100.200")) ||
		normalized.Equal(net.ParseIP("fd00:ec2::254"))
}
