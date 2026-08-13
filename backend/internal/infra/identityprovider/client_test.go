package identityprovider

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestClientSelectsTrustedClientOnlyForConfiguredOrigin(t *testing.T) {
	trustedEndpoint, err := trustedEndpointFor(
		"http://localhost:8080/token",
		[]string{"http://localhost:8080/issuer"},
	)
	if err != nil {
		t.Fatalf("select configured private origin: %v", err)
	}
	if trustedEndpoint != "http://localhost:8080/issuer" {
		t.Fatalf("unexpected trusted endpoint %q", trustedEndpoint)
	}

	strictEndpoint, err := trustedEndpointFor(
		"http://localhost:8081/token",
		[]string{"http://localhost:8080/issuer"},
	)
	if err != nil {
		t.Fatalf("select unrelated origin: %v", err)
	}
	if strictEndpoint != "" {
		t.Fatal("different port must not inherit the configured origin trust")
	}
}

func TestTrustedEndpointSelectionIsSafeUnderConcurrentAccess(t *testing.T) {
	const workers = 256
	errorsCh := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			endpoint := fmt.Sprintf("http://localhost:%d/issuer", 10000+(index%80))
			_, err := trustedEndpointFor(endpoint, []string{endpoint})
			errorsCh <- err
		}(index)
	}
	waitGroup.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent trusted origin lookup: %v", err)
		}
	}
}

func TestTrustedClientRejectsCrossOriginRedirect(t *testing.T) {
	managed, err := newIdentityProviderHTTPClient(
		security.NewStrictOutboundPolicy(true),
		"http://localhost:8080",
		"",
	)
	if err != nil {
		t.Fatalf("create configured private origin client: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "http://localhost:8081/redirected", nil)
	if err != nil {
		t.Fatalf("build redirect request: %v", err)
	}
	if err = managed.Client.CheckRedirect(request, nil); err == nil {
		t.Fatal("expected cross-origin redirect to be rejected")
	}
}

func TestClientCannotTrustMetadataEndpoint(t *testing.T) {
	_, err := trustedEndpointFor(
		"http://169.254.169.254/latest/meta-data",
		[]string{"http://169.254.169.254/latest/meta-data"},
	)
	if !errors.Is(err, security.ErrInvalidOutboundPolicy) {
		t.Fatalf("expected metadata endpoint to remain permanently blocked, got %v", err)
	}
}

func TestHTTPOriginNormalizesHostAndPreservesPort(t *testing.T) {
	origin, err := security.HTTPOrigin("HTTPS://Example.COM.:8443/path?query=1")
	if err != nil {
		t.Fatalf("normalize origin: %v", err)
	}
	if origin != "https://example.com:8443" {
		t.Fatalf("unexpected origin %q", origin)
	}
}

func TestHTTPOriginNormalizesDefaultPort(t *testing.T) {
	withPort, err := security.HTTPOrigin("https://example.com:443/issuer")
	if err != nil {
		t.Fatalf("normalize origin with default port: %v", err)
	}
	withoutPort, err := security.HTTPOrigin("https://example.com/token")
	if err != nil {
		t.Fatalf("normalize origin without port: %v", err)
	}
	if withPort != withoutPort {
		t.Fatalf("default port changed origin: with=%q without=%q", withPort, withoutPort)
	}
}
