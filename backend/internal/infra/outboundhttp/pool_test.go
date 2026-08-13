package outboundhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestPoolReusesClientsByExactOriginAndVariant(t *testing.T) {
	created := 0
	pool := NewPool(security.NewStrictOutboundPolicy(true), 4, func(_ security.OutboundPolicy, _ string, _ string) (ManagedClient, error) {
		created++
		return ManagedClient{Client: &http.Client{}}, nil
	})

	first, err := pool.clientForEndpoint("https://example.com/v1", "10000")
	if err != nil {
		t.Fatalf("first client: %v", err)
	}
	second, err := pool.clientForEndpoint("https://EXAMPLE.com:443/other", "10000")
	if err != nil {
		t.Fatalf("second client: %v", err)
	}
	differentPort, err := pool.clientForEndpoint("https://example.com:8443/v1", "10000")
	if err != nil {
		t.Fatalf("different port client: %v", err)
	}
	differentVariant, err := pool.clientForEndpoint("https://example.com/v1", "2500")
	if err != nil {
		t.Fatalf("different variant client: %v", err)
	}
	if first != second {
		t.Fatal("expected equivalent origins to reuse one client")
	}
	if first == differentPort || first == differentVariant {
		t.Fatal("expected port and transport variant to isolate clients")
	}
	if created != 3 {
		t.Fatalf("created clients = %d, want 3", created)
	}
}

func TestPoolReusesOneClientUnderConcurrentRequests(t *testing.T) {
	var created atomic.Int64
	pool := NewPool(security.NewStrictOutboundPolicy(true), 4, func(_ security.OutboundPolicy, _ string, _ string) (ManagedClient, error) {
		created.Add(1)
		return ManagedClient{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
		})}}, nil
	})

	const workers = 128
	errorsCh := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			request, err := http.NewRequest(http.MethodGet, "http://model.internal:8080/v1/models", nil)
			if err == nil {
				_, err = pool.Do(request, "http://model.internal:8080/v1", "10000")
			}
			errorsCh <- err
		}()
	}
	waitGroup.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent pooled request: %v", err)
		}
	}
	if created.Load() != 1 {
		t.Fatalf("created clients = %d, want 1", created.Load())
	}
}

func TestPoolEvictsAndClosesLeastRecentlyUsedClient(t *testing.T) {
	closed := 0
	pool := NewPool(security.NewStrictOutboundPolicy(true), 1, func(_ security.OutboundPolicy, _ string, _ string) (ManagedClient, error) {
		return ManagedClient{
			Client: &http.Client{},
			CloseIdleConnections: func() {
				closed++
			},
		}, nil
	})
	if _, err := pool.clientForEndpoint("https://first.example", ""); err != nil {
		t.Fatalf("first client: %v", err)
	}
	if _, err := pool.clientForEndpoint("https://second.example", ""); err != nil {
		t.Fatalf("second client: %v", err)
	}
	if closed != 1 {
		t.Fatalf("closed clients = %d, want 1", closed)
	}
	pool.CloseIdleConnections()
	if closed != 2 {
		t.Fatalf("closed clients after shutdown = %d, want 2", closed)
	}
}

func TestPoolRejectsRequestOutsideConfiguredOrigin(t *testing.T) {
	called := false
	pool := NewPool(security.NewStrictOutboundPolicy(true), 4, func(_ security.OutboundPolicy, _ string, _ string) (ManagedClient, error) {
		return ManagedClient{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
		})}}, nil
	})
	outsideRequest, err := http.NewRequest(http.MethodPost, "http://model.internal:8081/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Do(
		outsideRequest,
		"http://model.internal:8080/v1",
		"",
	); err == nil {
		t.Fatal("expected different target port to be rejected")
	}
	if called {
		t.Fatal("rejected request reached transport")
	}
	insideRequest, err := http.NewRequest(http.MethodPost, "http://model.internal:8080/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Do(
		insideRequest,
		"http://model.internal:8080/v1",
		"",
	); err != nil {
		t.Fatalf("same-origin request rejected: %v", err)
	}
	if !called {
		t.Fatal("same-origin request did not reach transport")
	}
}

func TestRedirectPolicyAllowsPublicAndAllowlistedTargets(t *testing.T) {
	policy, err := security.NewOutboundPolicy(true, []string{"redirect.internal"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkRedirect := NewRedirectPolicy(policy, "http://model.internal:8080", "test integration")
	for _, target := range []string{
		"http://model.internal:8080/next",
		"https://public.example/next",
		"http://redirect.internal:9000/next",
	} {
		request, requestErr := http.NewRequest(http.MethodGet, target, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if redirectErr := checkRedirect(request, nil); redirectErr != nil {
			t.Fatalf("redirect %q rejected: %v", target, redirectErr)
		}
	}
}

func TestRedirectPolicyRejectsUntrustedPrivateTarget(t *testing.T) {
	checkRedirect := NewRedirectPolicy(security.NewStrictOutboundPolicy(true), "http://model.internal:8080", "test integration")
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9000/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkRedirect(request, nil); !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected unsafe redirect error, got %v", err)
	}
}

func TestRedirectPolicyPreservesDisabledSSRFBehavior(t *testing.T) {
	checkRedirect := NewRedirectPolicy(security.NewStrictOutboundPolicy(false), "http://model.internal:8080", "test integration")
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9000/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkRedirect(request, nil); err != nil {
		t.Fatalf("redirect rejected while SSRF protection is disabled: %v", err)
	}
}

func TestPoolFollowsAllowlistedPrivateRedirectButRejectsItByDefault(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Redirect(responseWriter, request, target.URL+"/result", http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	newHTTPPool := func(policy security.OutboundPolicy) *Pool {
		return NewPool(policy, 4, func(trustedPolicy security.OutboundPolicy, trustedOrigin string, _ string) (ManagedClient, error) {
			transport := security.NewOutboundHTTPTransport(trustedPolicy, 0)
			return ManagedClient{
				Client: &http.Client{
					Transport:     transport,
					CheckRedirect: NewRedirectPolicy(policy, trustedOrigin, "test integration"),
				},
				CloseIdleConnections: transport.CloseIdleConnections,
			}, nil
		})
	}
	request, err := http.NewRequest(http.MethodGet, redirector.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	strictPool := newHTTPPool(security.NewStrictOutboundPolicy(true))
	defer strictPool.CloseIdleConnections()
	if _, err = strictPool.Do(request, redirector.URL, ""); !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected strict policy to reject private redirect, got %v", err)
	}

	allowlistedPolicy, err := security.NewOutboundPolicy(true, nil, []string{"127.0.0.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	request, err = http.NewRequest(http.MethodGet, redirector.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	allowlistedPool := newHTTPPool(allowlistedPolicy)
	defer allowlistedPool.CloseIdleConnections()
	response, err := allowlistedPool.Do(request, redirector.URL, "")
	if err != nil {
		t.Fatalf("allowlisted private redirect failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected redirect response status %d", response.StatusCode)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
