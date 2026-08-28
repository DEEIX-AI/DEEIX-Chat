package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		if body.Dimensions != 3 {
			t.Fatalf("request dimensions = %d, want 3", body.Dimensions)
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	result, err := client.CallAPI(context.Background(), server.URL+"/v1", "", "test", []string{"hello"}, 3, 5)
	if err != nil {
		t.Fatalf("call configured private embedding endpoint: %v", err)
	}
	if len(result) != 1 || len(result[0]) != 3 {
		t.Fatalf("unexpected embedding result: %#v", result)
	}
}

func TestCallAPIRejectsUnexpectedEmbeddingDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), server.URL+"/v1", "", "test", []string{"hello"}, 4, 5)
	if err == nil || !strings.Contains(err.Error(), "has 3 dimensions, expected 4") {
		t.Fatalf("expected explicit dimension mismatch, got %v", err)
	}
}

func TestCallAPIAcceptsDimensionChangesInBothDirections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body requestPayload
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"index": 0, "embedding": make([]float32, body.Dimensions)}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	for _, dimensions := range []int{4096, 1536, 4096} {
		result, err := client.CallAPI(context.Background(), server.URL+"/v1", "", "test", []string{"hello"}, dimensions, 5)
		if err != nil {
			t.Fatalf("CallAPI(%d) error = %v", dimensions, err)
		}
		if len(result) != 1 || len(result[0]) != dimensions {
			t.Fatalf("CallAPI(%d) returned dimensions %d", dimensions, len(result[0]))
		}
	}
}
