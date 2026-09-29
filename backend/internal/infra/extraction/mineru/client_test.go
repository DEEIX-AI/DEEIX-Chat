package mineru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBatchCreateResponseParsesFileURLsAsStrings(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"msg": "ok",
		"data": {
			"batch_id": "batch-1",
			"file_urls": ["https://example.com/upload"]
		}
	}`)

	var parsed batchCreateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal batch create response: %v", err)
	}
	if parsed.Data.BatchID != "batch-1" {
		t.Fatalf("unexpected batch id %q", parsed.Data.BatchID)
	}
	if len(parsed.Data.FileURLs) != 1 || parsed.Data.FileURLs[0] != "https://example.com/upload" {
		t.Fatalf("unexpected file urls %#v", parsed.Data.FileURLs)
	}
}

func TestBatchResultResponseParsesExtractResultStateAndZipURL(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"msg": "ok",
		"data": {
			"state": "running",
			"extract_result": [
				{
					"state": "done",
					"full_zip_url": "https://example.com/result.zip"
				}
			]
		}
	}`)

	var parsed batchResultResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal batch result response: %v", err)
	}
	if len(parsed.Data.ExtractResult) != 1 {
		t.Fatalf("unexpected extract result %#v", parsed.Data.ExtractResult)
	}
	if parsed.Data.ExtractResult[0].State != "done" {
		t.Fatalf("unexpected item state %q", parsed.Data.ExtractResult[0].State)
	}
	if parsed.Data.ExtractResult[0].FullZipURL != "https://example.com/result.zip" {
		t.Fatalf("unexpected full zip url %q", parsed.Data.ExtractResult[0].FullZipURL)
	}
}

func TestCreateBatchClassifiesEndpointMismatchErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		assert     func(*testing.T, error)
	}{
		{
			name:       "not found status",
			statusCode: http.StatusNotFound,
			body:       "missing endpoint",
			assert: func(t *testing.T, err error) {
				var statusErr *httpError
				if !errors.As(err, &statusErr) || statusErr.statusCode != http.StatusNotFound {
					t.Fatalf("expected typed 404 error, got %T %v", err, err)
				}
			},
		},
		{
			name:       "invalid success response",
			statusCode: http.StatusOK,
			body:       "not json",
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, errMinerUInvalidResponse) {
					t.Fatalf("expected invalid response sentinel, got %T %v", err, err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{
				baseURL: "https://mineru.example/api/v4",
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: test.statusCode,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(test.body)),
					}, nil
				})},
			}
			_, _, err := client.createBatch(context.Background(), Request{FileName: "document.pdf"})
			if err == nil {
				t.Fatal("expected create batch error")
			}
			test.assert(t, err)
			if !shouldFallback(err) {
				t.Fatal("endpoint mismatch should fall back to the alternate API")
			}
		})
	}
}

func TestPollBatchUsesExtractResultState(t *testing.T) {
	var calls int
	client := &Client{
		baseURL: "https://mineru.example/api/v4",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Path != "/api/v4/extract-results/batch/batch-1" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
			"code": 0,
			"msg": "ok",
			"data": {
				"state": "running",
				"extract_result": [
					{
						"state": "done",
						"full_zip_url": "https://example.com/result.zip"
					}
				]
			}
		}`)),
			}, nil
		})},
	}

	zipURL, err := client.pollBatch(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("poll batch: %v", err)
	}
	if zipURL != "https://example.com/result.zip" {
		t.Fatalf("unexpected zip url %q", zipURL)
	}
	if calls != 1 {
		t.Fatalf("expected one poll call, got %d", calls)
	}
}

func TestShouldFallbackUsesTypedHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "not found", err: &httpError{statusCode: http.StatusNotFound}, want: true},
		{name: "wrapped not found", err: fmt.Errorf("create batch: %w", &httpError{statusCode: http.StatusNotFound}), want: true},
		{name: "other status", err: &httpError{statusCode: http.StatusBadGateway}, want: false},
		{name: "message only", err: errors.New("mineru_http_404"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldFallback(test.err); got != test.want {
				t.Fatalf("shouldFallback() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestShouldFallbackUsesInvalidResponseSentinel(t *testing.T) {
	if !shouldFallback(fmt.Errorf("poll batch: %w", errMinerUInvalidResponse)) {
		t.Fatal("wrapped invalid response should fall back to the alternate API")
	}
}

func TestHTTPErrorPreservesLegacyDiagnostic(t *testing.T) {
	if got := (&httpError{statusCode: http.StatusNotFound}).Error(); got != "mineru_http_404" {
		t.Fatalf("httpError without detail = %q", got)
	}
	if got := (&httpError{statusCode: http.StatusBadGateway, detail: "provider detail"}).Error(); got != "mineru_http_502: provider detail" {
		t.Fatalf("httpError with detail = %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
