package mediaartifact

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNewClientDoesNotTraceSignedArtifactURLs(t *testing.T) {
	policy := security.NewStrictOutboundPolicy(true)
	managed := newMediaArtifactHTTPClient(policy, policy, "")
	httpClient := managed.Client
	if _, ok := httpClient.Transport.(*http.Transport); !ok {
		t.Fatalf("media artifact transport must avoid generic URL tracing, got %T", httpClient.Transport)
	}
}

func TestDownloadImageReturnsBoundedArtifact(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	client := testClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://cdn.example.test/generated/image" {
			t.Fatalf("unexpected request URL: %s", request.URL)
		}
		return response(http.StatusOK, "image/png; charset=binary", pngHeader), nil
	}))

	data, mimeType, err := client.DownloadImage(t.Context(), "https://cdn.example.test/generated/image", "", int64(len(pngHeader)))
	if err != nil {
		t.Fatalf("download image: %v", err)
	}
	if string(data) != string(pngHeader) || mimeType != "image/png" {
		t.Fatalf("unexpected image result: data=%q MIME=%q", data, mimeType)
	}
}

func TestDownloadImageRejectsOversizedArtifact(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "image/png", []byte("1234")), nil
	}))

	_, _, err := client.DownloadImage(t.Context(), "https://cdn.example.test/generated/image", "", 3)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestDownloadImageRejectsGeminiFilesURI(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unsupported Gemini image URI must not issue a request")
		return nil, nil
	}))

	_, _, err := client.DownloadImage(t.Context(), "https://generativelanguage.googleapis.com/v1beta/files/image_123", "", 1024)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported Gemini image error, got %v", err)
	}
}

func TestDownloadVideoPollsGeminiFileAndUsesResolvedMIME(t *testing.T) {
	requestURLs := make([]string, 0, 3)
	client := testClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestURLs = append(requestURLs, request.URL.String())
		if request.Header.Get(geminiAPIKeyHeader) != "secret" {
			t.Fatalf("missing Gemini API key for %s", request.URL)
		}
		switch len(requestURLs) {
		case 1:
			return response(http.StatusOK, "application/json", []byte(`{"state":"PROCESSING"}`)), nil
		case 2:
			return response(http.StatusOK, "application/json", []byte(`{"file":{"state":"ACTIVE","mimeType":"video/mp4"}}`)), nil
		case 3:
			return response(http.StatusOK, "application/octet-stream", []byte("video-bytes")), nil
		default:
			t.Fatalf("unexpected extra request: %s", request.URL)
			return nil, nil
		}
	}))
	client.pollAttempts = 3
	client.pollInterval = 0

	data, mimeType, err := client.DownloadVideo(
		t.Context(),
		"https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media&key=discarded",
		"",
		"secret",
		1024,
	)
	if err != nil {
		t.Fatalf("download Gemini video: %v", err)
	}
	if string(data) != "video-bytes" || mimeType != "video/mp4" {
		t.Fatalf("unexpected video result: data=%q MIME=%q", data, mimeType)
	}
	wantURLs := []string{
		"https://generativelanguage.googleapis.com/v1beta/files/video_123",
		"https://generativelanguage.googleapis.com/v1beta/files/video_123",
		"https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media",
	}
	if len(requestURLs) != len(wantURLs) {
		t.Fatalf("unexpected request count: %d", len(requestURLs))
	}
	for index := range wantURLs {
		if requestURLs[index] != wantURLs[index] {
			t.Fatalf("request %d: got %q, want %q", index, requestURLs[index], wantURLs[index])
		}
	}
}

func TestDownloadErrorDoesNotExposeSignedSourceURL(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	}))

	_, _, err := client.DownloadImage(t.Context(), "https://cdn.example.test/image?token=secret", "", 1024)
	if err == nil {
		t.Fatal("expected download failure")
	}
	if strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("download error exposed signed URL: %v", err)
	}
	if !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("download error lost underlying cause: %v", err)
	}
}

func TestDownloadVideoRequiresGeminiAPIKeyBeforeRequest(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("missing API key must be rejected before issuing a request")
		return nil, nil
	}))

	_, _, err := client.DownloadVideo(
		t.Context(),
		"https://generativelanguage.googleapis.com/v1beta/files/video_123",
		"",
		"",
		1024,
	)
	if err == nil || !strings.Contains(err.Error(), "requires an API key") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestDownloadVideoUsesBearerTokenForSameOriginProviderArtifact(t *testing.T) {
	client := testClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Fatalf("expected same-origin provider authorization, got %q", got)
		}
		return response(http.StatusOK, "video/mp4", []byte("video-bytes")), nil
	}))

	_, _, err := client.DownloadVideo(
		t.Context(),
		"https://provider.example.test/v1/artifacts/video.mp4",
		"https://provider.example.test/v1",
		"provider-key",
		1024,
	)
	if err != nil {
		t.Fatalf("download same-origin provider video: %v", err)
	}
}

