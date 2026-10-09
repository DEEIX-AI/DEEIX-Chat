package security

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"golang.org/x/net/http/httpproxy"
)

func TestValidatingProxyRejectsUnsafeTargetsBeforeSelectingProxy(t *testing.T) {
	cases := []struct {
		name   string
		policy OutboundPolicy
		target string
	}{
		{"metadata", NewStrictOutboundPolicy(true), "http://169.254.169.254/latest/meta-data"},
		{"metadata host", NewStrictOutboundPolicy(true), "http://metadata.google.internal/"},
		{"private literal", NewStrictOutboundPolicy(true), "http://10.0.0.1/"},
		{"user endpoint on local proxy port", NewPublicOnlyOutboundPolicy(), "http://127.0.0.1:7890/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			proxy := validatingProxy(tc.policy, func(*http.Request) (*url.URL, error) {
				called = true
				return nil, nil
			}, nil)
			request, err := http.NewRequest(http.MethodGet, tc.target, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := proxy(request); !errors.Is(err, ErrUnsafeOutboundURL) {
				t.Fatalf("expected ErrUnsafeOutboundURL, got %v", err)
			}
			if called {
				t.Fatal("proxy selector must not run for rejected targets")
			}
		})
	}
}

func TestValidatingProxyDelegatesForAllowedTargets(t *testing.T) {
	want, _ := url.Parse("http://proxy.example:3128")
	proxy := validatingProxy(NewStrictOutboundPolicy(true), func(*http.Request) (*url.URL, error) {
		return want, nil
	}, lookupAddresses("93.184.215.14"))
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/models", nil)
	got, err := proxy(request)
	if err != nil || got != want {
		t.Fatalf("expected delegated proxy %v, got %v (err %v)", want, got, err)
	}
}

func TestValidatingProxyHonorsLocallyTrustedAdminEndpoint(t *testing.T) {
	policy, err := NewStrictOutboundPolicy(true).WithTrustedHTTPURLs("http://10.8.0.7:11434")
	if err != nil {
		t.Fatal(err)
	}
	proxy := validatingProxy(policy, func(*http.Request) (*url.URL, error) { return nil, nil }, nil)
	request, _ := http.NewRequest(http.MethodPost, "http://10.8.0.7:11434/v1/chat/completions", nil)
	if _, err := proxy(request); err != nil {
		t.Fatalf("admin-configured private endpoint must stay allowed: %v", err)
	}
}

func TestAllowProxyDialBypassesGuardOnlyForProxyAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	errGuarded := errors.New("guarded")
	guarded := func(context.Context, string, string) (net.Conn, error) { return nil, errGuarded }
	dial := allowProxyDial(map[string]struct{}{listener.Addr().String(): {}}, guarded, time.Second)

	conn, err := dial(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("proxy address should dial directly: %v", err)
	}
	conn.Close()

	if _, err := dial(context.Background(), "tcp", "127.0.0.1:1"); !errors.Is(err, errGuarded) {
		t.Fatalf("non-proxy address must use guarded dialer, got %v", err)
	}
}

func TestAllowProxyDialReturnsGuardedDialerWithoutProxy(t *testing.T) {
	errGuarded := errors.New("guarded")
	guarded := func(context.Context, string, string) (net.Conn, error) { return nil, errGuarded }
	dial := allowProxyDial(nil, guarded, time.Second)
	if _, err := dial(context.Background(), "tcp", "127.0.0.1:7890"); !errors.Is(err, errGuarded) {
		t.Fatalf("without proxy addresses every dial must be guarded, got %v", err)
	}
}

