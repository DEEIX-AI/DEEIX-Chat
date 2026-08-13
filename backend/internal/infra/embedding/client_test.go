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
		_ = json.NewEncoder(responseWriter).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	result, err := client.CallAPI(context.Background(), server.URL+"/v1", "", "test", []string{"hello"}, 5)
	if err != nil {
		t.Fatalf("call configured private embedding endpoint: %v", err)
	}
	if len(result) != 1 || len(result[0]) != 3 {
		t.Fatalf("unexpected embedding result: %#v", result)
	}
}

func TestChunkTextHandlesCJKParagraphBoundary(t *testing.T) {
	text := strings.Repeat("中文内容", 900) + "\n\n" + strings.Repeat("后续内容", 900)

	chunks := ChunkText(text, 1024, 64)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for index, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunk %d is empty", index)
		}
	}
}

func TestChunkTextClampsInvalidOverlap(t *testing.T) {
	text := strings.Repeat("abcdef", 600)

	chunks := ChunkText(text, 128, 512)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	if len(chunks) > 20 {
		t.Fatalf("expected overlap clamp to avoid excessive chunks, got %d", len(chunks))
	}
}

func TestChunkTextHandlesMixedMultibyteContent(t *testing.T) {
	text := strings.Repeat("hello 世界 🚀\n", 1200)

	chunks := ChunkText(text, 128, 16)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	for index, chunk := range chunks {
		if strings.ContainsRune(chunk, '\uFFFD') {
			t.Fatalf("chunk %d contains replacement rune", index)
		}
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunk %d is empty", index)
		}
	}
}

func TestChunkTextTinyChunkStillProgresses(t *testing.T) {
	text := strings.Repeat("中", 50)

	chunks := ChunkText(text, 1, 100)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	if len(chunks) > len([]rune(text)) {
		t.Fatalf("expected bounded chunks, got %d", len(chunks))
	}
}
