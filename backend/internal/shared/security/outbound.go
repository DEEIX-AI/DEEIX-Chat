package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
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
	// publicOnly 额外拒绝所有非公网单播地址（CGNAT、NAT64、6to4、保留与文档网段等），用于不受信任的用户自配端点。
	publicOnly bool
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

// Enforced 报告策略是否启用 SSRF 防护。
func (p OutboundPolicy) Enforced() bool {
	return p.enforce
}

// NewPublicOnlyOutboundPolicy 创建只允许公网单播目标、始终强制执行的策略，用于普通用户配置的端点。
// 它不接受任何白名单：除私网与回环外，还拒绝 CGNAT（如 Tailscale）、NAT64/6to4 等可能映射回内网的网段。
func NewPublicOnlyOutboundPolicy() OutboundPolicy {
	return OutboundPolicy{enforce: true, publicOnly: true}
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
	if ip := net.ParseIP(host); ip != nil && (isNeverAllowedIP(ip) || (policy.isRestrictedLiteralIP(ip) && !policy.allowsIP(ip))) {
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
// 始终遵守标准代理环境变量（HTTP_PROXY / HTTPS_PROXY / NO_PROXY）。
// 启用策略时：目标 URL 仍按策略做字面量校验；环境变量中的代理地址视为部署方授权，拨号时放行。
// 经代理的请求由代理解析目标域名，应用层只能在本地预解析一次（见 validateProxiedTargetHost），
// DNS rebinding 等解析期差异由部署方的代理负责。
func NewOutboundHTTPTransport(policy OutboundPolicy, connectTimeout time.Duration) *http.Transport {
	env := outboundProxyEnvironment()
	proxy := func(request *http.Request) (*url.URL, error) {
		return env.proxyFunc(request.URL)
	}
	return newOutboundHTTPTransport(policy, connectTimeout, proxy, env.dialAddresses, net.DefaultResolver.LookupIPAddr)
}

// outboundProxySnapshot 是进程内唯一的代理环境快照：代理选择、拨号放行与启动日志共用它，避免三者读到不一致的配置。
type outboundProxySnapshot struct {
	proxyFunc     func(*url.URL) (*url.URL, error)
	dialAddresses map[string]struct{}
	status        OutboundProxyStatus
}

// outboundProxyEnvironment 与 http.ProxyFromEnvironment 一样只在进程内读取一次环境变量；
// 连接池会按 origin 构造大量 transport，逐个读取既浪费也可能读到不一致的值。修改代理环境变量需要重启进程。
var outboundProxyEnvironment = sync.OnceValue(func() outboundProxySnapshot {
	cfg := httpproxy.FromEnvironment()
	return outboundProxySnapshot{
		proxyFunc:     cfg.ProxyFunc(),
		dialAddresses: proxyDialAddresses(cfg),
		status:        newOutboundProxyStatus(cfg, firstEnv("ALL_PROXY", "all_proxy")),
	}
})

// OutboundProxyStatus 描述出站 transport 实际使用的代理环境变量，供启动日志使用；代理地址已去除凭据和路径。
type OutboundProxyStatus struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
	// IgnoredAllProxy 表示只设置了 ALL_PROXY：Go 标准库不读取它，出站请求实际仍然直连。
	IgnoredAllProxy bool
}

// Configured 报告是否存在可能经代理发出的请求。
func (s OutboundProxyStatus) Configured() bool {
	return s.HTTPProxy != "" || s.HTTPSProxy != ""
}

// DescribeOutboundProxy 返回出站 transport 使用的代理环境快照。
func DescribeOutboundProxy() OutboundProxyStatus {
	return outboundProxyEnvironment().status
}

func newOutboundProxyStatus(cfg *httpproxy.Config, allProxy string) OutboundProxyStatus {
	status := OutboundProxyStatus{
		HTTPProxy:  redactProxyURL(cfg.HTTPProxy),
		HTTPSProxy: redactProxyURL(cfg.HTTPSProxy),
		NoProxy:    strings.TrimSpace(cfg.NoProxy),
	}
	status.IgnoredAllProxy = strings.TrimSpace(allProxy) != "" && !status.Configured()
	return status
}

// redactProxyURL 只保留代理的 scheme 与 host:port，去掉可能携带的用户名、密码和路径。
func redactProxyURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "<invalid>"
	}
	return strings.ToLower(parsed.Scheme) + "://" + parsed.Host
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func newOutboundHTTPTransport(
	policy OutboundPolicy,
	connectTimeout time.Duration,
	proxy func(*http.Request) (*url.URL, error),
	proxyAddresses map[string]struct{},
	lookupIPAddr lookupIPAddrFunc,
) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = proxy
	transport.DialContext = NewOutboundDialContext(policy, connectTimeout, 30*time.Second)
	if policy.enforce {
		transport.Proxy = validatingProxy(policy, proxy, lookupIPAddr)
		transport.DialContext = allowProxyDial(proxyAddresses, transport.DialContext, connectTimeout)
	}
	return transport
}