func TestProxyDialAddress(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:7890":    "127.0.0.1:7890",
		"http://Proxy.LAN":  "proxy.lan:80",
		"https://proxy.lan": "proxy.lan:443",
		"socks5://[::1]":    "[::1]:1080",
	}
	for raw, want := range cases {
		if got, ok := proxyDialAddress(raw); !ok || got != want {
			t.Errorf("proxyDialAddress(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "   ", "ftp://proxy.lan"} {
		if _, ok := proxyDialAddress(raw); ok {
			t.Errorf("proxyDialAddress(%q) should be rejected", raw)
		}
	}
}

func TestProxyDialAddressesCoversHTTPAndHTTPSProxies(t *testing.T) {
	addresses := proxyDialAddresses(&httpproxy.Config{HTTPProxy: "127.0.0.1:7890", HTTPSProxy: "http://proxy.lan:3128"})
	for _, want := range []string{"127.0.0.1:7890", "proxy.lan:3128"} {
		if _, ok := addresses[want]; !ok {
			t.Errorf("expected proxy dial address %q in %v", want, addresses)
		}
	}
}

func TestOutboundTransportKeepsEnvironmentProxyWhenNotEnforced(t *testing.T) {
	want, _ := url.Parse("http://proxy.example:3128")
	transport := newOutboundHTTPTransport(NewStrictOutboundPolicy(false), time.Second,
		func(*http.Request) (*url.URL, error) { return want, nil }, nil, nil)
	request, _ := http.NewRequest(http.MethodGet, "http://10.0.0.1/", nil)
	if got, err := transport.Proxy(request); err != nil || got != want {
		t.Fatalf("unenforced transport should use proxy as-is, got %v (err %v)", got, err)
	}
}

func TestOutboundTransportValidatesTargetsWhenEnforced(t *testing.T) {
	want, _ := url.Parse("http://proxy.example:3128")
	transport := newOutboundHTTPTransport(NewStrictOutboundPolicy(true), time.Second,
		func(*http.Request) (*url.URL, error) { return want, nil }, nil, nil)
	request, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/", nil)
	if _, err := transport.Proxy(request); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("enforced transport must reject metadata targets, got %v", err)
	}
}

// TestOutboundTransportRoutesThroughLoopbackProxyWhenEnforced 端到端验证：
// 启用防护时，本机回环上的代理可以被拨通，公网目标的请求经代理转发。
func TestOutboundTransportRoutesThroughLoopbackProxyWhenEnforced(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proxied-Host", r.URL.Host)
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = proxyServer.Serve(listener) }()
	defer proxyServer.Close()

	proxyURL, _ := url.Parse("http://" + listener.Addr().String())
	transport := newOutboundHTTPTransport(NewStrictOutboundPolicy(true), time.Second,
		http.ProxyURL(proxyURL), map[string]struct{}{listener.Addr().String(): {}},
		lookupByHost(map[string]string{"api.example.com": "93.184.215.14", "evil.example": "127.0.0.1"}))
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	response, err := client.Get("http://api.example.com/v1/models")
	if err != nil {
		t.Fatalf("request through loopback proxy failed: %v", err)
	}
	response.Body.Close()
	if got := response.Header.Get("X-Proxied-Host"); got != "api.example.com" {
		t.Fatalf("expected request to reach proxy for api.example.com, got %q", got)
	}

	if _, err := client.Get("http://169.254.169.254/latest/meta-data"); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("metadata target must be rejected before reaching the proxy, got %v", err)
	}
	if _, err := client.Get("http://evil.example/"); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("host resolving to loopback must be rejected before reaching the proxy, got %v", err)
	}
	if _, err := client.Get("http://127.1/"); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("numeric host must be rejected before reaching the proxy, got %v", err)
	}
}

func TestValidatingProxyRejectsNumericHostsWhenProxied(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	selectProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }
	failLookup := func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("numeric hosts must be rejected without DNS lookup")
		return nil, nil
	}
	for _, policy := range []OutboundPolicy{NewStrictOutboundPolicy(true), NewPublicOnlyOutboundPolicy()} {
		proxy := validatingProxy(policy, selectProxy, failLookup)
		for _, target := range []string{
			"http://127.1/",
			"http://2130706433/",
			"http://0x7f000001/",
			"http://0177.0.0.1/",
			"http://10.0.0.0x1/",
			"http://127.1.:8080/",
		} {
			request, err := http.NewRequest(http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := proxy(request); !errors.Is(err, ErrUnsafeOutboundURL) {
				t.Errorf("%s: expected ErrUnsafeOutboundURL, got %v", target, err)
			}
		}
	}
}

