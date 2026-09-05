package contentmoderation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	cmport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/contentmoderation"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := map[string]string{
		"https://api.openai.com":                "https://api.openai.com/v1/moderations",
		"https://api.openai.com/v1":             "https://api.openai.com/v1/moderations",
		"https://api.openai.com/v1/moderations": "https://api.openai.com/v1/moderations",
		"http://localhost:8080/proxy":           "http://localhost:8080/proxy/v1/moderations",
	}
	for input, expected := range tests {
		actual, err := normalizeBaseURL(input)
		if err != nil {
			t.Fatalf("normalize %q: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("normalize %q = %q, want %q", input, actual, expected)
		}
	}
}

func TestNormalizeBaseURLRejectsEmbeddedCredentials(t *testing.T) {
	if _, err := normalizeBaseURL("https://user:secret@moderation.example/v1"); !errors.Is(err, cmport.ErrInvalidBaseURL) {
		t.Fatalf("error = %v, want invalid base URL", err)
	}
}

func TestModerateTextUsesProviderWireContract(t *testing.T) {
	client := &Client{doRequest: func(request *http.Request, configuredEndpoint string) (*http.Response, error) {
		if configuredEndpoint != "https://moderation.example/v1" {
			t.Fatalf("configured endpoint = %q", configuredEndpoint)
		}
		if request.URL.String() != "https://moderation.example/v1/moderations" {
			t.Fatalf("request URL = %q", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization header not set")
		}
		var body moderationRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "omni-moderation-latest" {
			t.Fatalf("model = %q", body.Model)
		}
		payload := `{"id":"modr_1","model":"omni-moderation-latest","results":[{"flagged":false,"categories":{"hate":false},"category_scores":{"hate":0.01},"category_applied_input_types":{"hate":["text"]}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	}}

	response, err := client.ModerateText(context.Background(), domaincm.ProviderConfig{
		BaseURL: "https://moderation.example/v1",
		APIKey:  "secret",
		Model:   "omni-moderation-latest",
		Timeout: time.Second,
	}, "hello", []string{"hate"}, domaincm.ModalityText)
	if err != nil {
		t.Fatalf("moderate text: %v", err)
	}
	if response.Model != "omni-moderation-latest" || len(response.Results) != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHTTPErrorNeverIncludesProviderBody(t *testing.T) {
	errorValue := mapHTTPStatus(http.StatusInternalServerError)
	if !errors.Is(errorValue, cmport.ErrService) {
		t.Fatalf("unexpected error classification: %v", errorValue)
	}
	if strings.Contains(errorValue.Error(), "sensitive echoed prompt") {
		t.Fatalf("provider body leaked into error: %v", errorValue)
	}
}

func TestSplitTextChunksPreservesUTF8(t *testing.T) {
	var input strings.Builder
	for input.Len() < maxTextChunkBytes+100 {
		input.WriteString("你好世界")
	}
	chunks := splitTextChunks(input.String())
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len([]byte(chunk)) > maxTextChunkBytes {
			t.Fatalf("chunk exceeds limit: %d", len([]byte(chunk)))
		}
	}
}

func TestModerateTextContinuesAfterUnselectedHit(t *testing.T) {
	calls := 0
	client := &Client{doRequest: func(*http.Request, string) (*http.Response, error) {
		calls++
		categories := `{"hate":true,"violence":false}`
		if calls > 1 {
			categories = `{"hate":false,"violence":true}`
		}
		payload := `{"model":"omni-moderation-latest","results":[{"categories":` + categories + `,"category_scores":{"hate":0.9,"violence":0.9},"category_applied_input_types":{"hate":["text"],"violence":["text"]}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	}}
	input := strings.Repeat("abcdefghij", (maxTextChunkBytes/10)+20)
	response, err := client.ModerateText(context.Background(), domaincm.ProviderConfig{
		BaseURL: "https://moderation.example/v1",
		Timeout: time.Second,
	}, input, []string{"violence"}, domaincm.ModalityText)
	if err != nil {
		t.Fatalf("moderate text: %v", err)
	}
	if calls < 2 {
		t.Fatalf("calls = %d, want at least 2", calls)
	}
	if evaluation := domaincm.EvaluateHit(response, []string{"violence"}, domaincm.ModalityText); !evaluation.Hit {
		t.Fatalf("selected hit was lost: %#v", evaluation)
	}
}

func TestMergeResponsesPreservesEveryResult(t *testing.T) {
	base := &domaincm.ProviderResponse{Results: []domaincm.CategoryResult{{Categories: map[string]bool{"violence": false}}}}
	next := &domaincm.ProviderResponse{Results: []domaincm.CategoryResult{
		{Categories: map[string]bool{"violence": false}},
		{Categories: map[string]bool{"violence": true}},
	}}
	merged := mergeResponses(base, next)
	if len(merged.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(merged.Results))
	}
}
