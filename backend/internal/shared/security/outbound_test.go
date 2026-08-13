package security

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestNewOutboundPolicyRejectsInvalidAllowlistEntries(t *testing.T) {
	tests := []struct {
		name  string
		hosts []string
		cidrs []string
	}{
		{name: "URL as host", hosts: []string{"http://new-api:3000"}},
		{name: "wildcard host", hosts: []string{"*.internal"}},
		{name: "IP as host", hosts: []string{"172.17.0.1"}},
		{name: "metadata host", hosts: []string{"metadata.google.internal"}},
		{name: "invalid CIDR", cidrs: []string{"172.17.0.1"}},
		{name: "IPv4 mapped IPv6 CIDR", cidrs: []string{"::ffff:127.0.0.0/120"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewOutboundPolicy(true, test.hosts, test.cidrs); !errors.Is(err, ErrInvalidOutboundPolicy) {
				t.Fatalf("expected invalid policy error, got %v", err)
			}
		})
	}
}

func TestValidateOutboundHTTPURLUsesExplicitAllowlist(t *testing.T) {
	strict := mustOutboundPolicy(t, true, nil, nil)
	if err := ValidateOutboundHTTPURL("http://127.0.0.1:8080", strict); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("expected loopback URL to be rejected, got %v", err)
	}

	allowed := mustOutboundPolicy(t, true, []string{"localhost"}, []string{"172.17.0.0/16"})
	for _, rawURL := range []string{"http://localhost:8080", "http://172.17.0.1:3000"} {
		if err := ValidateOutboundHTTPURL(rawURL, allowed); err != nil {
			t.Fatalf("expected %s to be allowed: %v", rawURL, err)
		}
	}
	if err := ValidateOutboundHTTPURL("http://10.0.0.1:3000", allowed); !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("expected non-allowlisted private URL to be rejected, got %v", err)
	}
}

func TestWithTrustedHTTPURLsAddsExactTargetsWithoutMutatingBasePolicy(t *testing.T) {
	base := NewStrictOutboundPolicy(true)
	trusted, err := base.WithTrustedHTTPURLs(
		"http://new-api:3000/oidc",
		"http://127.0.0.1:8080/verify",
	)
	if err != nil {
		t.Fatalf("derive trusted policy: %v", err)
	}

	for _, rawURL := range []string{
		"http://new-api:3000/.well-known/openid-configuration",
		"http://127.0.0.1:8080/token",
	} {
		if err = ValidateOutboundHTTPURL(rawURL, trusted); err != nil {
			t.Fatalf("expected trusted target %s to be allowed: %v", rawURL, err)
		}
	}
	if trusted.allowsHost("different-service") {
		t.Fatal("unrelated hostname was added to the trusted policy")
	}
	for _, rawURL := range []string{
		"http://127.0.0.2:8080/token",
	} {
		if err = ValidateOutboundHTTPURL(rawURL, trusted); !errors.Is(err, ErrUnsafeOutboundURL) {
			t.Fatalf("expected unrelated target %s to remain blocked, got %v", rawURL, err)
		}
	}
	if base.allowsHost("new-api") {
		t.Fatal("base policy was mutated")
	}
}

func TestWithTrustedHTTPURLsRejectsPermanentlyBlockedTargets(t *testing.T) {
	base := NewStrictOutboundPolicy(true)
	for _, rawURL := range []string{
		"http://metadata.google.internal/computeMetadata/v1",
		"http://169.254.169.254/latest/meta-data",
		"http://[fe80::1]/",
		"file:///etc/passwd",
		"http://user:password@localhost:8080/",
	} {
		if _, err := base.WithTrustedHTTPURLs(rawURL); !errors.Is(err, ErrInvalidOutboundPolicy) {
			t.Fatalf("expected trusted target %s to be rejected, got %v", rawURL, err)
		}
	}
}

func TestHTTPOriginNormalizesSchemeHostAndDefaultPort(t *testing.T) {
	origin, err := HTTPOrigin("HTTPS://Example.COM.:443/provider/path?query=1")
	if err != nil {
		t.Fatalf("normalize origin: %v", err)
	}
	if origin != "https://example.com" {
		t.Fatalf("origin = %q, want https://example.com", origin)
	}

	origin, err = HTTPOrigin("http://[::1]:8080/v1")
	if err != nil {
		t.Fatalf("normalize IPv6 origin: %v", err)
	}
	if origin != "http://[::1]:8080" {
		t.Fatalf("origin = %q, want http://[::1]:8080", origin)
	}
}

func TestValidateOutboundHTTPURLNeverAllowsMetadataTargets(t *testing.T) {
	policy := mustOutboundPolicy(t, true, nil, []string{"0.0.0.0/0", "::/0"})
	for _, rawURL := range []string{
		"http://169.254.169.254/latest/meta-data",
		"http://100.100.100.200/latest/meta-data",
		"http://[fd00:ec2::254]/latest/meta-data",
		"http://[fe80::1%25en0]/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1",
	} {
		if err := ValidateOutboundHTTPURL(rawURL, policy); !errors.Is(err, ErrUnsafeOutboundURL) {
			t.Fatalf("expected metadata URL %s to remain rejected, got %v", rawURL, err)
		}
	}
}