// validatingProxy 在选择代理前校验目标 URL；Transport 对每个请求（含重定向）都会调用它，
// 因而无论最终是否经过代理，目标 URL 都不会绕过策略的字面量校验。
// 确定走代理时，代理会自行解析目标主机、绕过受保护 dialer，因此再补一次目标主机校验。
func validatingProxy(policy OutboundPolicy, next func(*http.Request) (*url.URL, error), lookupIPAddr lookupIPAddrFunc) func(*http.Request) (*url.URL, error) {
	return func(request *http.Request) (*url.URL, error) {
		if request == nil || request.URL == nil {
			return nil, fmt.Errorf("%w: invalid url", ErrUnsafeOutboundURL)
		}
		// userinfo 在 transport 层只会变成 Authorization 头，不改变拨号目标；去掉后再校验，
		// 避免误拒 GeoIP 下载等带凭据的固定地址（主机名歧义已由上层的 URL 校验处理）。
		target := *request.URL
		target.User = nil
		if err := ValidateOutboundHTTPURL(target.String(), policy); err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		proxyURL, err := next(request)
		if err != nil || proxyURL == nil {
			return proxyURL, err
		}
		if err := validateProxiedTargetHost(request.Context(), request.URL.Hostname(), policy, lookupIPAddr); err != nil {
			return nil, err
		}
		return proxyURL, nil
	}
}

// validateProxiedTargetHost 校验将交给代理解析的目标主机：
// 拒绝会被 inet_aton 风格解析器当成 IPv4 的数字主机名（如 127.1、2130706433、0x7f000001），
// 并在本地预解析域名、按与受保护 dialer 相同的规则校验全部结果。
// 本地解析失败时放行：只有代理能解析的网络（如受限内网出口）仍需可用，此时出口控制由代理负责。
// 预解析只能拦截固定解析到受限地址的域名，无法防御代理解析时的 DNS rebinding。
func validateProxiedTargetHost(ctx context.Context, rawHost string, policy OutboundPolicy, lookupIPAddr lookupIPAddrFunc) error {
	host := normalizeURLHostname(rawHost)
	if net.ParseIP(host) != nil {
		// 字面 IP 已由 ValidateOutboundHTTPURL 校验，代理不会再解析。
		return nil
	}
	hostAllowed := policy.allowsHost(host)
	if isNumericHostname(host) && !hostAllowed {
		return fmt.Errorf("%w: numeric host", ErrUnsafeOutboundURL)
	}
	if lookupIPAddr == nil {
		return nil
	}
	records, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, record := range records {
		ip := record.IP
		if ip == nil {
			continue
		}
		if isNeverAllowedIP(ip) || (policy.isRestrictedIP(ip) && !hostAllowed && !policy.allowsIP(ip)) {
			return fmt.Errorf("%w: unsafe resolved ip", ErrUnsafeOutboundURL)
		}
	}
	return nil
}

// isNumericHostname 报告 host 的最后一段是否为纯十进制或 0x 十六进制。
// 合法顶级域名不会是纯数字，这类主机名只可能被解析器当成 IPv4 的简写、整数、八进制或十六进制形式。
func isNumericHostname(host string) bool {
	label := host[strings.LastIndex(host, ".")+1:]
	if label == "" {
		return false
	}
	digits := "0123456789"
	if strings.HasPrefix(label, "0x") {
		label = label[2:]
		digits = "0123456789abcdef"
	}
	for _, char := range label {
		if !strings.ContainsRune(digits, char) {
			return false
		}
	}
	return true
}

// allowProxyDial 对代理地址直接拨号，其余地址交给受策略保护的 dialer。
// 只按字面地址精确匹配：解析到代理 IP 的其他域名仍会经过策略校验。
func allowProxyDial(proxyAddresses map[string]struct{}, guarded dialContextFunc, timeout time.Duration) dialContextFunc {
	if len(proxyAddresses) == 0 {
		return guarded
	}
	if timeout <= 0 {
		timeout = defaultOutboundConnectTimeout
	}
	direct := (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		if _, ok := proxyAddresses[normalizeDialAddress(address)]; ok {
			return direct(ctx, network, address)
		}
		return guarded(ctx, network, address)
	}
}