func TestValidatingProxyChecksResolvedAddressesWhenProxied(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	selectProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }
	adminPolicy, err := NewStrictOutboundPolicy(true).WithTrustedHTTPURLs("http://ollama.internal:11434")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		policy  OutboundPolicy
		target  string
		lookup  lookupIPAddrFunc
		allowed bool
	}{
		{"public host", NewPublicOnlyOutboundPolicy(), "https://api.example.com/", lookupAddresses("93.184.215.14"), true},
		{"resolves to loopback", NewStrictOutboundPolicy(true), "http://127.0.0.1.nip.io/", lookupAddresses("127.0.0.1"), false},
		{"resolves to private for user endpoint", NewPublicOnlyOutboundPolicy(), "http://evil.example/", lookupAddresses("10.0.0.5"), false},
		{"mixed resolution", NewPublicOnlyOutboundPolicy(), "http://evil.example/", lookupAddresses("93.184.215.14", "10.0.0.5"), false},
		{"resolves to CGNAT for user endpoint", NewPublicOnlyOutboundPolicy(), "http://evil.example/", lookupAddresses("100.64.0.1"), false},
		{"resolves to metadata", NewStrictOutboundPolicy(true), "http://evil.example/", lookupAddresses("169.254.169.254"), false},
		{"fake-ip pool", NewPublicOnlyOutboundPolicy(), "https://api.example.com/", lookupAddresses("198.18.0.5"), true},
		{"lookup failure leaves resolution to proxy", NewPublicOnlyOutboundPolicy(), "https://only-proxy-resolves.example/",
			func(context.Context, string) ([]net.IPAddr, error) { return nil, errors.New("no such host") }, true},
		{"admin endpoint resolving to private", adminPolicy, "http://ollama.internal:11434/v1/models", lookupAddresses("10.8.0.7"), true},
		{"admin endpoint resolving to metadata", adminPolicy, "http://ollama.internal:11434/v1/models", lookupAddresses("169.254.169.254"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, tc.target, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := validatingProxy(tc.policy, selectProxy, tc.lookup)(request)
			if tc.allowed {
				if err != nil || got != proxyURL {
					t.Fatalf("expected proxied request to be allowed, got %v (err %v)", got, err)
				}
				return
			}
			if !errors.Is(err, ErrUnsafeOutboundURL) {
				t.Fatalf("expected ErrUnsafeOutboundURL, got %v", err)
			}
		})
	}
}

func TestValidatingProxySkipsHostChecksForDirectRequests(t *testing.T) {
	failLookup := func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("direct requests are validated by the guarded dialer, not the proxy selector")
		return nil, nil
	}
	proxy := validatingProxy(NewStrictOutboundPolicy(true), func(*http.Request) (*url.URL, error) { return nil, nil }, failLookup)
	request, _ := http.NewRequest(http.MethodGet, "http://127.1/", nil)
	if got, err := proxy(request); err != nil || got != nil {
		t.Fatalf("direct request should be left to the guarded dialer, got %v (err %v)", got, err)
	}
}

func TestValidatingProxyIgnoresUserinfoInTarget(t *testing.T) {
	proxy := validatingProxy(NewStrictOutboundPolicy(true), func(*http.Request) (*url.URL, error) { return nil, nil }, nil)
	request, _ := http.NewRequest(http.MethodGet, "https://account:license@download.maxmind.com/geoip/databases/GeoLite2-City/download", nil)
	if _, err := proxy(request); err != nil {
		t.Fatalf("credentials in a fixed download URL must not be rejected at transport level: %v", err)
	}
	if request.URL.User == nil {
		t.Fatal("validation must not strip credentials from the outgoing request")
	}

	unsafe, _ := http.NewRequest(http.MethodGet, "http://user:pw@169.254.169.254/", nil)
	if _, err := proxy(unsafe); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("userinfo must not mask an unsafe target, got %v", err)
	}
}

func TestOutboundProxyStatusRedactsAndDetectsIgnoredAllProxy(t *testing.T) {
	status := newOutboundProxyStatus(&httpproxy.Config{
		HTTPProxy:  "http://user:secret@proxy.lan:3128/path",
		HTTPSProxy: "127.0.0.1:7890",
		NoProxy:    " localhost,.internal ",
	}, "")
	if status.HTTPProxy != "http://proxy.lan:3128" || status.HTTPSProxy != "http://127.0.0.1:7890" {
		t.Fatalf("proxy URLs must keep only scheme and host, got %q / %q", status.HTTPProxy, status.HTTPSProxy)
	}
	if status.NoProxy != "localhost,.internal" || !status.Configured() || status.IgnoredAllProxy {
		t.Fatalf("unexpected status %+v", status)
	}

	onlyAllProxy := newOutboundProxyStatus(&httpproxy.Config{}, "socks5://127.0.0.1:7891")
	if !onlyAllProxy.IgnoredAllProxy || onlyAllProxy.Configured() {
		t.Fatalf("ALL_PROXY alone must be reported as ignored, got %+v", onlyAllProxy)
	}
	withStandard := newOutboundProxyStatus(&httpproxy.Config{HTTPSProxy: "http://proxy.lan:3128"}, "socks5://127.0.0.1:7891")
	if withStandard.IgnoredAllProxy {
		t.Fatal("ALL_PROXY must not be reported as ignored when standard proxy variables are set")
	}
}

func lookupByHost(records map[string]string) lookupIPAddrFunc {
	return func(_ context.Context, host string) ([]net.IPAddr, error) {
		value, ok := records[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		return []net.IPAddr{{IP: net.ParseIP(value)}}, nil
	}
}
