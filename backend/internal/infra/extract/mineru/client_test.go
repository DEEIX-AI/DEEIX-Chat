package mineru

import (
	"context"
	"encoding/json"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