func TestValidateOutboundHTTPURLSkipsNetworkChecksWhenDisabled(t *testing.T) {
	policy := mustOutboundPolicy(t, false, nil, nil)
	if err := ValidateOutboundHTTPURL("http://127.0.0.1:8080", policy); err != nil {
		t.Fatalf("disabled policy should not enforce network checks: %v", err)
	}
}

func TestOutboundDialerRejectsResolvedPrivateIP(t *testing.T) {
	dialCalled := false
	dial := newOutboundDialContext(
		mustOutboundPolicy(t, true, nil, nil),
		lookupAddresses("10.0.0.10"),
		func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("dial should not be called")
		},
	)

	_, err := dial(context.Background(), "tcp", "example.com:443")
	if !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("expected unsafe outbound error, got %v", err)
	}
	if dialCalled {
		t.Fatal("unsafe resolved IP must be rejected before dialing")
	}
}

func TestOutboundDialerRejectsMixedResolvedIPs(t *testing.T) {
	dialCalled := false
	dial := newOutboundDialContext(
		mustOutboundPolicy(t, true, nil, []string{"10.0.0.0/24"}),
		lookupAddresses("10.0.0.10", "10.0.1.10"),
		func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("dial should not be called")
		},
	)

	_, err := dial(context.Background(), "tcp", "example.com:443")
	if !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("expected unsafe outbound error, got %v", err)
	}
	if dialCalled {
		t.Fatal("mixed authorized and unauthorized DNS answers must be rejected before dialing")
	}
}

func TestOutboundDialerDialsResolvedPublicIP(t *testing.T) {
	assertDialAddress(t, mustOutboundPolicy(t, true, nil, nil), "example.com:443", lookupAddresses("8.8.8.8"), "8.8.8.8:443")
}

func TestOutboundDialerAllowsExactHostnameWithPrivateResolution(t *testing.T) {
	policy := mustOutboundPolicy(t, true, []string{"NEW-API."}, nil)
	assertDialAddress(t, policy, "new-api:3000", lookupAddresses("172.18.0.4"), "172.18.0.4:3000")
}

func TestOutboundDialerAllowsPrivateResolutionByCIDR(t *testing.T) {
	policy := mustOutboundPolicy(t, true, nil, []string{"172.18.0.0/16"})
	assertDialAddress(t, policy, "host.docker.internal:3000", lookupAddresses("172.18.0.1"), "172.18.0.1:3000")
}

func TestOutboundDialerNeverAllowsMetadataResolution(t *testing.T) {
	policy := mustOutboundPolicy(t, true, []string{"trusted-service"}, []string{"0.0.0.0/0"})
	dialCalled := false
	dial := newOutboundDialContext(policy, lookupAddresses("169.254.169.254"), func(context.Context, string, string) (net.Conn, error) {
		dialCalled = true
		return nil, errors.New("dial should not be called")
	})
	_, err := dial(context.Background(), "tcp", "trusted-service:80")
	if !errors.Is(err, ErrUnsafeOutboundURL) {
		t.Fatalf("expected metadata resolution to be rejected, got %v", err)
	}
	if dialCalled {
		t.Fatal("metadata target must be rejected before dialing")
	}
}

func TestOutboundDialerSkipsResolutionWhenPolicyIsNotEnforced(t *testing.T) {
	resolveCalled := false
	var dialAddress string
	dialErr := errors.New("dial sentinel")
	dial := newOutboundDialContext(
		mustOutboundPolicy(t, false, nil, nil),
		func(context.Context, string) ([]net.IPAddr, error) {
			resolveCalled = true
			return nil, errors.New("resolve should not be called")
		},
		func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialAddress = address
			return nil, dialErr
		},
	)

	_, err := dial(context.Background(), "tcp", "localhost:8080")
	if !errors.Is(err, dialErr) {
		t.Fatalf("expected dial sentinel, got %v", err)
	}
	if resolveCalled {
		t.Fatal("non-enforced policy should not resolve the target")
	}
	if dialAddress != "localhost:8080" {
		t.Fatalf("expected original address, got %q", dialAddress)
	}
}

func mustOutboundPolicy(t *testing.T, enforce bool, hosts []string, cidrs []string) OutboundPolicy {
	t.Helper()
	policy, err := NewOutboundPolicy(enforce, hosts, cidrs)
	if err != nil {
		t.Fatalf("build outbound policy: %v", err)
	}
	return policy
}

func lookupAddresses(values ...string) lookupIPAddrFunc {
	return func(context.Context, string) ([]net.IPAddr, error) {
		result := make([]net.IPAddr, 0, len(values))
		for _, value := range values {
			result = append(result, net.IPAddr{IP: net.ParseIP(value)})
		}
		return result, nil
	}
}

func assertDialAddress(t *testing.T, policy OutboundPolicy, address string, lookup lookupIPAddrFunc, want string) {
	t.Helper()
	var dialAddress string
	dialErr := errors.New("dial sentinel")
	dial := newOutboundDialContext(policy, lookup, func(_ context.Context, _ string, address string) (net.Conn, error) {
		dialAddress = address
		return nil, dialErr
	})
	_, err := dial(context.Background(), "tcp", address)
	if !errors.Is(err, dialErr) {
		t.Fatalf("expected dial sentinel, got %v", err)
	}
	if dialAddress != want {
		t.Fatalf("expected dial address %q, got %q", want, dialAddress)
	}
}