// proxyDialAddresses 返回代理配置中 transport 实际会拨打的代理地址集合。
func proxyDialAddresses(cfg *httpproxy.Config) map[string]struct{} {
	addresses := make(map[string]struct{}, 2)
	for _, raw := range []string{cfg.HTTPProxy, cfg.HTTPSProxy} {
		if address, ok := proxyDialAddress(raw); ok {
			addresses[address] = struct{}{}
		}
	}
	return addresses
}

// proxyDialAddress 与 net/http 对代理 URL 的规范化保持一致：无 scheme 视为 http，按 scheme 补默认端口。
func proxyDialAddress(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return "", false
	}
	port := parsed.Port()
	if port == "" {
		switch strings.ToLower(parsed.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			return "", false
		}
	}
	return net.JoinHostPort(normalizeURLHostname(parsed.Hostname()), port), true
}

func normalizeDialAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return net.JoinHostPort(normalizeURLHostname(host), port)
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
		if isNeverAllowedIP(ip) || (policy.isRestrictedLiteralIP(ip) && !policy.allowsIP(ip)) {
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
		if isNeverAllowedIP(ip) || (policy.isRestrictedIP(ip) && !hostAllowed && !policy.allowsIP(ip)) {
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

// IsValidHostname 报告 host 是否为合法的 DNS 主机名（不含协议、端口、路径，也不是 IP）。
func IsValidHostname(host string) bool {
	return isValidAllowlistedHostname(normalizeURLHostname(host))
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

// isRestrictedIP 报告 ip 是否需要显式授权才能访问：私网与回环始终如此，publicOnly 策略还包括所有非公网单播网段。
func (p OutboundPolicy) isRestrictedIP(ip net.IP) bool {
	return isPrivateOrLoopbackIP(ip) || (p.publicOnly && isNonPublicIP(ip))
}

// isRestrictedLiteralIP 用于直接填写的 IP：在 isRestrictedIP 之外，publicOnly 策略还拒绝 fake-IP 网段。
func (p OutboundPolicy) isRestrictedLiteralIP(ip net.IP) bool {
	return isPrivateOrLoopbackIP(ip) || (p.publicOnly && isLiteralNonPublicIP(ip))
}

// nonPublicPrefixes 是 IANA 特殊用途网段中不属于公网单播、或可能被转换回内网地址的部分。
var nonPublicPrefixes = func() []netip.Prefix {
	raw := []string{
		"0.0.0.0/8",       // 本网络
		"100.64.0.0/10",   // CGNAT，常见于 Tailscale 等内网
		"192.0.0.0/24",    // IETF 协议分配
		"192.0.2.0/24",    // 文档网段
		"198.51.100.0/24", // 文档网段
		"203.0.113.0/24",  // 文档网段
		"240.0.0.0/4",     // 保留（含广播）
		"64:ff9b::/96",    // NAT64，可映射到任意 IPv4（含内网）
		"64:ff9b:1::/48",  // 本地 NAT64
		"100::/64",        // 丢弃前缀
		"2001::/23",       // IETF 协议分配（含 Teredo）
		"2001:db8::/32",   // 文档网段
		"2002::/16",       // 6to4，可嵌入任意 IPv4（含内网）
		"fec0::/10",       // 已废弃的站点本地
	}
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}()

// fakeIPPrefix 是 RFC 2544 基准测试网段。Clash、Surge 等透明代理把它用作 fake-IP 池：
// 开启后任何域名都解析到这里、由本机代理接管真实连接，屏蔽它会让这类部署完全无法访问外部服务。
// 它不承载真实主机，因此只对“直接填写的 IP”拒绝，对域名解析结果放行。
var fakeIPPrefix = netip.MustParsePrefix("198.18.0.0/15")

// isLiteralNonPublicIP 在 isNonPublicIP 之外额外拒绝 fake-IP 网段，用于用户直接填写 IP 的场景。
func isLiteralNonPublicIP(ip net.IP) bool {
	if isNonPublicIP(ip) {
		return true
	}
	address, ok := netip.AddrFromSlice(ip)
	return ok && fakeIPPrefix.Contains(address.Unmap())
}

func isNonPublicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() {
		return true
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
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