func TestDownloadVideoDoesNotSendBearerTokenToCrossOriginArtifact(t *testing.T) {
	client := testClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("provider authorization leaked to cross-origin artifact: %q", got)
		}
		return response(http.StatusOK, "video/mp4", []byte("video-bytes")), nil
	}))

	_, _, err := client.DownloadVideo(
		t.Context(),
		"https://cdn.example.test/generated/video.mp4",
		"https://provider.example.test/v1",
		"provider-key",
		1024,
	)
	if err != nil {
		t.Fatalf("download cross-origin provider video: %v", err)
	}
}

func TestGeminiMetadataErrorDoesNotExposeResponseBody(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusBadGateway, "application/json", []byte(`{"error":"token=secret user-content"}`)), nil
	}))

	_, _, err := client.DownloadVideo(
		t.Context(),
		"https://generativelanguage.googleapis.com/v1beta/files/video_123",
		"",
		"api-key",
		1024,
	)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("expected status-only metadata error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "user-content") {
		t.Fatalf("metadata error exposed response body: %v", err)
	}
}

func TestRedirectPolicyStripsCredentialsAcrossOrigins(t *testing.T) {
	originalURL, err := url.Parse("https://generativelanguage.googleapis.com/v1beta/files/video_123:download")
	if err != nil {
		t.Fatal(err)
	}
	redirectURL, err := url.Parse("https://storage.googleapis.com/generated/video_123")
	if err != nil {
		t.Fatal(err)
	}
	original := &http.Request{URL: originalURL, Header: make(http.Header)}
	original.Header.Set(geminiAPIKeyHeader, "secret")
	original.Header.Set("Authorization", "Bearer provider-secret")
	redirect := &http.Request{URL: redirectURL, Header: original.Header.Clone()}
	if err = stripCredentialOnCrossOriginRedirect(redirect, []*http.Request{original}); err != nil {
		t.Fatalf("check redirect: %v", err)
	}
	if redirect.Header.Get(geminiAPIKeyHeader) != "" {
		t.Fatal("Gemini API key leaked to a different origin")
	}
	if redirect.Header.Get("Authorization") != "" {
		t.Fatal("provider authorization leaked to a different origin")
	}

	sameOriginRedirect := &http.Request{URL: originalURL, Header: original.Header.Clone()}
	if err = stripCredentialOnCrossOriginRedirect(sameOriginRedirect, []*http.Request{original}); err != nil {
		t.Fatalf("check same-origin redirect: %v", err)
	}
	if sameOriginRedirect.Header.Get(geminiAPIKeyHeader) != "secret" {
		t.Fatal("same-origin redirect unexpectedly removed Gemini API key")
	}
	if sameOriginRedirect.Header.Get("Authorization") != "Bearer provider-secret" {
		t.Fatal("same-origin redirect unexpectedly removed provider authorization")
	}

	canonicalURL, err := url.Parse("https://GENERATIVELANGUAGE.googleapis.com:443/v1beta/files/video_123:download")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRedirect := &http.Request{URL: canonicalURL, Header: original.Header.Clone()}
	if err = stripCredentialOnCrossOriginRedirect(canonicalRedirect, []*http.Request{original}); err != nil {
		t.Fatalf("check canonical same-origin redirect: %v", err)
	}
	if canonicalRedirect.Header.Get(geminiAPIKeyHeader) != "secret" {
		t.Fatal("canonical same-origin redirect unexpectedly removed Gemini API key")
	}
	if canonicalRedirect.Header.Get("Authorization") != "Bearer provider-secret" {
		t.Fatal("canonical same-origin redirect unexpectedly removed provider authorization")
	}
}

func TestRedirectPolicyStopsAfterLimit(t *testing.T) {
	redirectURL, err := url.Parse("https://cdn.example.test/generated/video")
	if err != nil {
		t.Fatal(err)
	}
	request := &http.Request{URL: redirectURL, Header: make(http.Header)}
	via := make([]*http.Request, maxRedirects)
	for index := range via {
		via[index] = &http.Request{URL: redirectURL}
	}
	if err = stripCredentialOnCrossOriginRedirect(request, via); err == nil {
		t.Fatalf("expected redirect limit after %d redirects", maxRedirects)
	}
}

