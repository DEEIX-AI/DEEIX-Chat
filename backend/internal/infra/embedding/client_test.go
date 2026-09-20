package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestCallAPITrustsConfiguredPrivateEmbeddingOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected embedding path: %s", request.URL.Path)
		}
		var body requestPayload
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Dimensions == nil || *body.Dimensions != 3 {
			t.Fatalf("request dimensions = %v, want 3", body.Dimensions)
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	result, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Texts: []string{"hello"}, Dimensions: 3, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("call configured private embedding endpoint: %v", err)
	}
	if len(result) != 1 || len(result[0]) != 3 {
		t.Fatalf("unexpected embedding result: %#v", result)
	}
}

func TestCallAPIRejectsUnexpectedEmbeddingDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Texts: []string{"hello"}, Dimensions: 4, TimeoutSeconds: 5})
	if err == nil || !strings.Contains(err.Error(), "has 3 dimensions, expected 4") {
		t.Fatalf("expected explicit dimension mismatch, got %v", err)
	}
}

func TestCallAPIOmitsDimensionsWithoutRelaxingResponseValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := body["dimensions"]; ok {
			t.Fatal("request unexpectedly included dimensions")
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), portembedding.Request{
		APIBase:        server.URL + "/v1",
		Model:          "test",
		Texts:          []string{"hello"},
		Dimensions:     4,
		OmitDimensions: true,
		TimeoutSeconds: 5,
	})
	if err == nil || !strings.Contains(err.Error(), "has 3 dimensions, expected 4") {
		t.Fatalf("expected response dimension validation, got %v", err)
	}
}

func TestCallAPIAcceptsDimensionChangesInBothDirections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body requestPayload
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Dimensions == nil {
			t.Fatal("request dimensions unexpectedly omitted")
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": make([]float32, *body.Dimensions)}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	for _, dimensions := range []int{4096, 1536, 4096} {
		result, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Texts: []string{"hello"}, Dimensions: dimensions, TimeoutSeconds: 5})
		if err != nil {
			t.Fatalf("CallAPI(%d) error = %v", dimensions, err)
		}
		if len(result) != 1 || len(result[0]) != dimensions {
			t.Fatalf("CallAPI(%d) returned dimensions %d", dimensions, len(result[0]))
		}
	}
}