func TestTrustedArtifactRedirectAllowsPublicButRejectsPrivateCrossOrigin(t *testing.T) {
	strictPolicy := security.NewStrictOutboundPolicy(true)
	trustedPolicy, err := strictPolicy.WithTrustedHTTPURLs("http://model.internal:8080/v1")
	if err != nil {
		t.Fatalf("trusted artifact policy: %v", err)
	}
	httpClient := newMediaArtifactHTTPClient(trustedPolicy, strictPolicy, "http://model.internal:8080").Client
	originalURL, err := url.Parse("http://model.internal:8080/artifacts/image.png")
	if err != nil {
		t.Fatal(err)
	}
	original := &http.Request{URL: originalURL, Header: make(http.Header)}

	publicURL, err := url.Parse("https://cdn.example.test/generated/image.png")
	if err != nil {
		t.Fatal(err)
	}
	publicRedirect := &http.Request{URL: publicURL, Header: make(http.Header)}
	if err = httpClient.CheckRedirect(publicRedirect, []*http.Request{original}); err != nil {
		t.Fatalf("public artifact redirect rejected: %v", err)
	}

	privateURL, err := url.Parse("http://model.internal:9090/generated/image.png")
	if err != nil {
		t.Fatal(err)
	}
	privateRedirect := &http.Request{URL: privateURL, Header: make(http.Header)}
	if err = httpClient.CheckRedirect(privateRedirect, []*http.Request{original}); !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected private cross-origin redirect rejection, got %v", err)
	}
}

func TestStrictClientRejectsPrivateArtifactURL(t *testing.T) {
	client := New(security.NewStrictOutboundPolicy(true))
	_, _, err := client.DownloadImage(t.Context(), "http://127.0.0.1:8080/image.png", "", 1024)
	if !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected strict SSRF rejection, got %v", err)
	}
}

func TestDownloadImageTrustsPrivateArtifactFromConfiguredProviderOrigin(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/artifacts/image.png" {
			t.Fatalf("unexpected artifact path: %s", request.URL.Path)
		}
		responseWriter.Header().Set("Content-Type", "image/png")
		_, _ = responseWriter.Write(pngHeader)
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	data, mimeType, err := client.DownloadImage(
		t.Context(),
		server.URL+"/artifacts/image.png",
		server.URL+"/v1",
		1024,
	)
	if err != nil {
		t.Fatalf("download trusted private artifact: %v", err)
	}
	if string(data) != string(pngHeader) || mimeType != "image/png" {
		t.Fatalf("unexpected trusted artifact: data=%q MIME=%q", data, mimeType)
	}
}

func TestDownloadImageDoesNotTrustCrossOriginPrivateArtifact(t *testing.T) {
	client := New(security.NewStrictOutboundPolicy(true))
	_, _, err := client.DownloadImage(
		t.Context(),
		"http://127.0.0.1:18081/artifacts/image.png",
		"http://127.0.0.1:18080/v1",
		1024,
	)
	if !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected cross-origin private artifact rejection, got %v", err)
	}
}

func TestDownloadRejectsURLUserInfoBeforeRequest(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("URL user info must be rejected before issuing a request")
		return nil, nil
	}))

	_, _, err := client.DownloadImage(t.Context(), "https://user:password@cdn.example.test/image", "", 1024)
	if !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected URL user info rejection, got %v", err)
	}
}

func TestCloseIdleConnectionsUsesOwnedTransport(t *testing.T) {
	closed := false
	pool := outboundhttp.NewPool(security.OutboundPolicy{}, 1, func(_ security.OutboundPolicy, _ string, _ string) (outboundhttp.ManagedClient, error) {
		return outboundhttp.ManagedClient{
			Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, "application/octet-stream", nil), nil
			})},
			CloseIdleConnections: func() { closed = true },
		}, nil
	})
	request, err := http.NewRequest(http.MethodGet, "https://example.test/artifact", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Do(request, "", ""); err != nil {
		t.Fatalf("use pooled client: %v", err)
	}
	client := &Client{httpClients: pool}
	client.CloseIdleConnections()
	if !closed {
		t.Fatal("expected idle connections to be closed")
	}
}

func testClient(transport http.RoundTripper) *Client {
	policy := security.OutboundPolicy{}
	return &Client{
		basePolicy: policy,
		httpClients: outboundhttp.NewPool(policy, outboundhttp.DefaultCacheLimit, func(_ security.OutboundPolicy, _ string, _ string) (outboundhttp.ManagedClient, error) {
			return outboundhttp.ManagedClient{Client: &http.Client{Transport: transport, CheckRedirect: mediaArtifactRedirectPolicy(policy, "")}}, nil
		}),
		pollAttempts: 1,
		pollInterval: 0,
	}
}

func response(statusCode int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
